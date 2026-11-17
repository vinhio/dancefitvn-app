// Adapted from jplayer-mobile/e2e/engine/main_ios_test.go at 49de3c2.

//go:build ios && !tvos

package engine

import (
	"context"
	"dancefitvn-app/e2e/e2emedia"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdeskdev/desk/bridge/gpu"
	"github.com/gdeskdev/desk/framework/host"
	"github.com/gdeskdev/desk/framework/ui"

	_ "github.com/gdeskdev/desk/kits/mobile" // the drivers of iOS and Android
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestMain runs the app UIKit launches and the tests in a goroutine once
// it has: UIApplicationMain never returns, the tests' end ends the process.
// Before the tests it serves the bundle's media on 127.0.0.1 (e2emedia:
// the mock's Server and the gate of signed links).
func TestMain(m *testing.M) {
	app, err := host.New(host.WithName("DanceFITVN e2e"), host.WithID("com.jivecode.dancefitvn.e2e"))
	if err != nil {
		fmt.Println("e2e:", err)
		os.Exit(1)
	}
	app.OnLaunch(func(context.Context) error {
		srv, err := e2emedia.Start(mediaDir(), log.Printf)
		if err != nil {
			return err
		}
		w, err := app.OpenWindow(host.WindowOptions{Title: "DanceFITVN e2e", Width: 393, Height: 852})
		if err != nil {
			return err
		}
		screen := newScreen()
		// The view draws on the GPU as the app's does (internal/app).
		opts := []ui.ViewOption{ui.WithTheme(ui.DarkTheme()), ui.WithFrameStats()}
		if g, gerr := gpu.New(); gerr == nil {
			opts = append(opts, ui.WithRenderMode(ui.RenderGPU), ui.WithGPU(g))
		} else {
			fmt.Printf("e2e: no GPU renderer (%v): the view draws on the CPU\n", gerr)
		}
		view, err := ui.Mount(w, screen, opts...)
		if err != nil {
			return err
		}
		theApp, theScreen, theView, theMedia = app, screen, view, srv
		go func() {
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

// mediaDir is where devtest-ios.sh put the test media: the app bundle's
// media folder.
func mediaDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "media"
	}

	return filepath.Join(filepath.Dir(exe), "media")
}
