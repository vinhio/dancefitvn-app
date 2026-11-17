//go:build tvos

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
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/ui/tv/shell"
	"dancefitvn-app/internal/ui/tv/tvtest"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tvRig is the TV's services over the repository's mock, in temporary
// folders, with a fake clock, a fake engine (220 s media), a recording
// keep-awake and lifecycle, and the shell mounted on a 1920×1080 Tester.
type tvRig struct {
	t     *testing.T
	q     *uiQueue
	log   *testLog
	clk   *clock.Fake
	media *clock.Fake
	deps  tvDeps
	root  appRoot
	tt    *ui.Tester
	sh    *shell.Shell
	w     *tvWire
	awake *recAwake
	life  []func(platform.LifecycleState)
	state store.Backend
}

// recAwake is an awakeSink that records what it was told.
type recAwake struct {
	mu sync.Mutex
	on bool
	n  int
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newTVRig returns the TV's services over fresh folders (or dir, a second
// launch on the same TV) and the vault secrets (nil: empty), over the mock
// in the process: the test is skipped in a -tags live build.
func newTVRig(t *testing.T, secrets *memSecrets, dir string) *tvRig {
	t.Helper()
	skipLiveBuild(t)

	return newTVRigOf(t, secrets, dir)
}

// newTVRigOf is newTVRig in any build: the caller sets the API (deps.Mock,
// deps.API).
func newTVRigOf(t *testing.T, secrets *memSecrets, dir string) *tvRig {
	t.Helper()
	if secrets == nil {
		secrets = &memSecrets{vals: map[string][]byte{}}
	}
	if dir == "" {
		dir = t.TempDir()
	}
	r := &tvRig{t: t, q: &uiQueue{}, log: &testLog{}, clk: clock.NewFake(tvDemoNow), media: clock.NewFake(tvDemoNow), awake: &recAwake{}}
	r.deps = tvDeps{
		Mock:      os.DirFS(mockDir),
		API:       mockOnly,
		Enqueue:   r.q.enqueue,
		Secrets:   func() (auth.SecretStore, error) { return secrets, nil },
		InstallID: func() (string, error) { return "test-tv-install", nil },
		DataDir:   func() (string, error) { return filepath.Join(dir, "data"), nil },
		CacheDir:  func() (string, error) { return filepath.Join(dir, "cache"), nil },
		State: func(d string, logf func(string, ...any)) store.Backend {
			r.state = newTVBackend(store.NewDir(filepath.Join(d, "defaults")), nil, d, logf)

			return r.state
		},
		MockWait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		Clock:    r.clk,
		Logf:     r.log.add,
		NewMedia: func(cfg media.Config) (platformMedia, error) {
			cfg.Clock = r.media

			return fakeMediaOf(fake.Options{VideoDuration: tvDemoSeconds, AudioDuration: tvDemoSeconds})(cfg)
		},
		Awake: r.awake,
		OnLifecycle: func(fn func(platform.LifecycleState)) func() {
			r.life = append(r.life, fn)

			return func() {}
		},
		DeviceName: func() string { return "Apple TV phòng khách" },
	}

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// mount makes the root of state id ("" for the app as it ships) and mounts
// it at 1920×1080 with the fake text engine.
func (r *tvRig) mount(id string) { r.t.Helper(); r.mountWith(id, false) }

// mountWith is mount, with Core Text and the app's fonts when real.
func (r *tvRig) mountWith(id string, real bool) {
	r.t.Helper()
	r.root = newTVRoot(r.deps, locales.MustTexts(), id)
	sh, ok := r.root.widget.(*shell.Shell)
	require.True(r.t, ok, "the root is %T, not the TV shell", r.root.widget)
	r.sh = sh
	if real {
		r.tt = tvtest.MountReal(r.t, r.root.widget)
	} else {
		r.tt = tvtest.Mount(r.t, r.root.widget)
	}
	tvtest.PinMotion(r.tt)
	r.w = sh.Env().Services.(*tvWire) //nolint:forcetypeassert // newTVRoot's
	r.t.Cleanup(r.root.close)
	r.root.mounted(r.tt.View())
}

// drive runs frames and the queued UI work until cond holds; the test fails
// when it does not within driveLimit.
func (r *tvRig) drive(what string, cond func() bool) {
	r.t.Helper()
	end := time.Now().Add(driveLimit)
	for time.Now().Before(end) {
		r.q.drain()
		if cond() {
			return
		}
		if r.tt != nil {
			r.tt.Elapse(tvtest.Frame)
		}
		runtime.Gosched()
	}
	r.t.Fatalf("%s did not happen in %v; log: %v", what, driveLimit, r.log.all())
}

// route is the route showing.
func (r *tvRig) route() string { return r.sh.Stack().Screen() }

// lifecycle tells the wire the app's standing, as the host does.
func (r *tvRig) lifecycle(s platform.LifecycleState) {
	for _, fn := range r.life {
		fn(s)
	}
}

// signedIn waits until the boot read the session and the home shows.
func (r *tvRig) home() {
	r.drive("TV04", func() bool { return r.route() == "home" && r.sh.Screen() != nil })
}

// adoptMock keeps the mock's confirmed TV session in the vault, as a
// pairing does, off the UI thread.
func (r *tvRig) adoptMock(ctx context.Context) {
	r.t.Helper()
	raw, err := os.ReadFile(filepath.Join(mockDir, "tv", "pair-poll.confirmed.json"))
	require.NoError(r.t, err)
	var p model.PairPoll
	require.NoError(r.t, jsonUnmarshal(raw, &p))
	require.NoError(r.t, r.w.setup(ctx))
	require.NoError(r.t, r.w.adopt(p.Session()))
}

// Set records on.
func (a *recAwake) Set(on bool) error {
	a.mu.Lock()
	a.on = on
	a.n++
	a.mu.Unlock()

	return nil
}

// Reapply does nothing.
func (a *recAwake) Reapply() error { return nil }

// On is what it was told last.
func (a *recAwake) On() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.on
}
