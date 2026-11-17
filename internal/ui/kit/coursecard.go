// Written from courseCardH (index.html:371); the pressable card's shape (a
// Describe naming the card, the clip to its radius) follows kit/card.go of
// jplayer-mobile at 49de3c2, built on ui.Pressable instead of its own pointer
// handling.

package kit

import (
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// CardCategory picks the color of the category line of a course card: the
// ternary of courseCardH (index.html:371).
type CardCategory uint8

// The categories. CardCategoryUnknown reads as CardRegular.
const (
	CardCategoryUnknown CardCategory = iota
	// CardRegular is "Thường": #FF6B6E.
	CardRegular
	// CardSpecial is "Đặc biệt": var(--info).
	CardSpecial
	// CardIntro is "Giới thiệu": var(--ok).
	CardIntro
)

// The numbers of courseCardH (index.html:371).
const (
	// cardWidth, cardRadius are `width:156px` and `.card{border-radius:16px}`.
	cardWidth, cardRadius float32 = 156, 16
	// cardPosterH is the poster's height, `poster(id,156,100,{ts:20})`.
	cardPosterH float32 = 100
	// cardPadTop, cardPadSide, cardPadBottom are `padding:10px 12px 12px`.
	cardPadTop, cardPadSide, cardPadBottom float32 = 10, 12, 12
	// cardNameSize is `<b style="font-size:14px">`, the bold 700.
	cardNameSize float32 = 14
	// cardQuoteSize, cardQuoteTop, cardQuoteBottom are `.muted` at 12 px with
	// `margin:3px 0 6px`.
	cardQuoteSize, cardQuoteTop, cardQuoteBottom float32 = 12, 3, 6
	// cardCatSize, cardCatSpacing are `font-size:10px;font-weight:800;
	// letter-spacing:.8px`.
	cardCatSize, cardCatSpacing float32 = 10, .8
)

// The names layout.json gives the parts of a course card.
const (
	// nameCardName is empty: the title is a bare `<b>` the capture does not record
	// (M03 names only the poster's `b`).
	nameCardName  = ""
	nameCardQuote = "div.muted"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// CourseCard is the card of a course in a horizontal strip (courseCardH,
// index.html:371): 156 wide, --sf, 16 rounded and clipped, a 156 x 100 poster on
// top, then, padded 10 12 12, the name (14/700, one line with an ellipsis), the
// quote (12, --tx2, one line with an ellipsis, margin 3 0 6; a blank line when
// there is none) and the category line (10/800, .8 px spacing, upper case, in the
// category's color), 183 tall in all.
//
// The poster is the caller's: the poster package imports this one, so the card
// takes the widget (`poster.New(poster.ForCourse(c), 156, 100)`) and lays it out
// 156 x 100. Set the Name to the data key (`go.detail:set35`).
type CourseCard struct {
	ui.Pressable
	// Poster is the cover, laid out 156 x 100 at the top.
	Poster ui.Widget
	// Name is the course's name, Quote its tagline without the curly quotes
	// (empty for a blank line), Tag the category line, "Thường · 10 bài".
	Name, Quote, Tag string
	// Category colors the category line.
	Category CardCategory
	// Width is the card's width; 0 reads as 156.
	Width float32

	name, quote, tag Label
	strut            theme.LineBox
	poster           ui.Widget // the Poster that is a child
	attached         bool
}

var (
	_ ui.Describer = (*CourseCard)(nil)
	_ ui.Labeler   = (*CourseCard)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AccessibleLabel returns the course's name.
func (cc *CourseCard) AccessibleLabel() string { return cc.Name }

// Describe describes the card as a button named by the course, valued by its
// category line.
func (cc *CourseCard) Describe(c *ui.Context, n *platform.SemanticNode) {
	cc.Pressable.Describe(c, n)
	n.Value = cc.Tag
}

// Layout makes the card as wide as set and as tall as its parts.
func (cc *CourseCard) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	cc.init()
	w := cc.Width
	if w <= 0 {
		w = cardWidth
	}
	cc.syncParts(theme.From(c))
	inner := max(w-2*cardPadSide, 0)
	y := float32(0)
	if cc.poster != nil {
		c.Layout(cc.poster, ui.Tight(geom.Sz(w, cardPosterH)))
		c.Place(cc.poster, geom.Point{})
		y = cardPosterH
	}
	y += cardPadTop
	loose := ui.Loose(geom.Sz(inner, ui.Unbounded))
	cc.strut = strut(c)
	ns := c.Layout(&cc.name, ui.Constraints{Min: geom.Sz(inner, 0), Max: geom.Sz(inner, ui.Unbounded)})
	c.Place(&cc.name, geom.Pt(cardPadSide, y))
	y += ns.H + cardQuoteTop
	qs := c.Layout(&cc.quote, ui.Constraints{Min: geom.Sz(inner, 0), Max: geom.Sz(inner, ui.Unbounded)})
	c.Place(&cc.quote, geom.Pt(cardPadSide, y))
	y += qs.H + cardQuoteBottom
	c.Layout(&cc.tag, loose)
	// the category is an inline span: its baseline sits on the strut's
	c.Place(&cc.tag, geom.Pt(cardPadSide, y+cc.strut.Baseline-cc.tag.Baseline()))
	y += cc.strut.Height + cardPadBottom

	return cs.Constrain(geom.Sz(w, y))
}

// Paint draws the card's surface and its parts, clipped to the radius.
func (cc *CourseCard) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	b := SnapBox(p, cc.Bounds()) // `.card`: whole-CSS-px box (Chrome)
	p.Fill(b, scene.Solid(look.T.Surface), cardRadius)
	if cc.FocusVisible() {
		focusRing(p, b, cardRadius, look.T.Primary)
	}
	p.Clip(b, cardRadius, func() { cc.Base.Paint(c, p) })
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// init adds the parts the first time and names them as layout.json does.
func (cc *CourseCard) init() {
	if cc.attached {
		return
	}
	cc.attached = true
	cc.Look = noLook
	cc.name.Name, cc.quote.Name = nameCardName, nameCardQuote
	labelStyle{Role: theme.RoleBody, Size: cardNameSize, Weight: 700, Ink: theme.InkText, Ellipsis: true}.apply(&cc.name)
	labelStyle{Role: theme.RoleBody, Size: cardQuoteSize, Ink: theme.InkText2, Ellipsis: true}.apply(&cc.quote)
	cc.Add(&cc.name, &cc.quote, &cc.tag)
}

// syncParts makes the parts match the fields: the poster, the texts, the color of
// the category line (var(--info), var(--ok) or #FF6B6E: the Colors of the look).
func (cc *CourseCard) syncParts(look *theme.Look) {
	if cc.poster != cc.Poster {
		if cc.poster != nil {
			cc.Remove(cc.poster)
		}
		cc.Remove(&cc.name)
		cc.Remove(&cc.quote)
		cc.Remove(&cc.tag)
		if cc.poster = cc.Poster; cc.poster != nil {
			cc.Add(cc.poster)
		}
		cc.Add(&cc.name, &cc.quote, &cc.tag) // the poster stays first in the tree
	}
	cc.name.SetText(cc.Name)
	cc.quote.SetText(cc.Quote)
	col := look.C.CatRegular
	switch cc.Category {
	case CardSpecial:
		col = look.C.CatSpecial
	case CardIntro:
		col = look.C.CatIntro
	case CardRegular, CardCategoryUnknown:
	default:
	}
	labelStyle{Role: theme.RoleBody, Size: cardCatSize, Weight: 800, Spacing: cardCatSpacing, Upper: true}.apply(&cc.tag)
	cc.tag.Color = col
	cc.tag.SetText(cc.Tag)
}
