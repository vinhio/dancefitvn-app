// Adapted from jplayer-mobile/internal/ui/kit/field.go at 49de3c2.

package kit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/theme"
	"math"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/components/headless"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of .inp (index.html:107-111).
const (
	// inputHeight, inputRadius are `.inp{height:54px;border-radius:14px}` (:107).
	inputHeight, inputRadius float32 = 54, 14
	// inputPad, inputGap are `.inp{padding:0 14px;gap:10px}` (:107).
	inputPad, inputGap float32 = 14, 10
	// inputBorder is `.inp{border:1px solid var(--bd)}` (:107).
	inputBorder float32 = 1
	// inputRing is `.inp:focus-within` and `.inp.err` `border-width:1.6px` (:108-109).
	inputRing float32 = 1.6
	// inputSize is `.inp input{font-size:15px}` (:110).
	inputSize float32 = 15
	// searchHeight, searchRadius are `style="height:44px;border-radius:22px"`
	// of the search field (:376).
	searchHeight, searchRadius float32 = 44, 22
	// inputWidth is the width of an input given no width limit.
	inputWidth float32 = 240
)

// The names layout.json gives the frame: tag and classes.
const (
	nameInput    = "div.inp"
	nameInputErr = "div.inp.err"
	// nameInputField is the key of the `<input>` itself.
	nameInputField = "input"
)

// The numbers of .field (index.html:104-106, :112).
const (
	// FieldGap is `.field{margin-bottom:16px}` (:104): the room a form leaves
	// under a Field.
	FieldGap float32 = 16
	// fieldLabelSize, fieldLabelSpacing are `.field label{font-size:11px;
	// letter-spacing:1.2px}` (:105).
	fieldLabelSize, fieldLabelSpacing float32 = 11, 1.2
	// fieldLabelGap is `.field label{margin-bottom:8px}` (:105).
	fieldLabelGap float32 = 8
	// helpSize, helpGap, helpTop, helpIcon are `.help{font-size:12px;
	// margin-top:7px;gap:6px}` (:112) and `ic('warn',15)` (:345).
	helpSize, helpGap, helpTop, helpIcon float32 = 12, 6, 7, 15
)

// The names layout.json gives the parts of a field.
const (
	nameFieldLabel = "label"
	nameHelp       = "div.help"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Input is `.inp` (index.html:107-111): a 54 tall, 14 rounded box of --sf with a
// 1 px --bd border, padded 14, holding a leading icon (22, --tx2), the text
// field (15, --tx; placeholder --tx3) and a trailing widget, 10 apart. While the
// field has the focus the border is 1.6 --pr; with Error it is 1.6 --dg, which
// wins (:109 comes after :108). Search makes it 44 tall and 22 rounded (:376).
//
// The text is edited by a headless.TextField. Its Name is `input`; the frame is
// `div.inp` or `div.inp.err`.
type Input struct {
	ui.Base
	// Icon is the symbol before the field; icon.NameUnknown for none.
	Icon icon.Name
	// IconSize is the symbol's size; 0 reads as 22 (`ic('pin',18)`, :492).
	IconSize float32
	// Leading, when set, is shown before the field in place of the Icon (the
	// social platform name of :490).
	Leading ui.Widget
	// Trailing, when set, is shown after the field (the eye of the password,
	// :345).
	Trailing ui.Widget
	// Placeholder is the hint of an empty field, in --tx3.
	Placeholder string
	// Mask, when not zero, shows each character as it (`type="password"`).
	Mask rune
	// Error is the `.err` state: a --dg border.
	Error bool
	// Search is the 44 tall, fully rounded field of the course search.
	Search bool
	// Keyboard and EnterAction choose the on-screen keyboard.
	Keyboard    platform.KeyboardType
	EnterAction platform.EnterAction
	// Label names the field for assistive technologies; the placeholder does
	// when it is empty.
	Label string
	// OnChange is told the text after every edit the user makes.
	OnChange func(s string)
	// OnSubmit is told the text when the user presses Enter.
	OnSubmit func(s string)

	field    inputField
	icon     Icon
	attached bool
	hasIcon  bool // the icon child is added
	lead     ui.Widget
	trail    ui.Widget
	named    string // the label of the Field it is in
}

// inputField is the `<input>` of an Input: a headless.TextField that draws its
// own text, caret and selection.
type inputField struct {
	headless.TextField
	in *Input
}

var _ ui.Labeler = (*inputField)(nil)

// Field is `.field` (index.html:104-106, :112): a small upper-case label over an
// Input over an optional error line. Place it FieldGap above what follows.
type Field struct {
	ui.Base
	// Label is the text over the input.
	Label string
	// Required adds the red `*` of `<em>*</em>` (:479).
	Required bool
	// Help is the line under the input, shown in --dg after a warning icon;
	// empty for none.
	Help string
	// Input is the control.
	Input *Input

	label, em Label
	help      helpRow
	input     *Input // the Input that is a child
	attached  bool
	helpAdded bool
	// shownFor and shownReq are the Label the text with its trailing space was
	// made from, and that text: built once per label, not per layout.
	shownFor, shownReq string
}

// helpRow is `.help`: a warning icon and the text, 6 apart, as wide as the
// field.
type helpRow struct {
	ui.Base
	warn Icon
	text Label
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewInput returns an input with a placeholder and an optional leading icon.
func NewInput(icn icon.Name, placeholder string) *Input {
	return &Input{Icon: icn, Placeholder: placeholder}
}

// Text returns what the field holds.
func (in *Input) Text() string { return in.field.Text() }

// SetText replaces the text; OnChange is not told.
func (in *Input) SetText(s string) { in.field.SetText(s) }

// Field returns the text field, to type into it, focus it or read the caret.
func (in *Input) Field() *headless.TextField { return &in.field.TextField }

// SetMask changes the character a password is shown as (0 shows the text), laying
// the field out again; the same mask changes nothing.
func (in *Input) SetMask(r rune) {
	if r != in.Mask {
		in.Mask = r
		in.field.Mask = r
		in.field.Relayout()
	}
}

// SetError turns the error state on or off; the same state changes nothing.
func (in *Input) SetError(e bool) {
	if e != in.Error {
		in.Error = e
		in.autoName()
		in.Repaint()
	}
}

// Focused tells whether the text field has the focus.
func (in *Input) Focused() bool { return in.field.Focused() }

// nameBy gives the input the label of the Field it is in as its name for
// assistive technologies.
func (in *Input) nameBy(label string) { in.named = label }

// BorderWidth returns the width the frame's border is drawn at now: 1, or 1.6
// when focused or in error.
func (in *Input) BorderWidth() float32 {
	if in.Error || in.FocusWithin() {
		return inputRing
	}

	return inputBorder
}

// AccessibleLabel returns the field's label, its Field's, or its placeholder.
func (f *inputField) AccessibleLabel() string {
	switch {
	case f.in.Label != "":
		return f.in.Label
	case f.in.named != "":
		return f.in.named
	default:
		return f.Placeholder
	}
}

// DragExempt tells that a press on the field never starts the drag of a sheet
// around it.
func (f *inputField) DragExempt() bool { return true }

// Layout takes the room's width and the box's height, and places the icon, the
// field and the trailing widget in a row, 10 apart.
func (in *Input) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	in.init()
	in.autoName()
	h, face := inputHeight, theme.From(c).Faces.Face(theme.RoleBody, inputSize, 400)
	if in.Search {
		h = searchHeight
	}
	w := inputWidth
	if cs.Max.W < ui.Unbounded {
		w = cs.Max.W
	}
	size := cs.Constrain(geom.Sz(w, h))

	f := &in.field
	f.Font, f.Placeholder, f.Mask = face, in.Placeholder, in.Mask
	f.Keyboard, f.EnterAction = in.Keyboard, in.EnterAction
	f.OnSubmit = in.OnSubmit
	in.syncParts()

	x := inputBorder + inputPad
	end := size.W - inputBorder - inputPad
	loose := ui.Loose(geom.Sz(ui.Unbounded, size.H))
	if lead := in.leading(); lead != nil {
		ls := c.Layout(lead, loose)
		c.Place(lead, geom.Pt(x, centeredY(size.H, ls.H)))
		x += ls.W + inputGap
	}
	if in.trail != nil {
		ts := c.Layout(in.trail, loose)
		end -= ts.W
		c.Place(in.trail, geom.Pt(end, centeredY(size.H, ts.H)))
		end -= inputGap
	}
	// The `<input>` is as tall as Chrome's line box for its text.
	fh := inputLine(c, face)
	fs := c.Layout(f, ui.Tight(geom.Sz(max(end-x, 0), fh)))
	c.Place(f, geom.Pt(x, centeredY(size.H, fs.H)))

	return size
}

// Paint draws the box and its border; the children draw what is in it.
func (in *Input) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	t := &look.T
	r := SnapBox(p, in.Bounds()) // `.inp`: whole-CSS-px box (Chrome)
	radius := capRadius(r, inputRadius)
	if in.Search {
		radius = capRadius(r, searchRadius)
	}
	border, width := t.Border, inputBorder
	switch {
	case in.Error:
		border, width = t.Danger, inputRing
	case in.FocusWithin():
		border, width = t.Primary, inputRing
	default: // `.inp`: the plain 1 px --bd border
	}
	p.Fill(r, scene.Solid(t.Surface), radius)
	// `.inp:focus-within`, `.inp.err` border-width 1.6px: Chrome floors a border to
	// whole CSS px (the 3x shot of M02B draws the ring 3 device px thick).
	p.Border(r, geom.Uniform(float32(math.Floor(float64(width)))), border, radius)
	in.Base.Paint(c, p)
}

// Paint draws the text, or the placeholder, with the selection, the
// composition's underline and the caret.
func (f *inputField) Paint(c *ui.Context, p *ui.Painter) {
	t := &theme.From(c).T
	p.Clip(f.ContentRect(), 0, func() {
		o := f.TextOrigin()
		o.X += textShiftX()
		// `.inp input`: Chrome snaps the line box to whole CSS px before it adds the
		// ascent (M15C: a box at y 436.5 draws its text 0.67 pt lower than gDesk).
		top := p.Offset().Y + f.ContentRect().Y
		o.Y += float32(math.Floor(float64(top)+0.5)) - top
		l := f.TextLayout()
		sel, hasSel := f.SelectionRect()
		hasSel = hasSel && f.Focused()
		if hasSel {
			p.Fill(sel, scene.Solid(t.Primary), 0)
		}
		switch {
		case l != nil && len(l.Glyphs) > 0:
			p.Text(l, o, t.Text)
			if hasSel {
				p.Clip(sel, 0, func() { p.Text(l, o, t.OnPrimary) })
			}
		case f.Placeholder != "":
			p.Text(c.TextLayout(f.Placeholder, f.FontFace(c.Theme()), 0), o, t.Text3)
		default: // nothing typed and no placeholder: nothing to draw
		}
		if u, ok := f.CompositionRect(); ok {
			p.Fill(geom.XYWH(u.X, u.Y+u.H-1, u.W, 1), scene.Solid(t.Text), 0)
		}
		if f.CaretVisible() && !f.HasSelection() {
			p.Fill(f.CaretRect(), scene.Solid(t.Text), 0)
		}
	})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// init adds the children the first time.
func (in *Input) init() {
	if in.attached {
		return
	}
	in.attached = true
	in.field.in = in
	in.field.Name = nameInputField
	in.field.OnChange = in.changed
	in.icon.Name = iconName
	in.Add(&in.field)
}

// changed forwards an edit of the user to OnChange.
func (in *Input) changed(s string) {
	if in.OnChange != nil {
		in.OnChange(s)
	}
}

// autoName keeps the name of the frame in step with the error state while the
// caller set none.
func (in *Input) autoName() {
	if in.Name == "" || in.Name == nameInput || in.Name == nameInputErr {
		if in.Error {
			in.Name = nameInputErr
		} else {
			in.Name = nameInput
		}
	}
}

// leading returns the widget before the field: Leading, else the icon.
func (in *Input) leading() ui.Widget {
	if in.Leading != nil {
		return in.Leading
	}
	if in.Icon != icon.NameUnknown {
		return &in.icon
	}

	return nil
}

// syncParts makes the children match Leading, Icon and Trailing.
func (in *Input) syncParts() {
	in.icon.Glyph, in.icon.Size, in.icon.Ink = in.Icon, in.IconSize, theme.InkText2
	if in.lead != in.Leading {
		if in.lead != nil {
			in.Remove(in.lead)
		}
		if in.lead = in.Leading; in.lead != nil {
			in.Add(in.lead)
		}
	}
	if want := in.Leading == nil && in.Icon != icon.NameUnknown; want != in.hasIcon {
		if want {
			in.Add(&in.icon)
		} else {
			in.Remove(&in.icon)
		}
		in.hasIcon = want
	}
	if in.trail != in.Trailing {
		if in.trail != nil {
			in.Remove(in.trail)
		}
		if in.trail = in.Trailing; in.trail != nil {
			in.Add(in.trail)
		}
	}
}

// inputLine returns the height of the `<input>`: Chrome's line box for its font.
func inputLine(c *ui.Context, face text.Face) float32 {
	m, err := c.Shaper().Metrics(face)
	if err != nil {
		m = text.Metrics{Ascent: face.Size * .96875, Descent: face.Size * .2421875}
	}

	return theme.Chrome(m, 0).Height
}

// LabelFrame returns where the label is, in the field's coordinates, as of the
// last layout.
func (fl *Field) LabelFrame() geom.Rect { return fl.label.Frame() }

// HelpFrame returns where the help line is, in the field's coordinates, as of
// the last layout; the zero rectangle when there is none.
func (fl *Field) HelpFrame() geom.Rect {
	if fl.Help == "" {
		return geom.Rect{}
	}

	return fl.help.Frame()
}

// Layout stacks the label, the input and the help line.
func (fl *Field) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !fl.attached {
		fl.attached = true
		fl.label.Name, fl.help.Name, fl.help.warn.Name = nameFieldLabel, nameHelp, iconName
		fl.help.Add(&fl.help.warn, &fl.help.text)
		fl.Add(&fl.label, &fl.em)
	}
	// The children go in the order of the DOM (label, input, help) which is the
	// order of layout.json's keys; the help row is added after the input.
	if fl.input != fl.Input || !fl.helpAdded {
		if fl.helpAdded {
			fl.Remove(&fl.help)
		}
		if fl.input != fl.Input {
			if fl.input != nil {
				fl.Remove(fl.input)
			}
			if fl.input = fl.Input; fl.input != nil {
				fl.Add(fl.input)
			}
		}
		fl.Add(&fl.help)
		fl.helpAdded = true
	}
	if fl.input != nil {
		fl.input.nameBy(fl.Label)
	}
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = inputWidth
	}
	labelStyle{Role: theme.RoleBody, Size: fieldLabelSize, Weight: 700, Spacing: fieldLabelSpacing, Upper: true, Ink: theme.InkText2}.apply(&fl.label)
	labelStyle{Role: theme.RoleBody, Size: fieldLabelSize, Weight: 700, Spacing: fieldLabelSpacing, Upper: true, Ink: theme.InkDanger}.apply(&fl.em)
	// `<label>` is a block as wide as the field; its `<em>` follows the text
	// (a space, then the star).
	if fl.shownFor != fl.Label || fl.shownReq == "" {
		fl.shownFor, fl.shownReq = fl.Label, fl.Label+" "
	}
	if fl.Required {
		fl.label.SetText(fl.shownReq)
	} else {
		fl.label.SetText(fl.Label)
	}
	fl.em.SetText("*")
	fl.em.Hidden = !fl.Required

	ls := c.Layout(&fl.label, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(&fl.label, geom.Point{})
	if fl.Required {
		c.Layout(&fl.em, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
		c.Place(&fl.em, geom.Pt(fl.label.TextLayout().Size.W, 0))
	}
	y := ls.H + fieldLabelGap
	if fl.input != nil {
		is := c.Layout(fl.input, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
		c.Place(fl.input, geom.Pt(0, y))
		y += is.H
	}
	fl.help.Hidden = fl.Help == ""
	if fl.Help != "" {
		y += helpTop
		fl.help.text.SetText(fl.Help)
		hs := c.Layout(&fl.help, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
		c.Place(&fl.help, geom.Pt(0, y))
		y += hs.H
	}

	return cs.Constrain(geom.Sz(w, y))
}

// Layout puts the warning icon and the text in a row, centered, 6 apart.
func (h *helpRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	labelStyle{Role: theme.RoleBody, Size: helpSize, Ink: theme.InkDanger, Wrap: true}.apply(&h.text)
	h.warn.Glyph, h.warn.Size, h.warn.Ink = icon.Warn, helpIcon, theme.InkDanger
	w := cs.Max.W
	loose := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	c.Layout(&h.warn, loose)
	ts := c.Layout(&h.text, ui.Loose(geom.Sz(max(w-helpIcon-helpGap, 0), ui.Unbounded)))
	rowH := max(ts.H, helpIcon)
	c.Place(&h.warn, geom.Pt(0, centeredY(rowH, helpIcon)))
	c.Place(&h.text, geom.Pt(helpIcon+helpGap, centeredY(rowH, ts.H)))

	return cs.Constrain(geom.Sz(w, rowH))
}
