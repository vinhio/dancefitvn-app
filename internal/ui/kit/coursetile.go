// The tile of a course in the two-column grid of Home (K39). It takes the parts of
// courseCardH (index.html:371; CourseCard) and changes what the owner decided: the
// poster scales to the column keeping the 156:100 aspect, the name may take two
// lines, and the tagline is left out.

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

// The numbers of the tile; the rest are courseCardH's (coursecard.go).
const (
	// tilePosterW, tilePosterH make the poster's 156:100 aspect
	// (`poster(id,156,100,{ts:20})`).
	tilePosterW, tilePosterH float32 = 156, 100
	// tileNameLines is the most lines of the name (K39: 1 to 2, then "…").
	tileNameLines = 2
	// tileMetaTop is the space between the name and the category line: the 6 of
	// `.muted{margin:3px 0 6px}` that was under the quote.
	tileMetaTop float32 = 6
)

// CourseTile is a course in a grid column (K39): --sf, 16 rounded and clipped, the
// poster as wide as the tile at the 156:100 aspect, then, padded 10 12 12, the name
// (14/700, up to two lines, "…") and the category line (10/800, .8 px spacing, upper
// case, in the category's color). A tile asked for a minimum height (the other tile
// of its row is taller) takes it and keeps its parts at the top.
//
// The poster is the caller's, as CourseCard's. Set the Name to the data key
// (`go.detail:set35`).
type CourseTile struct {
	ui.Pressable
	// Poster is the cover; it is laid out as wide as the tile.
	Poster ui.Widget
	// Name is the course's name, Tag the category line, "Thường · 10 bài".
	Name, Tag string
	// Category colors the category line.
	Category CardCategory

	name, tag Label
	strut     theme.LineBox
	poster    ui.Widget // the Poster that is a child
	attached  bool
}

var (
	_ ui.Describer = (*CourseTile)(nil)
	_ ui.Labeler   = (*CourseTile)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// PosterHeight returns the height of a poster w wide: w x 100 / 156.
func PosterHeight(w float32) float32 { return w * tilePosterH / tilePosterW }

// AccessibleLabel returns the course's name.
func (ct *CourseTile) AccessibleLabel() string { return ct.Name }

// Describe describes the tile as a button named by the course, valued by its
// category line.
func (ct *CourseTile) Describe(c *ui.Context, n *platform.SemanticNode) {
	ct.Pressable.Describe(c, n)
	n.Value = ct.Tag
}

// Layout makes the tile as wide as the room's maximum and as tall as its parts, or
// as the minimum height if that is more.
func (ct *CourseTile) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	ct.init()
	w := cs.Max.W
	ct.syncParts(theme.From(c))
	inner := max(w-2*cardPadSide, 0)
	y := float32(0)
	if ct.poster != nil {
		ph := PosterHeight(w)
		c.Layout(ct.poster, ui.Tight(geom.Sz(w, ph)))
		c.Place(ct.poster, geom.Point{})
		y = ph
	}
	y += cardPadTop
	ct.strut = strut(c)
	ns := c.Layout(&ct.name, ui.Constraints{Min: geom.Sz(inner, 0), Max: geom.Sz(inner, ui.Unbounded)})
	c.Place(&ct.name, geom.Pt(cardPadSide, y))
	y += ns.H + tileMetaTop
	c.Layout(&ct.tag, ui.Loose(geom.Sz(inner, ui.Unbounded)))
	// the category is an inline span: its baseline sits on the strut's
	c.Place(&ct.tag, geom.Pt(cardPadSide, y+ct.strut.Baseline-ct.tag.Baseline()))
	y += ct.strut.Height + cardPadBottom

	return cs.Constrain(geom.Sz(w, max(y, cs.Min.H)))
}

// Paint draws the tile's surface and its parts, clipped to the radius.
func (ct *CourseTile) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	b := SnapBox(p, ct.Bounds())
	p.Fill(b, scene.Solid(look.T.Surface), cardRadius)
	if ct.FocusVisible() {
		focusRing(p, b, cardRadius, look.T.Primary)
	}
	p.Clip(b, cardRadius, func() { ct.Base.Paint(c, p) })
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// init adds the parts the first time.
func (ct *CourseTile) init() {
	if ct.attached {
		return
	}
	ct.attached = true
	ct.Look = noLook
	ct.name.Name = nameCardName
	labelStyle{
		Role: theme.RoleBody, Size: cardNameSize, Weight: 700, Ink: theme.InkText,
		Ellipsis: true, MaxLines: tileNameLines,
	}.apply(&ct.name)
	ct.Add(&ct.name, &ct.tag)
}

// syncParts makes the parts match the fields, as CourseCard's.
func (ct *CourseTile) syncParts(look *theme.Look) {
	if ct.poster != ct.Poster {
		if ct.poster != nil {
			ct.Remove(ct.poster)
		}
		ct.Remove(&ct.name)
		ct.Remove(&ct.tag)
		if ct.poster = ct.Poster; ct.poster != nil {
			ct.Add(ct.poster)
		}
		ct.Add(&ct.name, &ct.tag) // the poster stays first in the tree
	}
	ct.name.SetText(ct.Name)
	col := look.C.CatRegular
	switch ct.Category {
	case CardSpecial:
		col = look.C.CatSpecial
	case CardIntro:
		col = look.C.CatIntro
	case CardRegular, CardCategoryUnknown:
	default:
	}
	labelStyle{Role: theme.RoleBody, Size: cardCatSize, Weight: 800, Spacing: cardCatSpacing, Upper: true}.apply(&ct.tag)
	ct.tag.Color = col
	ct.tag.SetText(ct.Tag)
}
