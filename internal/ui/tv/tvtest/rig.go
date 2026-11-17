//go:build tvos

package tvtest

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The stage of the TV: 1920x1080 DIPs (docs/plan/02 section 1), with the safe
// area tvOS reports (60 at the sides, 80 above and below).
const (
	Width, Height float32 = 1920, 1080
)

// Safe is the safe area of the Apple TV.
var Safe = geom.Insets{Top: 80, Left: 60, Bottom: 80, Right: 60}

// Frame is one 60 Hz refresh.
const Frame = time.Second / 60

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Mount mounts root as the view's root at 1920x1080, scale 1, with the app's
// theme, directional focus and a transparent background, the safe area set,
// and runs a frame. Text is shaped by gDesk's fake engine: widths are not the
// fonts', so use it for behavior; MountReal for sizes. Under the race detector
// motion is reduced (kittest.Race). The tester closes with the test.
func Mount(t testing.TB, root ui.Widget) *ui.Tester {
	t.Helper()

	return mount(t, root, true)
}

// MountReal is Mount with Core Text and the app's fonts: boxes measured with
// it are the ones a device draws. It skips the test where there is no Core
// Text.
func MountReal(t testing.TB, root ui.Widget) *ui.Tester {
	t.Helper()

	return mount(t, root, false)
}

// PinMotion puts the motion back that Mount reduces under the race detector.
func PinMotion(tt *ui.Tester) { kittest.PinMotion(tt) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// mount builds the tester.
func mount(t testing.TB, root ui.Widget, fake bool) *ui.Tester {
	t.Helper()
	th := theme.New(theme.NamingCoreText)
	th.ReduceMotion = kittest.Race
	opts := []ui.ViewOption{ui.WithTheme(th), ui.WithDirectionalFocus(), ui.WithTransparentBackground()}
	if !fake {
		opts = append(opts, ui.WithShaper(kittest.Shaper(t, kittest.Phone{Naming: theme.NamingCoreText})))
	}
	tt, err := ui.NewTester(root, Width, Height, opts...)
	if err != nil {
		t.Fatalf("tester: %v", err)
	}
	t.Cleanup(tt.Close)
	tt.SetSafeArea(Safe)
	tt.Elapse(Frame)

	return tt
}
