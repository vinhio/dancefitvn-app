//go:build tvos

package tvkit

import (
	"time"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The focus look of `.f.focus` (reference/prototype/index.html:179-181).
const (
	// FocusScale is `#tv .f.focus{transform:scale(1.06)}` (:181).
	FocusScale float32 = 1.06
	// NavFocusScale is `#tv .tnav .f.focus{transform:scale(1.04)}` (:185).
	NavFocusScale float32 = 1.04
	// RowFocusScale is `#tv .trow.focus{transform:scale(1.02)}` (:193).
	RowFocusScale float32 = 1.02
	// Transition is `#tv .f{transition:transform .15s,box-shadow .15s}` (:180).
	// The rule names no timing function: CSS `ease`. The fill and the text
	// color are not in the list and switch at once.
	Transition = 150 * time.Millisecond
	// RingWidth is the first shadow of `#tv .f.focus`, `0 0 0 6px var(--pr)` (:181).
	RingWidth float32 = 6
	// DropY, DropBlur and DropAlpha are its second, `0 26px 50px rgba(0,0,0,.6)`
	// (:181).
	DropY, DropBlur, DropAlpha float32 = 26, 50, .6
)

// PrototypeInlineFocus reproduces the five places where the prototype's inline
// styles beat the focus CSS (F-9 of local/plans/P7.md, K26 proposed): a row
// that keeps its inline fill or ink while focused. true is the prototype; the
// user may flip it to focus fill and ink everywhere (Q-P7-3).
var PrototypeInlineFocus = true

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// FocusShadow paints the box-shadow of a focused `.f` for the transition
// progress blend (0 at rest, 1 focused): CSS interpolates a shadow list from
// `none` component by component, so the ring grows from 0 to 6 px while its
// color fades in, and the drop shadow's offset, blur and alpha grow from 0.
// The drop shadow is the second of the list and is painted first; the ring,
// the first, over it. A ring is skipped when ringColor is zero (`.tbtn` has
// the drop shadow only, :182).
func FocusShadow(p *ui.Painter, r geom.Rect, radius, blend float32, ringColor color.Color) {
	if blend <= 0 {
		return
	}
	drop := color.RGBA(0, 0, 0, DropAlpha*blend)
	p.Shadow(r, radius, ui.Shadow{Offset: geom.Pt(0, DropY*blend), Blur: DropBlur * blend, Color: drop})
	if ringColor.A != 0 {
		ringColor.A = uint8(float32(ringColor.A) * blend)
		p.Shadow(r, radius, ui.Shadow{Spread: RingWidth * blend, Color: ringColor})
	}
}
