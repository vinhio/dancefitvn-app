//go:build tvos

package tvkit

import (
	"strconv"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the player's beat bar (index.html:684, `#tBeats`): `.beats`
// with `style="gap:10px;margin-bottom:30px"` and cells of `style="height:64px;
// font-size:28px;border-radius:12px"`; the cell's color and fill are `.beats b`
// (:161-162).
const (
	// BeatCells is the eight cells, `[1,2,3,4,5,6,7,8]`.
	BeatCells = kit.BeatCount
	// BeatsHeight is the cells' height.
	BeatsHeight float32 = 64
	// beatsGap, beatsRadius, beatsSize are `gap:10px`, `border-radius:12px` and
	// `font-size:28px`.
	beatsGap, beatsRadius, beatsSize float32 = 10, 12, 28
	// beatsWeight is `.beats b{font-weight:800}`.
	beatsWeight = 800
	// beatsBgAlpha, beatsInkAlpha are `rgba(255,255,255,.12)` and
	// `rgba(255,255,255,.6)`.
	beatsBgAlpha, beatsInkAlpha float32 = .12, .6
)

// nameBeats is the key layout.json gives the bar: `div.beats`.
const nameBeats = "div.beats"

// Beats is the TV's `.beats`: eight cells, 64 tall and 10 apart, numbered 1 to 8,
// one lit in --red under white text. The beat is floor(pos x 2) mod 8: the
// owner sets the lit cell with SetLit; the same cell repaints nothing and
// nothing allocates.
type Beats struct {
	ui.Base
	// Label names the bar for a screen reader ("Thanh đếm nhịp"): the bar is one
	// element and its eight numbers are not read.
	Label string

	lit   int
	cells [BeatCells]kit.Label
	ready bool
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

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

// Describe describes the bar as one text named by Label; its cells are the
// numbers of the beat, which would be eight elements to swipe through.
func (b *Beats) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label = platform.RoleText, b.Label
}

// Lit returns the cell lit.
func (b *Beats) Lit() int { return b.lit }

// Layout is as wide as it is given and 64 tall; the cells share the width.
func (b *Beats) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !b.ready {
		b.ready = true
		for i := range b.cells {
			l := &b.cells[i]
			l.Text, l.Role, l.Size, l.Weight = strconv.Itoa(i+1), theme.RoleBody, beatsSize, beatsWeight
			b.Add(l)
		}
	}
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = cs.Min.W
	}
	size := cs.Constrain(geom.Sz(w, BeatsHeight))
	cw := b.cellWidth(size.W)
	for i := range b.cells {
		l := &b.cells[i]
		ls := c.Layout(l, ui.Loose(geom.Sz(cw, ui.Unbounded)))
		c.Place(l, geom.Pt(float32(i)*(cw+beatsGap)+(cw-ls.W)/2, (size.H-ls.H)/2))
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
		bg, ink := white.WithAlpha(beatsBgAlpha), white.WithAlpha(beatsInkAlpha)
		if i == b.lit {
			bg, ink = look.T.Red, white
		}
		l.Color = ink
		p.Fill(geom.XYWH(float32(i)*(cw+beatsGap), 0, cw, BeatsHeight), scene.Solid(bg), beatsRadius)
	}
	b.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// cellWidth is the width of one cell in a bar w wide: `flex:1`.
func (b *Beats) cellWidth(w float32) float32 {
	return max(w-(BeatCells-1)*beatsGap, 0) / BeatCells
}
