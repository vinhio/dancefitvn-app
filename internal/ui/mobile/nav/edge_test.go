package nav_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/gesture"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
)

// TestEdgeSwipe checks the swipe from the left edge is Back: a 26 DIP strip, 80
// DIPs to the right or faster than 0.6 DIP/ms, going sideways first (Q-P2-4).
func TestEdgeSwipe(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		from geom.Point
		dx   float32
		dy   float32
		over time.Duration
		back bool
	}{
		{"81 slow commits", geom.Pt(10, 400), 81, 0, 1500 * time.Millisecond, true},
		{"79 slow cancels", geom.Pt(10, 400), 79, 0, 1500 * time.Millisecond, false},
		{"40 at .8 DIP/ms is a fling", geom.Pt(10, 400), 40, 0, 50 * time.Millisecond, true},
		{"50 at .33 DIP/ms is not", geom.Pt(10, 400), 50, 0, 150 * time.Millisecond, false},
		{"the strip's last DIP", geom.Pt(25, 400), 120, 0, 300 * time.Millisecond, true},
		{"just outside the strip", geom.Pt(26, 400), 120, 0, 300 * time.Millisecond, false},
		{"far from the edge", geom.Pt(200, 400), 120, 0, 300 * time.Millisecond, false},
		{"going down first is a scroll's", geom.Pt(10, 300), 0, 200, 300 * time.Millisecond, false},
		{"mostly down", geom.Pt(10, 300), 60, 200, 300 * time.Millisecond, false},
		{"to the left", geom.Pt(20, 400), -15, 0, 300 * time.Millisecond, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
				r := newRig(t, ph)
				r.n.Go(route("detail"), false)
				r.tt.Elapse(kittest.Frame)
				r.tt.Swipe(tc.from, geom.Pt(tc.from.X+tc.dx, tc.from.Y+tc.dy), tc.over)
				if tc.back {
					assert.Equal(t, "home", r.n.Current().Screen)
					assert.Zero(t, r.n.Depth())
				} else {
					assert.Equal(t, "detail", r.n.Current().Screen)
					assert.Equal(t, 1, r.n.Depth())
				}
			})
		})
	}
}

// TestEdgeSwipeIsBack checks the swipe does what the Back key does at each step:
// the sheet first, a root goes Home, Home does nothing.
func TestEdgeSwipeIsBack(t *testing.T) {
	t.Parallel()
	swipe := func(r *rig) { r.tt.Swipe(geom.Pt(5, 500), geom.Pt(205, 500), 400*time.Millisecond) }
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		r.n.Go(route("courses"), true)
		r.n.Go(route("detail"), false)
		r.n.Sheet().Open(&block{h: 100}, kit.SheetOptions{})
		r.tt.Elapse(400 * time.Millisecond)

		swipe(r)
		assert.False(t, r.n.Sheet().IsOpen(), "the sheet closes first, over the dim")
		assert.Equal(t, "detail", r.n.Current().Screen)
		swipe(r)
		assert.Equal(t, "courses", r.n.Current().Screen)
		swipe(r)
		assert.Equal(t, "home", r.n.Current().Screen, "Courses is a root: Home, as back() does")
		built := len(r.built)
		swipe(r)
		assert.Equal(t, "home", r.n.Current().Screen, "Home: nothing to go back to")
		assert.Len(t, r.built, built, "and nothing rebuilt")
		assert.False(t, r.n.EdgeArmed(r.tt.View().Context()))
	})
}

// TestEdgeLeavesTheStripToTheSystem checks the Nav does not take the edge where the
// system does (Android's gesture navigation, SystemGestures().Left > 0): the
// system sends the Back key instead (TestBackChain), the behavior is one.
func TestEdgeLeavesTheStripToTheSystem(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		r.n.Go(route("detail"), false)
		r.tt.SetSystemGestures(geom.Insets{Top: 57.333, Left: 30, Right: 30, Bottom: 32})
		r.tt.Elapse(kittest.Frame)
		assert.False(t, r.n.EdgeArmed(r.tt.View().Context()))
		r.tt.Swipe(geom.Pt(10, 400), geom.Pt(210, 400), 400*time.Millisecond)
		assert.Equal(t, "detail", r.n.Current().Screen, "no swipe under gesture navigation")

		// three-button navigation: Left is 0 again, and the Nav takes the edge
		r.tt.SetSystemGestures(geom.Insets{Top: 57.333, Bottom: 48})
		r.tt.Elapse(kittest.Frame)
		assert.True(t, r.n.EdgeArmed(r.tt.View().Context()))
		r.tt.Swipe(geom.Pt(10, 400), geom.Pt(210, 400), 400*time.Millisecond)
		assert.Equal(t, "home", r.n.Current().Screen)
	})
}

// TestEdgeDoesNotStealTaps checks a tap in the strip still reaches what is under
// it, and a swipe from the strip does not scroll the screen or press its buttons.
func TestEdgeDoesNotStealTaps(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		taps := 0
		var btn *kit.Button
		r := newRigWith(t, ph, func(rt string) ui.Widget {
			if rt != "detail" {
				return &block{h: 2000}
			}
			btn = kit.NewButton(kit.ButtonP, "Mở", 0)
			btn.OnPress = func() { taps++ }

			return btn
		})
		r.n.Go(route("detail"), false)
		r.tt.Elapse(kittest.Frame)
		// the button spans the page's 20 side padding: tap in its middle
		r.tt.ClickAt(btn.WindowFrame().Center())
		assert.Equal(t, 1, taps)
		// a tap in the strip, over the padding, does nothing and goes nowhere
		r.tt.ClickAt(geom.Pt(10, btn.WindowFrame().Center().Y))
		assert.Equal(t, "detail", r.n.Current().Screen)
		assert.Equal(t, 1, taps)
		// a swipe that starts on the button but not in the strip is the button's
		r.tt.Swipe(geom.Pt(60, btn.WindowFrame().Center().Y), geom.Pt(260, btn.WindowFrame().Center().Y), 300*time.Millisecond)
		assert.Equal(t, "detail", r.n.Current().Screen)
	})
}

// TestEdgeThresholdsAreTheGesturePackages checks the Nav uses the numbers of
// kit/gesture without restating them.
func TestEdgeThresholdsAreTheGesturePackages(t *testing.T) {
	t.Parallel()
	e := gesture.EdgeBack()
	assert.InDelta(t, 26, float64(e.Strip), 0)
	assert.InDelta(t, 80, float64(e.Distance), 0)
	assert.InDelta(t, .6, float64(e.Fling), 1e-6)
}
