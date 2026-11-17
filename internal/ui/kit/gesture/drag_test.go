// Adapted from jplayer-mobile/internal/ui/gesture/drag_test.go at 49de3c2.

package gesture

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dragPage is a page that is pulled down to dismiss, filling the view.
type dragPage struct {
	page
	d         Drag
	dismissed int
	released  int
	canceled  int
}

func (p *dragPage) InterceptPointer(c *ui.Context, e *ui.PointerEvent) bool {
	at := e.Pos.Add(p.WindowFrame().Min())
	switch e.Kind {
	case ui.PointerDown:
		p.d.PressAt(at, e.Time, c.Now())
	case ui.PointerMove:
		return p.d.Armed() && !p.d.Active() && p.d.MoveAt(at, e.Time, c.Now()) == StepStart
	case ui.PointerUnknown, ui.PointerUp, ui.PointerEnter, ui.PointerLeave, ui.PointerCancel, ui.PointerScroll:
	}

	return false
}

func (p *dragPage) HandlePointer(c *ui.Context, e *ui.PointerEvent) {
	at := e.Pos.Add(p.WindowFrame().Min())
	switch e.Kind {
	case ui.PointerDown:
		// A page that is the target of the press is not asked to intercept
		// it, only to handle it: it notes the press here too.
		p.d.PressAt(at, e.Time, c.Now())
		e.Handle()
	case ui.PointerMove:
		if p.d.Active() {
			p.d.MoveAt(at, e.Time, c.Now())
			e.Handle()
		}
	case ui.PointerUp:
		if p.d.Active() {
			p.released++
			if p.d.ReleaseAt(at, e.Time, c.Now()) {
				p.dismissed++
			}
		}
		e.Handle()
	case ui.PointerCancel:
		if p.d.Active() {
			p.canceled++
			p.d.Cancel()
		}
	case ui.PointerUnknown, ui.PointerEnter, ui.PointerLeave, ui.PointerScroll:
	}
}

func newDragPage(t testing.TB, ph kittest.Phone) (*dragPage, *ui.Tester) {
	t.Helper()
	p := &dragPage{d: SheetDrag()}

	return p, kittest.Mount(t, ph, p)
}

// pull presses at y 100 and pulls dy down in 10 slow steps of 50 ms, and lifts.
func pull(tt *ui.Tester, dy float32) {
	finger(tt, true, geom.Pt(200, 100), geom.Pt(200, 100+dy), 10, 50*time.Millisecond)
	tt.TouchUp(geom.Pt(200, 100+dy))
}

func TestDragDismissDistance(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range []struct {
			dy      float32
			dismiss bool
		}{{79, false}, {80, false}, {81, true}, {40, false}, {300, true}} {
			p, tt := newDragPage(t, ph)
			pull(tt, tc.dy)
			assert.Equal(t, 1, p.released, "dy %v", tc.dy)
			assert.Equal(t, tc.dismiss, p.dismissed == 1, "dy %v", tc.dy)
			assert.False(t, p.d.Active())
			assert.InDelta(t, tc.dy, p.d.Offset(), 1e-2, "it keeps where the page was, to go from")
		}
	})
}

func TestDragDismissFling(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range []struct {
			name    string
			dy      float32 // pulled in 100 ms: speed dy/100 DIPs a millisecond
			dismiss bool
		}{{"0.7 DIP/ms is a fling", 70, true}, {"0.5 DIP/ms is not", 50, false}, {"0.61 DIP/ms is", 61, true}} {
			p, tt := newDragPage(t, ph)
			finger(tt, true, geom.Pt(200, 100), geom.Pt(200, 100+tc.dy), 10, 10*time.Millisecond)
			tt.TouchUp(geom.Pt(200, 100+tc.dy))
			assert.Equal(t, tc.dismiss, p.dismissed == 1, tc.name)
		}
	})
}

func TestDragRestedFingerDoesNotFling(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p, tt := newDragPage(t, ph)
		// 70 DIPs in 100 ms (0.7 DIP/ms), then the finger rests 300 ms before it lifts.
		finger(tt, true, geom.Pt(200, 100), geom.Pt(200, 170), 10, 10*time.Millisecond)
		tt.Elapse(300 * time.Millisecond)
		tt.TouchUp(geom.Pt(200, 170))
		assert.Equal(t, 1, p.released)
		assert.Zero(t, p.dismissed)
	})
}

func TestDragPullUpHolds(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p, tt := newDragPage(t, ph)
		finger(tt, true, geom.Pt(200, 300), geom.Pt(200, 260), 10, 50*time.Millisecond)
		assert.True(t, p.d.Active())
		assert.Zero(t, p.d.Offset(), "the sheet does not leave its place upward")
		tt.TouchUp(geom.Pt(200, 260))
		assert.Zero(t, p.dismissed)
	})
}

func TestDragSidewaysFirstIsNotADrag(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p, tt := newDragPage(t, ph)
		finger(tt, true, geom.Pt(200, 300), geom.Pt(240, 306), 4, 20*time.Millisecond)
		finger(tt, false, geom.Pt(240, 306), geom.Pt(240, 700), 8, 20*time.Millisecond)
		tt.TouchUp(geom.Pt(240, 700))
		assert.False(t, p.d.Active())
		assert.Zero(t, p.d.Offset())
		assert.Zero(t, p.released)
	})
}

func TestDragCancel(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p, tt := newDragPage(t, ph)
		finger(tt, true, geom.Pt(200, 100), geom.Pt(200, 200), 8, 30*time.Millisecond)
		require.True(t, p.d.Active())
		cancelTouch(tt)
		assert.Equal(t, 1, p.canceled)
		assert.False(t, p.d.Active())
		assert.Zero(t, p.dismissed)
	})
}

func TestDragNumbers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Drag{Distance: 80, Fling: .6, Resist: 0}, SheetDrag())
	assert.Equal(t, Edge{Strip: 26, Distance: 80, Fling: .6}, EdgeBack())
}

func TestDragMoveDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		p, tt := newDragPage(t, ph)
		finger(tt, true, geom.Pt(200, 100), geom.Pt(200, 200), 8, 30*time.Millisecond)
		require.True(t, p.d.Active())
		y, dir := float32(200), float32(1)
		assert.Zero(t, testing.AllocsPerRun(50, func() {
			if y > 400 || y < 120 {
				dir = -dir
			}
			y += 6 * dir
			tt.Elapse(frame)
			tt.TouchMove(geom.Pt(200, y))
		}), "%s: a move of the finger", ph.Name)
		tt.TouchUp(geom.Pt(200, y))
	}
}

func BenchmarkDragMove(b *testing.B) {
	_, tt := newDragPage(b, kittest.Phones[0])
	finger(tt, true, geom.Pt(200, 100), geom.Pt(200, 200), 8, 30*time.Millisecond)
	y, dir := float32(200), float32(1)
	b.ReportAllocs()
	for b.Loop() {
		if y > 400 || y < 120 {
			dir = -dir
		}
		y += 6 * dir
		tt.Elapse(frame)
		tt.TouchMove(geom.Pt(200, y))
	}
}
