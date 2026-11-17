//go:build !tvos

// The pressable card's shape (a Describe-free Pressable with the Look taken off, the
// clip to the radius) follows internal/ui/kit/coursecard.go, which follows
// jplayer-mobile/internal/ui/kit/card.go at 49de3c2; the parts here are written
// from continueCard and MS.home (index.html:353-369).

package screens

import (
	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of the header row of MS.home (index.html:362).
const (
	// homeEyebrowGap is the `.d` of the greeting `margin-top:8px`, under the `.eb`.
	homeEyebrowGap float32 = 8
	// homeGreetSize is the greeting's `font-size:28px`; homeAvatar the photo,
	// `photo(44,'var(--pr)')`.
	homeGreetSize, homeAvatar float32 = 28, 44
)

// The measures of continueCard (index.html:353-361).
const (
	// contH is `poster(l.cid,353,232,...)`; contRadius `{r:20}`, which the scrim
	// follows (`border-radius:20px`).
	contH, contRadius float32 = 232, 20
	// contScrimFrom is `linear-gradient(transparent 40%, rgba(0,0,0,.85))`.
	contScrimFrom, contScrimAlpha float32 = .4, .85
	// contChipX, contChipY are `left:16px;top:16px` of the chip.
	contChipX, contChipY float32 = 16, 16
	// contTextLeft, contTextRight, contTextBottom are `left:16px;right:80px;
	// bottom:16px` of the text block.
	contTextLeft, contTextRight, contTextBottom float32 = 16, 80, 16
	// contMetaSize, contMetaWeight, contMetaAlpha are `font-size:12px;
	// font-weight:600;opacity:.9`.
	contMetaSize, contMetaAlpha float32 = 12, .9
	contMetaWeight                      = 600
	// contTitleSize, contTitleTop, contTitleBottom are `.d` at `font-size:26px;
	// margin:6px 0 10px`.
	contTitleSize, contTitleTop, contTitleBottom float32 = 26, 6, 10
	// contDisc, contDiscRight, contDiscBottom are `right:16px;bottom:18px;
	// width:52px;height:52px`.
	contDisc, contDiscRight, contDiscBottom float32 = 52, 16, 18
	// contPlayX, contPlayW, contPlayH are the play glyph's box in the disc,
	// `padding:14px 13px 14px 16px`: 23 wide and 24 high from (16, 14); the 24 unit
	// svg is scaled to fit the narrower side, 23, and centred in the height.
	contPlayX, contPlayY, contPlay float32 = 16, 14.5, 23
	// contChipWeight, contChipSpacing are the chip's `font-weight:800;
	// letter-spacing:1px`.
	contChipWeight  = 800
	contChipSpacing = 1
)

// The measures of the section head `.sec` (index.html:88-90).
const nameSec = "div.sec"

// The measures of the update card (index.html:367) and the stats (:361).
const (
	// updPoster, updRadius, updTitleSize are `poster('set36',72,72,{r:12,ts:14})`.
	updPoster, updRadius, updTitleSize float32 = 72, 12, 14
	// updPad, updGap are `padding:12px;gap:12px`.
	updPad, updGap float32 = 12, 12
	// updNameSize is `<b style="font-size:15px">`; updSubSize, updSubTop,
	// updSubBottom are `.muted` at 12 px with `margin:4px 0 8px`.
	updNameSize, updSubSize, updSubTop, updSubBottom float32 = 15, 12, 4, 8
	// updChipDY is the chip's place in its line: an inline-flex 22 high on the
	// 16 px strut's baseline stands 1 under the line's top (M03: `span.chip.c-th`
	// at y 807 under the line at 806).
	updChipDY float32 = 1
	// statPad, statGap, statNumSize, statLabelSize, statLabelTop are `.card` of
	// the numbers: `padding:14px`, `gap:8px`, `.d` 26, `.muted` 12 `margin-top:6px`.
	statPad, statGap, statNumSize, statLabelSize, statLabelTop float32 = 14, 8, 26, 12, 6
)

// continueCard is continueCard (index.html:353-361): the cover with the scrim over
// it, the yellow chip, the lesson's line, title and progress, and the play disc,
// all of it one button.
type continueCard struct {
	ui.Pressable

	cover *poster.Poster
	scrim scrim
	chip  *kit.Chip
	meta  *kit.Label
	title *kit.Label
	prog  *kit.Progress
	disc  playDisc
	label string
}

// scrim is `position:absolute;inset:0;border-radius:20px;background:
// linear-gradient(transparent 40%, rgba(0,0,0,.85))`.
type scrim struct {
	ui.Base

	paint scene.Paint
	h     float32 // the height the paint was made for
}

// playDisc is the 52 circle of --pr with the play glyph in it.
type playDisc struct {
	ui.Base

	glyph *kit.Icon
}

// homeHeader is the row `display:flex;justify-content:space-between;
// align-items:center;margin:14px 0 18px` (index.html:362): the eyebrow over the
// greeting at the left, the avatar button at the right.
type homeHeader struct {
	ui.Base

	eyebrow *kit.Label
	greet   *kit.Label
	avatar  *kit.AvatarButton
}

// secHead is `.sec` (index.html:88-90): the title at the left and, when there is
// one, the link at the right, on one baseline.
type secHead struct {
	ui.Base

	title *kit.Label
	link  *secLink
}

// secLink is the `a` of a `.sec`: 13/600 in --pr, a button.
type secLink struct {
	ui.Pressable

	label *kit.Label
}

// updateCard is the card of "Mới cập nhật" (index.html:367): a kit.Card that is a
// button.
type updateCard struct {
	ui.Pressable

	card  *kit.Card
	label string
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newContinueCard returns the card of what to watch next; onPress is its tap.
func newContinueCard(e *env.Env, c *model.Continue, onPress func()) *continueCard {
	k := &continueCard{label: c.Lesson.Title}
	k.Name = "playVideo:" + LessonKey(c.Course.ID, c.Lesson.No) // `data-act="playVideo" data-k`
	k.Look = noDecoration
	k.OnPress = onPress

	spec := poster.ForCourse(model.Course{ID: c.Course.ID, Name: c.Course.Name})
	spec.Radius, spec.NoTitle = contRadius, true // {r:20,title:false}
	k.cover = poster.New(spec, 0, contH)
	k.cover.SetCover(e.Cover, c.Course.CoverURL) // the real cover once loaded (K14)

	k.chip = kit.NewChip(kit.ChipY, e.T("home.continue")) // `span.chip.y`
	k.chip.Weight, k.chip.Spacing = contChipWeight, contChipSpacing

	k.meta = &kit.Label{
		Text: e.Fill("home.continueMetaTpl",
			"course", c.Course.Name, "no", format.No(c.Lesson.No), "tag", c.Lesson.Tag,
			"min", itoa(format.RemainingMinutes(float64(c.Lesson.DurationSec), float64(c.PositionSec)))),
		Size: contMetaSize, Weight: contMetaWeight, Wrap: true,
		Color: tokens.Dark.Text.WithAlpha(contMetaAlpha),
	}
	k.title = kit.Display(c.Lesson.Title, contTitleSize)
	k.title.Wrap = true
	k.prog = &kit.Progress{Track: color.RGBA(255, 255, 255, .3)} // `.prog` on rgba(255,255,255,.3)
	if c.Lesson.DurationSec > 0 {
		k.prog.SetValue(float32(c.PositionSec) / float32(c.Lesson.DurationSec))
	}
	k.disc.glyph = kit.NewIcon(icon.Play, contPlay, theme.InkOnPrimary)
	k.disc.glyph.Name = "" // PLAY has no class: not in layout.json
	k.disc.Add(k.disc.glyph)
	k.Add(k.cover, &k.scrim, k.chip, k.meta, k.title, k.prog, &k.disc)

	return k
}

// newHomeHeader returns the header of the greeting; onAvatar is the avatar's tap.
func newHomeHeader(e *env.Env, eyebrow, name string, onAvatar func()) *homeHeader {
	h := &homeHeader{
		eyebrow: kit.Eyebrow(eyebrow),
		greet:   heading(kit.Display(e.Fill("home.helloTpl", "name", name), homeGreetSize), 1),
		avatar:  kit.NewAvatarButton(homeAvatar, tokens.Dark.Primary, e.T("tabs.account")),
	}
	h.eyebrow.Wrap, h.greet.Wrap = true, true
	h.avatar.Name = "go.account" // `data-go="account" data-root="1"`
	h.avatar.OnPress = onAvatar
	h.Add(h.eyebrow, h.greet, h.avatar)

	return h
}

// newSecHead returns `.sec` with title and, for a non-empty linkText, the link
// named linkName whose tap is onLink.
func newSecHead(title, linkText, linkName string, onLink func()) *secHead {
	s := &secHead{title: kit.SecTitle(title)}
	s.Name = nameSec
	s.Add(s.title)
	if linkText != "" {
		l := &secLink{label: kit.SecLink(linkText)}
		l.Name = linkName
		l.Look = noDecoration
		l.OnPress = onLink
		l.Add(l.label)
		s.link = l
		s.Add(l)
	}

	return s
}

// newUpdateCard returns the card of an update: the poster, the name, the line, the
// chip. Its Name is `go.detail:<key>`.
func newUpdateCard(e *env.Env, u model.Course, onPress func()) *updateCard {
	ps := poster.ForCourse(u)
	ps.Radius, ps.TitleSize = updRadius, updTitleSize
	name := newInline(&kit.Label{Text: u.Name, Size: updNameSize, Weight: 700})
	sub := kit.Muted(updateLine(e, u), updSubSize)
	sub.Name, sub.Wrap = "div.muted", true
	chip := kit.NewChip(ChipKind(u.Category), e.Fill("home.updateChipTpl", "cat", upper(u.CategoryLabel), "n", itoa(u.LessonCount)))
	text := ui.Column(0,
		name, &ui.Box{Height: updSubTop}, sub, &ui.Box{Height: updSubBottom},
		boxed(chip, geom.Insets{Top: updChipDY}),
	)
	text.Grow = 1
	text.Align = ui.CrossStretch
	cover := poster.New(ps, updPoster, updPoster)
	cover.SetCover(e.Cover, u.CoverURL) // the real cover once loaded (K14)
	row := ui.Row(updGap, cover, text)
	row.Align = ui.CrossStart // `display:flex` stretches; the poster and the text start at the top
	card := kit.NewCard(row)
	card.Name = "" // the button around it is the key
	card.Padding = geom.Uniform(updPad)
	k := &updateCard{card: card, label: u.Name}
	k.Name = "go.detail:" + CourseKey(u.ID)
	k.Look = noDecoration
	k.OnPress = onPress
	k.Add(card)

	return k
}

// newStat returns one of the three numbers: `.card` with `.d` 26 and `.muted` 12.
// The number's label is returned too, for the one that follows the downloads.
func newStat(num, label string, accent bool) (*kit.Card, *kit.Label) {
	n := kit.Display(num, statNumSize)
	if accent {
		n.Ink, n.Name = theme.InkPrimary, "div.d.acc"
	}
	m := kit.Muted(label, statLabelSize)
	m.Name, m.Wrap = "div.muted", true
	col := ui.Column(0, n, &ui.Box{Height: statLabelTop}, m)
	col.Align = ui.CrossStretch
	card := kit.NewCard(col)
	card.Padding = geom.Uniform(statPad)
	card.Grow = 1

	return card, n
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AccessibleLabel returns the lesson's title.
func (k *continueCard) AccessibleLabel() string { return k.label }

// Layout puts the parts in the card, as wide as the room and 232 tall: the cover and
// the scrim over all of it, the chip at 16, 16, the text block 16 from the left, 80
// from the right and 16 from the bottom, the disc 16 and 18 from the corner.
func (k *continueCard) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	size := cs.Constrain(geom.Sz(w, contH))
	full := ui.Tight(size)
	c.Layout(k.cover, full)
	c.Place(k.cover, geom.Point{})
	c.Layout(&k.scrim, full)
	c.Place(&k.scrim, geom.Point{})
	c.Layout(k.chip, ui.Loose(size))
	c.Place(k.chip, geom.Pt(contChipX, contChipY))

	tw := max(size.W-contTextLeft-contTextRight, 0)
	block := ui.Constraints{Min: geom.Sz(tw, 0), Max: geom.Sz(tw, ui.Unbounded)}
	ps := c.Layout(k.prog, block)
	ts := c.Layout(k.title, block)
	ms := c.Layout(k.meta, block)
	y := size.H - contTextBottom - ps.H
	c.Place(k.prog, geom.Pt(contTextLeft, y))
	y -= contTitleBottom + ts.H
	c.Place(k.title, geom.Pt(contTextLeft, y))
	y -= contTitleTop + ms.H
	c.Place(k.meta, geom.Pt(contTextLeft, y))

	c.Layout(&k.disc, ui.Tight(geom.Sz(contDisc, contDisc)))
	c.Place(&k.disc, geom.Pt(size.W-contDiscRight-contDisc, size.H-contDiscBottom-contDisc))

	return size
}

// Layout makes the scrim as large as the card.
func (s *scrim) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	if s.paint.Gradient == nil || s.h != size.H {
		s.h = size.H
		s.paint = scene.Linear(geom.Point{}, geom.Pt(0, size.H),
			scene.Stop{Color: color.RGBA(0, 0, 0, 0), Offset: contScrimFrom},
			scene.Stop{Color: color.RGBA(0, 0, 0, contScrimAlpha), Offset: 1})
	}

	return size
}

// Paint draws the gradient over the cover, rounded as it is.
func (s *scrim) Paint(_ *ui.Context, p *ui.Painter) { p.Fill(s.Bounds(), s.paint, contRadius) }

// Layout puts the glyph in the disc, in the box its padding leaves it.
func (d *playDisc) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(contDisc, contDisc))
	c.Layout(d.glyph, ui.Tight(geom.Sz(contPlay, contPlay)))
	c.Place(d.glyph, geom.Pt(contPlayX, contPlayY))

	return size
}

// Paint draws the disc in --pr, then the glyph.
func (d *playDisc) Paint(c *ui.Context, p *ui.Painter) {
	r := d.Bounds()
	p.Fill(r, scene.Solid(theme.From(c).T.Primary), r.W/2)
	d.Base.Paint(c, p)
}

// Layout puts the eyebrow over the greeting, centred on the row with the avatar at
// the right edge. The text block is as wide as its widest line, or the room the
// avatar leaves.
func (h *homeHeader) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	as := c.Layout(h.avatar, ui.Loose(geom.Sz(w, ui.Unbounded)))
	unb := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	es := c.Layout(h.eyebrow, unb)
	gs := c.Layout(h.greet, unb)
	bw := min(max(es.W, gs.W), max(w-as.W, 0))
	block := ui.Constraints{Min: geom.Sz(bw, 0), Max: geom.Sz(bw, ui.Unbounded)}
	es = c.Layout(h.eyebrow, block)
	gs = c.Layout(h.greet, block)
	lh := es.H + homeEyebrowGap + gs.H
	rowH := max(lh, as.H)
	top := (rowH - lh) / 2
	c.Place(h.eyebrow, geom.Pt(0, top))
	c.Place(h.greet, geom.Pt(0, top+es.H+homeEyebrowGap))
	c.Place(h.avatar, geom.Pt(w-as.W, (rowH-as.H)/2))

	return cs.Constrain(geom.Sz(w, rowH))
}

// Layout puts the title and the link on one baseline (`align-items:baseline`).
func (s *secHead) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	unb := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	ts := c.Layout(s.title, unb)
	top := s.title.Baseline()
	var ls geom.Size
	if s.link != nil {
		ls = c.Layout(s.link, unb)
		top = max(top, s.link.label.Baseline())
	}
	ty := top - s.title.Baseline()
	h := ty + ts.H
	c.Place(s.title, geom.Pt(0, ty))
	if s.link != nil {
		ly := top - s.link.label.Baseline()
		h = max(h, ly+ls.H)
		c.Place(s.link, geom.Pt(w-ls.W, ly))
	}

	return cs.Constrain(geom.Sz(w, h))
}

// LayoutStyle is the style of the link's text (`a`).
func (l *secLink) LayoutStyle(c *ui.Context) (kit.CSS, string) { return l.label.LayoutStyle(c) }

// AccessibleLabel returns the link's text.
func (l *secLink) AccessibleLabel() string { return l.label.Text }

// Layout is the text's size.
func (l *secLink) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	s := c.Layout(l.label, ui.Loose(geom.Sz(cs.Max.W, ui.Unbounded)))
	c.Place(l.label, geom.Point{})

	return cs.Constrain(s)
}

// AccessibleLabel returns the course's name.
func (u *updateCard) AccessibleLabel() string { return u.label }

// Layout is the card's size.
func (u *updateCard) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	s := c.Layout(u.card, cs)
	c.Place(u.card, geom.Point{})

	return cs.Constrain(s)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// noDecoration is the Look of a pressable the prototype gives no state (`cursor:
// pointer` only): nothing is drawn for hover, press or focus.
func noDecoration(*ui.Context, ui.Interaction) ui.Decoration { return ui.Decoration{} }

// boxed returns w in a box with pad around it.
func boxed(w ui.Widget, pad geom.Insets) *ui.Box {
	b := &ui.Box{Padding: pad}
	b.Add(w)

	return b
}

// updateLine is the muted line of an update: "{tagline} · bạn chưa tham gia" for
// a course the user is not in, the tagline alone for one they are.
func updateLine(e *env.Env, u model.Course) string {
	switch {
	case u.Enrolled:
		return u.Tagline
	case u.Tagline == "":
		return e.T("home.notEnrolled")
	default:
		return e.Fill("home.updateSubTpl", "tagline", u.Tagline)
	}
}
