//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/kit/hero.go at 49de3c2 (a text block
// stood on the bottom of the art, the numbers as constants with their selector).
// The hero is the first block of MS.detail and MS.locked (index.html:396-397,
// :405).

package screens

import (
	"unicode/utf8"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
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

// The measures of the hero (index.html:396-397).
const (
	// heroH is `poster(id,393,300,{r:0,title:false})`: the cover is as wide as the
	// window (`margin:0 -20px`) and 300 tall.
	heroH float32 = 300
	// heroSide is `left:20px;right:20px` of the buttons and of the text block.
	heroSide float32 = 20
	// heroTextBottom is `bottom:6px` of the text block.
	heroTextBottom float32 = 6
	// heroTitleTop, heroTitleBottom are the `.d`'s `margin:8px 0 6px`.
	heroTitleTop, heroTitleBottom float32 = 8, 6
	// heroTitleSize, heroTitleSmall are `font-size:${c.name.length>18?30:36}px`,
	// heroTitleLong the length from which the small one stands.
	heroTitleSize, heroTitleSmall float32 = 36, 30
	heroTitleLong                         = 18
	// heroSubSize is the muted line's `font-size:13px`.
	heroSubSize float32 = 13
	// The scrim of the cover, `linear-gradient(rgba(0,0,0,.5),transparent 30%,
	// transparent 45%,var(--bg))`: the stops.
	heroScrimTop                     float32 = .5
	heroScrimClear, heroScrimBgStart float32 = .3, .45
)

// The names layout.json gives the hero's text.
const (
	nameHeroEyebrow = "div.eb.acc"
	nameHeroSub     = "div.muted"
	nameBack        = "back" // `data-act="back"`
)

// courseHero is the head of a course's screen: the cover with the scrim over it,
// the back button (and "more", which has no action, `$P:397`) at the top and the
// text block standing 6 above the bottom edge. In the detail the block is as wide
// as the window less 20 each side; in the locked course (`left:20px` only) it is
// as wide as its widest line.
type courseHero struct {
	ui.Base

	cover   *poster.Poster
	scrim   heroScrim
	back    *kit.RoundButton
	more    *kit.RoundButton // nil on the locked course
	eyebrow *kit.Label
	title   *kit.Label
	sub     *kit.Label
	shrink  bool
}

// heroScrim is `position:absolute;inset:0;background:linear-gradient(...)` over the
// cover.
type heroScrim struct {
	ui.Base

	paint   scene.Paint
	paintH  float32
	paintBg color.Color
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newCourseHero returns the hero of c: eyebrow, name and sub are the three lines of
// text; more adds the round button on the right; shrink makes the text block as
// wide as its widest line.
func newCourseHero(e *env.Env, c model.Course, eyebrow, sub string, more, shrink bool) *courseHero {
	h := &courseHero{shrink: shrink}
	spec := poster.ForCourse(c)
	spec.Radius, spec.NoTitle = 0, true // {r:0,title:false}
	h.cover = poster.New(spec, 0, heroH)
	h.cover.SetCover(e.Cover, c.CoverURL) // the real cover once loaded (K14)

	h.back = newBackButton(e)
	if more {
		h.more = kit.NewRoundButton(icon.More, true, e.T("a11y.more")) // `<button class="rb dk">`, no action
	}

	h.eyebrow = kit.Eyebrow(eyebrow)
	h.eyebrow.Ink, h.eyebrow.Name = theme.InkPrimary, nameHeroEyebrow // `div.eb.acc`
	h.eyebrow.Wrap = true
	size := heroTitleSize
	if utf8.RuneCountInString(c.Name) > heroTitleLong {
		size = heroTitleSmall
	}
	h.title = heading(kit.Display(c.Name, size), 1)
	h.title.Wrap = true
	h.sub = kit.Muted(sub, heroSubSize)
	h.sub.Name, h.sub.Wrap = nameHeroSub, true

	h.Add(h.cover, &h.scrim, h.back)
	if h.more != nil {
		h.Add(h.more)
	}
	h.Add(h.eyebrow, h.title, h.sub)

	return h
}

// newBackButton returns `backBtn(1)`: the dark round button with the back arrow.
func newBackButton(e *env.Env) *kit.RoundButton {
	b := kit.NewRoundButton(icon.Cl, true, e.T("a11y.back"))
	b.Name = nameBack
	b.OnPress = e.Act.Back

	return b
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout makes the hero the window's width and 300 tall: the cover and the scrim over
// all of it, the buttons at the top line, the text block on the bottom.
func (h *courseHero) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(cs.Max.W, heroH))
	full := ui.Tight(size)
	c.Layout(h.cover, full)
	c.Place(h.cover, geom.Point{})
	c.Layout(&h.scrim, full)
	c.Place(&h.scrim, geom.Point{})

	top := nav.ContentTop(c) // `top:54px`
	c.Layout(h.back, ui.Loose(size))
	c.Place(h.back, geom.Pt(heroSide, top))
	if h.more != nil {
		ms := c.Layout(h.more, ui.Loose(size))
		c.Place(h.more, geom.Pt(size.W-heroSide-ms.W, top))
	}

	room := max(size.W-2*heroSide, 0)
	bw := room
	if h.shrink {
		unb := ui.Loose(geom.Sz(room, ui.Unbounded))
		bw = max(c.Layout(h.eyebrow, unb).W, c.Layout(h.title, unb).W, c.Layout(h.sub, unb).W)
	}
	block := ui.Constraints{Min: geom.Sz(bw, 0), Max: geom.Sz(bw, ui.Unbounded)}
	es := c.Layout(h.eyebrow, block)
	ts := c.Layout(h.title, block)
	ss := c.Layout(h.sub, block)
	y := size.H - heroTextBottom - ss.H
	c.Place(h.sub, geom.Pt(heroSide, y))
	y -= heroTitleBottom + ts.H
	c.Place(h.title, geom.Pt(heroSide, y))
	y -= heroTitleTop + es.H
	c.Place(h.eyebrow, geom.Pt(heroSide, y))
	h.scrim.prepare(scrimBg(c), size.H)

	return size
}

// Paint draws the gradient over the cover.
func (s *heroScrim) Paint(_ *ui.Context, p *ui.Painter) { p.Fill(s.Bounds(), s.paint, 0) }

// Layout makes the scrim as large as the room.
func (s *heroScrim) Layout(_ *ui.Context, cs ui.Constraints) geom.Size { return cs.Constrain(cs.Max) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// prepare makes the paint for the height and --bg, once per change.
func (s *heroScrim) prepare(bg color.Color, h float32) {
	if s.paint.Gradient != nil && s.paintH == h && s.paintBg == bg {
		return
	}
	s.paintH, s.paintBg = h, bg
	// `transparent` is black at no opacity, so the first ramp is black .5 to none;
	// the second starts as --bg at no opacity, which is how CSS's premultiplied mix
	// reads `transparent` next to a color.
	s.paint = scene.Linear(geom.Point{}, geom.Pt(0, h),
		scene.Stop{Color: color.RGBA(0, 0, 0, heroScrimTop), Offset: 0},
		scene.Stop{Color: color.RGBA(0, 0, 0, 0), Offset: heroScrimClear},
		scene.Stop{Color: bg.WithAlpha(0), Offset: heroScrimBgStart},
		scene.Stop{Color: bg, Offset: 1})
}

// scrimBg is --bg, the color the hero and the footer fade into.
func scrimBg(c *ui.Context) color.Color { return theme.From(c).T.Bg }
