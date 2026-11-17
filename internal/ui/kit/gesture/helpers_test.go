package gesture

import (
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// frame is one 60 Hz refresh.
const frame = kittest.Frame

// page is a widget that fills the view and is handed to a test as the root.
type page struct {
	ui.Base
}

// Layout fills the room.
func (p *page) Layout(_ *ui.Context, cs ui.Constraints) geom.Size { return cs.Max }

// Paint draws nothing.
func (p *page) Paint(*ui.Context, *ui.Painter) {}

// finger moves the finger from `from` to `to` in n equal steps, `every` apart
// (TouchDown first when down), and leaves it there.
func finger(tt *ui.Tester, down bool, from, to geom.Point, n int, every time.Duration) {
	if down {
		tt.TouchDown(from)
	}
	for k := 1; k <= n; k++ {
		f := float32(k) / float32(n)
		tt.Elapse(every)
		tt.TouchMove(geom.Pt(from.X+(to.X-from.X)*f, from.Y+(to.Y-from.Y)*f))
	}
}

// cancelTouch sends the system's cancel of the finger TouchDown put down.
func cancelTouch(tt *ui.Tester) {
	tt.View().HandleEvent(platform.Event{Kind: platform.EventPointerCancel, Pointer: platform.PointerTouch, PointerID: 31})
	tt.Elapse(frame)
}
