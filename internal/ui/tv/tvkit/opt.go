//go:build tvos

package tvkit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of `.topt` and its parts (index.html:196-198, :689-693).
const (
	// optHeight, optPadX, optRadius are `.topt{height:74px;padding:0 26px;
	// border-radius:18px}`.
	optHeight, optPadX, optRadius float32 = 74, 26, 18
	// optSize, optWeight are `.topt{font-size:30px;font-weight:600}`.
	optSize   float32     = 30
	optWeight text.Weight = 600
	// optSubSize, optSubOpacity are the speed row's hint, `<span style=
	// "font-size:22px;opacity:.6">`.
	optSubSize, optSubOpacity float32 = 22, .6
	// optSubGap stands for the space the markup puts between "0.5x" and the hint.
	optSubGap float32 = 8
	// optValueSize, optValueOpacity are the toggle row's value, `<span style=
	// "font-size:26px;opacity:.75">`.
	optValueSize, optValueOpacity float32 = 26, .75
	// optCheckSize, optCheckStroke are the current speed's `ic('check',34,3)`.
	optCheckSize, optCheckStroke float32 = 34, 3
)

// optSaid is the value a speed row speaks and the three things it was made from;
// Describe makes it again only when one of them changed, so a steady reading
// allocates nothing.
type optSaid struct {
	sub, now string
	check    bool
	text     string
	made     bool
}

// OptKind is the look of an Opt: the three kinds of row the panel has.
type OptKind uint8

// The kinds. OptKindUnknown reads as OptRow.
const (
	OptKindUnknown OptKind = iota
	// OptRow is a row with its text at the left, the hint after it and, at the
	// right, the check or the value (`o0..o3`, `ot0..ot2`).
	OptRow
	// OptClose is `oc`: the text centred on --sf2, 16 under the rows
	// (`style="margin-top:16px;justify-content:center;background:var(--sf2)"`).
	OptClose
)

// Opt is `.f.topt` (index.html:196-198): a row of the player's panel, 74 tall,
// 18 rounded, the text in 30/600 at the left, a hint after it in 22 at .6, and
// at the right the check of the current speed (34, stroke 3) or a value in 26
// at .75. Focused it is --pr with --on text, with no shadow and no scale
// (`#tv .topt.focus`, :198). `oc` has `background:var(--sf2)` inline, which
// which beats the focus rule (F-9): it stayed --sf2 under --on text, unreadable
// (K26 is accepted for this row, owner 2026-11-18); focused it is --pr with --on.
//
// Set Key to the `data-fid` (it is also the Name layout.json knows the row by)
// and OnSelect to the `data-tact`.
type Opt struct {
	Item
	// Kind picks the row or the close button.
	Kind OptKind
	// Text is the row's text; Sub the hint after it; Value the text at the
	// right.
	Text, Sub, Value string
	// Check shows the check at the right.
	Check bool
	// Now is what a screen reader adds to the hint of the row that has the check
	// ("Đang dùng", a11y.current): the check is a picture, and the selected trait is
	// the focus's (the bridge puts it on the focused node), so the current speed
	// says it in its value instead.
	Now string
	// Switch makes the row a switch for a screen reader (`ot0..ot2`), on while On
	// is set; its Value ("Bật", "Tắt") is spoken as the state, as the screen shows it.
	Switch, On bool

	label kit.Label
	sub   kit.Label
	value kit.Label
	glyph kit.Icon
	ready bool
	said  optSaid
	body  func(c *ui.Context, p *ui.Painter) // paintBody as a method value, made once
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewOpt returns a row of kind k with the key and the text.
func NewOpt(key string, k OptKind, txt string) *Opt {
	o := &Opt{Kind: k, Text: txt}
	o.Key, o.Name, o.Item.Label = key, key, txt

	return o
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetSub changes the hint; the same text changes nothing.
func (o *Opt) SetSub(s string) {
	if s != o.Sub {
		o.Sub = s
		o.sub.SetText(s)
		o.Relayout()
	}
}

// SetValue changes the text at the right; the same text changes nothing.
func (o *Opt) SetValue(s string) {
	if s != o.Value {
		o.Value = s
		o.value.SetText(s)
		o.Relayout()
	}
}

// Describe describes a row of the panel for a screen reader: the close button is
// a button; a speed (`o0..o3`, text and hint) a radio button valued by its hint
// and, while it has the check, by Now after it ("Chậm để tách động tác · Đang
// dùng"); a toggle (`ot0..ot2`) a switch in its state, valued by the Value it
// shows. Neither a speed nor a filter is a toggle for the bridge (it would speak 0
// or 1 in place of the hint, P8.3d dump) and a speed is not FlagSelected: the
// focused row is the one selected, and the current one would be a second.
// Adapted from jplayer-tv/internal/ui/kit/rowbtn.go
// at 781bee5 (RowBtn.Describe, `.sel` as FlagSelected).
func (o *Opt) Describe(c *ui.Context, n *platform.SemanticNode) {
	o.Item.Describe(c, n)
	n.Label, n.Value = o.Text, o.Value
	switch {
	case o.Kind == OptClose:
	case o.Switch:
		n.Role, n.Toggle = platform.RoleSwitch, platform.ToggledOff
		if o.On {
			n.Toggle = platform.ToggledOn
		}
	default:
		n.Role, n.Value = platform.RoleRadio, o.spoken()
	}
}

// spoken returns the hint, then Now when the row has the check, joined by " · ".
func (o *Opt) spoken() string {
	s := &o.said
	if s.made && s.sub == o.Sub && s.now == o.Now && s.check == o.Check {
		return s.text
	}
	s.sub, s.now, s.check, s.made = o.Sub, o.Now, o.Check, true
	switch {
	case !o.Check || o.Now == "":
		s.text = o.Sub
	case o.Sub == "":
		s.text = o.Now
	default:
		s.text = o.Sub + " · " + o.Now
	}

	return s.text
}

// SetCheck shows or hides the check.
func (o *Opt) SetCheck(on bool) {
	if o.Check != on {
		o.Check = on
		kit.SetHidden(&o.glyph, !on)
		o.Relayout()
	}
}

// Layout is the width it gets and 74: the text and its hint at the left, the
// check or the value at the right, all centred on the height.
func (o *Opt) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	o.attach()
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	size := cs.Constrain(geom.Sz(cs.Max.W, optHeight))
	ls := c.Layout(&o.label, free)
	if o.Kind == OptClose {
		c.Place(&o.label, geom.Pt((size.W-ls.W)/2, (size.H-ls.H)/2))

		return size
	}
	c.Place(&o.label, geom.Pt(optPadX, (size.H-ls.H)/2))
	if o.Sub != "" {
		c.Layout(&o.sub, free)
		// both are in one line: the hint sits on the text's baseline
		c.Place(&o.sub, geom.Pt(optPadX+ls.W+optSubGap, (size.H-ls.H)/2+o.label.Baseline()-o.sub.Baseline()))
	}
	right := size.W - optPadX
	if o.Check {
		gs := c.Layout(&o.glyph, ui.Tight(geom.Sz(optCheckSize, optCheckSize)))
		c.Place(&o.glyph, geom.Pt(right-gs.W, (size.H-gs.H)/2))
	}
	if o.Value != "" {
		vs := c.Layout(&o.value, free)
		c.Place(&o.value, geom.Pt(right-vs.W, (size.H-vs.H)/2))
	}

	return size
}

// Paint draws the row.
func (o *Opt) Paint(c *ui.Context, p *ui.Painter) {
	if o.body == nil {
		o.body = o.paintBody
	}
	o.Item.Scaled(c, p, o.body)
}

// LayoutStyle is `div.f.topt`: the fill, the radius and the text it holds.
func (o *Opt) LayoutStyle(c *ui.Context) (kit.CSS, string) {
	look := theme.From(c)
	s, _ := o.label.LayoutStyle(c)
	s.Color = kit.CSSColor(o.ink().Of(look))
	s.Background = kit.CSSColor(o.fill(look))
	s.Radius = "18px"
	txt := o.Text
	if o.Sub != "" {
		txt += " " + o.Sub
	}
	if o.Value != "" {
		txt += "\n" + o.Value
	}

	return s, txt
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// attach makes the parts the first time the row is laid out.
func (o *Opt) attach() {
	if o.ready {
		return
	}
	o.ready = true
	o.label.Role, o.label.Size, o.label.Weight, o.label.Text = theme.RoleBody, optSize, optWeight, o.Text
	o.sub.Role, o.sub.Size, o.sub.Weight, o.sub.Text = theme.RoleBody, optSubSize, optWeight, o.Sub
	o.value.Role, o.value.Size, o.value.Weight, o.value.Text = theme.RoleBody, optValueSize, optWeight, o.Value
	o.glyph.Glyph, o.glyph.Size, o.glyph.Stroke = icon.Check, optCheckSize, optCheckStroke
	o.glyph.Name = "svg.i"
	o.glyph.Hidden = !o.Check
	o.Add(&o.label, &o.sub, &o.value, &o.glyph)
}

// paintBody draws the fill and the parts, in the row's ink.
func (o *Opt) paintBody(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	p.Fill(o.Bounds(), scene.Solid(o.fill(look)), optRadius)
	ink := o.ink().Of(look)
	o.label.Color = ink
	o.sub.Color = ink.WithAlpha(optSubOpacity)
	o.value.Color = ink.WithAlpha(optValueOpacity)
	o.glyph.Color = ink
	o.Base.Paint(c, p)
}

// fill is --pr when focused, except for `oc`, whose inline --sf2 stays; with no
// fill at rest (the panel's --sf shows).
func (o *Opt) fill(look *theme.Look) color.Color {
	switch {
	case o.Kind == OptClose && !o.IsFocused():
		return look.T.Surface2 // K26 (accepted for `oc`): focused it is --pr, as the other rows
	case o.IsFocused():
		return look.T.Primary
	default:
		return color.Color{}
	}
}

// ink is --on when focused, --tx at rest.
func (o *Opt) ink() theme.Ink {
	if o.IsFocused() {
		return theme.InkOnPrimary
	}

	return theme.InkText
}
