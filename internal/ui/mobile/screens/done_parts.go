//go:build !tvos

// The parts of MS.done (index.html:441-448, boxes of M10.layout.json): the disc with
// the check, the card of the course's progress with its ring, the card of the
// next lesson. jPlayer has nothing like them (docs/plan/P4.md section 1: "Nothing
// to copy for the done countdown").

package screens

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of screen 10 (index.html:441-448), in DIPs.
const (
	// dBackTop is the back button's `margin-top:6px` (:442).
	dBackTop float32 = 6
	// dDisc, dHalo, dCheck, dCheckStroke are the disc's `width:96px;height:96px;
	// box-shadow:0 0 0 16px rgba(255,208,40,.16)` and `ic('check',48,3)` (:443);
	// dHaloAlpha the ring's .16; dDiscTop the block's `margin-top:10px`.
	dDisc, dHalo, dCheck, dCheckStroke float32 = 96, 16, 48, 3
	dHaloAlpha                         float32 = .16
	dDiscTop                           float32 = 10
	// dTitleTop, dTitleSize are the title's `font-size:36px;margin-top:34px`.
	dTitleTop, dTitleSize float32 = 34, 36
	// dTagTop is the song line's `margin-top:10px` (16 px, 700, --pr).
	dTagTop float32 = 10
	// dCourseTop, dCourseSize are the course line's `font-size:13px;margin-top:6px`.
	dCourseTop, dCourseSize float32 = 6, 13
	// dCardTop is the first card's `margin-top:22px`; dNextTop the second's 12.
	dCardTop, dNextTop float32 = 22, 12
	// dCardPad, dCardGap are the first card's `padding:18px;gap:16px`.
	dCardPad, dCardGap float32 = 18, 16
	// dWatchedSize, dLeftSize, dLeftTop are `<b style="font-size:17px">` and the
	// muted line's `font-size:13px;margin-top:4px` (:445).
	dWatchedSize, dLeftSize, dLeftTop float32 = 17, 13, 4
	// dNextPad, dNextGap are the second card's `padding:12px;gap:12px`.
	dNextPad, dNextGap float32 = 12, 12
	// dNo, dNoRadius are the number box's `width:48px;height:48px;border-radius:12px`.
	dNo, dNoRadius float32 = 48, 12
	// dEbSize, dSongSize, dMetaSize are the eyebrow's `font-size:10px`, the title's
	// `<b style="font-size:14px">` and the muted line's `font-size:12px` (:446).
	dEbSize, dSongSize, dMetaSize float32 = 10, 14, 12
	// dBtnTop is the button's `margin-top:20px` (:447); the last lesson's
	// paragraph has the same.
	dBtnTop float32 = 20
	// dBgAt, dBgEnd, dBgAlpha are `radial-gradient(circle at 50% 22%,
	// rgba(255,208,40,.3), transparent 50%)` (:441).
	dBgAt, dBgEnd, dBgAlpha = .22, .5, .3
)

// The names layout.json gives the parts.
const (
	nameDoneTag = "div.acc"
	namePara    = "p.muted"
)

// doneContent is what the page scrolls: the back button, the block of the finished
// lesson, the two cards, the buttons.
type doneContent struct {
	ui.Base

	back    *kit.RoundButton
	badge   *doneBadge
	title   *kit.Label
	tag     *kit.Label
	course  *kit.Label
	progCrd *kit.Card
	prog    *progressRow
	nextCrd *kit.Card // nil on the last lesson
	next    *nextRow
	watch   *kit.Button
	para    *kit.Label // the last lesson's line
	toCrs   *kit.Button
}

// doneBadge is the 96 DIP disc of --pr with the check in --on, and the ring of
// --pr at .16 that is its box-shadow, 16 outside it.
type doneBadge struct {
	ui.Base

	glyph *kit.Icon
}

// progressRow is the first card's content: the ring, and the two lines of text at
// its right.
type progressRow struct {
	ui.Base

	ring    *kit.Ring
	watched *inlineLine
	left    *kit.Label
	wl      *kit.Label
}

// nextRow is the second card's content: the box with the lesson's number, and the
// eyebrow with the countdown, the title and the line of the tag and the length.
type nextRow struct {
	ui.Base

	no    *noBox
	eb    *kit.Label
	song  *inlineLine
	sl    *kit.Label
	meta  *kit.Label
	count int
}

// noBox is the 48 DIP box of the number, --sf2 under --pr at 800.
type noBox struct {
	ui.Base

	label *kit.Label
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newDoneBadge returns the disc with the check.
func newDoneBadge() *doneBadge {
	b := &doneBadge{}
	b.glyph = kit.NewIcon(icon.Check, dCheck, theme.InkOnPrimary)
	b.glyph.Stroke = dCheckStroke
	b.Add(b.glyph)

	return b
}

// newProgressRow returns the row with the ring and the two lines.
func newProgressRow() *progressRow {
	r := &progressRow{ring: kit.NewRing("")}
	r.wl = &kit.Label{Size: dWatchedSize, Weight: 700, Ink: theme.InkText}
	r.watched = newInline(r.wl)
	r.left = kit.Muted("", dLeftSize)
	r.left.Name = nameMuted
	r.Add(r.ring, r.watched, r.left)

	return r
}

// newNextRow returns the row of the next lesson.
func newNextRow() *nextRow {
	r := &nextRow{no: &noBox{}}
	r.no.label = &kit.Label{Weight: 800, Ink: theme.InkPrimary}
	r.no.Add(r.no.label)
	r.eb = kit.Eyebrow("")
	r.eb.Size = dEbSize
	r.eb.Upper = false // the countdown's strings are made upper case, once (Done)
	r.sl = &kit.Label{Size: dSongSize, Weight: 700, Ink: theme.InkText}
	r.song = newInline(r.sl)
	r.meta = kit.Muted("", dMetaSize)
	r.meta.Name = nameMuted
	r.Add(r.no, r.eb, r.song, r.meta)

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout stacks the parts from the top of the page, the centered ones in the middle
// of the width.
func (d *doneContent) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	tight := ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}
	y := dBackTop
	bs := c.Layout(d.back, ui.Loose(cs.Max))
	c.Place(d.back, geom.Pt(0, y))
	y += bs.H + dDiscTop

	ds := c.Layout(d.badge, ui.Loose(cs.Max))
	c.Place(d.badge, geom.Pt((w-ds.W)/2, y))
	y += ds.H + dTitleTop
	flow := func(wd ui.Widget, gapAfter float32) {
		s := c.Layout(wd, tight)
		c.Place(wd, geom.Pt(0, y))
		y += s.H + gapAfter
	}
	flow(d.title, dTagTop)
	flow(d.tag, dCourseTop)
	flow(d.course, dCardTop)
	flow(d.progCrd, dNextTop)
	if d.nextCrd != nil {
		flow(d.nextCrd, dBtnTop)
		flow(d.watch, 0)
	} else {
		flow(d.para, 0)
	}
	flow(d.toCrs, 0)

	return cs.Constrain(geom.Sz(w, y))
}

// Layout makes the badge as wide as the halo's disc and as tall (the halo is
// painted outside the box, as box-shadow is).
func (b *doneBadge) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(dDisc, dDisc))
	gs := c.Layout(b.glyph, ui.Tight(geom.Sz(dCheck, dCheck)))
	c.Place(b.glyph, geom.Pt((size.W-gs.W)/2, (size.H-gs.H)/2))

	return size
}

// Paint draws the halo, the disc and the check over them.
func (b *doneBadge) Paint(c *ui.Context, p *ui.Painter) {
	t := &theme.From(c).T
	r := b.Bounds()
	halo := geom.XYWH(r.X-dHalo, r.Y-dHalo, r.W+2*dHalo, r.H+2*dHalo)
	p.Fill(halo, scene.Solid(t.Primary.WithAlpha(dHaloAlpha)), halo.W/2)
	p.Fill(r, scene.Solid(t.Primary), r.W/2)
	b.Base.Paint(c, p)
}

// Layout puts the ring at the left, the two lines at its right 16 away, the pair
// centered on the ring.
func (r *progressRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	rs := c.Layout(r.ring, ui.Loose(cs.Max))
	c.Place(r.ring, geom.Point{})
	x := rs.W + dCardGap
	room := ui.Loose(geom.Sz(max(cs.Max.W-x, 0), ui.Unbounded))
	ws := c.Layout(r.watched, room)
	ls := c.Layout(r.left, room)
	h := ws.H + dLeftTop + ls.H
	top := (rs.H - h) / 2
	c.Place(r.watched, geom.Pt(x, top))
	c.Place(r.left, geom.Pt(x, top+ws.H+dLeftTop))

	return cs.Constrain(geom.Sz(cs.Max.W, max(rs.H, h)))
}

// Layout puts the number box at the left and the three lines in the rest, 12 away,
// centered on the box.
func (r *nextRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	ns := c.Layout(r.no, ui.Tight(geom.Sz(dNo, dNo)))
	c.Place(r.no, geom.Point{})
	x := ns.W + dNextGap
	room := geom.Sz(max(cs.Max.W-x, 0), ui.Unbounded)
	col := ui.Constraints{Min: geom.Sz(room.W, 0), Max: room}
	es := c.Layout(r.eb, col)
	ss := c.Layout(r.song, ui.Loose(room))
	ms := c.Layout(r.meta, col)
	h := es.H + ss.H + ms.H
	top := (ns.H - h) / 2
	c.Place(r.eb, geom.Pt(x, top))
	c.Place(r.song, geom.Pt(x, top+es.H))
	c.Place(r.meta, geom.Pt(x, top+es.H+ss.H))

	return cs.Constrain(geom.Sz(cs.Max.W, max(ns.H, h)))
}

// SetCount shows the seconds left in the eyebrow, the text made for it: nothing
// is built at a tick.
func (r *nextRow) SetCount(n int, txt string) {
	r.count = n
	r.eb.SetText(txt)
}

// Layout makes the box 48 square and centers the number.
func (b *noBox) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(dNo, dNo))
	ls := c.Layout(b.label, ui.Loose(size))
	c.Place(b.label, geom.Pt((size.W-ls.W)/2, (size.H-ls.H)/2))

	return size
}

// Paint draws the --sf2 box.
func (b *noBox) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(b.Bounds(), scene.Solid(theme.From(c).T.Surface2), dNoRadius)
	b.Base.Paint(c, p)
}
