package kit

import (
	"math"
	"sync/atomic"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// SnapBox returns r, in the painter's current widget, with each edge moved to
// the whole-DIP grid of the window, as the prototype's captures show it: Chrome
// pixel-snaps the boxes of its layout at 1x (the `button.btn.p` of M02 at
// y 524.6 is painted from y 525 to 579) and the 3x picture scales that. Left and
// right, top and bottom are snapped on their own, so a box keeps the pixel size
// the browser gives it (P3.7, local/plans/P3-notes-from-P1.md Q-P3.2-1).
func SnapBox(p *ui.Painter, r geom.Rect) geom.Rect {
	off := p.Offset()
	x0 := snapAxis(off.X+r.X, iconSnapUnit)
	y0 := snapAxis(off.Y+r.Y, iconSnapUnit)
	x1 := snapAxis(off.X+r.X+r.W, iconSnapUnit)
	y1 := snapAxis(off.Y+r.Y+r.H, iconSnapUnit)

	return geom.Rect{X: x0 - off.X, Y: y0 - off.Y, W: x1 - x0, H: y1 - y0}
}

// DefaultTextShift is the DIPs the text origin moves left of where layout puts it.
// Measured at 3x on M02, M03 and M04 against the prototype: the glyphs of the
// app stand 1 to 2 device pixels right of Chrome's for the same box (ink of
// "Dùng tài khoản…" from x 64 against 62, the "SET 35" of a poster 2 px), and
// moving them 0.5 DIP left cut the whole-screen share off by more than 32/255
// from 1.94 % to 1.36 % (M02) and made 15 of the 21 failing text crops of M03
// match (0.33 and 0.67 do worse). The cause is the glyph rasteriser's
// subpixel origin, not the layout, so layout boxes are not touched.
const DefaultTextShift float32 = -0.5

// textShiftBits holds the shift in use as float32 bits: the TV's shell sets it
// once when it mounts, and tests that mount screens in parallel may set it at the
// same time as others paint, so it is atomic. A load costs nothing and allocates
// nothing.
var textShiftBits atomic.Uint32

func init() { textShiftBits.Store(math.Float32bits(DefaultTextShift)) }

// SetTextShift sets the DIPs the text origin moves left of where layout puts it,
// for the windows that follow. The phones keep DefaultTextShift, measured at 3x;
// the TV sets its own once, from its scale (P7.7, local/shots/app-report-tvos.md).
func SetTextShift(x float32) { textShiftBits.Store(math.Float32bits(x)) }

// TextShift returns the shift in use.
func TextShift() float32 { return math.Float32frombits(textShiftBits.Load()) }

// textShiftX is the shift in use.
func textShiftX() float32 { return TextShift() }
