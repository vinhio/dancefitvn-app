package nav_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the pull, from gDesk's Scroll.Bounce: UIScrollView's rubber
// band (1 - 1/(x*0.55/d + 1))*d and the 500 ms the content takes to spring back.
const (
	bounceResist = 0.55
	bounceTime   = 500 * time.Millisecond
)

// bounceRig is a Nav at Home whose page is a tall block, and the block.
type bounceRig struct {
	*rig
	body *block
	sc   *ui.Scroll
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestPageBouncesPastItsTopAndBottom pulls the page past each end with a finger
// (K36): the content follows with resistance (less than the finger, never a whole
// screen), is placed that far past the end, and springs back within 500 ms
// without crossing the end, with no frame asked for after.
func TestPageBouncesPastItsTopAndBottom(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		b := newBounceRig(t, ph)
		require.True(t, b.sc.Bounce, "the page's scroll bounces")
		at := geom.Pt(200, 400)
		for _, end := range []struct {
			name string
			off  float32 // where the page is scrolled to first
			dir  float32 // the finger's way: +1 down (the content past the top)
		}{{"top", 0, 1}, {"bottom", b.sc.MaxOffset().Y, -1}} {
			b.sc.ScrollTo(geom.Pt(0, end.off), false)
			b.tt.Elapse(kittest.Frame)
			bodyY := b.body.WindowFrame().Y
			// 120 DIPs of finger, of which the first 8 are the slop.
			b.tt.TouchDown(at)
			for i := 1; i <= 12; i++ {
				b.tt.Elapse(kittest.Frame)
				b.tt.TouchMove(geom.Pt(at.X, at.Y+end.dir*float32(i)*10))
			}
			past := (b.sc.Offset().Y - end.off) * -end.dir // how far the content shows past the end
			d := b.sc.Frame().H
			want := rubber(112, d)
			assert.InDelta(t, want, past, 0.6, "%s: the rubber band", end.name)
			assert.Less(t, past, float32(112), "%s: less than the finger moved", end.name)
			assert.InDelta(t, bodyY+end.dir*want, b.body.WindowFrame().Y, 1, "%s: the content is placed past the end", end.name)

			b.tt.TouchUp(geom.Pt(at.X, at.Y+end.dir*120))
			// The content goes on outwards a moment at the finger's speed, then
			// only back.
			prev, back := past, false
			for range int(bounceTime/kittest.Frame) + 2 {
				b.tt.Elapse(kittest.Frame)
				now := (b.sc.Offset().Y - end.off) * -end.dir
				assert.GreaterOrEqual(t, now, float32(-0.01), "%s: never crosses the end", end.name)
				back = back || now < prev
				if back {
					assert.LessOrEqual(t, now, prev+0.01, "%s: once back, only goes back", end.name)
				}
				prev = now
			}
			assert.True(t, back, "%s: it came back", end.name)
			assert.InDelta(t, end.off, b.sc.Offset().Y, 0.01, "%s: springs back within 500 ms", end.name)
			assert.False(t, b.tt.Pending(), "%s: no frame asked for once it is back", end.name)
		}
	})
}

// TestPageFlingIntoTheEdgeBounces throws the page at its top from lower down: the
// fling carries on past the top by the spring and the page comes to rest on it.
func TestPageFlingIntoTheEdgeBounces(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		b := newBounceRig(t, ph)
		var low, high float32
		b.sc.OnScroll = func(o geom.Point) { low, high = min(low, o.Y), max(high, o.Y) }
		b.sc.ScrollTo(geom.Pt(0, 400), false)
		b.tt.Elapse(kittest.Frame)
		b.tt.Swipe(geom.Pt(200, 300), geom.Pt(200, 400), 32*time.Millisecond)
		b.tt.Elapse(2 * time.Second)
		assert.Less(t, low, float32(-1), "the fling went past the top")
		assert.Greater(t, low, float32(-b.sc.Frame().H), "by less than a screen")
		assert.InDelta(t, 0, b.sc.Offset().Y, 0.01, "and came to rest on it")
		assert.False(t, b.tt.Pending())

		// And off the bottom.
		low, high = 0, 0
		max := b.sc.MaxOffset().Y
		b.sc.ScrollTo(geom.Pt(0, max-400), false)
		b.tt.Elapse(kittest.Frame)
		b.tt.Swipe(geom.Pt(200, 400), geom.Pt(200, 300), 32*time.Millisecond)
		b.tt.Elapse(2 * time.Second)
		assert.Greater(t, high, max+1, "the fling went past the bottom")
		assert.InDelta(t, max, b.sc.Offset().Y, 0.01, "and came to rest on it")
	})
}

// TestPagePulledPastItsEndsKeepsTheLayout checks what stands around the scroll while
// it bounces: the tab bar and the page frame keep their places (the pull moves the
// content only), and a Refresh during the pull leaves the new screen on its edge.
func TestPagePulledPastItsEndsKeepsTheLayout(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		b := newBounceRig(t, ph)
		bar := b.n.TabBar().WindowFrame()
		scr := b.sc.WindowFrame()
		b.tt.TouchDown(geom.Pt(200, 400))
		for i := 1; i <= 15; i++ {
			b.tt.Elapse(kittest.Frame)
			b.tt.TouchMove(geom.Pt(200, 400+float32(i)*10))
		}
		require.Less(t, b.sc.Offset().Y, float32(0))
		assert.Equal(t, bar, b.n.TabBar().WindowFrame(), "the tab bar stays")
		assert.Equal(t, scr, b.sc.WindowFrame(), "the scroll's frame stays: the pull is inside it")
		assert.InDelta(t, nav.ContentTop(b.tt.View().Context()), scr.Y, 0.01)

		b.n.Refresh()
		b.tt.Elapse(kittest.Frame)
		sc := b.n.Screen().(nav.Scroller).Scroll()
		assert.InDelta(t, 0, sc.Offset().Y, 0.01, "a screen built again does not keep the pull")
		assert.True(t, sc.Bounce)
		b.tt.TouchUp(geom.Pt(200, 550))
	})
}

// TestPageBounceFrameDoesNotAllocate checks that the frames of a page bouncing
// allocate nothing: a finger pulling it past the top and back, and the spring.
func TestPageBounceFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		b := newBounceRig(t, ph)
		y := float32(400)
		dir := float32(1)
		b.tt.TouchDown(geom.Pt(200, y))
		drag := func() {
			if y > 560 || y < 420 {
				dir = -dir
			}
			y += 4 * dir
			b.tt.TouchMove(geom.Pt(200, y))
			b.tt.Elapse(kittest.Frame)
		}
		for range 40 {
			drag()
		}
		assert.Zero(t, testing.AllocsPerRun(50, drag), "%s: a finger pulling the page past its top", ph.Name)
		b.tt.TouchUp(geom.Pt(200, y))

		// the spring: 20 frames of the 30 it takes, from a pull of 150
		b.sc.ScrollTo(geom.Point{}, false)
		b.tt.TouchDown(geom.Pt(200, 300))
		for i := 1; i <= 15; i++ {
			b.tt.Elapse(kittest.Frame)
			b.tt.TouchMove(geom.Pt(200, 300+float32(i)*10))
		}
		b.tt.Elapse(5 * time.Millisecond) // the finger rests: no speed to go on with
		b.tt.TouchUp(geom.Pt(200, 450))
		frame := func() { b.tt.Elapse(kittest.Frame) }
		frame()
		assert.Zero(t, testing.AllocsPerRun(20, frame), "%s: the spring back", ph.Name)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// newBounceRig mounts a Nav at Home whose page is a block 2000 tall.
func newBounceRig(t testing.TB, ph kittest.Phone) *bounceRig {
	t.Helper()
	b := &bounceRig{body: &block{h: 2000}}
	b.rig = newRigWith(t, ph, func(string) ui.Widget { return b.body })
	b.sc = b.n.Screen().(nav.Scroller).Scroll()

	return b
}

// rubber is UIScrollView's rubber band: how far a pull of x past an end shows,
// along an extent d.
func rubber(x, d float32) float32 { return (1 - 1/(x*bounceResist/d+1)) * d }
