//go:build !tvos

// Adapted from dancefitvn-app/internal/ui/mobile/kitdemo/pages_account.go (the
// boxes of M15B that the demo lays out at absolute positions, here the flow that
// gives them: the same numbers, each from its selector in index.html:477-484).

package screens

import (
	"math"
	"strings"

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

// The measures of MS.info (index.html:477-484), in DIPs.
const (
	// infoTitleSize, infoTitleTop, infoTitle2Top are the two `.d` at
	// `font-size:30px`, `margin-top:14px` and `margin-top:2px`.
	infoTitleSize, infoTitleTop, infoTitle2Top float32 = 30, 14, 2
	// infoLeadSize, infoLeadTop, infoLeadBottom are `p.muted` at
	// `font-size:13px;margin:10px 0 22px`.
	infoLeadSize, infoLeadTop, infoLeadBottom float32 = 13, 10, 22
	// infoNameGap is the `gap:12px` of the two fields "Tên" and "Họ"
	// (`display:flex;gap:12px`, index.html:480). Each takes half of what the gap
	// leaves of the column. The prototype's `.field` has no `min-width:0`, so
	// its `<input>` stays 171 wide at least and the pair runs 414 wide off the
	// 353 of the column and off the screen (M15B [20,252,201,54] and
	// [233,252,201,54]): K29, which the owner approved fixing (2026-11-17):
	// the pair keeps to the column.
	infoNameGap float32 = 12
	// infoPwHeight, infoPwTop are the row of "Đổi mật khẩu" (`.row` 52 high, no
	// border) after the fields' `margin-bottom:16px`; infoPwGap the 12 between the
	// parts of the row, infoPwIcon its icons, infoPwIconTop where the lock's box
	// stands (`<span style="color:var(--tx)">` around the 20 icon: the box is the
	// icon on a line of the page's 16 px text, centred on the row, so the icon is
	// at its top, 2 over the middle: M15B svg.i#4 y 520 in a row at 506).
	infoPwHeight, infoPwGap, infoPwIcon, infoPwIconTop float32 = 52, 12, 20, 14
	// infoPwSize is `.row .tt b` at 15; infoPwUnder, infoPwOffset the underline
	// `text-decoration-thickness:3px;text-underline-offset:5px` in --pr.
	infoPwSize, infoPwUnder, infoPwOffset float32 = 15, 3, 5
	// infoNoteTop, infoNoteBottom are the card at `margin:10px 0 90px` (the 90 keeps
	// the button over the page's end clear of it); infoNotePad, infoNoteGap its
	// `padding:14px;gap:12px`; infoNoteIcon the `ic('info')` of 22; infoNoteSize the
	// text's `font-size:12px`.
	infoNoteTop, infoNoteBottom, infoNotePad, infoNoteGap, infoNoteIcon, infoNoteSize float32 = 10, 90, 14, 12, 22, 12
	// infoButtonSide, infoButtonBottom are the button's `left:20px;right:20px;
	// bottom:38px`; the button is the 54 of `.btn`.
	infoButtonSide, infoButtonBottom float32 = 20, 38
)

// The names layout.json gives the parts of the info page that are not a kit
// widget's.
const (
	nameInfoTT   = "span.tt"    // the title box of a row
	nameInfoMute = "span.muted" // the boxes of the note
)

// nameRow is the pair of fields "Tên" and "Họ" (`display:flex;gap:12px`).
type nameRow struct {
	ui.Base

	first, last *kit.Field
}

// pwLink is the bare row "Đổi mật khẩu" of the form (`.row` with `border:0`): the
// lock, the title underlined in --pr, the chevron.
type pwLink struct {
	ui.Pressable

	lock  *kit.Icon
	title *ttBox
	chev  *kit.Icon
}

// ttBox is `span.tt` around the underlined title: it paints the underline.
type ttBox struct {
	ui.Base

	label *kit.Label
}

// noteBox is the content of the card under the row: the info icon in a box of the
// text's height, and the text.
type noteBox struct {
	ui.Base

	box  *iconBox
	text *kit.Label
}

// iconBox is `span.muted` around an icon: a box of the icon's width and its
// neighbour's height.
type iconBox struct {
	ui.Base

	icon *kit.Icon
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newNameRow returns the pair of fields.
func newNameRow(first, last *kit.Field) *nameRow {
	r := &nameRow{first: first, last: last}
	r.Add(first, last)

	return r
}

// newPwLink returns the row, opening the sheet through onPress.
func newPwLink(title string, onPress func()) *pwLink {
	l := &pwLink{
		lock:  kit.NewIcon(icon.Lock, infoPwIcon, theme.InkText),
		title: &ttBox{label: &kit.Label{Text: title, Size: infoPwSize, Weight: 600}},
		chev:  kit.NewIcon(icon.Cr, infoPwIcon, theme.InkText),
	}
	l.lock.Stroke = 2 // `ic('lock',20,2)`
	l.title.Name, l.title.label.Name = nameInfoTT, "b"
	l.OnPress = onPress
	l.Look = func(*ui.Context, ui.Interaction) ui.Decoration { return ui.Decoration{} }
	l.Add(l.lock, l.title, l.chev)
	l.title.Add(l.title.label)

	return l
}

// newNoteBox returns the card's content.
func newNoteBox(text string) *noteBox {
	n := &noteBox{
		box:  &iconBox{icon: kit.NewIcon(icon.Info, infoNoteIcon, theme.InkText2)},
		text: &kit.Label{Text: text, Size: infoNoteSize, Ink: theme.InkText2, Wrap: true},
	}
	n.box.Name, n.text.Name = nameInfoMute, nameInfoMute
	n.box.Add(n.box.icon)
	n.Add(n.box, n.text)

	return n
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AccessibleLabel is the title of the row.
func (l *pwLink) AccessibleLabel() string { return l.title.label.Text }

// Layout puts the two fields side by side, each half of the room less the gap, so
// that the pair never passes the column (K29).
func (r *nameRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := max((cs.Max.W-infoNameGap)/2, 0)
	part := ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}
	fs := c.Layout(r.first, part)
	c.Place(r.first, geom.Point{})
	ls := c.Layout(r.last, part)
	c.Place(r.last, geom.Pt(w+infoNameGap, 0))

	return cs.Constrain(geom.Sz(cs.Max.W, max(fs.H, ls.H)))
}

// Layout puts the lock at its top, the title in the middle of the row, the chevron
// at the end.
func (l *pwLink) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	size := cs.Constrain(geom.Sz(w, infoPwHeight))
	c.Layout(l.lock, ui.Tight(geom.Sz(infoPwIcon, infoPwIcon)))
	c.Place(l.lock, geom.Pt(0, infoPwIconTop))
	c.Layout(l.chev, ui.Tight(geom.Sz(infoPwIcon, infoPwIcon)))
	c.Place(l.chev, geom.Pt(size.W-infoPwIcon, (size.H-infoPwIcon)/2))
	x := infoPwIcon + infoPwGap
	room := max(size.W-infoPwIcon-infoPwGap-x, 0)
	ts := c.Layout(l.title, ui.Constraints{Min: geom.Sz(room, 0), Max: geom.Sz(room, ui.Unbounded)})
	c.Place(l.title, geom.Pt(x, (size.H-ts.H)/2))

	return size
}

// Layout puts the label in its box, as wide as the room.
func (t *ttBox) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	ls := c.Layout(t.label, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}) // a block: as wide as its box
	c.Place(t.label, geom.Point{})

	return cs.Constrain(geom.Sz(w, ls.H))
}

// Paint draws the underline under the title's text, then the text.
func (t *ttBox) Paint(c *ui.Context, p *ui.Painter) {
	w := t.label.TextLayout().Size.W
	// Chrome puts the baseline on a whole pixel and draws the underline from there
	top := t.WindowFrame().Y
	y := float32(math.Floor(float64(top+t.label.Baseline())+.5)) - top + infoPwOffset
	p.Fill(geom.XYWH(0, y, w, infoPwUnder), scene.Solid(theme.From(c).T.Primary), 0)
	t.Base.Paint(c, p)
}

// Layout puts the icon and the text side by side, the box of the icon as tall as
// the text.
func (n *noteBox) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	room := max(w-infoNoteIcon-infoNoteGap, 0)
	ts := c.Layout(n.text, ui.Constraints{Min: geom.Sz(room, 0), Max: geom.Sz(room, ui.Unbounded)}) // a flex item: the room
	c.Layout(n.box, ui.Tight(geom.Sz(infoNoteIcon, max(ts.H, infoNoteIcon))))
	c.Place(n.box, geom.Point{})
	c.Place(n.text, geom.Pt(infoNoteIcon+infoNoteGap, 0))

	return cs.Constrain(geom.Sz(w, max(ts.H, infoNoteIcon)))
}

// Layout puts the icon at the top of its box.
func (b *iconBox) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(b.icon, ui.Tight(geom.Sz(infoNoteIcon, infoNoteIcon)))
	c.Place(b.icon, geom.Point{})

	return cs.Constrain(geom.Sz(infoNoteIcon, infoNoteIcon))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// phoneText groups a Vietnamese mobile number as the prototype shows it,
// "0905 123 456" (Q-P6-7); any other text is shown as it is.
func phoneText(p string) string {
	if len(p) != 10 || strings.Trim(p, "0123456789") != "" {
		return p
	}

	return p[:4] + " " + p[4:7] + " " + p[7:]
}

// phoneDigits is what is sent for the phone field: the digits typed, with the
// "+" of an international number kept (Q-P6-7).
func phoneDigits(s string) string {
	var b strings.Builder
	s = strings.TrimSpace(s)
	for i, r := range s {
		if r >= '0' && r <= '9' || (i == 0 && r == '+') {
			b.WriteRune(r)
		}
	}

	return b.String()
}
