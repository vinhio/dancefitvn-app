// Adapted from jplayer-tv/e2e/storecheck/main.go at 781bee5.

// Command tvstore is the Apple TV simulator check of the TV's state
// (P7.5d, local/plans/P7.md Q-P7-6): one launch reads the preferences
// (store.PrefsKey), the progress queue (progress.QueueKey) and the device
// document ("device") through the Mirror the app uses on tvOS, of
// NSUserDefaults (the primary, internal/store/tvstore) and the state
// folder in Library/Caches (the copy, store.Dir under sys.DataDir),
// compares them with what an earlier launch wrote, and writes the next
// values. scripts/tvos-store-check.sh runs it across relaunches, a purge
// of Library/Caches, spoilt slots, a reboot, an install over and a
// reinstall.
//
// It takes launch arguments, not environment variables (jPlayer TV T0.5
// saw simctl's SIMCTL_CHILD_ variables keep stale values):
//
//	tvstore <label> [-corrupt=primary|copy] [-expect=<id>|defaults] [-write=<id>]
//
// and prints "tvstore:" lines on standard output, among them one
// "tvstore: PASS <label>" or "tvstore: FAIL <label> ...". Then it stays
// alive, a window painted with the Night Stage background, until it is
// terminated. Off tvOS both slots are folders under os.TempDir() and the
// program cannot open a window (no desktop kit): Check is what the Mac
// tests.
package main

import (
	"context"
	"fmt"
	"os"

	"dancefitvn-app/internal/tokens"

	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/host"
	"github.com/gdeskdev/desk/framework/ui"

	_ "github.com/gdeskdev/desk/kits/mobile" // the drivers of iOS (and so tvOS) and Android
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func main() {
	cfg, err := ParseArgs(os.Args[1:])
	if err != nil {
		fail("%v", err)
	}
	slots, where, err := openSlots()
	if err != nil {
		fail("%v", err)
	}
	say("start label=%s args=%q %s", cfg.Label, os.Args[1:], where)
	app, err := host.New(host.WithName("TVStore"))
	if err != nil {
		fail("host: %v", err)
	}
	app.OnLaunch(func(context.Context) error {
		Check(cfg, slots, installID, os.Stdout)

		return openWindow(app)
	})
	if err := app.Run(); err != nil {
		fail("run: %v", err)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// openWindow shows the Night Stage background.
func openWindow(app *host.App) error {
	w, err := app.OpenWindow(host.WindowOptions{Title: "TVStore", Width: 1920, Height: 1080})
	if err != nil {
		return err
	}
	view, err := ui.Mount(w, &ui.Box{Background: scene.Solid(tokens.Dark.Bg)})
	if err != nil {
		return err
	}
	w.OnClosed(func(*host.Window) { view.Close() })

	return nil
}

// say prints one "tvstore:" line on standard output, where simctl's
// --console-pty shows it.
func say(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stdout, "tvstore: "+format+"\n", args...)
}

// fail prints a line and exits with status 2.
func fail(format string, args ...any) {
	say("fatal "+format, args...)
	os.Exit(2)
}
