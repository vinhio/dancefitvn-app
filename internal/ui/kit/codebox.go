package kit

import (
	"strings"
	"unicode"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/components/headless"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of .codebox (index.html:176-177, :500).
const (
	// CodeLength is the length of a pairing code: `slice(0,6)` (:570).
	CodeLength = 6
	// codeW, codeH, codeRadius are `.codebox{width:46px;height:60px;
	// border-radius:12px}` (:176).
	codeW, codeH, codeRadius float32 = 46, 60, 12
	// codeSize is `.codebox{font-size:30px}` in Barlow Condensed 700 (:176).
	codeSize float32 = 30
	// codeBorder is `.codebox{border:1px solid var(--bd)}`; codeFocusBorder is
	// `.codebox.f{border:1.8px solid var(--pr)}` (:177).
	codeBorder, codeFocusBorder float32 = 1, 1.8
	// codeGap is the `gap:8px` of the row (:500).
	codeGap float32 = 8
	// codeDashW, codeDashH are the 12 x 3 `<i>` before the fourth box (:500).
	codeDashW, codeDashH float32 = 12, 3
	// codeDashAt is the index of the box the dash comes before: `i===3`.
	codeDashAt = 3
	// codeRowW is the width of the whole row: six boxes, the dash and 6 gaps.
	codeRowW = CodeLength*codeW + codeDashW + CodeLength*codeGap
)

// The names layout.json gives the parts: the boxes by their class, the
// focused one with `.f`.
const (
	// nameCodeRow and nameCodeDash are empty: the row and the dash are bare
	// `<div>` and `<i>` the capture does not record in M16.
	nameCodeRow   = ""
	nameCodeBox   = "span.codebox"
	nameCodeBoxF  = "span.codebox.f"
	nameCodeDash  = ""
	nameCodeInput = "input"
)

// The letters a Vietnamese keyboard makes of the keys of a code are read as the
// key that stands for them in a code (codeKey): a letter with marks is the letter
// under them, the D with a stroke is D, and the U with a horn is W, the key that
// types it alone on a Telex keyboard (a W key is how a code gets its W: the
// keyboard of iOS types U+01AF for it, B-code3). They are the precomposed letters
// of Unicode's Latin-1 Supplement, Latin Extended-A and B, and of Latin Extended
// Additional from U+1EA0 on, where each base letter's are one run, upper case on
// the even code points.
const (
	viAFirst, viALast = 0x1EA0, 0x1EB7 // A with a dot below ... A with breve and dot below
	viEFirst, viELast = 0x1EB8, 0x1EC7 // E with a dot below ... E with circumflex and dot below
	viIFirst, viILast = 0x1EC8, 0x1ECB // I with a hook above, I with a dot below
	viOFirst, viOLast = 0x1ECC, 0x1EE3 // O with a dot below ... O with horn and dot below
	viUFirst, viULast = 0x1EE4, 0x1EE7 // U with a dot below, U with a hook above
	viWFirst, viWLast = 0x1EE8, 0x1EF1 // U with horn and acute ... U with horn and dot below
	viYFirst, viYLast = 0x1EF2, 0x1EF9 // Y with grave ... Y with tilde
	viABreve          = 0x0102         // A with breve
	viDStroke         = 0x0110         // D with stroke
	viITilde          = 0x0128         // I with tilde
	viUTilde          = 0x0168         // U with tilde
	viOHorn           = 0x01A0         // O with horn
	viUHorn           = 0x01AF         // U with horn
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// CodeBoxes is the pairing code field of the TV screen (index.html:176-177, :500):
// six 46 x 60 boxes of --sf with a 1 px --bd border and the character in Barlow
// Condensed 700 30, 8 apart, a 12 x 3 --tx3 dash before the fourth, the row
// centered. The box the next character goes to (`min(len,5)`) has a 1.8 --pr
// border. A hidden field over the row takes the typing: it is upper-cased, only
// 0-9 and A-Z are kept, and at most six (`oninput`, :570). A tap anywhere on the
// row focuses it.
//
// The field has no caret: a code is typed, deleted and pasted at its end, as in
// `<input id="codeInp">` of a prototype whose caret nobody sees. A character goes
// after the last one and Backspace takes the last one back, whatever was tapped.
//
// The field asks for gDesk's keyboard of a code (platform.KeyboardCode) and lends
// the keyboard no text (it is no ui.TextDocument). A keyboard that reads the text
// it types into rewrites it: the Vietnamese keyboards of iOS select the letters
// typed before, delete them and type them again with their marks (`a` then `s` is
// `á`), which a field holding only 0-9 and A-Z cannot follow, so that the boxes
// lost, doubled and moved their letters from the first such key on (B-code3: `7`,
// `a`, `s` showed `77`). For a code the system's keyboard is lent no text and,
// on iOS, told to forget each key on the turn after it; on Android it is asked
// for a visible password without suggestions. It has nothing to put a mark on,
// and every key arrives as the key reads.
//
// What a keyboard still combines counts as the keys of a code (FilterCode): an
// input method that composes all the same (an Android keyboard that pays no heed
// to the input type) shows its composition in the boxes from the moment it is
// composed, a letter with marks as the letter under them, and the W key the iOS
// Telex keyboard types alone as `Ư` is W; the composition is committed when the
// field loses the focus.
type CodeBoxes struct {
	ui.Base
	// Value is the code typed so far, at most six characters, an input method's
	// composition included; change it with SetValue.
	Value string
	// Label names the field for assistive technologies.
	Label string
	// OnChange is told the code after every edit of the user that changes it.
	OnChange func(code string)
	// Error is the state of a code the server refused: every box has a 1.8 --dg
	// border (the `.err` of an input, no prototype design: the prototype shows
	// a toast only). The next edit of the user takes it back; change it with
	// SetError.
	Error bool

	boxes    [CodeLength]codeBox
	dash     codeDash
	field    codeField
	attached bool
}

// codeBox is `span.codebox`: one character.
type codeBox struct {
	ui.Base
	Decor // the characters are read as the field's value, not one by one

	label   Label
	focused bool
	err     bool
}

// codeDash is the `<i>` before the fourth box.
type codeDash struct {
	ui.Base
	Decor
}

// codeField is the `<input id="codeInp">` of the row: it takes the keyboard's text
// and keys over the whole row and draws nothing (`opacity:0`). It holds the code
// itself, not a text with a caret, and must stay no ui.TextDocument (see
// CodeBoxes).
type codeField struct {
	ui.Base

	row *CodeBoxes
	// code is the characters entered, as FilterCode leaves them; composing is an
	// input method's composition, as it sent it.
	code, composing string
}

var (
	_ ui.Focuser             = (*codeField)(nil)
	_ ui.PointerHandler      = (*codeField)(nil)
	_ ui.KeyHandler          = (*codeField)(nil)
	_ ui.TextKeyboardHandler = (*codeField)(nil)
	_ ui.FocusHandler        = (*codeField)(nil)
	_ ui.Describer           = (*codeField)(nil)
	_ ui.Labeler             = (*codeField)(nil)
	_ ui.Labeler             = (*CodeBoxes)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewCodeBoxes returns an empty row with the label of its field.
func NewCodeBoxes(label string) *CodeBoxes { return &CodeBoxes{Label: label} }

// SetValue replaces the code, filtered as typing is, and ends a composition;
// OnChange is not told.
func (c *CodeBoxes) SetValue(s string) {
	v := FilterCode(s)
	if v == c.Value && c.field.code == v && c.field.composing == "" {
		return
	}
	c.Value, c.field.code, c.field.composing = v, v, ""
	c.Relayout()
}

// SetError turns the error state on or off; the same state changes nothing.
func (c *CodeBoxes) SetError(e bool) {
	if e != c.Error {
		c.Error = e
		c.Relayout()
	}
}

// Field returns the hidden field, to focus it or to ask whether it is focused
// (Node().Focused()).
func (c *CodeBoxes) Field() ui.Widget { return &c.field }

// FocusedBox returns the index of the box that shows the focus border:
// `min(len, 5)`, the characters being those shown, a composition's included.
func (c *CodeBoxes) FocusedBox() int { return min(len(c.Value), CodeLength-1) }

// AccessibleLabel returns the field's label.
func (c *CodeBoxes) AccessibleLabel() string { return c.Label }

// AccessibleLabel returns the row's label.
func (f *codeField) AccessibleLabel() string { return f.row.Label }

// Describe describes the field as a text field valued by the code.
func (f *codeField) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label, n.Value = platform.RoleTextField, f.row.Label, f.row.Value
}

// AcceptsFocus lets the field take the focus unless it is disabled or hidden.
func (f *codeField) AcceptsFocus() bool { return !f.Disabled && !f.Hidden }

// DragExempt tells that a press on the field never starts the drag of a sheet
// around it.
func (f *codeField) DragExempt() bool { return true }

// Layout takes the size of the row.
func (f *codeField) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Sz(codeRowW, codeH))
}

// Paint draws nothing: the field is `opacity:0`; the boxes show the code.
func (f *codeField) Paint(*ui.Context, *ui.Painter) {}

// HandlePointer takes a press, which focuses the field: there is no caret to
// place and nothing to select.
func (f *codeField) HandlePointer(_ *ui.Context, e *ui.PointerEvent) {
	if e.Kind == ui.PointerDown && (e.Button == platform.ButtonPrimary || e.Button == platform.ButtonNone) {
		e.Handle()
	}
}

// HandleKey takes the last character back at Backspace and pastes the clipboard
// after the code at Cmd+V or Ctrl+V. The keys that move a caret, Delete and Enter
// are taken and do nothing, so that they do not reach the page.
func (f *codeField) HandleKey(_ *ui.Context, e *ui.KeyEvent) {
	if !e.Down || f.Disabled {
		return
	}
	switch e.Key {
	case keys.KeyBackspace:
		if n := len(f.code); n > 0 {
			f.code = f.code[:n-1] // the code is ASCII: one byte a character
		}
	case keys.KeyV:
		if !e.Mods.Has(keys.ModSuper) && !e.Mods.Has(keys.ModCtrl) {
			return
		}
		if s, err := headless.ClipboardFor(f.View(), nil).ReadText(); err == nil {
			f.code = FilterCode(f.code + s)
		}
	case keys.KeyLeft, keys.KeyRight, keys.KeyUp, keys.KeyDown, keys.KeyHome, keys.KeyEnd, keys.KeyDelete, keys.KeyEnter:
	default:
		return
	}
	e.Handle()
	f.row.follow()
}

// HandleText puts typed text after the code, and shows an input method's
// composition after it until the text that commits it, or an empty composition
// that takes it back.
func (f *codeField) HandleText(_ *ui.Context, e *ui.TextEvent) {
	if e.Composing {
		f.composing = e.Text
	} else {
		f.composing = ""
		f.code = FilterCode(f.code + e.Text)
	}
	f.row.follow()
}

// TextCaret returns the box the next character goes to, where the keyboard and
// what it shows beside the text belong, while the field is enabled.
func (f *codeField) TextCaret() (geom.Rect, bool) {
	return f.row.boxes[f.row.FocusedBox()].Frame(), !f.Disabled
}

// TextKeyboard asks for the keyboard of a code (B-code3, see CodeBoxes), its Enter
// key the system's, on one line.
func (f *codeField) TextKeyboard() (platform.KeyboardType, platform.EnterAction, bool) {
	return platform.KeyboardCode, platform.EnterUnknown, false
}

// FocusChanged commits the composition of an input method when the field loses
// the focus: its characters are in the boxes and in Value, and the keyboard going
// away is what ends them.
func (f *codeField) FocusChanged(_ *ui.Context, focused bool) {
	if !focused && f.composing != "" {
		f.code, f.composing = FilterCode(f.code+f.composing), ""
		f.row.follow()
	}
}

// FilterCode returns the pairing code s makes: upper-cased, only 0-9 and A-Z
// kept, at most six characters (`toUpperCase().replace(/[^0-9A-Z]/g,”)
// .slice(0,6)`, index.html:570). A letter a Vietnamese keyboard made of the keys
// of a code counts as its key, where the prototype drops it: a letter with marks
// is the letter under them, and the U with a horn is W (see viAFirst).
func FilterCode(s string) string {
	if cleanCode(s) {
		return s // the common case, a code typed a character at a time, costs nothing
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		k := codeKey(unicode.ToUpper(r))
		if k == 0 {
			continue
		}
		b.WriteByte(k)
		if n++; n == CodeLength {
			break
		}
	}

	return b.String()
}

// Layout takes the width it gets (the row is centered in it) and 60 of height.
func (c *CodeBoxes) Layout(ctx *ui.Context, cs ui.Constraints) geom.Size {
	c.init()
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = codeRowW
	}
	size := cs.Constrain(geom.Sz(w, codeH))
	shown := c.Value
	focus := c.FocusedBox()
	x := (size.W - codeRowW) / 2
	for i := range c.boxes {
		if i == codeDashAt {
			ctx.Layout(&c.dash, ui.Tight(geom.Sz(codeDashW, codeDashH)))
			ctx.Place(&c.dash, geom.Pt(x, centeredY(size.H, codeDashH)))
			x += codeDashW + codeGap
		}
		b := &c.boxes[i]
		b.focused = i == focus
		b.err = c.Error
		b.Name = nameCodeBox
		if b.focused {
			b.Name = nameCodeBoxF
		}
		ch := "" // the code is ASCII: one byte a character
		if i < len(shown) {
			ch = shown[i : i+1]
		}
		b.label.SetText(ch)
		ctx.Layout(b, ui.Tight(geom.Sz(codeW, codeH)))
		ctx.Place(b, geom.Pt(x, centeredY(size.H, codeH)))
		x += codeW + codeGap
	}
	ctx.Layout(&c.field, ui.Tight(size))
	ctx.Place(&c.field, geom.Point{})

	return size
}

// Layout centers the character in the box.
func (b *codeBox) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(codeW, codeH))
	// An empty box is laid out too: Label.Paint draws the layout of the last
	// Layout, so a label emptied and not laid out again keeps painting the
	// character it held (the boxes kept their letters after Backspace).
	ls := c.Layout(&b.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&b.label, geom.Pt(centeredY(size.W, ls.W), centeredY(size.H, ls.H)))

	return size
}

// Paint draws the box: --sf, with a 1 px --bd border or the 1.8 --pr one of the
// focused box.
func (b *codeBox) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	r := b.Bounds()
	p.Fill(r, scene.Solid(look.T.Surface), codeRadius)
	switch {
	case b.err:
		p.Border(r, geom.Uniform(codeFocusBorder), look.T.Danger, codeRadius)
	case b.focused:
		p.Border(r, geom.Uniform(codeFocusBorder), look.T.Primary, codeRadius)
	default:
		p.Border(r, geom.Uniform(codeBorder), look.T.Border, codeRadius)
	}
	b.Base.Paint(c, p)
}

// Paint draws the dash in --tx3.
func (d *codeDash) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(d.Bounds(), scene.Solid(theme.From(c).T.Text3), 0)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// cleanCode tells whether s is already what FilterCode would return.
func cleanCode(s string) bool {
	if len(s) > CodeLength {
		return false
	}
	for i := range len(s) {
		if c := s[i]; (c < '0' || c > '9') && (c < 'A' || c > 'Z') {
			return false
		}
	}

	return true
}

// codeKey returns the character of a code the upper-case r stands for: itself for
// 0-9 and A-Z, the key of a letter a Vietnamese keyboard makes (see viAFirst), and
// 0 for a character a code never holds.
func codeKey(r rune) byte {
	switch {
	case (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z'):
		return byte(r)
	case r < 0xC0:
		return 0
	case (r >= 0xC0 && r <= 0xC3) || r == viABreve || (r >= viAFirst && r <= viALast): // À Á Â Ã
		return 'A'
	case r == viDStroke:
		return 'D'
	case (r >= 0xC8 && r <= 0xCA) || (r >= viEFirst && r <= viELast): // È É Ê
		return 'E'
	case r == 0xCC || r == 0xCD || r == viITilde || (r >= viIFirst && r <= viILast): // Ì Í
		return 'I'
	case (r >= 0xD2 && r <= 0xD5) || r == viOHorn || (r >= viOFirst && r <= viOLast): // Ò Ó Ô Õ
		return 'O'
	case r == 0xD9 || r == 0xDA || r == viUTilde || (r >= viUFirst && r <= viULast): // Ù Ú
		return 'U'
	case r == viUHorn || (r >= viWFirst && r <= viWLast):
		return 'W'
	case r == 0xDD || (r >= viYFirst && r <= viYLast): // Ý
		return 'Y'
	default:
		return 0
	}
}

// follow makes Value the code and the composition after it, filtered as the code
// is, takes back the error and tells OnChange, when that changes Value.
func (c *CodeBoxes) follow() {
	v := FilterCode(c.field.code + c.field.composing)
	if v == c.Value {
		return
	}
	c.Error = false
	c.Value = v
	c.Relayout()
	if c.OnChange != nil {
		c.OnChange(v)
	}
}

// init adds the parts the first time and gives them the names of layout.json.
func (c *CodeBoxes) init() {
	if c.attached {
		return
	}
	c.attached = true
	c.Name = nameCodeRow
	c.dash.Name = nameCodeDash
	c.field.Name, c.field.row = nameCodeInput, c
	for i := range c.boxes {
		b := &c.boxes[i]
		labelStyle{Role: theme.RoleDisplay, Size: codeSize, Weight: 700, Ink: theme.InkText}.apply(&b.label)
		b.Add(&b.label)
		if i == codeDashAt {
			c.Add(&c.dash)
		}
		c.Add(b)
	}
	c.Add(&c.field)
}
