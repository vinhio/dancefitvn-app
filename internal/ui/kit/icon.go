package kit

import (
	"math"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// defaultIconSize is `svg.i{width:22px;height:22px}` (index.html:64) and the
// default of `ic(n,s=22,sw)` (index.html:239).
const defaultIconSize float32 = 22

// iconName is the key an icon has in the prototype's layout.json: tag and
// class, `svg.i`.
const iconName = "svg.i"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Icon is one of the prototype's icons (`ic(n,s,sw)`): a square of Size DIPs
// holding the glyph, drawn in its Ink (`stroke:currentColor`, so the color of
// the text around it) with a stroke of Stroke units of the 24-unit box.
type Icon struct {
	ui.Base
	// Glyph is the icon drawn; icon.NameUnknown draws nothing.
	Glyph icon.Name
	// Size is the width and height, in DIPs; 0 reads as 22.
	Size float32
	// Stroke is the stroke width in the 24-unit box: `ic(n,s,sw)`'s sw. 0 is
	// 1.9 (`svg.i`), or the glyph's own.
	Stroke float32
	// Ink is the color the glyph is drawn in; InkUnknown reads as --tx.
	Ink theme.Ink
	// Color, when not zero, is used in place of Ink.
	Color color.Color
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewIcon returns the icon n of size DIPs (0: 22), named `svg.i` as the
// prototype's layout.json names it.
func NewIcon(n icon.Name, size float32, ink theme.Ink) *Icon {
	i := &Icon{Glyph: n, Size: size, Ink: ink}
	i.Name = iconName

	return i
}

// SetGlyph changes the icon drawn; the same one changes nothing.
func (i *Icon) SetGlyph(n icon.Name) {
	if n != i.Glyph {
		i.Glyph = n
		i.Repaint()
	}
}

// Layout makes the icon a square of its size.
func (i *Icon) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	s := i.Size
	if s <= 0 {
		s = defaultIconSize
	}

	return cs.Constrain(geom.Sz(s, s))
}

// Paint draws the glyph fitted in the icon's box.
func (i *Icon) Paint(c *ui.Context, p *ui.Painter) {
	col := i.Color
	if col.A == 0 {
		col = i.Ink.Of(theme.From(c))
	}
	b := i.Bounds()
	icon.Draw(p, i.Glyph, snapOrigin(p, geom.XYWH(0, 0, b.W, b.H)), col, i.Stroke)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// iconSnapUnit is the grid, in DIPs, that the origin of an icon is snapped to:
// whole DIPs (CSS px). Chrome snaps the box of a replaced element (an `<svg>`) to
// whole CSS px of its layout, not to device pixels, whatever the pixel ratio of
// the capture: measured on the dpr 3 shots of M02, the eye (y 428.59) and the
// mail icon (y 338.59) stand at device y 1287 and 1017, 1 pixel under the device
// grid's 1286 and 1016, and shifting the app's icon by that pixel makes them
// identical (0.00 % of pixels off; P3.2 pre-step). Snapping to round(v x scale) /
// scale left 75 of the 105 icon crops of the kit's 3x gate off; this snap, 32.
const iconSnapUnit float32 = 1

// iconFloorSlack is the DIPs added to x before it is floored, so that a position
// that is whole up to float noise (35 - 1e-5) stays on its pixel.
const iconFloorSlack float32 = 0.01

// snapOrigin moves the origin of r, in the painter's current widget, to the
// grid of iconSnapUnit and keeps its size (P2.8 fix round,
// local/plans/P3-notes-from-P1.md): a glyph left at y 338.59 is smeared over two
// pixel rows. The snap is round(v / unit) x unit of the position in the window,
// taken back to the widget's own coordinates.
func snapOrigin(p *ui.Painter, r geom.Rect) geom.Rect {
	off := p.Offset()
	// x is floored, y rounded: measured on M02, M03 and M04 at 3x, the icons at
	// x 38.13, 136.38, 234.63, 332.88 and 60.69 stand at 38, 136, 234, 332 and 60,
	// those at y 338.59 and 650.59 at 339 and 651.
	r.X = float32(math.Floor(float64(off.X+r.X+iconFloorSlack))) - off.X
	r.Y = snapAxis(off.Y+r.Y, iconSnapUnit) - off.Y

	return r
}

// snapAxis rounds v, in DIPs, to the nearest multiple of unit.
func snapAxis(v, unit float32) float32 { return float32(math.Round(float64(v/unit))) * unit }
