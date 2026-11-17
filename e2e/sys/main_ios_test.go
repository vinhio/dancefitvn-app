// Adapted from jplayer-mobile/e2e/sys/main_ios_test.go at 49de3c2.
// Changed: the app's name and id. The build tag ios covers tvOS too.

//go:build ios

package sys

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gdeskdev/desk/framework/host"
	"github.com/gdeskdev/desk/framework/ui"

	_ "github.com/gdeskdev/desk/kits/mobile" // the drivers of iOS and Android
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestMain runs the app UIKit launches and the tests in a goroutine once
// it has: UIApplicationMain never returns, the tests' end ends the process.
func TestMain(m *testing.M) {
	app, err := host.New(host.WithName("DanceFITVN e2e"), host.WithID("com.jivecode.dancefitvn.e2e"))
	if err != nil {
		fmt.Println("e2e:", err)
		os.Exit(1)
	}
	app.OnLaunch(func(context.Context) error {
		w, err := app.OpenWindow(host.WindowOptions{Title: "DanceFITVN e2e", Width: 393, Height: 852})
		if err != nil {
			return err
		}
		if _, err := ui.Mount(w, &ui.Text{Value: "DanceFITVN e2e: sys", Wrap: true}, ui.WithTheme(ui.DarkTheme())); err != nil {
			return err
		}
		theApp, theWindow = app, w
		go func() {
			waitWindow(app, w)
			code := m.Run()
			time.Sleep(time.Second) // lets the console take the verdict
			os.Exit(code)
		}()

		return nil
	})
	if err := app.Run(); err != nil {
		fmt.Println("e2e:", err)
		os.Exit(1)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// waitWindow waits until w has its UIWindow, which UIKit makes once the
// scene connects, after the launch.
func waitWindow(app *host.App, w *host.Window) {
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for range tick.C {
		ready := false
		_ = app.Invoke(func() { ready = w.Canvas() != nil && w.Canvas().Handles().Window != 0 })
		if ready {
			return
		}
	}
}
