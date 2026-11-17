package gesture

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
)

// edgePage is a page whose left edge is a swipe-back strip.
type edgePage struct {
	page
	e      Edge
	backs  int
	taken  int
	cancel int
}

func (p *edgePage) InterceptPointer(c *ui.Context, ev *ui.PointerEvent) bool {
	switch ev.Kind {
	case ui.PointerDown:
		p.e.Press(ev.Pos, p.WindowFrame().X, ev.Time, c.Now())
	case ui.PointerMove:
		if !p.e.Armed() || p.e.Taken() {
			return false
		}
		if p.e.MoveAt(ev.Pos, ev.Time, c.Now()) == StepStart {
			p.taken++

			return true
		}
	case ui.PointerUnknown, ui.PointerUp, ui.PointerEnter, ui.PointerLeave, ui.PointerCancel, ui.PointerScroll:
	}

	return false
}

func (p *edgePage) HandlePointer(c *ui.Context, ev *ui.PointerEvent) {
	switch ev.Kind {
	case ui.PointerDown:
		// A page that is the target of the press is not asked to intercept it,
		// only to handle it: it notes the press here too.
		if p.e.Press(ev.Pos, p.WindowFrame().X, ev.Time, c.Now()) {
			ev.Handle()
		}
	case ui.PointerMove:
		if p.e.Taken() {
			ev.Handle()
			p.e.MoveAt(ev.Pos, ev.Time, c.Now())
		}
	case ui.PointerUp:
		if p.e.Taken() {
			ev.Handle()
			if p.e.ReleaseAt(ev.Pos, ev.Time, c.Now()) {
				p.backs++
			}
		}
		p.e.Cancel()
	case ui.PointerCancel:
		if p.e.Taken() {
			p.cancel++
		}
		p.e.Cancel()
	case ui.PointerUnknown, ui.PointerEnter, ui.PointerLeave, ui.PointerScroll:
	}
}

func newEdgePage(t testing.TB, ph kittest.Phone) (*edgePage, *ui.Tester) {
	t.Helper()
	p := &edgePage{e: EdgeBack()}

	return p, kittest.Mount(t, ph, p)
}

// swipe presses at (x, 300), goes dx right in 10 steps of every, and lifts.
func swipeRight(tt *ui.Tester, x, dx float32, every time.Duration) {
	finger(tt, true, geom.Pt(x, 300), geom.Pt(x+dx, 300), 10, every)
	tt.TouchUp(geom.Pt(x+dx, 300))
}

func TestEdgeStripWidth(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range []struct {
			x    float32
			back bool
		}{{0, true}, {25.5, true}, {26, false}, {40, false}} {
			p, tt := newEdgePage(t, ph)
			swipeRight(tt, tc.x, 200, 50*time.Millisecond)
			assert.Equal(t, tc.back, p.backs == 1, "press at x %v", tc.x)
		}
	})
}

func TestEdgeCommitDistance(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range []struct {
			dx   float32
			back bool
		}{{79, false}, {80, true}, {81, true}, {30, false}} {
			p, tt := newEdgePage(t, ph)
			swipeRight(tt, 10, tc.dx, 50*time.Millisecond) // slow: 0.16 DIP/ms
			assert.Equal(t, tc.back, p.backs == 1, "dx %v", tc.dx)
		}
	})
}

func TestEdgeCommitFling(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range []struct {
			name string
			dx   float32 // in 100 ms
			back bool
		}{{"0.7 DIP/ms", 70, true}, {"0.5 DIP/ms", 50, false}} {
			p, tt := newEdgePage(t, ph)
			swipeRight(tt, 10, tc.dx, 10*time.Millisecond)
			assert.Equal(t, tc.back, p.backs == 1, tc.name)
		}
		// a finger that rests before it lifts does not fling
		p, tt := newEdgePage(t, ph)
		finger(tt, true, geom.Pt(10, 300), geom.Pt(80, 300), 10, 10*time.Millisecond)
		tt.Elapse(300 * time.Millisecond)
		tt.TouchUp(geom.Pt(80, 300))
		assert.Zero(t, p.backs)
	})
}

func TestEdgeRefuses(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p, tt := newEdgePage(t, ph)
		// vertical first: the press is a scroll's
		finger(tt, true, geom.Pt(10, 300), geom.Pt(14, 380), 6, 20*time.Millisecond)
		finger(tt, false, geom.Pt(14, 380), geom.Pt(200, 380), 6, 20*time.Millisecond)
		tt.TouchUp(geom.Pt(200, 380))
		assert.Zero(t, p.taken)
		assert.Zero(t, p.backs)

		// leftward from the strip is not a swipe back
		p, tt = newEdgePage(t, ph)
		finger(tt, true, geom.Pt(20, 300), geom.Pt(0, 300), 6, 20*time.Millisecond)
		tt.TouchUp(geom.Pt(0, 300))
		assert.Zero(t, p.taken)
		assert.Zero(t, p.backs)
	})
}

func TestEdgeCancel(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p, tt := newEdgePage(t, ph)
		finger(tt, true, geom.Pt(10, 300), geom.Pt(120, 300), 8, 30*time.Millisecond)
		assert.True(t, p.e.Taken())
		cancelTouch(tt)
		assert.Equal(t, 1, p.cancel)
		assert.False(t, p.e.Armed())
		assert.Zero(t, p.backs)
	})
}

func TestEdgeMoveDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		p, tt := newEdgePage(t, ph)
		finger(tt, true, geom.Pt(10, 300), geom.Pt(60, 300), 6, 30*time.Millisecond)
		assert.True(t, p.e.Taken())
		x, dir := float32(60), float32(1)
		assert.Zero(t, testing.AllocsPerRun(50, func() {
			if x > 300 || x < 30 {
				dir = -dir
			}
			x += 6 * dir
			tt.Elapse(frame)
			tt.TouchMove(geom.Pt(x, 300))
		}), "%s: a move of the finger", ph.Name)
		tt.TouchUp(geom.Pt(x, 300))
	}
}
