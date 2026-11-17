//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/coll.go at 49de3c2 (the page's
// head, its rows and the foot that comes last, in one scroll column) and
// home_content.go (a column whose children each sit a number of DIPs under the one
// before). The column is MS.locked (index.html:404-412), the screen of a course the
// user has not joined: the same hero, the card that says so, the description and the
// first rows of the content, with the pinned foot. The prototype's foot is the enroll
// button; K30 (user decision 2026-11-17) takes it out: the app sells and links to
// nothing, so the foot is a line of plain text that says whom to contact.

package screens

import (
	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of MS.locked's column (index.html:404-412), in DIPs.
const (
	// lockedBottom is the `.scroll`'s own `padding-bottom:120px` (this screen sets none).
	lockedBottom float32 = 120
	// lockedRows is how many rows of the content the screen shows (`[1,2,3,4]`, F-16).
	lockedRows = 4
	// lockPad, lockGap are the card's `padding:16px;gap:12px`; lockDisc, lockIcon the
	// disc's `width:34px;height:34px` and its `ic('lock',18)`.
	lockPad, lockGap, lockDisc, lockIcon float32 = 16, 12, 34, 18
	// lockTitleSize is `<b style="font-size:14px">`; lockBodySize, lockBodyTop,
	// lockBodyLine the paragraph's `font-size:12px;margin-top:4px;line-height:1.5`.
	lockTitleSize, lockBodySize, lockBodyTop, lockBodyLine float32 = 14, 12, 4, 1.5
	// lockHeadSize is `.d` at `font-size:16px`; lockIntroTop, lockIntroBottom its
	// `margin:22px 0 8px`, lockContentBottom the content head's `margin:22px 0 4px`.
	lockHeadSize                                    float32 = 16
	lockHeadTop, lockIntroBottom, lockContentBottom float32 = 22, 8, 4
	lockDescSize, lockDescLine                      float32 = 14, 1.5
	// lockSupportSize, lockSupportTop, lockSupportBottom are the support line's
	// `font-size:12px;margin:20px 0 90px`.
	lockSupportSize, lockSupportTop, lockSupportBottom float32 = 12, 20, 90
	// footLockedTop is the footer's `padding:16px 20px 38px` (the prototype's, kept
	// for the K30 text); lockContactSize is the K30 text's size, the 14 of
	// lockDescSize, the paragraph of this screen.
	footLockedTop   float32 = 16
	lockContactSize float32 = 14
)

// lockedContent is what the locked course scrolls: the hero, the card, the
// description, the first rows of the content and the support line.
type lockedContent struct {
	ui.Base

	hero     *courseHero
	card     *kit.Card
	introH   *kit.Label
	intro    *kit.Label
	contentH *kit.Label
	rows     []*kit.LessonRow
	support  *kit.Label // nil: the app config says nobody to call
}

// lockBody is the card's content: the lock's disc at the left, the title over the
// paragraph at the right, 12 apart.
type lockBody struct {
	ui.Base

	disc  lockDiscView
	title *inlineLine
	body  *kit.Label
}

// lockDiscView is `<span style="width:34px;height:34px;border-radius:50%;
// background:rgba(255,208,40,.16);color:var(--pr)">` with the lock in it.
type lockDiscView struct {
	ui.Base

	glyph *kit.Icon
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newLockedFrame returns the screen's frame for the course c the user has not joined:
// the content and the footer with the contact text (K30: no button, no link; the
// course's `enroll_url` is a contract field the app does not open).
func newLockedFrame(e *env.Env, c model.Course) (*scrFrame, *lockedContent) {
	n := lessonTotal(c)
	k := &lockedContent{}
	k.hero = newCourseHero(e, c,
		e.Fill("detail.eyebrowTpl", "cat", c.CategoryLabel, "n", itoa(n)),
		heroSub(e, c, "locked.subTpl", false), false, true)

	body := &lockBody{
		title: newInline(&kit.Label{Text: e.T("locked.title"), Size: lockTitleSize, Weight: 700, Wrap: true}),
		body:  kit.Muted(e.T("locked.body"), lockBodySize),
	}
	body.body.Name, body.body.Wrap, body.body.LineHeight = "p.muted", true, lockBodyLine
	body.disc.glyph = kit.NewIcon(icon.Lock, lockIcon, theme.InkPrimary)
	body.disc.Add(body.disc.glyph)
	body.Add(&body.disc, body.title, body.body)
	k.card = kit.NewCard(body)
	k.card.Border, k.card.Padding = true, geom.Uniform(lockPad)

	k.introH = heading(kit.Display(e.T("locked.intro"), lockHeadSize), 2)
	k.intro = kit.Muted(c.Description, lockDescSize)
	k.intro.Name, k.intro.Wrap, k.intro.LineHeight = "p.muted", true, lockDescLine
	k.contentH = heading(kit.Display(e.Fill("locked.contentTpl", "n", itoa(n)), lockHeadSize), 2)
	k.Add(k.hero, k.card, k.introH, k.intro, k.contentH)

	for i, l := range c.Lessons {
		if i >= lockedRows {
			break
		}
		k.rows = append(k.rows, newLockedLesson(e, l))
		k.Add(k.rows[i])
	}
	if cfg, ok := e.Cfg(); ok && cfg.Support.Hotline != "" {
		k.support = kit.Muted(e.Fill("locked.supportTpl", "name", cfg.Support.HotlineName, "phone", cfg.Support.Hotline), lockSupportSize)
		k.support.Name, k.support.Align, k.support.Wrap = "p.muted", ui.TextAlignCenter, true
		k.Add(k.support)
	}

	contact := kit.Muted(e.T("locked.contact"), lockContactSize)
	contact.Name, contact.Align, contact.Wrap, contact.LineHeight = "p.muted", ui.TextAlignCenter, true, lockDescLine
	foot := newFoot(contact, geom.Insets{Top: footLockedTop, Right: footPadSide, Bottom: footPadBottom, Left: footPadSide}, false)

	return newFrame(k, lockedBottom, foot), k
}

// newLockedLesson returns the row of lesson l: the preview with its `XEM THỬ` chip,
// or a dim row with the lock that tells it opens when the user joins.
func newLockedLesson(e *env.Env, l model.Lesson) *kit.LessonRow {
	if l.Preview && !l.Locked {
		chip := kit.NewChip(kit.ChipY, e.T("locked.previewChip"))
		// The preview's tap is the player's, P4 (Q-P3-21); the prototype's toast of it
		// is a demo text (K2), so until then it does nothing.
		return lockedRow(l.No, l.Title, format.Join(l.Tag, e.T("locked.preview")), chip, false, func() {})
	}
	lock := kit.NewIcon(icon.Lock, lockIcon, theme.InkText)

	return lockedRow(l.No, e.Fill("locked.lessonTpl", "no", format.No(l.No)), e.T("locked.opensWhenEnrolled"), lock, true,
		func() { e.Act.Toast(e.T("locked.opensWhenEnrolled")) })
}

// lockedRow is a row of the locked course (index.html:410): a kit.LessonRow in
// kit.LessonModeLocked, `data-act="toast"`, with its trailing part and the dim of
// the locked lessons.
func lockedRow(no int, title, meta string, trailing ui.Widget, dim bool, onPress func()) *kit.LessonRow {
	return &kit.LessonRow{
		Mode: kit.LessonModeLocked, No: no, Title: title, Meta: meta, Trailing: trailing, Dim: dim,
		Pressable: ui.Pressable{Base: ui.Base{Name: "toast"}, OnPress: onPress},
	}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Rows returns the rows of the content, for the tests.
func (k *lockedContent) Rows() []*kit.LessonRow { return k.rows }

// Layout puts the hero across the window and the rest 20 from the sides, each block
// under the one before by its margins.
func (k *lockedContent) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	inner := max(w-2*detailSide, 0)
	col := ui.Constraints{Min: geom.Sz(inner, 0), Max: geom.Sz(inner, ui.Unbounded)}
	hs := c.Layout(k.hero, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(k.hero, geom.Point{})
	y := hs.H + watchTop
	put := func(wd ui.Widget) float32 {
		s := c.Layout(wd, col)
		c.Place(wd, geom.Pt(detailSide, y))

		return s.H
	}
	y += put(k.card) + lockHeadTop
	y += put(k.introH) + lockIntroBottom
	y += put(k.intro) + lockHeadTop
	y += put(k.contentH) + lockContentBottom
	for _, r := range k.rows {
		y += put(r)
	}
	if k.support != nil {
		y += lockSupportTop
		y += put(k.support)
	}
	y += lockSupportBottom

	return cs.Constrain(geom.Sz(w, y))
}

// Layout puts the disc at the left corner and the text beside it.
func (b *lockBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	c.Layout(&b.disc, ui.Tight(geom.Sz(lockDisc, lockDisc)))
	c.Place(&b.disc, geom.Point{})
	tw := max(w-lockDisc-lockGap, 0)
	col := ui.Constraints{Min: geom.Sz(tw, 0), Max: geom.Sz(tw, ui.Unbounded)}
	ts := c.Layout(b.title, col)
	c.Place(b.title, geom.Pt(lockDisc+lockGap, 0))
	bs := c.Layout(b.body, col)
	c.Place(b.body, geom.Pt(lockDisc+lockGap, ts.H+lockBodyTop))

	return cs.Constrain(geom.Sz(w, max(lockDisc, ts.H+lockBodyTop+bs.H)))
}

// Layout centres the glyph in the disc.
func (d *lockDiscView) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(lockDisc, lockDisc))
	c.Layout(d.glyph, ui.Tight(geom.Sz(lockIcon, lockIcon)))
	c.Place(d.glyph, geom.Pt((size.W-lockIcon)/2, (size.H-lockIcon)/2))

	return size
}

// Paint draws the round disc in --pr at .16.
func (d *lockDiscView) Paint(c *ui.Context, p *ui.Painter) {
	r := d.Bounds()
	p.Fill(r, scene.Solid(tokens.Dark.Primary.WithAlpha(lockDiscAlpha)), r.W/2) // `rgba(255,208,40,.16)`
	d.Base.Paint(c, p)
}

// lockDiscAlpha is the disc's `rgba(255,208,40,.16)` over --pr.
const lockDiscAlpha float32 = .16
