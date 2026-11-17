package kit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of .radio (index.html:133-136).
const (
	// radioPadV, radioPadH, radioGap, radioRadius are `.radio{gap:12px;
	// padding:12px 14px;border-radius:14px}` (:133).
	radioPadV, radioPadH, radioGap, radioRadius float32 = 12, 14, 12, 14
	// RadioGap is `.radio{margin-bottom:10px}` (:133): the room a list of
	// them leaves under each.
	RadioGap float32 = 10
	// radioDot, radioDotBorder are `.radio .dot{width:20px;height:20px;
	// border:2px solid var(--tx3)}` (:135).
	radioDot, radioDotBorder float32 = 20, 2
	// radioDotInner is the dot of the chosen one: `.dot:after{inset:3px}` in
	// the 16 px padding box of a 20 px dot with a 2 px border is 10 across
	// (:136; 02 section 6 says 14, the code wins).
	radioDotInner float32 = 10
	// radioTitleSize, radioSubSize, radioTrailSize are `font-size:14px` of the
	// title, 12px of the sub line and 13px of the size at the end (:512).
	radioTitleSize, radioSubSize, radioTrailSize float32 = 14, 12, 13
	// radioWidth is the width of a radio given no width limit.
	radioWidth float32 = 280
	// radioIconSize is the `ic(n,22)` of a radio with a symbol (the device
	// rows of the sheet of devices, P6 section P6.4: no prototype design);
	// radioDisabled is the `opacity:.45` of a row that takes no press, as
	// `.btn[disabled]` (:57).
	radioIconSize, radioDisabled float32 = 22, .45
)

// The names layout.json gives the parts of a radio.
const (
	nameRadioDot = "span.dot"
	// nameRadioTitle is empty: the radio's `<b>` and its size are not recorded
	// (M09 names the `b` of the rows only).
	nameRadioTitle = ""
	nameRadioSub   = "div.muted"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Radio is `.radio` (index.html:133-136, :512): a 14 rounded row on --bg with a
// 1 px --bd border, padded 12 and 14, with a 20 dot, a title (14/700) over a sub
// line (12, --tx2) and a 13/700 text at the end, 12 apart. The chosen one has a
// 1.6 px --pr border and a --pr dot with a 10 DIP center. The Radio takes the
// press; the page moves the choice.
//
// Set the Name to the data key (`dlChoice:one`).
type Radio struct {
	ui.Pressable
	// On is the chosen state; change it with SetOn.
	On bool
	// Title is the first line, 14/700.
	Title string
	// Sub is the second line, 12 --tx2; empty for none.
	Sub string
	// Trailing is the text at the end, 13/700 (the size of the download).
	Trailing string
	// Icon, when set, is a 22 symbol in --tx2 between the dot and the text (a
	// device row of the sheet of devices); icon.NameUnknown for none.
	Icon icon.Name

	dot      radioDotBox
	ico      Icon
	text     radioText
	trail    Label
	attached bool
	// dimmed, c and p paint a disabled radio under its layer: made once, so
	// that a frame allocates nothing.
	dimmed func()
	c      *ui.Context
	p      *ui.Painter
}

// radioDotBox is `.radio .dot`: the ring and, when the radio is chosen, its
// center.
type radioDotBox struct {
	ui.Base
	r *Radio
}

// radioText is the `<span style="flex:1">` of a radio: a title line and a
// sub line.
type radioText struct {
	ui.Base
	title, sub Label
	strut      theme.LineBox
	hasSub     bool
}

var _ ui.Labeler = (*Radio)(nil)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewRadio returns a radio with a title, a sub line and the size at the end.
func NewRadio(title, sub, trailing string) *Radio {
	return &Radio{Title: title, Sub: sub, Trailing: trailing}
}

// SetOn chooses or unchooses the radio; the same state changes nothing.
func (r *Radio) SetOn(on bool) {
	if on != r.On {
		r.On = on
		r.Repaint()
	}
}

// AccessibleLabel returns the title.
func (r *Radio) AccessibleLabel() string { return r.Title }

// Describe describes the radio as a radio button, selected when chosen. It sets
// no Toggle: the iOS bridge turns a toggle into a "toggle button" that speaks 0
// or 1 (P8.3d dump); a choice is a button that is selected.
func (r *Radio) Describe(c *ui.Context, n *platform.SemanticNode) {
	r.Pressable.Describe(c, n)
	n.Role, n.Value = platform.RoleRadio, r.Sub
	if r.On {
		n.Flags |= platform.FlagSelected
	}
}

// Layout fills the width it gets; the dot, the text and the trailing size are
// centered in the height.
func (r *Radio) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	r.init()
	w := radioWidth
	if cs.Max.W < ui.Unbounded {
		w = cs.Max.W
	}
	loose := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	inset := borderInset + radioPadH
	avail := w - 2*inset - radioDot - radioGap
	r.ico.Glyph, r.ico.Hidden = r.Icon, r.Icon == icon.NameUnknown
	textX := inset + radioDot + radioGap
	if r.Icon != icon.NameUnknown {
		avail -= radioIconSize + radioGap
		textX += radioIconSize + radioGap
	}
	var ts geom.Size
	r.trail.Hidden = r.Trailing == ""
	if r.Trailing != "" {
		r.trail.SetText(r.Trailing)
		ts = c.Layout(&r.trail, loose)
		avail -= ts.W + radioGap
	}
	r.text.strut = strut(c)
	r.text.hasSub = r.Sub != ""
	r.text.title.SetText(r.Title)
	r.text.sub.SetText(r.Sub)
	r.text.sub.Hidden = r.Sub == ""
	tt := c.Layout(&r.text, ui.Constraints{Max: geom.Sz(max(avail, 0), ui.Unbounded)})

	content := max(tt.H, ts.H, radioDot)
	h := content + 2*radioPadV + 2*borderInset
	size := cs.Constrain(geom.Sz(w, h))
	inner := size.H - 2*borderInset - 2*radioPadV
	top := borderInset + radioPadV
	c.Layout(&r.dot, ui.Tight(geom.Sz(radioDot, radioDot)))
	c.Place(&r.dot, geom.Pt(inset, top+centeredY(inner, radioDot)))
	if r.Icon != icon.NameUnknown {
		c.Layout(&r.ico, ui.Tight(geom.Sz(radioIconSize, radioIconSize)))
		c.Place(&r.ico, geom.Pt(inset+radioDot+radioGap, top+centeredY(inner, radioIconSize)))
	}
	c.Place(&r.text, geom.Pt(textX, top+centeredY(inner, tt.H)))
	if r.Trailing != "" {
		c.Place(&r.trail, geom.Pt(size.W-inset-ts.W, top+centeredY(inner, ts.H)))
	}

	return size
}

// Paint draws the radio, at .45 opacity while it is Disabled.
func (r *Radio) Paint(c *ui.Context, p *ui.Painter) {
	if !r.Disabled {
		r.paint(c, p)

		return
	}
	if r.dimmed == nil {
		r.dimmed = func() { r.paint(r.c, r.p) }
	}
	r.c, r.p = c, p
	p.Layer(radioDisabled, 0, r.dimmed)
	r.c, r.p = nil, nil
}

// paint draws the box and its border; the children draw the dot and the text.
func (r *Radio) paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	t := &look.T
	b := r.Bounds()
	radius := capRadius(b, radioRadius)
	border, width := t.Border, borderInset
	if r.On {
		border, width = t.Primary, inputRing
	}
	p.Fill(b, scene.Solid(t.Bg), radius)
	p.Border(b, geom.Uniform(width), border, radius)
	if r.FocusVisible() {
		focusRing(p, b, radius, t.Primary)
	}
	r.Base.Paint(c, p)
}

// Layout stacks the title line and the sub line. The title is an inline run in
// the strut's line box (20 tall, baseline 16 down); the sub line is a block of
// its own height.
func (t *radioText) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	loose := ui.Loose(geom.Sz(w, ui.Unbounded))
	ts := c.Layout(&t.title, loose)
	line := max(t.strut.Height, ts.H)
	c.Place(&t.title, geom.Pt(0, max(t.strut.Baseline-t.title.Baseline(), 0)))
	h := line
	if t.hasSub {
		// the sub line is a `div`: as wide as the text column
		ss := c.Layout(&t.sub, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
		c.Place(&t.sub, geom.Pt(0, h))
		h += ss.H
	}

	return cs.Constrain(geom.Sz(w, h))
}

// Paint draws the ring of the dot and, when chosen, its center.
func (d *radioDotBox) Paint(c *ui.Context, p *ui.Painter) {
	t := &theme.From(c).T
	b := d.Bounds()
	ring := t.Text3
	if d.r.On {
		ring = t.Primary
	}
	p.Border(b, geom.Uniform(radioDotBorder), ring, radioDot/2)
	if d.r.On {
		in := geom.XYWH((b.W-radioDotInner)/2, (b.H-radioDotInner)/2, radioDotInner, radioDotInner)
		p.Fill(in, scene.Solid(t.Primary), radioDotInner/2)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// init adds the parts the first time and gives them the names of layout.json.
func (r *Radio) init() {
	if r.attached {
		return
	}
	r.attached = true
	r.Look = noLook
	r.dot.r = r
	r.ico.Size, r.ico.Ink, r.ico.Name = radioIconSize, theme.InkText2, iconName
	r.dot.Name, r.text.title.Name, r.text.sub.Name = nameRadioDot, nameRadioTitle, nameRadioSub
	r.trail.Name = nameRadioTitle
	r.text.Add(&r.text.title, &r.text.sub)
	r.Add(&r.dot, &r.ico, &r.text, &r.trail)
	labelStyle{Role: theme.RoleBody, Size: radioTitleSize, Weight: 700, Ink: theme.InkText, Wrap: true}.apply(&r.text.title)
	labelStyle{Role: theme.RoleBody, Size: radioSubSize, Ink: theme.InkText2, Wrap: true}.apply(&r.text.sub)
	labelStyle{Role: theme.RoleBody, Size: radioTrailSize, Weight: 700, Ink: theme.InkText}.apply(&r.trail)
}
