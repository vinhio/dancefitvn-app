//go:build !tvos

package app

import (
	"log"
	"os"
	"runtime"

	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/host"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// The window asked for on a phone: the prototype's phone (PLAN D2). The
// driver gives the window the screen whatever the size asked.
const (
	windowW = mobileW
	windowH = mobileH
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// demoRoot, when a build sets it (root_kitdemo.go, -tags kitdemo), is the root
// in place of the shell's: the kit's gallery.
var demoRoot func(texts map[string]string) ui.Widget

// newAppRoot returns the root of the phone: the shell over the app's
// services (services_mobile.go) on h. A developer's build launched with
// DFV_STATE draws that state and writes its layout (states_mobile.go,
// devshot_mobile.go). A kitdemo build shows the kit's gallery instead.
func newAppRoot(h *host.App, texts map[string]string) (appRoot, error) {
	if demoRoot != nil {
		return appRoot{widget: demoRoot(texts)}, nil
	}

	return newMobileRoot(hostDeps(h), texts, devEnv(stateEnv)), nil
}

// newMobileRoot returns the shell over services made of deps; stateID,
// when not empty, names the state to draw (a state the app does not know
// is logged, and the app starts as it ships).
func newMobileRoot(deps mobileDeps, texts map[string]string, stateID string) appRoot {
	var st shotState
	shot := false
	if devParts && stateID != "" { // the shipped live build has no states
		var err error
		st, err = lookupState(stateID)
		if err != nil {
			log.Printf("dancefitvn: %v", err)
		}
		shot = err == nil
	}
	knobs := wireKnobsOf(st)
	knobs.memOverlay = shot && !st.diskOverlay
	if devEnv(offlineEnv) == "1" {
		knobs.offline = true
	}
	dlKnobs(&knobs)
	w := newMobileWire(deps, knobs)
	sh := shell.New(w, texts, shell.Options{HoldSplash: st.hold})
	var flog *frameLog // nil without DFV_FRAMELOG=1
	var mark func()    // the mark of the first Home frame (devperf.go)
	if devBuild {
		if devEnv(frameLogEnv) == "1" {
			flog = newFrameLog(os.Stderr, nil, 0)
		}
		mark = newHomeMark(func() bool { return mobileHomeShown(sh) }, w.deps.Logf).after
	}
	var flogFrame func()
	if flog != nil {
		flogFrame = flog.onFrame
	}

	return appRoot{
		widget: sh,
		close: func() {
			flog.close()
			w.close()
		},
		afterFrame: chainFrames(w.pb.firstFrame, mark, flogFrame),
		mounted: func(v *ui.View) {
			flog.attach(v)
			w.attach(v)
			if devParts && shot {
				d := newDevShot(stateID, "mobile", v, w.shotSource(), settleTime())
				newStateRunner(stateID, st, sh, w, d.arm).start(v.Context())
			}
		},
	}
}

// viewOptions returns the options of the phone's view, before the GPU's:
// the theme, and the coarse backdrops a phone's GPU affords.
func viewOptions() []ui.ViewOption {
	return []ui.ViewOption{ui.WithTheme(newTheme()), ui.WithCoarseBackdrops()}
}

// mounted sets up the phone's view once mounted: portrait only (D2),
// light status bar content over the dark stage.
func mounted(v *ui.View) {
	v.SetOrientations(platform.OrientationsPortrait)
	v.SetSystemBars(platform.SystemBars{Content: platform.BarContentLight})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// hostDeps returns the system's parts of the phone's services on h: its
// UI thread, its driver's secrets, the app's folders, the biometric sheet,
// the URL opener and the keep-awake of internal/sys, the system's Now
// Playing, the lifecycle, the downloads' folder, free space, network and
// background task, the photo picker, the mock API main was built with. The
// media are the system's (NewMedia nil).
func hostDeps(h *host.App) mobileDeps {
	enqueue := func(fn func()) { h.Enqueue(fn) }

	return mobileDeps{
		Mock:    options.Mock,
		Enqueue: h.Enqueue,
		Secrets: func() (auth.SecretStore, error) {
			s, err := sys.OpenSecrets(h.Driver(), h.Invoke)
			if err != nil {
				return nil, err
			}

			return s, nil
		},
		InstallID:  sys.InstallID,
		DataDir:    sys.DataDir,
		CacheDir:   sys.CacheDir,
		Bio:        sys.NewBiometric(enqueue),
		OpenURL:    sys.OpenURL,
		Platform:   phonePlatform(runtime.GOOS),
		Awake:      sysAwake{driver: h.Driver},
		NowPlaying: driverNowPlaying{driver: h.Driver},
		OnLifecycle: func(fn func(platform.LifecycleState)) func() {
			return h.OnLifecycle(fn)
		},
		AudioDir:  sys.PrepareAudioDir,
		FreeSpace: sys.FreeSpace,
		Network: func(dispatch func(func())) netWatch {
			return watchNetwork(dispatch, log.Printf, devNetworkFile(), devEnv(networkEnv))
		},
		BackgroundTask: sys.BeginBackgroundTask,
		PickImage:      sys.PickImage,
	}
}

// mobileHomeShown tells whether Home is on show with its data: the
// screen of the mark of the first Home frame (devperf.go).
func mobileHomeShown(sh *shell.Shell) bool {
	h, ok := sh.Nav().Screen().(*screens.Home)

	return ok && h.Content() != nil
}

// phonePlatform is the X-Platform of the phone the app runs on: "android"
// on Android, "ios" elsewhere (the simulator, and the desktop the tests run
// on).
func phonePlatform(goos string) string {
	if goos == "android" {
		return "android"
	}

	return "ios"
}

// shotSource is what a developer's shot reads of the phone's services.
func (w *mobileWire) shotSource() shotSource {
	return shotSource{idle: func() (bool, int) {
		if w.tr == nil {
			return true, 0
		}

		return w.tr.idle()
	}, cacheDir: w.deps.CacheDir, logf: w.deps.Logf}
}
