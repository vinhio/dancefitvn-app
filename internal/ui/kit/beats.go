package kit

import (
	"strconv"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of `.beats` (index.html:160-161), the bar of eight beats on
// screen 06.
const (
	// BeatCount is the cells of the bar, `[1,2,3,4,5,6,7,8]` (:426).
	BeatCount = 8
	// beatGap is `.beats{gap:5px}` (:160).
	beatGap float32 = 5
	// beatHeight, beatRadius, beatSize, beatWeight are `.beats b{height:24px;
	// border-radius:6px;font-size:11px;font-weight:800}` (:161).
	beatHeight, beatRadius, beatSize float32 = 24, 6, 11
	beatWeight                               = 800
	// beatBgAlpha, beatInkAlpha are `.beats b{background:rgba(255,255,255,.12);
	// color:rgba(255,255,255,.6)}` (:161); a cell `.on` is --red under #fff.
	beatBgAlpha, beatInkAlpha float32 = .12, .6
)

// nameBeats is the key of the bar: `#mBeats`.
const nameBeats = "div.beats"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Beats is `.beats`: eight equal cells, 24 tall, 5 apart, numbered 1 to 8, one of
// them lit in --red under white text (`.on`). The beat is floor(pos x 2) mod 8
// (K16, 120 BPM): the owner sets the lit cell with SetLit; the same cell repaints
// nothing and nothing allocates.
type Beats struct {
	ui.Base
	// Decor leaves the eight numbers out of a reading: the bar is one element,
	// named Label.
	Decor
	// Label names the bar for assistive technologies ("Thanh đếm nhịp").
	Label string

	lit   int
	cells [BeatCount]Label
	ready bool
}

// NewBeats returns the bar with the first cell lit.
func NewBeats() *Beats {
	b := &Beats{}
	b.Name = nameBeats

	return b
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetLit lights cell i (0 to 7), or none for a value out of range.
func (b *Beats) SetLit(i int) {
	if b.lit != i {
		b.lit = i
		b.Repaint()
	}
}

// Describe describes the bar as one text, named by Label: its cells are not
// read one by one.
func (b *Beats) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label = platform.RoleText, b.Label
}

// Lit returns the cell lit.
func (b *Beats) Lit() int { return b.lit }

// Layout is as wide as it is given and 24 tall; the cells share the width.
func (b *Beats) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !b.ready {
		b.ready = true
		for i := range b.cells {
			l := &b.cells[i]
			l.Text, l.Size, l.Weight = strconv.Itoa(i+1), beatSize, beatWeight
			b.Add(l)
		}
	}
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = cs.Min.W
	}
	size := cs.Constrain(geom.Sz(w, beatHeight))
	cw := b.cellWidth(size.W)
	for i := range b.cells {
		l := &b.cells[i]
		ls := c.Layout(l, ui.Loose(geom.Sz(cw, ui.Unbounded)))
		c.Place(l, geom.Pt(float32(i)*(cw+beatGap)+centeredY(cw, ls.W), centeredY(size.H, ls.H)))
	}

	return size
}

// Paint draws the cells under their numbers, which are its children.
func (b *Beats) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	white := theme.InkWhite.Of(look)
	cw := b.cellWidth(b.Bounds().W)
	for i := range b.cells {
		l := &b.cells[i]
		bg, ink := white.WithAlpha(beatBgAlpha), white.WithAlpha(beatInkAlpha)
		if i == b.lit {
			bg, ink = look.T.Red, white
		}
		l.Color = ink
		p.Fill(geom.XYWH(float32(i)*(cw+beatGap), 0, cw, beatHeight), scene.Solid(bg), beatRadius)
	}
	b.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// cellWidth is the width of one cell in a bar w wide: `flex:1`.
func (b *Beats) cellWidth(w float32) float32 {
	return max(w-(BeatCount-1)*beatGap, 0) / BeatCount
}
