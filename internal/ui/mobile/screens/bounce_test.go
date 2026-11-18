//go:build !tvos

package screens_test

import (
	"image"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestEveryPhoneScreenScrollBounces checks that the scroll of each screen that has
// one bounces past its ends (K36), on both phones: the page of Home (its content
// and its skeleton), Courses, Downloads, Account, the edit screens, Detail, Coach,
// Connect, the offline list, Login and Done.
func TestEveryPhoneScreenScrollBounces(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		scroll := func(r *shelltest.Rig) *ui.Scroll {
			s, ok := r.Shell.Nav().Screen().(nav.Scroller)
			require.True(t, ok, "the screen is a Scroller")

			return s.Scroll()
		}
		tests := []struct {
			name string
			open func() *ui.Scroll
		}{
			{"home", func() *ui.Scroll { return scroll(atHome(t, ph, false)) }},
			{"home loading", func() *ui.Scroll { return scroll(atHomeLoading(t, ph, false)) }},
			{"courses", func() *ui.Scroll { return scroll(atCourses(t, ph, false)) }},
			{"downloads", func() *ui.Scroll { return scroll(atDownloads(t, ph, false)) }},
			{"account", func() *ui.Scroll { return scroll(atAccount(t, ph, false)) }},
			{"info", func() *ui.Scroll { _, in := atInfo(t, ph, false); return in.Scroll() }},
			{"detail", func() *ui.Scroll { return scroll(atDetail(t, ph, false, "set-35")) }},
			{"locked detail", func() *ui.Scroll { return scroll(atDetail(t, ph, false, "set-36")) }},
			{"coach", func() *ui.Scroll { _, s := atCoach(t, ph, false); return s.Scroll() }},
			{"connect", func() *ui.Scroll { _, s := atConnect(t, ph, false); return s.Scroll() }},
			{"offline", func() *ui.Scroll { return scroll(atOffline(t, ph, false)) }},
			{"login", func() *ui.Scroll { _, l := atLogin(t, ph, false); return l.Scroll() }},
			{"done", func() *ui.Scroll { _, _, d := atDone(t, ph, false); return d.Scroll() }},
		}
		for _, tc := range tests {
			sc := tc.open()
			assert.True(t, sc.Bounce, tc.name)
			assert.Equal(t, ui.ScrollVertical, sc.Axes, tc.name)
		}
	})
}

// TestHomeAndAccountPullPastTheTopShowTheBackground pulls Home and Account 200
// DIPs down at their top: the content follows with resistance, and the strip it
// leaves under the status bar shows --bg, as the prototype's `.scr` shows behind
// its `.scroll` (index.html:73-74); then it springs back and the strip is gone.
func TestHomeAndAccountPullPastTheTopShowTheBackground(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		look, _ := theme.New(ph.Naming).Ext[*theme.Look]()
		for _, name := range []string{"home", "account"} {
			var r *shelltest.Rig
			if name == "home" {
				r = atHome(t, ph, false)
			} else {
				r = atAccount(t, ph, false)
			}
			kittest.PinMotion(r.Tester)
			sc := r.Shell.Nav().Screen().(nav.Scroller).Scroll()
			top := nav.ContentTop(r.Tester.View().Context())
			before := r.Tester.Snapshot()

			r.Tester.TouchDown(geom.Pt(200, 300))
			for i := 1; i <= 20; i++ {
				r.Tester.Elapse(kittest.Frame)
				r.Tester.TouchMove(geom.Pt(200, 300+float32(i)*10))
			}
			over := -sc.Offset().Y
			assert.Greater(t, over, float32(50), name+": the content follows the finger")
			assert.Less(t, over, float32(192), name+": with resistance")
			img := r.Tester.Snapshot()
			for _, y := range []int{int(top) + 2, int(top) + int(over)/2, int(top) + int(over) - 3} {
				assert.Equal(t, look.T.Bg, pixel(img, 200, y), "%s: --bg in the pulled strip at y=%d", name, y)
				assert.Equal(t, look.T.Bg, pixel(img, 3, y), "%s: --bg in the pulled strip at y=%d (edge)", name, y)
			}
			r.Tester.TouchUp(geom.Pt(200, 500))
			r.Tester.Elapse(time.Second)
			assert.InDelta(t, 0, sc.Offset().Y, 0.01, name+": back")
			assert.Equal(t, before.Pix, r.Tester.Snapshot().Pix, name+": the same picture as before the pull")
		}
	})
}

// TestDetailPulledPastTheTopShowsTheBackground is the same for the detail, whose
// scroll starts at the top of the window (`.scroll{top:0}`): the hero moves down and
// --bg shows above it, as in the prototype's `.scr`; the pinned foot stays.
func TestDetailPulledPastTheTopShowsTheBackground(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		look, _ := theme.New(ph.Naming).Ext[*theme.Look]()
		r := atDetail(t, ph, false, "set-35")
		kittest.PinMotion(r.Tester)
		sc := detailOf(t, r).Scroll()
		r.Tester.TouchDown(geom.Pt(200, 300))
		for i := 1; i <= 20; i++ {
			r.Tester.Elapse(kittest.Frame)
			r.Tester.TouchMove(geom.Pt(200, 300+float32(i)*10))
		}
		over := int(-sc.Offset().Y)
		require.Greater(t, over, 50)
		img := r.Tester.Snapshot()
		for _, y := range []int{2, over / 2, over - 3} {
			assert.Equal(t, look.T.Bg, pixel(img, 200, y), "--bg above the hero at y=%d", y)
		}
		r.Tester.TouchUp(geom.Pt(200, 500))
		r.Tester.Elapse(time.Second)
		assert.InDelta(t, 0, sc.Offset().Y, 0.01)
	})
}

// TestHomeAndAccountBounceFramesDoNotAllocate checks that frames of the heavy
// screens bouncing allocate nothing: a finger pulling the page past its top and
// back, and past its bottom, and the spring after it lets go.
func TestHomeAndAccountBounceFramesDoNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		for _, name := range []string{"home", "account"} {
			var r *shelltest.Rig
			if name == "home" {
				r = atHome(t, ph, false)
			} else {
				r = atAccount(t, ph, false)
			}
			kittest.PinMotion(r.Tester)
			sc := r.Shell.Nav().Screen().(nav.Scroller).Scroll()
			warm(r)
			for y := float32(0); y < sc.MaxOffset().Y+1; y += 60 { // the whole page is laid out and shaped once
				sc.ScrollTo(geom.Pt(0, y), false)
				r.Tester.Elapse(kittest.Frame)
			}
			for _, end := range []struct {
				at  float32
				dir float32
			}{{0, 1}, {sc.MaxOffset().Y, -1}} {
				sc.ScrollTo(geom.Pt(0, end.at), false)
				r.Tester.Elapse(kittest.Frame)
				// the finger pulls the page 40 to 160 DIPs past the end, back and forth
				y, dir := float32(40), float32(1)
				finger := func() geom.Point { return geom.Pt(200, 400+end.dir*y) }
				r.Tester.TouchDown(geom.Pt(200, 400))
				drag := func() {
					if y > 160 || y < 40 {
						dir = -dir
					}
					y += 4 * dir
					r.Tester.TouchMove(finger())
					r.Tester.Elapse(kittest.Frame)
				}
				for range 40 {
					drag()
				}
				assert.NotZero(t, sc.Offset().Y-end.at, "%s %s: the page is past its end", ph.Name, name)
				assert.Zero(t, testing.AllocsPerRun(50, drag), "%s %s: a finger pulling past the end (%v)", ph.Name, name, end.dir)
				r.Tester.TouchUp(finger())
				frame := func() { r.Tester.Elapse(kittest.Frame) }
				frame()
				assert.Zero(t, testing.AllocsPerRun(15, frame), "%s %s: the spring back (%v)", ph.Name, name, end.dir)
				r.Tester.Elapse(time.Second)
			}
			sc.ScrollTo(geom.Pt(0, 0), false)
		}
	}
}

// BenchmarkHomeBounceFrame is a frame of Home pulled past its top by a finger.
func BenchmarkHomeBounceFrame(b *testing.B) {
	r := atHome(b, kittest.Phones[0], false)
	warm(r)
	y, dir := float32(300), float32(1)
	r.Tester.TouchDown(geom.Pt(200, y))
	b.ReportAllocs()
	for b.Loop() {
		if y > 460 || y < 320 {
			dir = -dir
		}
		y += 4 * dir
		r.Tester.TouchMove(geom.Pt(200, y))
		r.Tester.Elapse(kittest.Frame)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// pixel returns the color of the snapshot at (x, y) as the theme's color type.
func pixel(img *image.RGBA, x, y int) color.Color {
	p := img.RGBAAt(x, y)

	return color.Color{R: p.R, G: p.G, B: p.B, A: p.A}
}
