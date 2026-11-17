//go:build tvos

package app

import (
	"context"
	"log"
	"os"

	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/sys"
	tvenv "dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/shell"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/host"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// The window asked for on the Apple TV: tvOS gives the app 1920×1080
// points whatever the display (gDesk framework/ui/README.md, "TV:
// directional focus and the 1920×1080 frame"), the prototype's TV size.
const (
	windowW = 1920
	windowH = 1080
)

// tvBoot puts the first screen up once the session is read (K12: --bg
// until then), and TV01 back with the expired toast when the server ends
// the session (Q13's string). UI thread.
type tvBoot struct {
	w     *tvWire
	sh    *shell.Shell
	texts map[string]string
	// restored is set once the first screen is up; a shot state's steps
	// wait for it.
	restored bool
	signed   bool
	stop     func()
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// newAppRoot returns the root of the Apple TV: the TV shell over the app's
// services (services_tv.go) on h. A developer's build launched with
// DFV_STATE draws that state and writes its layout (states_tv.go).
func newAppRoot(h *host.App, texts map[string]string) (appRoot, error) {
	return newTVRoot(tvHostDeps(h), texts, devEnv(stateEnv)), nil
}

// newTVRoot returns the shell over services made of deps; stateID, when not
// empty, names the state to draw (a state the app does not know is logged,
// and the app starts as it ships).
func newTVRoot(deps tvDeps, texts map[string]string, stateID string) appRoot {
	var st tvShotState
	shot := false
	if devParts && stateID != "" { // the shipped live build has no states
		var err error
		st, err = lookupTVState(stateID)
		if err != nil {
			log.Printf("dancefitvn: %v", err)
		}
		shot = err == nil
	}
	knobs := tvKnobsOf(st)
	knobs.memOverlay = shot
	if devEnv(offlineEnv) == "1" {
		knobs.offline = true
	}
	w := newTVWire(deps, knobs)
	sh := shell.New(w, texts)
	b := &tvBoot{w: w, sh: sh, texts: texts}
	var flog *frameLog // nil without DFV_FRAMELOG=1
	mem := newTVMemory(sh)
	var flogFrame, mark func()
	if devEnv(frameLogEnv) == "1" { // the developer's frame log
		flog = newFrameLog(os.Stderr, nil, 0)
		flogFrame = flog.onFrame
	}
	if devBuild { // the mark of the first TV04 frame (devperf.go)
		mark = newHomeMark(func() bool { return b.homeShown() }, w.deps.Logf).after
	}
	afterFrame := chainFrames(mem.after, mark, flogFrame)

	return appRoot{
		widget:     sh,
		afterFrame: afterFrame,
		close: func() {
			flog.close()
			b.close()
			w.close()
		},
		mounted: func(v *ui.View) {
			flog.attach(v)
			w.attach(v)
			b.start(v.Context())
			if devParts && shot {
				d := newDevShot(stateID, "tv", v, w.shotSource(), settleTime())
				if deps.ShotWall != nil {
					d.wall = deps.ShotWall
				}
				newTVStateRunner(stateID, st, b, d.arm).start(v.Context())
			}
		},
	}
}

// viewOptions returns the options of the TV's view, before the GPU's: the
// theme, directional focus, and a transparent background, so that the
// native video plane shows under the view (D4); the shell paints --bg
// itself around the plane.
func viewOptions() []ui.ViewOption {
	return []ui.ViewOption{ui.WithTheme(newTheme()), ui.WithDirectionalFocus(), ui.WithTransparentBackground()}
}

// mounted sets up the TV's view once mounted: nothing to do, tvOS having
// no orientation and no status bar.
func mounted(*ui.View) {}

// start watches the sign-outs and reads the session off the UI thread at
// the next turn (a view being mounted has no host yet, and ui.Go would
// run the work on the UI thread then).
func (b *tvBoot) start(c *ui.Context) {
	b.stop = b.w.OnSignedOut(b.signedOut)
	c.AfterFunc(0, func() {
		ui.Go(b.sh, func(ctx context.Context) (bool, error) {
			return b.w.Restore(ctx)
		}, func(_ *ui.Context, signed bool, err error) {
			if err != nil {
				b.w.deps.Logf("dancefitvn: restore: %v", err)
			}
			b.restored, b.signed = true, signed && err == nil
			first := tvenv.RouteWelcome
			if b.signed {
				first = tvenv.RouteHome
			}
			b.w.deps.Logf("dancefitvn: first screen %s", first)
			b.sh.Go(nav.Route{Screen: first}, true)
		})
	})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// signedOut is the server ending the session (the refresh refused): TV01
// as the root, and the toast of the expired session.
func (b *tvBoot) signedOut(error) {
	b.signed = false
	b.sh.Go(nav.Route{Screen: tvenv.RouteWelcome}, true)
	b.sh.Toast(b.texts["session.expired"])
}

// homeShown tells whether TV04 is on show with its data (tvHomeShows):
// the screen of the mark of the first Home frame (devperf.go).
func (b *tvBoot) homeShown() bool {
	if !b.restored || b.sh.Stack().Screen() != tvenv.RouteHome {
		return false
	}
	for _, k := range tvHomeShows {
		if tvFindKey(b.sh, k) == nil {
			return false
		}
	}

	return true
}

// close ends the watch.
func (b *tvBoot) close() {
	if b.stop != nil {
		b.stop()
		b.stop = nil
	}
}

// shotSource is what a developer's shot reads of the TV's services.
func (w *tvWire) shotSource() shotSource {
	return shotSource{idle: func() (bool, int) {
		if w.tr == nil {
			return true, 0
		}

		return w.tr.idle()
	}, cacheDir: w.deps.CacheDir, logf: w.deps.Logf}
}

// tvHostDeps returns the system's parts of the TV's services on h: its UI
// thread, its driver's secrets (F1), the app's folders, the Mirror of
// NSUserDefaults and files for the state (tvBackend), the keep-awake of
// internal/sys, the system's Now Playing, the lifecycle, the network, the
// device's name and the mock API main was built with. The engine is
// avftv's (NewMedia nil).
func tvHostDeps(h *host.App) tvDeps {
	return tvDeps{
		Mock:    options.Mock,
		Enqueue: h.Enqueue,
		Secrets: func() (auth.SecretStore, error) {
			s, err := sys.OpenSecrets(h.Driver(), h.Invoke)
			if err != nil {
				return nil, err
			}

			return s, nil
		},
		InstallID: sys.InstallID,
		DataDir:   sys.DataDir,
		CacheDir:  sys.CacheDir,
		State: func(dir string, logf func(string, ...any)) store.Backend {
			return tvBackend(dir, logf)
		},
		Awake:      devAwake(sysAwake{driver: h.Driver}, log.Printf),
		NowPlaying: driverNowPlaying{driver: h.Driver},
		OnLifecycle: func(fn func(platform.LifecycleState)) func() {
			return h.OnLifecycle(fn)
		},
		Network: func(dispatch func(func())) netWatch {
			return watchNetwork(dispatch, log.Printf, devNetworkFile(), devEnv(networkEnv))
		},
		DeviceName: sys.DeviceName,
	}
}
