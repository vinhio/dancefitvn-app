//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/iptv_page.go at 49de3c2 (tabChips:
// a row of tabs at the head of a page, the chosen one marked) for the tabs row, and
// from the pressable card's shape in home_parts.go (a Pressable with its Look taken
// off). The parts are the "now watching" card and the tabs of MS.detail
// (index.html:398-399).

package screens

import (
	"dancefitvn-app/internal/tokens"
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

// The measures of the "now watching" card (index.html:398).
const (
	// watchTop is the card's `margin-top:10px`; watchPad, watchGap `padding:14px`
	// and `gap:12px`.
	watchTop, watchPad, watchGap float32 = 10, 14, 12
	// tileSize, tileRadius are the play square's `width:40px;height:40px;
	// border-radius:12px`; its glyph sits in the box `padding:11px 10px 11px 13px`
	// leaves, 17 wide and 18 high, the 24 unit svg scaled to the narrower side.
	tileSize, tileRadius                float32 = 40, 12
	tileGlyphX, tileGlyphY, tileGlyphSz float32 = 13, 11.5, 17
	// watchTitleSize is `<b style="font-size:14px">`; watchSubSize, watchSubTop the
	// muted line's `font-size:12px;margin-top:3px`.
	watchTitleSize, watchSubSize, watchSubTop float32 = 14, 12, 3
	// allHeight, allRadius, allSize are `height:36px;border-radius:18px;
	// font-size:13px` of the "Tải hết" chip, allIcon its `ic('dl',18)`.
	allHeight, allRadius, allSize, allIcon float32 = 36, 18, 13, 18
)

// The measures of the tabs row (index.html:399).
const (
	// tabsTop, tabsBottom are `margin:22px 0 2px`; tabsGap `gap:24px`.
	tabsTop, tabsBottom, tabsGap float32 = 22, 2, 24
	// tabSize is `font-size:15px` of both tabs; tabPadBottom, tabLine are the chosen
	// one's `padding-bottom:10px;border-bottom:3px solid var(--pr)`; tabsLine is the
	// row's `border-bottom:1px solid var(--bd)`.
	tabSize, tabPadBottom, tabLine, tabsLine float32 = 15, 10, 3, 1
	// tabWeight is the chosen tab's `<b>`.
	tabWeight = 700
)

// The names layout.json gives the parts.
const nameTabMuted = "span.muted" // `<span class="muted">Giới thiệu</span>`

// playTile is the play square of the card: `span` 40 by 40, 12 rounded, --pr, the
// PLAY glyph in --on. It is a plain `span`: layout.json does not hold it.
type playTile struct {
	ui.Base

	glyph *kit.Icon
}

// watchText is the card's text, `<div style="flex:1;cursor:pointer" data-act=
// "playVideo">`: the title (a `<b>` on the page's 16 px strut, 20 tall) over the
// muted line, 3 below. It is a button.
type watchText struct {
	ui.Pressable

	title *inlineLine
	sub   *kit.Label
	label string
}

// chipButton is a button around a Chip: `<button class="chip" data-act="dlAll">`.
// The Name of layout.json is the chip's, so that its icon is recorded after it.
type chipButton struct {
	ui.Pressable

	chip *kit.Chip
}

// watchRow is the card's content: the tile, the text, which takes the room, and the
// chip, centred on the row, 12 apart.
type watchRow struct {
	ui.Base

	tile *playTile
	text *watchText
	all  *chipButton
}

// detailTabs is the row of the two tabs: "Bài học · n" chosen, with its 3 px line,
// and "Giới thiệu" muted; static, neither has an action ($P:399).
type detailTabs struct {
	ui.Base

	lessons *kit.Label
	intro   *kit.Label
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newPlayTile returns the play square.
func newPlayTile() *playTile {
	t := &playTile{glyph: kit.NewIcon(icon.Play, tileGlyphSz, theme.InkOnPrimary)}
	t.glyph.Name = "" // PLAY has no class: not in layout.json
	t.Add(t.glyph)

	return t
}

// newWatchRow returns the card's row: title and counts of the lesson to go on with,
// the download-all chip; onPlay is the text's tap, onAll the chip's. key is the
// lesson's key and courseKey the course's, for the names of layout.json.
func newWatchRow(title, counts, key, allText, courseKey string, onPlay, onAll func()) *watchRow {
	r := &watchRow{tile: newPlayTile()}
	t := &watchText{
		title: newInline(&kit.Label{Text: title, Size: watchTitleSize, Weight: 700, Wrap: true}),
		sub:   kit.Muted(counts, watchSubSize),
		label: title,
	}
	t.sub.Name, t.sub.Wrap = "div.muted", true
	t.Name = "playVideo:" + key // `data-act="playVideo" data-k`
	t.Look = noDecoration
	t.OnPress = onPlay
	t.Add(t.title, t.sub)
	r.text = t

	chip := kit.NewChip(kit.ChipKindUnknown, allText)
	chip.Name, chip.Icon, chip.IconSize = "dlAll:"+courseKey, icon.Dl, allIcon // `data-act="dlAll" data-set`
	chip.Height, chip.Radius, chip.Size = allHeight, allRadius, allSize
	chip.Bg, chip.Fg = tokens.Dark.Surface2, tokens.Dark.Text // `background:var(--sf2)`, `color:var(--tx)`
	b := &chipButton{chip: chip}
	b.Look = noDecoration
	b.OnPress = onAll
	b.Add(chip)
	r.all = b
	r.Add(r.tile, r.text, r.all)

	return r
}

// newDetailTabs returns the tabs row with the first tab's text.
func newDetailTabs(lessons, intro string) *detailTabs {
	t := &detailTabs{
		lessons: &kit.Label{Text: lessons, Size: tabSize, Weight: tabWeight},
		intro:   kit.Muted(intro, tabSize),
	}
	t.intro.Name = nameTabMuted
	t.Add(t.lessons, t.intro)

	return t
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout puts the glyph in the square.
func (t *playTile) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(tileSize, tileSize))
	c.Layout(t.glyph, ui.Tight(geom.Sz(tileGlyphSz, tileGlyphSz)))
	c.Place(t.glyph, geom.Pt(tileGlyphX, tileGlyphY))

	return size
}

// Paint draws the --pr square under the glyph.
func (t *playTile) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(t.Bounds(), scene.Solid(theme.From(c).T.Primary), tileRadius)
	t.Base.Paint(c, p)
}

// AccessibleLabel returns the title.
func (t *watchText) AccessibleLabel() string { return t.label }

// Layout stacks the title and the counts, the room's width.
func (t *watchText) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	col := ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}
	ts := c.Layout(t.title, col)
	c.Place(t.title, geom.Point{})
	ss := c.Layout(t.sub, col)
	c.Place(t.sub, geom.Pt(0, ts.H+watchSubTop))

	return cs.Constrain(geom.Sz(w, ts.H+watchSubTop+ss.H))
}

// AccessibleLabel returns the chip's text.
func (b *chipButton) AccessibleLabel() string { return b.chip.Text }

// Layout is the chip's size.
func (b *chipButton) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	s := c.Layout(b.chip, ui.Loose(geom.Sz(cs.Max.W, ui.Unbounded)))
	c.Place(b.chip, geom.Point{})

	return cs.Constrain(s)
}

// Layout puts the chip at the right, the text between the tile and the chip, all
// centred in the row's height (`align-items:center`).
func (r *watchRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	loose := ui.Loose(geom.Sz(w, ui.Unbounded))
	c.Layout(r.tile, ui.Tight(geom.Sz(tileSize, tileSize)))
	as := c.Layout(r.all, loose)
	tw := max(w-tileSize-watchGap-watchGap-as.W, 0)
	ts := c.Layout(r.text, ui.Constraints{Min: geom.Sz(tw, 0), Max: geom.Sz(tw, ui.Unbounded)})
	h := max(tileSize, ts.H, as.H)
	c.Place(r.tile, geom.Pt(0, (h-tileSize)/2))
	c.Place(r.text, geom.Pt(tileSize+watchGap, (h-ts.H)/2))
	c.Place(r.all, geom.Pt(w-as.W, (h-as.H)/2))

	return cs.Constrain(geom.Sz(w, h))
}

// Layout puts the two tabs side by side, 24 apart; the row is as tall as the chosen
// tab with its 10 of padding and its 3 line, and the second tab stretches to it
// (`align-items:stretch`); the row's own line is the 1 under it.
func (t *detailTabs) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	unb := ui.Loose(geom.Sz(w, ui.Unbounded))
	ls := c.Layout(t.lessons, unb)
	h := ls.H + tabPadBottom + tabLine
	c.Place(t.lessons, geom.Point{})
	iw := c.Layout(t.intro, unb).W
	c.Layout(t.intro, ui.Tight(geom.Sz(iw, h)))
	c.Place(t.intro, geom.Pt(ls.W+tabsGap, 0))

	return cs.Constrain(geom.Sz(w, h+tabsLine))
}

// Paint draws the chosen tab's 3 px line and the row's 1 px line under both.
func (t *detailTabs) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	b := t.Bounds()
	lw := t.lessons.Bounds().W
	p.Fill(geom.XYWH(0, b.H-tabsLine-tabLine, lw, tabLine), scene.Solid(look.T.Primary), 0)
	p.Fill(geom.XYWH(0, b.H-tabsLine, b.W, tabsLine), scene.Solid(look.T.Border), 0)
	t.Base.Paint(c, p)
}
