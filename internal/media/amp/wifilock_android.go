// Adapted from jplayer-mobile/internal/media/amp/wifilock_android.go at 49de3c2.

//go:build android && cgo

package amp

import (
	"fmt"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// wifiTag is the name of the engine's Wi-Fi lock, as dumpsys wifi shows
// it.
const wifiTag = "jPlayer"

// wifiLock keeps the Wi-Fi up while a link plays (D7 of M9): with the
// screen off the system lets the Wi-Fi sleep otherwise, and the stream
// stalls; MediaPlayer's wake mode keeps only the CPU. It is
// WifiManager.WifiLock of WIFI_MODE_FULL_HIGH_PERF, not reference
// counted, so that one release undoes any acquires. Without the
// WAKE_LOCK permission, which acquire takes, or a WifiManager, it does
// nothing. UI thread.
type wifiLock struct {
	lock jni.Object // global, 0 when there is none
	held bool       // what the engine last asked of it
	want bool       // the next state, for applyDo
	// applyDo is apply, made once: a closure handed to jni.Do would be
	// allocated at every call.
	applyDo func(jni.Env) error
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newWifiLock returns the engine's lock, made on env when may (the app
// holds WAKE_LOCK); otherwise, or when the system has no WifiManager, a
// lock that does nothing.
func newWifiLock(env jni.Env, may bool) (*wifiLock, error) {
	w := &wifiLock{}
	w.applyDo = w.apply
	if !may {
		return w, nil
	}
	ctx := jni.Context()
	if ctx == 0 {
		return w, nil
	}
	name, err := env.NewString("wifi") // Context.WIFI_SERVICE
	if err != nil {
		return nil, err
	}
	wm, err := env.CallObject(ctx, ids.getSystemService, jni.Ref(name))
	env.DeleteLocalRef(name)
	if err != nil {
		return nil, err
	}
	if wm == 0 {
		logf("amp: no WifiManager: links play without a Wi-Fi lock")

		return w, nil
	}
	defer env.DeleteLocalRef(wm)
	tag, err := env.NewString(wifiTag)
	if err != nil {
		return nil, err
	}
	lock, err := env.CallObject(wm, ids.createWifiLock, jni.Int(wifiHighPerf), jni.Ref(tag))
	env.DeleteLocalRef(tag)
	if err != nil {
		return nil, err
	}
	if lock == 0 {
		return nil, fmt.Errorf("amp: createWifiLock: %w", core.ErrNotFound)
	}
	defer env.DeleteLocalRef(lock)
	if err := env.CallVoid(lock, ids.wifiCounted, jni.Bool(false)); err != nil {
		return nil, err
	}
	w.lock = newGlobal(env, lock)

	return w, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// set acquires the lock (on) or releases it, when that changes. It
// allocates nothing. UI thread.
func (w *wifiLock) set(on bool) {
	if w.lock == 0 || w.held == on {
		return
	}
	w.want = on
	if err := jni.Do(w.applyDo); err != nil {
		logf("amp: wifi lock %t: %v", on, err)

		return
	}
	w.held = on
}

// isHeld asks the system whether the lock is held (Stats). UI thread.
func (w *wifiLock) isHeld() bool {
	if w.lock == 0 {
		return false
	}
	held := false
	_ = jni.Do(func(env jni.Env) error {
		var err error
		held, err = env.CallBool(w.lock, ids.wifiHeld)

		return err
	})

	return held
}

// close releases the lock and deletes it, on env.
func (w *wifiLock) close(env jni.Env) {
	if w.lock == 0 {
		return
	}
	if w.held {
		_ = env.CallVoid(w.lock, ids.wifiRelease)
		w.held = false
	}
	deleteGlobal(env, w.lock)
	w.lock = 0
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// apply is set's call into Java.
func (w *wifiLock) apply(env jni.Env) error {
	if w.want {
		return env.CallVoid(w.lock, ids.wifiAcquire)
	}

	return env.CallVoid(w.lock, ids.wifiRelease)
}
