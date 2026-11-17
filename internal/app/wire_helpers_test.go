//go:build !tvos

package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// urlCall is a call of the fake OpenURL.
type urlCall struct {
	canvas platform.Canvas
	raw    string
}

// rig is the phone's services over the repository's mock, in temporary
// folders, with a fake biometry, clock and URL opener, and the shell
// mounted on a Tester when mount is called.
type rig struct {
	t     *testing.T
	q     *uiQueue
	log   *testLog
	clk   *clock.Fake
	bio   *sys.FakeBiometric
	deps  mobileDeps
	w     *mobileWire
	root  appRoot
	tt    *ui.Tester
	sh    *shell.Shell
	urlMu sync.Mutex
	urls  []urlCall
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newRig returns services over fresh folders and an empty vault; secrets
// may be shared with another rig (a second launch on the same phone). Its
// API is the mock in the process: the test is skipped in a -tags live
// build, which has none (newRigOf).
func newRig(t *testing.T, secrets *memSecrets, dir string) *rig {
	t.Helper()
	skipLiveBuild(t)

	return newRigOf(t, secrets, dir)
}

// newRigOf is newRig in any build: the caller sets the API (deps.Mock,
// deps.API).
func newRigOf(t *testing.T, secrets *memSecrets, dir string) *rig {
	t.Helper()
	if secrets == nil {
		secrets = &memSecrets{vals: map[string][]byte{}}
	}
	if dir == "" {
		dir = t.TempDir()
	}
	r := &rig{t: t, q: &uiQueue{}, log: &testLog{}, clk: clock.NewFake(demoNow), bio: sys.NewFakeBiometric(sys.BiometryNone)}
	r.deps = mobileDeps{
		Mock:      os.DirFS(mockDir),
		Enqueue:   r.q.enqueue,
		Secrets:   func() (auth.SecretStore, error) { return secrets, nil },
		InstallID: func() (string, error) { return "test-install", nil },
		DataDir:   func() (string, error) { return filepath.Join(dir, "data"), nil },
		CacheDir:  func() (string, error) { return filepath.Join(dir, "cache"), nil },
		Bio:       r.bio,
		OpenURL: func(c platform.Canvas, d sys.Dispatch, raw string, done func(error)) error {
			r.urlMu.Lock()
			r.urls = append(r.urls, urlCall{canvas: c, raw: raw})
			r.urlMu.Unlock()
			d(func() { done(nil) })

			return nil
		},
		Platform: "ios",
		MockWait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		Clock:    r.clk,
		Logf:     r.log.add,
	}

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// wire makes the services with knobs (and closes them with the test).
func (r *rig) wire(knobs wireKnobs) *mobileWire {
	r.w = newMobileWire(r.deps, knobs)
	r.t.Cleanup(r.w.close)

	return r.w
}

// mount makes the root of state id ("" for none) and mounts it on the
// phone, with the real text engine when real; the services are r.w.
func (r *rig) mount(ph kittest.Phone, id string, real bool) {
	r.t.Helper()
	r.root = newMobileRoot(r.deps, locales.MustTexts(), id)
	var ok bool
	r.sh, ok = r.root.widget.(*shell.Shell)
	if !ok {
		r.t.Fatalf("the root is %T, not the shell", r.root.widget)
	}
	if real {
		r.tt = kittest.MountReal(r.t, ph, r.root.widget)
	} else {
		r.tt = kittest.Mount(r.t, ph, r.root.widget)
	}
	kittest.PinMotion(r.tt)
	r.w = r.sh.Env().Services.(*mobileWire) //nolint:forcetypeassert // newMobileRoot's
	r.t.Cleanup(r.w.close)
	r.root.mounted(r.tt.View())
}

// drive runs frames and the queued UI work until cond holds; the test
// fails when it does not within driveLimit.
func (r *rig) drive(what string, cond func() bool) {
	r.t.Helper()
	end := time.Now().Add(driveLimit)
	for time.Now().Before(end) {
		r.q.drain()
		if cond() {
			return
		}
		if r.tt != nil {
			r.tt.Elapse(kittest.Frame)
		}
		runtime.Gosched()
	}
	r.t.Fatalf("%s did not happen in %v; log: %v", what, driveLimit, r.log.all())
}

// settle runs the queued UI work until cond holds without moving the
// clock of the Tester: for work off the UI thread whose result does not
// depend on time. drive would let a playing sound run to its end while
// the test waits for the machine, so the wait depends on the load.
func (r *rig) settle(what string, cond func() bool) {
	r.t.Helper()
	end := time.Now().Add(driveLimit)
	for time.Now().Before(end) {
		r.q.drain()
		if cond() {
			return
		}
		runtime.Gosched()
	}
	r.t.Fatalf("%s did not happen in %v; log: %v", what, driveLimit, r.log.all())
}

// opened returns the URLs OpenURL was given.
func (r *rig) opened() []urlCall {
	r.urlMu.Lock()
	defer r.urlMu.Unlock()

	return append([]urlCall(nil), r.urls...)
}
