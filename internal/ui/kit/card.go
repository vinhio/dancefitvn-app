package kit

import (
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of .card (index.html:86); its radius, 16, is cardRadius of coursecard.go.
const (
	// cardBorder is the `border:1px solid var(--bd)` of the locked course's card
	// (index.html:406).
	cardBorder float32 = 1
)

// nameCard is the key layout.json gives the card: tag and class.
const nameCard = "div.card"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Card is `.card` (index.html:86): a 16 rounded box of --sf holding one child,
// padded, as wide as the room it gets (a block, and so is its child) and as tall as its content
// unless Height says. With Border it has the 1 px --bd border of the locked
// course's card (:406), inside the box and before the padding, as CSS lays a
// border-box out.
//
// Its Name is `div.card` unless the caller sets another (`go.detail:set35` for
// a card that is a button).
type Card struct {
	ui.Base
	// Padding is `padding`, inside the border.
	Padding geom.Insets
	// Border draws the 1 px --bd border.
	Border bool
	// Height is the box's height; 0 is the content's plus the padding.
	Height float32

	content ui.Widget
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewCard returns a card around content; nil makes an empty one, of Height.
func NewCard(content ui.Widget) *Card {
	c := &Card{content: content}
	c.Name = nameCard
	if content != nil {
		c.Add(content)
	}

	return c
}

// inset returns the room taken on each side by the border and the padding.
func (c *Card) inset() geom.Insets {
	b := float32(0)
	if c.Border {
		b = cardBorder
	}

	return geom.Insets{
		Top: c.Padding.Top + b, Right: c.Padding.Right + b, Bottom: c.Padding.Bottom + b, Left: c.Padding.Left + b,
	}
}

// Layout takes the room's width (the content's when there is none) and the
// height asked for, or the content's with the padding.
func (c *Card) Layout(ctx *ui.Context, cs ui.Constraints) geom.Size {
	in := c.inset()
	w := cs.Max.W
	var content geom.Size
	if c.content != nil {
		room, least := ui.Unbounded, float32(0)
		if w < ui.Unbounded {
			room = max(w-in.Left-in.Right, 0)
			least = room // the content is a block: as wide as the room
		}
		content = ctx.Layout(c.content, ui.Constraints{Min: geom.Sz(least, 0), Max: geom.Sz(room, ui.Unbounded)})
		ctx.Place(c.content, geom.Pt(in.Left, in.Top))
	}
	if w >= ui.Unbounded {
		w = content.W + in.Left + in.Right
	}
	h := c.Height
	if h <= 0 {
		h = content.H + in.Top + in.Bottom
	}

	return cs.Constrain(geom.Sz(w, h))
}

// Paint draws the box and its border under the content.
func (c *Card) Paint(ctx *ui.Context, p *ui.Painter) {
	t := &theme.From(ctx).T
	r := SnapBox(p, c.Bounds()) // `.card`: whole-CSS-px box (Chrome)
	radius := capRadius(r, cardRadius)
	p.Fill(r, scene.Solid(t.Surface), radius)
	if c.Border {
		p.Border(r, geom.Uniform(cardBorder), t.Border, radius)
	}
	c.Base.Paint(ctx, p)
}

// LayoutStyle is `div.card` (index.html:86): --sf, 16 rounded, the border when
// there is one.
func (c *Card) LayoutStyle(ctx *ui.Context) (CSS, string) {
	t := &theme.From(ctx).T
	width := float32(0)
	if c.Border {
		width = cardBorder
	}

	return boxCSS(t.Surface, cssPx(cardRadius), width, t.Border), ""
}
