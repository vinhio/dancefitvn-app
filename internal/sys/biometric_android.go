//go:build android

package sys

import (
	"fmt"
	"sync"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The app's Java (android/java, compiled by gdesk build with gDesk's):
// the bridge to BiometricPrompt and the interface its answer comes
// through. FindClass finds them through the app's class loader.
const (
	bridgeClassName  = "com/jivecode/dancefitvn/BiometricBridge"
	bridgeListenerIf = "com/jivecode/dancefitvn/BiometricBridge$Listener"
)

// bridge holds BiometricBridge's class (a global reference) and the
// methods called, looked up once.
var bridge struct {
	mu sync.Mutex
	ok bool

	class                                jni.Class
	availability, features, authenticate jni.MethodID
	cancel                               jni.MethodID // CancellationSignal.cancel
}

// promptSheet is one sheet showing: its listener (a jni.Proxy), its
// CancellationSignal (a global reference) and whether the answer came.
// The main thread (show, the listener) and the UI thread (cancel) share
// it under mu.
type promptSheet struct {
	d    Dispatch
	done func(error)

	mu       sync.Mutex
	finished bool
	canceled bool
	proxy    *jni.Proxy
	signal   jni.Object
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// biometry asks BiometricBridge.availability and .features through the
// application's context; any goroutine (jni.Do attaches it).
func biometry() (BiometryKind, error) {
	if !jni.HasJava() {
		return BiometryNone, fmt.Errorf("sys: biometric: no Java of gDesk's: %w", core.ErrUnsupported)
	}
	ctx := jni.Context()
	if ctx == 0 {
		return BiometryNone, fmt.Errorf("sys: biometric: no context: %w", core.ErrUnsupported)
	}
	var avail, feats int32
	err := jni.Do(func(env jni.Env) error {
		if err := lookupBridge(env); err != nil {
			return err
		}
		var err error
		if avail, err = env.CallStaticInt(bridge.class, bridge.availability, jni.Ref(ctx)); err != nil {
			return err
		}
		feats, err = env.CallStaticInt(bridge.class, bridge.features, jni.Ref(ctx))

		return err
	})
	if err != nil {
		return BiometryNone, fmt.Errorf("sys: biometric: %w", err)
	}

	return androidKind(avail, feats), nil
}

// authenticate shows BiometricPrompt from Java's main thread over the
// activity; its answer comes back there through the listener and goes
// to the UI thread through d.
func authenticate(d Dispatch, p BioPrompt, done func(error)) (func(), error) {
	if !jni.HasJava() {
		return nil, fmt.Errorf("sys: biometric: no Java of gDesk's: %w", core.ErrUnsupported)
	}
	s := &promptSheet{d: d, done: done}
	title, desc := p.Reason, ""
	if p.Title != "" {
		title, desc = p.Title, p.Reason
	}
	err := jni.PostMain(func(env jni.Env) {
		if err := s.show(env, title, desc, p.Cancel); err != nil {
			s.finish(fmt.Errorf("sys: biometric: %w", err))
		}
	})
	if err != nil {
		return nil, fmt.Errorf("sys: biometric: %w", err)
	}

	return s.cancel, nil
}

// show makes the listener and calls BiometricBridge.authenticate, on
// Java's main thread. The strings are local references of PostMain's
// frame.
func (s *promptSheet) show(env jni.Env, title, desc, negative string) error {
	if err := lookupBridge(env); err != nil {
		return err
	}
	act := jni.Activity()
	if act == 0 {
		return fmt.Errorf("no activity to show the sheet over: %w", ErrBioUnavailable)
	}
	proxy, err := jni.NewProxy(env, []string{bridgeListenerIf}, []string{"onResult"},
		func(_ jni.Env, _ int, args jni.Args) jni.Value {
			s.finish(bridgeResult(args.Int(0), args.Int(1), args.String(2)))

			return 0
		})
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.finished || s.canceled {
		s.mu.Unlock()
		_ = proxy.Close()
		s.finish(ErrBioCanceled)

		return nil
	}
	s.proxy = proxy
	s.mu.Unlock()
	strs := [3]jni.Object{}
	for i, v := range [3]string{title, desc, negative} {
		if strs[i], err = env.NewString(v); err != nil {
			return err
		}
	}
	sig, err := env.CallStaticObject(bridge.class, bridge.authenticate,
		jni.Ref(act), jni.Ref(strs[0]), jni.Ref(strs[1]), jni.Ref(strs[2]), jni.Ref(proxy.Object()))
	if err != nil {
		return err
	}
	if sig != 0 {
		s.mu.Lock()
		if !s.finished {
			s.signal = env.NewGlobalRef(sig)
		}
		s.mu.Unlock()
		env.DeleteLocalRef(sig)
	}

	return nil
}

// cancel dismisses the sheet through its CancellationSignal, on Java's
// main thread; the answer is then BIOMETRIC_ERROR_CANCELED. After the
// answer it does nothing. UI thread.
func (s *promptSheet) cancel() {
	s.mu.Lock()
	if s.finished || s.canceled {
		s.mu.Unlock()

		return
	}
	s.canceled = true
	s.mu.Unlock()
	_ = jni.PostMain(func(env jni.Env) {
		s.mu.Lock()
		sig := s.signal
		if s.finished {
			sig = 0
		}
		s.mu.Unlock()
		if sig != 0 {
			_ = env.CallVoid(sig, bridge.cancel)
		}
	})
}

// finish takes the one answer: the listener and the signal are freed,
// and done gets err on the UI thread. A later answer is dropped.
func (s *promptSheet) finish(err error) {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()

		return
	}
	s.finished = true
	proxy, sig := s.proxy, s.signal
	s.proxy, s.signal = nil, 0
	s.mu.Unlock()
	_ = proxy.Close()
	if sig != 0 {
		_ = jni.Do(func(env jni.Env) error {
			env.DeleteGlobalRef(sig)

			return nil
		})
	}
	s.d(func() { s.done(err) })
}

// lookupBridge finds BiometricBridge and CancellationSignal.cancel, once.
func lookupBridge(env jni.Env) error {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.ok {
		return nil
	}
	c, err := env.FindClass(bridgeClassName)
	if err != nil {
		return err
	}
	defer env.DeleteLocalRef(jni.Object(c))
	sigClass, err := env.FindClass("android/os/CancellationSignal")
	if err != nil {
		return err
	}
	defer env.DeleteLocalRef(jni.Object(sigClass))
	for _, m := range []struct {
		dst       *jni.MethodID
		c         jni.Class
		static    bool
		name, sig string
	}{
		{&bridge.availability, c, true, "availability", "(Landroid/content/Context;)I"},
		{&bridge.features, c, true, "features", "(Landroid/content/Context;)I"},
		{&bridge.authenticate, c, true, "authenticate",
			"(Landroid/content/Context;Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;L" + bridgeListenerIf + ";)Landroid/os/CancellationSignal;"},
		{&bridge.cancel, sigClass, false, "cancel", "()V"},
	} {
		if m.static {
			*m.dst, err = env.StaticMethod(m.c, m.name, m.sig)
		} else {
			*m.dst, err = env.Method(m.c, m.name, m.sig)
		}
		if err != nil {
			return err
		}
	}
	bridge.class = jni.Class(env.NewGlobalRef(jni.Object(c)))
	bridge.ok = true

	return nil
}
