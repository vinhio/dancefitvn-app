//go:build !tvos

// Adapted from dancefitvn-app/internal/ui/mobile/screens/info_parts.go (the way a
// part that CSS lays out by line boxes is placed by the numbers its layout.json
// gives) and from dancefitvn-app/internal/ui/kit/chip.go (the chips). The parts are
// those of MS.coach (index.html:486-494, M15C) and the links of SH.avatar
// (index.html:524-525).

package screens

import (
	"math"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of MS.coach (index.html:486-494), in DIPs.
const (
	// coachLeadTop, coachLeadSize are `<p class="muted" style="font-size:12px;
	// margin-top:10px">`.
	coachLeadTop, coachLeadSize float32 = 10, 12
	// coachLinkTop, coachLinkLine are the link `margin:6px 0 18px;display:inline-block`
	// at 13/800: it stands 6 under the lead, and with its 18 and the line box it sits
	// on the field under it starts 44 under the lead (M15C: p.muted ends at 146, the
	// first label is at 190).
	coachLinkTop, coachLinkLine, coachLinkSize float32 = 7, 44, 13 // 7: a.acc text measured 1 pt under the 6 of the margin (inline-block on the parent line box, M15C 3x)
	// coachGapAfterArea is what a `<textarea>` leaves under it in a `.field`: the 4 of
	// the line it sits on and `.field{margin-bottom:16px}`.
	coachGapAfterArea = kit.TextAreaGap + kit.FieldGap
	// coachChipGap, coachChipHeight, coachChipPad, coachChipSize, coachChipSpacing are
	// `display:flex;gap:8px;flex-wrap:wrap` and `.chip` at `height:28px;padding:0 14px;
	// font-weight:800;letter-spacing:.8px`.
	coachChipGap, coachChipHeight, coachChipPad, coachChipSpacing float32 = 8, 28, 14, .8
	// coachLockTop, coachLockSize, coachLockIcon, coachLockLine are `<div class="muted"
	// style="font-size:12px;margin-top:8px">` with `ic('lock',13,2)`, a line of 16.
	coachLockTop, coachLockSize, coachLockIcon, coachLockStroke, coachLockLine float32 = 8, 12, 13, 2, 16
	// coachRowH, coachRowGap, coachRmSize, coachRmIcon are the row of a link or a place:
	// `.inp` at `height:44px;border-radius:22px` beside `.rb` of 44 with `ic('x',16)`,
	// `display:flex;gap:8px;margin-bottom:8px`.
	coachRowH, coachRowGap, coachRmSize, coachRmIcon float32 = 44, 8, 44, 16
	// coachSocTop, coachSocLine, coachSocHintTop are `+ Thêm liên kết` at
	// `margin-top:4px;font-size:13px;font-weight:800`: the button stands 4 under the
	// rows (their last margin included), and the hint at 11 px 27 under them (M15C:
	// rows end at 580, the button at 584, the hint at 607).
	coachSocTop, coachSocHintTop, coachSocHintSize float32 = 4, 27, 11
	// coachPlaceTop, coachPlaceLine are `+ Thêm địa điểm` with no margin: its line
	// box (the 16 px strut's 19.4) puts the 13 px button 3 under the rows, the field
	// ends 19.4 under them.
	coachPlaceTop, coachPlaceLine float32 = 3, 19.375
	// coachAddSize is the add buttons' `font-size:13px;font-weight:800`.
	coachAddSize float32 = 13
	// coachEnd is `<div style="height:90px">` under the last field.
	coachEnd float32 = 90
	// coachBarTop, coachBarSide, coachBarBottom are the fixed bar's `padding:14px 20px
	// 38px` around the 54 button.
	coachBarTop, coachBarSide, coachBarBottom float32 = 14, 20, 38
	// coachPlatSize is `<b class="acc" style="font-size:11px;text-transform:uppercase">`,
	// the platform in front of a link; coachPinIcon the `ic('pin',18)` of a place.
	coachPlatSize, coachPinIcon float32 = 11, 18
)

// The names layout.json gives the parts of MS.coach that are not a kit widget's.
const (
	nameRmSoc    = "rmSoc"    // `data-act="rmSoc"`
	nameRmPlace  = "rmPlace"  // `data-act="rmPlace"`
	nameAddSoc   = "addSoc"   // `data-act="addSoc"`
	nameAddPlace = "addPlace" // `data-act="addPlace"`
	nameSaveCoch = "saveCoach"
	nameMutedDiv = "div.muted"
	nameLinkView = "a.acc" // the link to the public page; the capture does not record it
)

// chipBlue is `background:#2F6FE0` of a subject's chip (index.html:489).
var chipBlue = color.RGB(0x2F, 0x6F, 0xE0)

// inlineLink is a button that is only text (and an icon): `<button>` with no frame at a
// given size and weight, as `+ Thêm liên kết` (13/800), the links of the avatar sheet
// (14/700 in --pr, with `ic('cam',18)`) and the anchor of the public page (underlined).
// With an icon the text's baseline is at the icon's bottom (an inline svg sits on
// the baseline), as the prototype's line boxes put it; the box is at least lineH
// tall.
type inlineLink struct {
	ui.Pressable

	icon      *kit.Icon // nil for none
	text      *kit.Label
	name      string  // what assistive technologies read
	iconSize  float32 // the icon's side
	lineH     float32 // the least height; 0 for the text's own
	underline bool    // the default underline of an `<a>`
	gap       float32 // the space between the icon and the text, once measured
}

// iconLine is a line of text that begins with an icon on its baseline, not pressable:
// `<div class="muted">${ic('lock',13,2)} text</div>`.
type iconLine struct {
	ui.Base

	icon     *kit.Icon
	text     *kit.Label
	iconSize float32
	lineH    float32
	gap      float32 // the space between the icon and the text, once measured
}

// rmButton is `<button class="rb" style="width:44px;height:44px;border:1px solid
// var(--bd)">${ic('x',16)}</button>`: a round button of --sf with a 1 px --bd border.
type rmButton struct {
	ui.Pressable

	icon  *kit.Icon
	label string
}

// entryRow is a row of a link or a place: the input (44 tall, pill) and the round
// button that removes it, 8 apart.
type entryRow struct {
	ui.Base

	input *kit.Input
	rm    *rmButton
	plat  *kit.Label // the platform in front of a link; nil for a place
}

// chipFlow is `display:flex;gap:8px;flex-wrap:wrap` of the subjects' chips.
type chipFlow struct {
	ui.Base
}

// rowStack is a column of rows each followed by `margin-bottom:8px`.
type rowStack struct {
	ui.Base
}

// coachBar is the fixed bar of the save button: `--bg` under the button, padded
// 14 20 38 (index.html:493).
type coachBar struct {
	ui.Base

	button *kit.Button
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newInlineLink returns a text button at size and weight in ink, with the icon icn
// (icon.NameUnknown for none) before the text.
func newInlineLink(text string, size float32, weight text.Weight, ink theme.Ink, icn icon.Name, iconSize float32, onPress func()) *inlineLink {
	l := &inlineLink{name: text, iconSize: iconSize}
	l.text = &kit.Label{Size: size, Weight: weight, Ink: ink}
	if icn != icon.NameUnknown {
		l.icon = kit.NewIcon(icn, iconSize, ink)
		l.Add(l.icon)
	}
	l.text.Text = text
	l.Add(l.text)
	l.OnPress = onPress
	l.Look = noDecoration

	return l
}

// newPickLink returns one of the links of the avatar sheet (index.html:525): 14/700
// in --pr, 21 tall, named `pickPhoto`.
func newPickLink(text string, icn icon.Name, onPress func()) *inlineLink {
	l := newInlineLink(text, avPickSize, 700, theme.InkPrimary, icn, avPickIcon, onPress)
	l.Name, l.lineH = namePickPhoto, avPickHeight

	return l
}

// newIconLine returns the line of text with the icon before it.
func newIconLine(icn icon.Name, iconSize, stroke float32, text string, size, lineH float32) *iconLine {
	n := &iconLine{iconSize: iconSize, lineH: lineH}
	n.icon = kit.NewIcon(icn, iconSize, theme.InkText2)
	n.icon.Stroke = stroke
	n.text = kit.Muted(text, size)
	n.Add(n.icon, n.text)

	return n
}

// newRmButton returns the button that removes a row, named key.
func newRmButton(key, label string, onPress func()) *rmButton {
	b := &rmButton{label: label}
	b.icon = kit.NewIcon(icon.X, coachRmIcon, theme.InkText)
	b.Name, b.OnPress, b.Look = key, onPress, noDecoration
	b.Add(b.icon)

	return b
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AccessibleLabel is the link's text.
func (l *inlineLink) AccessibleLabel() string { return l.name }

// Layout puts the icon at the top and the text on the icon's baseline.
func (l *inlineLink) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if l.icon != nil && l.gap == 0 {
		l.gap = spaceWidth(c, l.text)
	}

	return layoutInline(c, cs, l.icon, l.text, l.iconSize, l.gap, l.lineH)
}

// Paint draws the children, then the underline of an anchor under the text: Chrome's
// default, a 1 DIP rule on the row after the baseline.
func (l *inlineLink) Paint(c *ui.Context, p *ui.Painter) {
	l.Base.Paint(c, p)
	if !l.underline {
		return
	}
	f := l.text.Frame()
	off := p.Offset()
	base := float32(math.Floor(float64(off.Y+f.Y+l.text.Baseline()) + .5))
	p.Fill(kit.SnapBox(p, geom.XYWH(f.X, base+1-off.Y, l.text.TextLayout().Size.W, 1)), scene.Solid(theme.From(c).T.Primary), 0)
}

// Layout puts the icon at the top and the text on the icon's baseline, in a box as
// wide as the room.
func (n *iconLine) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if n.gap == 0 {
		n.gap = spaceWidth(c, n.text)
	}
	sz := layoutInline(c, cs, n.icon, n.text, n.iconSize, n.gap, n.lineH)
	// a `<div>`: a block as wide as its room
	if cs.Max.W < ui.Unbounded {
		sz.W = cs.Max.W
	}

	return cs.Constrain(sz)
}

// AccessibleLabel is the button's label.
func (b *rmButton) AccessibleLabel() string { return b.label }

// Layout is the 44 circle with the icon in its middle.
func (b *rmButton) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	is := c.Layout(b.icon, ui.Tight(geom.Sz(coachRmIcon, coachRmIcon)))
	c.Place(b.icon, geom.Pt((coachRmSize-is.W)/2, (coachRmSize-is.H)/2))

	return cs.Constrain(geom.Sz(coachRmSize, coachRmSize))
}

// Paint draws the disc with its border, then the icon.
func (b *rmButton) Paint(c *ui.Context, p *ui.Painter) {
	t := &theme.From(c).T
	r := kit.SnapBox(p, b.Bounds()) // `.rb`: whole-CSS-px box (Chrome)
	p.Fill(r, scene.Solid(t.Surface), r.W/2)
	p.Border(r, geom.Uniform(1), t.Border, r.W/2)
	b.Base.Paint(c, p)
}

// Layout puts the input and, at its end, the button, as tall as the row.
func (r *entryRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	c.Layout(r.rm, ui.Tight(geom.Sz(coachRmSize, coachRmSize)))
	c.Place(r.rm, geom.Pt(w-coachRmSize, 0))
	iw := max(w-coachRmSize-coachRowGap, 0)
	c.Layout(r.input, ui.Tight(geom.Sz(iw, coachRowH)))
	c.Place(r.input, geom.Point{})

	return cs.Constrain(geom.Sz(w, coachRowH))
}

// Layout flows the chips left to right, wrapping at the width, 8 apart each way.
func (f *chipFlow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	x, y := float32(0), float32(0)
	for ch := range f.Children() {
		sz := c.Layout(ch, ui.Loose(geom.Sz(w, ui.Unbounded)))
		if x > 0 && x+sz.W > w {
			x, y = 0, y+coachChipHeight+coachChipGap
		}
		c.Place(ch, geom.Pt(x, y))
		x += sz.W + coachChipGap
	}
	h := float32(0)
	if x > 0 || y > 0 {
		h = y + coachChipHeight
	}

	return cs.Constrain(geom.Sz(w, h))
}

// Layout stacks the rows, each followed by 8.
func (s *rowStack) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	y := float32(0)
	for ch := range s.Children() {
		sz := c.Layout(ch, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
		c.Place(ch, geom.Pt(0, y))
		y += sz.H + coachRowGap
	}

	return cs.Constrain(geom.Sz(w, y))
}

// Layout puts the button in its place: 20 from the sides, 38 from the bottom of the
// bar.
func (b *coachBar) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	bs := c.Layout(b.button, ui.Constraints{Min: geom.Sz(max(w-2*coachBarSide, 0), 0), Max: geom.Sz(max(w-2*coachBarSide, 0), ui.Unbounded)})
	c.Place(b.button, geom.Pt(coachBarSide, coachBarTop))

	return cs.Constrain(geom.Sz(w, coachBarTop+bs.H+coachBarBottom))
}

// Paint fills the bar with the page's background, then draws the button.
func (b *coachBar) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(b.Bounds(), scene.Solid(theme.From(c).T.Bg), 0)
	b.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// layoutInline lays out an optional icon at the top and a text after it whose
// baseline is at the icon's bottom (an inline svg sits on the baseline), the box at
// least lineH tall; with no icon the text is at the top.
func layoutInline(c *ui.Context, cs ui.Constraints, ic *kit.Icon, text *kit.Label, iconSize, gap, lineH float32) geom.Size {
	x, ty := float32(0), float32(0)
	if ic != nil {
		c.Layout(ic, ui.Tight(geom.Sz(iconSize, iconSize)))
		c.Place(ic, geom.Point{})
		x = iconSize + gap
	}
	room := float32(ui.Unbounded)
	if cs.Max.W < ui.Unbounded {
		room = max(cs.Max.W-x, 0)
	}
	ts := c.Layout(text, ui.Loose(geom.Sz(room, ui.Unbounded)))
	if ic != nil {
		ty = iconSize - text.Baseline()
	}
	c.Place(text, geom.Pt(x, ty))

	return cs.Constrain(geom.Sz(x+ts.W, max(lineH, ty+ts.H, iconSize*flagF(ic != nil))))
}

// spaceWidth returns the width of a space in the label's font: the one the source's
// `${ic(...)} text` has between the icon and the text.
func spaceWidth(c *ui.Context, l *kit.Label) float32 {
	face := theme.From(c).Faces.Face(l.Role, l.Size, l.Weight)

	// a layout of a lone space is empty: the space is what "a a" has over "aa"
	return c.TextLayout("a a", face, 0).Size.W - c.TextLayout("aa", face, 0).Size.W
}

// flagF is 1 for true, 0 for false.
func flagF(b bool) float32 {
	if b {
		return 1
	}

	return 0
}
