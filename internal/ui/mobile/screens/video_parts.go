//go:build !tvos

package screens

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of screen 06's controls (MS.video, index.html:415-430, CSS
// :139-166, boxes of M06.layout.json).
const (
	// vTopH is the height of the top gradient: `inset:0 0 auto;height:130px`
	// (:417).
	vTopH float32 = 130
	// vTopRowDown is `top:58px` of the top row (:418): the content top (54) and 4.
	vTopRowDown float32 = 4
	// vSide, vSideTop, vSideGap are `.vside{right:14px;top:300px;gap:16px}` (:157).
	vSide, vSideTop, vSideGap float32 = 14, 300, 16
	// vSideAbove is how far the audio file's button stands below the column's top
	// in the prototype: the three buttons above it (speed, mirror, A-B loop,
	// 62 and a 16 gap each), which K38 took away. The button stays where it was.
	vSideAbove = 3 * (kit.SideHeight + vSideGap)
	// vPanelSide is `.vctrl{padding:0 20px 34px}` (:150): the side padding; the
	// bottom is max(34, the safe area's bottom) (Q-P2-1).
	vPanelSide, vPanelBottom float32 = 20, 34
	// vTopGap is the air kept between the course and lesson lines and each round
	// button: an addition of ours (K-ellipsis), the prototype's text never meets a
	// button, a long production course name does.
	vTopGap float32 = 12
	// vChipsGap, vChipsBottom are the chips row's `gap:8px;margin-bottom:10px`
	// (:424).
	vChipsGap, vChipsBottom float32 = 8, 10
	// vTitleSize is the title's `font-size:32px` (:425).
	vTitleSize float32 = 32
	// vCourseSize, vCourseTop, vCourseBottom are the course line's
	// `font-size:13px;opacity:.8;margin:6px 0 14px` (:426).
	vCourseSize, vCourseTop, vCourseBottom float32 = 13, 6, 14
	vCourseAlpha                           float32 = .8
	// vBeatsBottom is `#mBeats` `margin-bottom:12px` (:427).
	vBeatsBottom float32 = 12
	// vTimesTop, vTimesSize are the times row's `font-size:11px;font-weight:700;
	// margin-top:4px` (:429), vTimesDurAlpha the duration's `opacity:.7`.
	vTimesTop, vTimesSize float32 = 4, 11
	vTimesDurAlpha        float32 = .7
	// vTransportTop is the transport row's `margin-top:8px` (:430).
	vTransportTop float32 = 8
	// vSeekIcon is `ic('b10',28)` and `ic('f10',28)` (:430).
	vSeekIcon float32 = 28
	// vLesSize, vLesAlpha are the prev and next buttons' `font-weight:700;
	// opacity:.75;font-size:13px` (:430).
	vLesSize, vLesAlpha float32 = 13, .75
	// vEbAlpha is the course name's `color:rgba(255,255,255,.75)` (:418).
	vEbAlpha float32 = .75
	// vLineSize is the lesson line's `font-size:14px` (`<b>`, :418).
	vLineSize float32 = 14
	// vTopScrimAlpha is `linear-gradient(rgba(0,0,0,.6),transparent)` (:417).
	vTopScrimAlpha float32 = .6
	// vPanelShadeAt, vPanelShadeAlpha are `.vctrl{background:linear-gradient(
	// transparent,rgba(0,0,0,.85) 35%)}` (:150).
	vPanelShadeAt, vPanelShadeAlpha float32 = .35, .85
)

// The names layout.json gives the parts.
const (
	nameStageTap  = "stageTap"
	nameSheetOpen = "sheet:options"
	nameLoop      = "loop"
	// nameLoopBBack and nameLoopBFwd are the buttons of the options sheet that move
	// the loop's end (K38).
	nameLoopBBack = "loopB:-30"
	nameLoopBFwd  = "loopB:+30"
	nameTogglePl  = "togglePlay"
	namePrevLes   = "prevLes"
	nameNextLes   = "nextLes"
	nameSeekBack  = "seek:-10"
	nameSeekFwd   = "seek:10"
)

// stageButton is the picture of screen 06 as one element for a screen reader: a
// button named by the lesson's title whose press is the tap on the picture,
// `stageTap` (index.html:785): it shows the controls and starts the 3 s again, or,
// while the video plays and they are shown, hides them. It paints nothing and
// takes no touch. While the controls are hidden (hushed) it is all that is read.
type stageButton struct {
	ui.Base

	v *Video
}

var (
	_ ui.Describer     = (*stageButton)(nil)
	_ ui.SemanticActor = (*stageButton)(nil)
	_ ui.HitTester     = (*stageButton)(nil)
)

// deadZone is a part of the layer over the video that takes the presses nobody
// under it wants: the top gradient, the side column's box and the bottom panel lie
// over `#mStage` in the prototype and swallow a tap on their parts that are no
// button (F-10 of local/plans/P4.md). It does nothing with them.
type deadZone struct {
	ui.Base
}

var _ ui.PointerHandler = (*deadZone)(nil)

// vTop is the top of screen 06: the gradient, the back button, the course and the
// lesson in the middle and the "more" button that opens the options sheet.
type vTop struct {
	deadZone

	back  *kit.RoundButton
	more  *kit.RoundButton
	eb    *kit.Label
	line  *kit.Label
	block *inlineLine
	scrim scene.Paint
}

// vSideCol is `.vside`: the round buttons in a column. Since K38 it holds the
// audio file's button alone.
type vSideCol struct {
	deadZone

	dl *kit.SideButton
}

// vPanel is `.vctrl`: the chips, the title, the course line, the beats, the
// scrub bar with its times and the transport row, over a gradient that fades from
// nothing to black at .85.
type vPanel struct {
	deadZone

	tag, kind   *kit.Chip
	title       *kit.Label
	course      *kit.Label
	beats       *kit.Beats
	scrub       *kit.Scrub
	pad         *kit.SeekPad
	mpos, dur   *kit.Label
	prev, next  *lesButton
	back10, f10 *kit.IconButton
	toggle      *kit.DiscButton

	shade  scene.Paint
	shadeH float32
}

// lesButton is the text button of the transport row, `prevLes` and `nextLes`:
// 13/700 text in --tx at .75.
type lesButton struct {
	ui.Pressable

	label kit.Label
	text  string
	// speak, when set, names the button for a screen reader and the text is its
	// value (the speed button, whose text "0.75x" says nothing alone), or value is
	// when that is set (the prev button: "Bài trước", "02", not "‹ 02").
	speak, value string
}

var (
	_ ui.Labeler   = (*lesButton)(nil)
	_ ui.Describer = (*lesButton)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newVTop returns the top of the screen with the back button, and the "more"
// button named after the sheet it opens.
func newVTop(back, more *kit.RoundButton) *vTop {
	t := &vTop{back: back, more: more}
	t.eb = kit.Eyebrow("")
	t.eb.Ellipsis = true // a long course name ends in "…" between the buttons
	t.line = &kit.Label{Size: vLineSize, Weight: 700, Ink: theme.InkText, Ellipsis: true}
	t.block = newInline(t.line)
	t.scrim = scene.Linear(geom.Point{}, geom.Pt(0, vTopH),
		scene.Stop{Color: color.RGBA(0, 0, 0, vTopScrimAlpha), Offset: 0},
		scene.Stop{Color: color.RGBA(0, 0, 0, 0), Offset: 1})
	t.Add(back, t.eb, t.block, more)

	return t
}

// newLesButton returns a prev or next button named name.
func newLesButton(name string) *lesButton {
	b := &lesButton{}
	b.Name = name
	b.Look = func(*ui.Context, ui.Interaction) ui.Decoration { return ui.Decoration{} }
	b.label.Size, b.label.Weight = vLesSize, 700
	b.Add(&b.label)

	return b
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Describe describes the picture as a button named by the lesson's title.
func (s *stageButton) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label = platform.RoleButton, s.v.pb.Lesson().Title
	n.Actions |= platform.ActPress
}

// PerformAction taps the picture.
func (s *stageButton) PerformAction(_ *ui.Context, a platform.SemanticAction) bool {
	if a != platform.ActPress {
		return false
	}
	s.v.hide.StageTap()

	return true
}

// HitTest takes no touch: the screen's own handler has them.
func (*stageButton) HitTest(geom.Point) bool { return false }

// Layout is as large as the room it is given, over the picture.
func (*stageButton) Layout(_ *ui.Context, cs ui.Constraints) geom.Size { return cs.Max }

// Paint draws nothing.
func (*stageButton) Paint(*ui.Context, *ui.Painter) {}

// HandlePointer takes the press and its release, which lead nowhere.
func (d *deadZone) HandlePointer(_ *ui.Context, e *ui.PointerEvent) {
	switch e.Kind {
	case ui.PointerDown:
		if isPrimary(e) {
			e.Handle()
		}
	case ui.PointerUp, ui.PointerMove:
		e.Handle()
	case ui.PointerUnknown, ui.PointerEnter, ui.PointerLeave, ui.PointerCancel, ui.PointerScroll:
	default:
	}
}

// Layout makes the top as wide as it is given and 130 tall: the back button at
// 20, the "more" button at 20 from the right, both 40 and centered on the row 58
// from the top, and the course and lesson lines centered between them.
func (t *vTop) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(cs.Max.W, vTopH))
	look := theme.From(c)
	t.eb.Color = theme.InkWhite.Of(look).WithAlpha(vEbAlpha)
	y := nav.ContentTop(c) + vTopRowDown
	bs := c.Layout(t.back, ui.Loose(size))
	ms := c.Layout(t.more, ui.Loose(size))
	c.Place(t.back, geom.Pt(vPanelSide, y))
	c.Place(t.more, geom.Pt(size.W-vPanelSide-ms.W, y))

	free := max(size.W-2*vPanelSide-bs.W-ms.W, 0)
	room := ui.Loose(geom.Sz(max(free-2*vTopGap, 0), ui.Unbounded)) // never under a button
	es := c.Layout(t.eb, room)
	ls := c.Layout(t.block, room)
	bw, bh := max(es.W, ls.W), es.H+ls.H
	// `<div style="text-align:center">`: a block as wide as its widest line, in
	// the middle of the row (`justify-content:space-between` with two 40 DIP
	// buttons), each line centered in it.
	x := vPanelSide + bs.W + (free-bw)/2
	top := y + (max(bs.H, ms.H)-bh)/2
	c.Place(t.eb, geom.Pt(x+(bw-es.W)/2, top))
	c.Place(t.block, geom.Pt(x+(bw-ls.W)/2, top+es.H))

	return size
}

// Paint draws the gradient under the buttons.
func (t *vTop) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(t.Bounds(), t.scrim, 0)
	t.Base.Paint(c, p)
}

// Layout makes the column as wide as its widest button (46, or a label wider) and
// as tall as the buttons and their gaps.
func (s *vSideCol) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	btns := [...]*kit.SideButton{s.dl}
	w := float32(0)
	for _, b := range btns {
		if b.Hidden { // the audio file's button of a lesson with none
			c.Layout(b, ui.Tight(geom.Size{}))

			continue
		}
		w = max(w, c.Layout(b, ui.Loose(cs.Max)).W)
	}
	y := float32(0)
	for _, b := range btns {
		if b.Hidden {
			continue
		}
		c.Layout(b, ui.Tight(geom.Sz(w, kit.SideHeight)))
		c.Place(b, geom.Pt(0, y))
		y += kit.SideHeight + vSideGap
	}

	return cs.Constrain(geom.Sz(w, y-vSideGap))
}

// Layout stacks the parts from the top and puts the transport row 34 (or the
// safe area's bottom) above the bottom edge: the panel is as tall as they need.
func (p *vPanel) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	inner := max(w-2*vPanelSide, 0)
	look := theme.From(c)
	p.course.Color = look.T.Text.WithAlpha(vCourseAlpha)
	p.dur.Color = look.T.Text.WithAlpha(vTimesDurAlpha)
	p.prev.label.Color = look.T.Text.WithAlpha(vLesAlpha)
	p.next.label.Color = look.T.Text.WithAlpha(vLesAlpha)

	loose := ui.Loose(geom.Sz(inner, ui.Unbounded))
	tight := ui.Constraints{Min: geom.Sz(inner, 0), Max: geom.Sz(inner, ui.Unbounded)}
	y := float32(0)
	cs1 := c.Layout(p.tag, loose)
	cs2 := c.Layout(p.kind, loose)
	c.Place(p.tag, geom.Pt(vPanelSide, y))
	chipsGap := vChipsGap
	if p.tag.Hidden { // no tag, no chip and no gap after it
		chipsGap = 0
	}
	c.Place(p.kind, geom.Pt(vPanelSide+cs1.W+chipsGap, y))
	y += max(cs1.H, cs2.H) + vChipsBottom
	ts := c.Layout(p.title, tight)
	c.Place(p.title, geom.Pt(vPanelSide, y))
	y += ts.H + vCourseTop
	crs := c.Layout(p.course, tight)
	c.Place(p.course, geom.Pt(vPanelSide, y))
	y += crs.H + vCourseBottom
	if !p.beats.Hidden {
		bs := c.Layout(p.beats, tight)
		c.Place(p.beats, geom.Pt(vPanelSide, y))
		y += bs.H + vBeatsBottom
	}
	ss := c.Layout(p.scrub, tight)
	c.Place(p.scrub, geom.Pt(vPanelSide, y))
	p.pad.Place(c) // the touch target, over the bar, which keeps its 18 DIPs (K38)
	y += ss.H + vTimesTop
	mp := c.Layout(p.mpos, loose)
	dr := c.Layout(p.dur, loose)
	c.Place(p.mpos, geom.Pt(vPanelSide, y))
	c.Place(p.dur, geom.Pt(vPanelSide+inner-dr.W, y))
	y += max(mp.H, dr.H) + vTransportTop

	// the transport row: five items, space-between, each centered on the row
	items := [...]ui.Widget{p.prev, p.back10, p.toggle, p.f10, p.next}
	var sizes [len(items)]geom.Size
	sum, rowH := float32(0), float32(0)
	for i, it := range items {
		sizes[i] = c.Layout(it, ui.Loose(geom.Sz(inner, ui.Unbounded)))
		sum += sizes[i].W
		rowH = max(rowH, sizes[i].H)
	}
	gap := max(inner-sum, 0) / float32(len(items)-1)
	x := vPanelSide
	for i, it := range items {
		c.Place(it, geom.Pt(x, y+(rowH-sizes[i].H)/2))
		x += sizes[i].W + gap
	}
	y += rowH
	size := cs.Constrain(geom.Sz(w, y+max(vPanelBottom, c.SafeArea().Bottom)))
	p.prepare(size.H)

	return size
}

// Paint draws the gradient under the parts.
func (p *vPanel) Paint(c *ui.Context, pt *ui.Painter) {
	pt.Fill(p.Bounds(), p.shade, 0)
	p.Base.Paint(c, pt)
}

// AccessibleLabel returns the text of the button.
func (b *lesButton) AccessibleLabel() string { return b.text }

// Describe describes the button as a button named by its text, or by speak with
// the text as its value.
func (b *lesButton) Describe(c *ui.Context, n *platform.SemanticNode) {
	b.Pressable.Describe(c, n)
	if b.speak != "" {
		n.Label, n.Value = b.speak, b.text
		if b.value != "" {
			n.Value = b.value
		}
	}
}

// SetText changes the text; the same one changes nothing.
func (b *lesButton) SetText(s string) {
	if s != b.text {
		b.text = s
		b.label.SetText(s)
		b.Relayout()
	}
}

// Layout is as large as the text.
func (b *lesButton) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	ls := c.Layout(&b.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&b.label, geom.Point{})

	return cs.Constrain(ls)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// prepare makes the panel's gradient for a panel h tall, once per height.
func (p *vPanel) prepare(h float32) {
	if p.shade.Gradient != nil && p.shadeH == h {
		return
	}
	p.shadeH = h
	p.shade = scene.Linear(geom.Point{}, geom.Pt(0, h),
		scene.Stop{Color: color.RGBA(0, 0, 0, 0), Offset: 0},
		scene.Stop{Color: color.RGBA(0, 0, 0, vPanelShadeAlpha), Offset: vPanelShadeAt},
		scene.Stop{Color: color.RGBA(0, 0, 0, vPanelShadeAlpha), Offset: 1})
}

// seekIcon returns the 28 DIP icon button of a seek.
func seekIcon(name string, glyph icon.Name, label string) *kit.IconButton {
	b := kit.NewIconButton(glyph, label)
	b.Name, b.Size, b.Ink = name, vSeekIcon, theme.InkText

	return b
}
