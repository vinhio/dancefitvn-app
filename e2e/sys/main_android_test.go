// Adapted from jplayer-mobile/e2e/sys/main_android_test.go at 49de3c2.
// Changed: the app's name and id.

//go:build android && cgo

package sys

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
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
func TestMain(m *testing.M) {
	app, err := host.New(host.WithName("DanceFITVN e2e"), host.WithID(appID))
	if err != nil {
		fmt.Println("e2e:", err)
		os.Exit(1)
	}
	app.OnLaunch(func(context.Context) error {
		w, err := app.OpenWindow(host.WindowOptions{Title: "DanceFITVN e2e"})
		if err != nil {
			return err
		}
		if _, err := ui.Mount(w, &ui.Text{Value: "DanceFITVN e2e: sys", Wrap: true}, ui.WithTheme(ui.DarkTheme())); err != nil {
			return err
		}
		theApp, theWindow = app, w
		go func() {
			// run.txt, pushed by devtest-android.sh when DEVTEST_RUN is set,
			// holds the -test.run pattern: an app gets no command line.
			if b, err := os.ReadFile(filepath.Join(mediaDir(), "run.txt")); err == nil {
				if pat := strings.TrimSpace(string(b)); pat != "" {
					_ = flag.Set("test.run", pat)
				}
			}
			waitFocus()
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

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// waitFocus waits, at most 10 s, until the activity's window has the
// focus: the tests start once the app is in front.
func waitFocus() {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	deadline := time.Now().Add(10 * time.Second)
	for range tick.C {
		if hasFocus() || time.Now().After(deadline) {
			return
		}
	}
}

// hasFocus asks Activity.hasWindowFocus on Java's main thread.
func hasFocus() bool {
	ch := make(chan bool, 1)
	if err := jni.PostMain(func(env jni.Env) {
		act := jni.Activity()
		if act == 0 {
			ch <- false

			return
		}
		c, err := env.FindClass("android/app/Activity")
		if err != nil {
			ch <- false

			return
		}
		m, err := env.Method(c, "hasWindowFocus", "()Z")
		if err != nil {
			ch <- false

			return
		}
		ok, err := env.CallBool(act, m)
		ch <- ok && err == nil
	}); err != nil {
		return false
	}

	return <-ch
}

// mediaDir is the app's external files folder,
// Context.getExternalFilesDir(null), where devtest-android.sh pushes the
// media and the answers of its driver.
func mediaDir() string { return "/sdcard/Android/data/" + appID + "/files" }
