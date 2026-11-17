// Adapted from jplayer-mobile/e2e/sys/harness_test.go at 49de3c2
// (theApp, theWindow, shown, within, image as picture, onUI, dispatch,
// canvas, await, eventually; the file pickers' types left out).

//go:build ios || (android && cgo)

package sys

import (
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/host"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The app TestMain started (main_ios_test.go, main_android_test.go) and
// its window. Set before the tests run, read-only after.
var (
	theApp    *host.App
	theWindow *host.Window
)

// shown names what a service shows over the app, for the platform's
// expectShown and cancelShown (platform_ios_test.go,
// platform_android_test.go).
type shown int

// The things shown.
const (
	shownUnknown shown = iota
	shownImage         // the photo picker
)

// within bounds every wait of the tests: the monitor's first answer, a
// picker's animation, the emulator's driver (a second a turn) and a slow
// emulator.
const within = 30 * time.Second

// picture is what PickImage's done got.
type picture struct {
	data []byte
	mime string
	err  error
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onUI runs fn on the app's UI thread and returns once it has.
func onUI(t *testing.T, fn func()) {
	t.Helper()

	require.NoError(t, theApp.Invoke(fn))
}

// dispatch is the services' Dispatch: the app's queue to its UI thread.
func dispatch(fn func()) { theApp.Enqueue(fn) }

// canvas returns the window's canvas; UI thread.
func canvas() platform.Canvas { return theWindow.Canvas() }

// await returns what ch receives within d, failing the test at once after.
func await[T any](t *testing.T, ch <-chan T, d time.Duration, what string) T {
	t.Helper()

	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case v := <-ch:
		return v
	case <-timer.C:
		require.FailNow(t, "no answer", "%s within %v", what, d)

		var zero T

		return zero
	}
}

// eventually waits until cond holds, asked every 100 ms, failing the test
// after d.
func eventually(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()

	deadline := time.NewTimer(d)
	defer deadline.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for !cond() {
		select {
		case <-tick.C:
		case <-deadline.C:
			require.FailNow(t, "never", "%s within %v", what, d)
		}
	}
}
