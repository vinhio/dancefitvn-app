//go:build !tvos

// The pressable card's shape (a Pressable with the Look taken off around a kit.Card)
// follows updateCard of home_parts.go, which follows internal/ui/kit/coursecard.go
// and jplayer-mobile/internal/ui/kit/card.go at 49de3c2; the row's parts are written
// from the card of MS.courses (index.html:379).

package screens

import (
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of a card of MS.courses (index.html:379), in DIPs.
const (
	// rowPad, rowGap are `padding:12px;gap:14px` of the `.card`.
	rowPad, rowGap float32 = 12, 14
	// rowPoster, rowPosterR, rowPosterTS are `poster(id,80,80,{r:12,ts:13})`.
	rowPoster, rowPosterR, rowPosterTS float32 = 80, 12, 13
	// rowNameSize is `<b style="font-size:15px">`, bold; rowCountSize the lessons'
	// `font-size:12px;color:var(--tx3)`.
	rowNameSize, rowCountSize float32 = 15, 12
	// rowQuoteSize, rowQuoteTop, rowQuoteBottom are `.muted` at 13 px with
	// `margin:4px 0 8px`.
	rowQuoteSize, rowQuoteTop, rowQuoteBottom float32 = 13, 4, 8
	// rowChipDY is the chips' place in their line: an inline-flex 22 high on the 16 px
	// strut's baseline stands 1 under the line's top (M04: `span.chip.c-th` at y 311
	// under the quote's bottom margin at 310).
	rowChipDY float32 = 1
	// rowChipGap is the space between the two chips (`</span> ${...}`): Inter's space
	// at 16 px (M04: `span.chip.y` at x 197.33, the first chip ends at 192.83).
	rowChipGap float32 = 4.5
	// rowMargin is the card's `margin-bottom:12px`.
	rowMargin float32 = 12
)

// courseRow is the card of a course in the list: `.card` with the 80 poster, the
// name with the number of lessons at the right, the tagline in quotes and the chips,
// all of it one button that opens the course.
type courseRow struct {
	ui.Pressable

	card  *kit.Card
	label string
}

// nameCount is the first line of the text: `display:flex;justify-content:
// space-between` with the name at the left and the lessons at the right, both at
// the top (`align-items` stretches the boxes, the text starts at their top).
type nameCount struct {
	ui.Base

	name  *kit.Label
	count *kit.Label
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newCourseRow returns the card of c; onPress is its tap. Its Name is
// `go.detail:<key>`.
func newCourseRow(e *env.Env, c model.Course, onPress func()) *courseRow {
	ps := poster.ForCourse(c)
	ps.Radius, ps.TitleSize = rowPosterR, rowPosterTS
	line := &nameCount{
		name:  &kit.Label{Text: c.Name, Size: rowNameSize, Weight: 700, Ellipsis: true},
		count: &kit.Label{Text: e.Fill("courses.countTpl", "n", itoa(c.LessonCount)), Size: rowCountSize, Ink: theme.InkText3},
	}
	line.Add(line.name, line.count)

	quote := "—" // `c.quote?'“'+esc(c.quote)+'”':'—'`
	if c.Tagline != "" {
		quote = "“" + c.Tagline + "”"
	}
	q := kit.Muted(quote, rowQuoteSize)
	q.Name, q.Wrap = "div.muted", true

	chips := ui.Row(rowChipGap, kit.NewChip(ChipKind(c.Category), upper(c.CategoryLabel)))
	chips.Align = ui.CrossStart
	if c.IsNew {
		chips.Add(kit.NewChip(kit.ChipY, e.T("course.new")))
	}

	text := ui.Column(0,
		line, &ui.Box{Height: rowQuoteTop}, q, &ui.Box{Height: rowQuoteBottom},
		boxed(chips, geom.Insets{Top: rowChipDY}),
	)
	text.Grow = 1
	text.Align = ui.CrossStretch
	cover := poster.New(ps, rowPoster, rowPoster)
	cover.SetCover(e.Cover, c.CoverURL) // the real cover once loaded (K14)
	row := ui.Row(rowGap, cover, text)
	row.Align = ui.CrossStart // the poster and the text start at the top
	card := kit.NewCard(row)
	card.Name = "" // the button around it is the key
	card.Padding = geom.Uniform(rowPad)

	k := &courseRow{card: card, label: c.Name}
	k.Name = "go.detail:" + CourseKey(c.ID) // `data-go="detail" data-set`
	k.Look = noDecoration
	k.OnPress = onPress
	k.Add(card)

	return k
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AccessibleLabel returns the course's name.
func (k *courseRow) AccessibleLabel() string { return k.label }

// Layout is the card's size.
func (k *courseRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	s := c.Layout(k.card, cs)
	c.Place(k.card, geom.Point{})

	return cs.Constrain(s)
}

// Layout puts the lessons at the right edge and the name at the left, in what the
// count leaves.
func (n *nameCount) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	cz := c.Layout(n.count, ui.Loose(geom.Sz(w, ui.Unbounded)))
	nz := c.Layout(n.name, ui.Loose(geom.Sz(max(w-cz.W, 0), ui.Unbounded)))
	c.Place(n.name, geom.Point{})
	c.Place(n.count, geom.Pt(w-cz.W, 0))

	return cs.Constrain(geom.Sz(w, max(nz.H, cz.H)))
}
