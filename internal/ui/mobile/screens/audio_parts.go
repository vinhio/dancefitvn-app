//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/np/controls.go at 49de3c2 (the
// transport row spread over the column with the play button the tallest item,
// the row of three buttons under it) and np/meta.go (the title block and the
// times line). The parts are the controls of MS.audio (index.html:432-440,
// CSS :45-56, :84-85, :164-166, boxes of M08.layout.json).

package screens

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of screen 08 (MS.audio, index.html:432-440), in DIPs from the
// top of the window on a phone whose content starts at 54.
const (
	// aSide is `left:20px;right:20px` of the top row and of the block of the
	// lesson (:433, :435).
	aSide float32 = 20
	// aRowTop is the top row's `top:58px` (:433): the content top (54) and 4.
	aRowTop float32 = 58
	// aPosterTop, aPosterSize, aPosterRadius, aPosterTitle are `left:46px;top:
	// 120px` and `poster(l.cid,301,301,{r:26,ts:44})` (:434). The poster is
	// centered: 46 is (393 - 301) / 2.
	aPosterTop, aPosterSize, aPosterRadius, aPosterTitle float32 = 120, 301, 26, 44
	// aBlockTop is the block's `top:440px` (:435).
	aBlockTop float32 = 440
	// aTitleSize is the title's `.d` at `font-size:30px` (:435).
	aTitleSize float32 = 30
	// aSubTop, aSubSize, aSubBottom are the line's `font-size:14px;
	// margin:6px 0 12px` (:435).
	aSubTop, aSubSize, aSubBottom float32 = 6, 14, 12
	// aChipsGap is the chips row's `gap:8px` (:436).
	aChipsGap float32 = 8
	// aWaveTop is the wave's `margin-top:28px` (:437).
	aWaveTop float32 = 28
	// aTimesTop, aTimesSize are the times row's `font-size:12px;margin-top:10px`
	// (:438).
	aTimesTop, aTimesSize float32 = 10, 12
	// aCtlTop is the transport row's `margin-top:20px` (:439).
	aCtlTop float32 = 20
	// aLoopIcon, aSeekIcon are `ic('rep',24)` and `ic('b10',32)` (:439); aPlayD,
	// aPlayPad the play button's `width:76px;height:76px;padding:22px`.
	aLoopIcon, aSeekIcon, aPlayD, aPlayPad float32 = 24, 32, 76, 22
	// aBadgeSize, aBadgeIn, aBadgeUp are the text on the repeat button's corner
	// (K38, not in the prototype): 9 DIPs, 800, with this share of its width
	// inside the button's right edge and this many DIPs above its top.
	aBadgeSize, aBadgeIn, aBadgeUp float32 = 9, .6, 2
	// aSpeedSize is the speed button's `font-weight:800;font-size:15px`.
	aSpeedSize float32 = 15
	// aActsTop is the last row's `margin-top:34px` (:440); aActGap, aActSize its
	// buttons' `gap:6px` and the row's `font-size:11px`.
	aActsTop, aActGap, aActSize float32 = 34, 6, 11
	// aNameSize is the course name's `<b style="font-size:13px">` (:433).
	aNameSize float32 = 13
	// aTagAlpha is the tag chip's `background:rgba(255,208,40,.16)` (:436).
	aTagAlpha float32 = .16
	// aChipIcon, aChipStroke are `ic('dl',13,2.4)` of the downloaded chip.
	aChipIcon, aChipStroke float32 = 13, 2.4
	// aBgAt, aBgEnd, aBgAlpha are `radial-gradient(circle at 50% 20%,
	// rgba(255,208,40,.28), transparent 55%)` (:432).
	aBgAt, aBgEnd, aBgAlpha = .2, .55, .28
)

// The names layout.json gives the parts.
const (
	nameAudioMore  = "toast"      // `data-act="toast"` of the "more" button (:433)
	nameCycleSpeed = "cycleSpeed" // :439
	nameBeat       = "beat"       // :440
	nameBackRow    = "back"       // the list button's `data-act="back"` (:440)
	nameTimesRow   = "div.mono.muted"
	nameMuted      = "div.muted"
)

// audioTop is the top row: the back button, the course in the middle, the "more"
// button.
type audioTop struct {
	ui.Base

	back  *kit.RoundButton
	more  *kit.RoundButton
	eb    *kit.Label
	name  *kit.Label
	block *inlineLine
}

// timesRow is `<div style="display:flex;justify-content:space-between;
// font-size:12px;margin-top:10px" class="mono muted">`: the position at the left
// and the length at the right.
type timesRow struct {
	ui.Base

	pos, dur *kit.Label
}

// transport is the row of the controls: the loop, the two seeks, the play button
// and the speed, spread over the width (`justify-content:space-between`) and
// centered on the row.
type transport struct {
	ui.Base

	loop, back10, f10 *kit.IconButton
	toggle            *kit.DiscButton
	speed             *lesButton
	// badge is the small text on the corner of the repeat button that says which
	// repeat it is ("1", "A–B"; K38); hidden while the repeat is off.
	badge *repeatBadge
}

// repeatBadge is the label on the repeat button's corner. It takes no touch: a press
// on it is a press on the button under it.
type repeatBadge struct {
	kit.Label
}

var _ ui.HitTester = (*repeatBadge)(nil)

// actions is the last row: three buttons, an icon over a line of text, spread
// with space around them (`justify-content:space-around`).
type actions struct {
	ui.Base

	btns [3]*actButton
}

// actButton is `<button style="display:flex;flex-direction:column;
// align-items:center;gap:6px;...">`: a 22 DIP icon over a line of 11 px text, in
// --tx2 at 500 or, when on, --pr at 700.
type actButton struct {
	ui.Pressable
	// toggle makes the button read as a toggle that On says is on (the beats).
	toggle bool

	icon  *kit.Icon
	label kit.Label
	text  string
	on    bool
}

var (
	_ ui.Labeler   = (*actButton)(nil)
	_ ui.Describer = (*actButton)(nil)
)

// chipsRow is the two chips side by side, 8 apart.
type chipsRow struct {
	ui.Base

	tag     *kit.Chip
	okc     *kit.Chip // downloaded, with its size
	stream  *kit.Chip // not on the device
	showOkc bool
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newAudioTop returns the top row around the two buttons.
func newAudioTop(back, more *kit.RoundButton) *audioTop {
	t := &audioTop{back: back, more: more}
	t.eb = kit.Eyebrow("")
	t.eb.Align = ui.TextAlignCenter
	t.name = &kit.Label{Size: aNameSize, Weight: 700, Ink: theme.InkText}
	t.block = newInline(t.name)
	t.Add(back, t.eb, t.block, more)

	return t
}

// newTimesRow returns the row with 00:00 at both ends.
func newTimesRow() *timesRow {
	r := &timesRow{}
	r.Name = nameTimesRow
	mk := func() *kit.Label {
		return &kit.Label{Role: theme.RoleMono, Size: aTimesSize, Ink: theme.InkText2}
	}
	r.pos, r.dur = mk(), mk()
	r.Add(r.pos, r.dur)

	return r
}

// newActButton returns the button of glyph g reading text, named name.
func newActButton(name string, g icon.Name, text string) *actButton {
	b := &actButton{text: text}
	b.Name = name
	b.Look = func(*ui.Context, ui.Interaction) ui.Decoration { return ui.Decoration{} }
	b.icon = kit.NewIcon(g, 0, theme.InkText2)
	b.label = kit.Label{Size: aActSize, Weight: 500, Ink: theme.InkText2}
	b.label.SetText(text)
	b.Add(b.icon, &b.label)

	return b
}

// newChipsRow returns the row: the tag chip (rgba(255,208,40,.16) under --pr),
// then the downloaded chip or the streaming one (--sf2 under --tx2).
func newChipsRow(downloaded, streaming string) *chipsRow {
	r := &chipsRow{}
	r.tag = kit.NewChip(kit.ChipKindUnknown, "")
	r.okc = kit.NewChip(kit.ChipOkc, downloaded)
	r.okc.Icon, r.okc.IconSize, r.okc.IconStroke = icon.Dl, aChipIcon, aChipStroke
	r.stream = kit.NewChip(kit.ChipKindUnknown, streaming)
	r.Add(r.tag, r.okc, r.stream)
	r.show(false)

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout puts the back button at 20, the "more" button at 20 from the right, both
// 40 and centered on the row, and the course in the middle, a block as wide as its
// widest line with the lines centered in it.
func (t *audioTop) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Max
	bs := c.Layout(t.back, ui.Loose(size))
	ms := c.Layout(t.more, ui.Loose(size))
	c.Place(t.back, geom.Pt(aSide, 0))
	c.Place(t.more, geom.Pt(size.W-aSide-ms.W, 0))
	room := ui.Loose(geom.Sz(max(size.W-2*aSide-bs.W-ms.W, 0), ui.Unbounded))
	es := c.Layout(t.eb, room)
	ls := c.Layout(t.block, room)
	bw, bh := max(es.W, ls.W), es.H+ls.H
	x := aSide + bs.W + (size.W-2*aSide-bs.W-ms.W-bw)/2
	top := (max(bs.H, ms.H) - bh) / 2
	// `.eb` is a block in the centered div: as wide as the widest line, its text
	// centered in it (layout.json's `div.eb` is the 104.03 of the course name).
	c.Layout(t.eb, ui.Tight(geom.Sz(bw, es.H)))
	c.Place(t.eb, geom.Pt(x, top))
	c.Place(t.block, geom.Pt(x+(bw-ls.W)/2, top+es.H))

	return geom.Sz(size.W, max(bs.H, ms.H))
}

// Layout puts the position at the left and the length at the right.
func (r *timesRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	loose := ui.Loose(geom.Sz(w, ui.Unbounded))
	ps, ds := c.Layout(r.pos, loose), c.Layout(r.dur, loose)
	c.Place(r.pos, geom.Point{})
	c.Place(r.dur, geom.Pt(w-ds.W, 0))

	return cs.Constrain(geom.Sz(w, max(ps.H, ds.H)))
}

// HitTest takes no touch.
func (*repeatBadge) HitTest(geom.Point) bool { return false }

// Layout spreads the five items over the width with equal gaps, each centered on
// the row, as tall as the play button.
func (t *transport) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	items := [...]ui.Widget{t.loop, t.back10, t.toggle, t.f10, t.speed}
	var sizes [len(items)]geom.Size
	sum, rowH := float32(0), float32(0)
	for i, it := range items {
		sizes[i] = c.Layout(it, ui.Loose(geom.Sz(w, ui.Unbounded)))
		sum += sizes[i].W
		rowH = max(rowH, sizes[i].H)
	}
	gap := max(w-sum, 0) / float32(len(items)-1)
	x := float32(0)
	for i, it := range items {
		c.Place(it, geom.Pt(x, (rowH-sizes[i].H)/2))
		x += sizes[i].W + gap
	}
	// the badge sits on the repeat button's upper right corner
	bs := c.Layout(t.badge, ui.Loose(geom.Sz(w, ui.Unbounded)))
	lf := t.loop.Frame()
	c.Place(t.badge, geom.Pt(lf.X+lf.W-bs.W*aBadgeIn, lf.Y-aBadgeUp))

	return cs.Constrain(geom.Sz(w, rowH))
}

// Layout spreads the three buttons with the free room shared around them: half a
// share before each and half after.
func (a *actions) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	var sizes [len(a.btns)]geom.Size
	sum, rowH := float32(0), float32(0)
	for i, b := range a.btns {
		sizes[i] = c.Layout(b, ui.Loose(geom.Sz(w, ui.Unbounded)))
		sum += sizes[i].W
		rowH = max(rowH, sizes[i].H)
	}
	half := max(w-sum, 0) / float32(2*len(a.btns))
	x := float32(0)
	for i, b := range a.btns {
		x += half
		c.Place(b, geom.Pt(x, 0))
		x += sizes[i].W + half
	}

	return cs.Constrain(geom.Sz(w, rowH))
}

// AccessibleLabel returns the text of the button.
func (b *actButton) AccessibleLabel() string { return b.text }

// Describe describes the button as a button named by its text; a toggle says
// whether it is on.
func (b *actButton) Describe(c *ui.Context, n *platform.SemanticNode) {
	b.Pressable.Describe(c, n)
	if b.toggle {
		n.Toggle = platform.ToggledOff
		if b.on {
			n.Toggle = platform.ToggledOn
		}
	}
}

// SetOn makes the button the "on" one (--pr at 700) or the plain one (--tx2 at
// 500); the same state changes nothing.
func (b *actButton) SetOn(on bool) {
	if on == b.on {
		return
	}
	b.on = on
	ink, weight := theme.InkText2, text.Weight(500)
	if on {
		ink, weight = theme.InkPrimary, 700
	}
	b.label.Ink, b.label.Weight = ink, weight
	b.icon.Ink = ink
	b.label.Relayout()
	b.Repaint()
}

// On tells whether the button is the "on" one.
func (b *actButton) On() bool { return b.on }

// SetText changes the text; the same one changes nothing.
func (b *actButton) SetText(s string) {
	if s != b.text {
		b.text = s
		b.label.SetText(s)
		b.Relayout()
	}
}

// Layout is as wide as the wider of the icon and the text, with the icon over the
// text 6 apart, both centered.
func (b *actButton) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	loose := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	is := c.Layout(b.icon, loose)
	ls := c.Layout(&b.label, loose)
	w := max(is.W, ls.W)
	c.Place(b.icon, geom.Pt((w-is.W)/2, 0))
	c.Place(&b.label, geom.Pt((w-ls.W)/2, is.H+aActGap))

	return cs.Constrain(geom.Sz(w, is.H+aActGap+ls.H))
}

// show picks the chip of the file: the downloaded one with its size, or the one
// that says the sound streams.
func (r *chipsRow) show(downloaded bool) {
	r.showOkc = downloaded
	kit.SetHidden(r.okc, !downloaded)
	kit.SetHidden(r.stream, downloaded)
	r.Relayout()
}

// Layout puts the tag chip and the file's chip side by side, 8 apart, and gives
// the chips their colors (tokens, read at layout).
func (r *chipsRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	t := &theme.From(c).T
	r.tag.Bg, r.tag.Fg = t.Primary.WithAlpha(aTagAlpha), t.Primary
	r.stream.Bg, r.stream.Fg = t.Surface2, t.Text2
	loose := ui.Loose(cs.Max)
	ts := c.Layout(r.tag, loose)
	c.Place(r.tag, geom.Point{})
	other := r.stream
	if r.showOkc {
		other = r.okc
	}
	os := c.Layout(other, loose)
	// A lesson with no tag has no tag chip: the file's chip starts the row, with no gap
	// before it (a flex item that is not there takes no gap, index.html:436).
	gap := aChipsGap
	if r.tag.Hidden {
		gap = 0
	}
	c.Place(other, geom.Pt(ts.W+gap, 0))

	return cs.Constrain(geom.Sz(ts.W+gap+os.W, max(ts.H, os.H)))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// audioPoster returns the cover of the lesson's course: 301 square, rounded 26,
// the title at 44.
func audioPoster() *poster.Poster {
	return poster.New(poster.Spec{Radius: aPosterRadius, TitleSize: aPosterTitle}, aPosterSize, aPosterSize)
}
