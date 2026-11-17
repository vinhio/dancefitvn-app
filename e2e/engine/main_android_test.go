// Adapted from jplayer-mobile/e2e/engine/main_android_test.go at 49de3c2.
// Changed: the app's id and name; the media served on 127.0.0.1 by
// e2emedia (as main_ios_test.go), not read as files; no prober.

//go:build android && cgo

package engine

import (
	"context"
	"dancefitvn-app/e2e/e2emedia"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdeskdev/desk/bridge/gpu"
	"github.com/gdeskdev/desk/framework/host"
	"github.com/gdeskdev/desk/framework/ui"

	_ "github.com/gdeskdev/desk/kits/mobile" // the drivers of iOS and Android
)

// appID is the application id devtest-android.sh installs.
const appID = "com.jivecode.dancefitvn.e2e"

// init asks for every test's name in the log: an app gets no command line.
func init() {
	testing.Init()
	_ = flag.Set("test.v", "true")
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestMain runs the app the activity launches and the tests in a goroutine
// once it has, as the iOS side does; the tests' end ends the process.
// Before the tests it serves the media folder on 127.0.0.1 (e2emedia: the
// mock's Server and the gate of signed links).
func TestMain(m *testing.M) {
	app, err := host.New(host.WithName("DanceFITVN e2e"), host.WithID(appID))
	if err != nil {
		fmt.Println("e2e:", err)
		os.Exit(1)
	}
	app.OnLaunch(func(context.Context) error {
		srv, err := e2emedia.Start(mediaDir(), log.Printf)
		if err != nil {
			return err
		}
		w, err := app.OpenWindow(host.WindowOptions{Title: "DanceFITVN e2e"})
		if err != nil {
			return err
		}
		screen := newScreen()
		// The view draws on the GPU as the app's does (internal/app): on
		// Android's OpenGL ES the window takes a swap chain only from the
		// first view's first frame. Without a renderer it draws on the CPU.
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
			// run.txt, pushed by devtest-android.sh when DEVTEST_RUN is set,
			// holds the -test.run pattern: an app gets no command line.
			if b, err := os.ReadFile(filepath.Join(mediaDir(), "run.txt")); err == nil {
				if pat := strings.TrimSpace(string(b)); pat != "" {
					_ = flag.Set("test.run", pat)
				}
			}
			code := m.Run()
			// logcat reads the app's output asynchronously: exiting at once
			// loses the last lines, the verdict among them.
			time.Sleep(time.Second)
			os.Exit(code)
		}()

		return nil
	})
	if err := app.Run(); err != nil {
		fmt.Println("e2e:", err)
		os.Exit(1)
	}
}

// mediaDir is where devtest-android.sh pushed the test media: the app's
// external files folder, Context.getExternalFilesDir(null).
func mediaDir() string { return "/sdcard/Android/data/" + appID + "/files" }
