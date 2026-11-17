package kit

import (
	"strings"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
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

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// CodeBoxes is the pairing code field of the TV screen (index.html:176-177, :500):
// six 46 x 60 boxes of --sf with a 1 px --bd border and the character in Barlow
// Condensed 700 30, 8 apart, a 12 x 3 --tx3 dash before the fourth, the row
// centered. The box the next character goes to (`min(len,5)`) has a 1.8 --pr
// border. A hidden text field over the row takes the typing: it is upper-cased,
// only 0-9 and A-Z are kept, and at most six (`oninput`, :570). A tap anywhere on
// the row focuses it.
//
// The caret of the hidden field is pinned at the end of the code: the field is
// invisible and as wide as the row, so a tap or a drag on a box would otherwise
// put the caret inside the text (or at its start), where the system keyboard's
// Backspace deletes another character or none, and the wrong last character could
// not be taken back. Typing, deleting and pasting therefore always work at the end,
// as they do in `<input id="codeInp">` of a prototype whose caret nobody sees. A
// composition of an input method (a Telex or predictive keyboard) is part of the
// code from the moment it is composed, as it is in the boxes: a keyboard that
// composes every letter it is typed (Vietnamese Telex) commits nothing until a
// space or a key of another kind, which a code never has, so that waiting for
// the commit would leave the code empty to the screen (its button off) while six
// letters show. The composition is committed when the field loses the focus.
type CodeBoxes struct {
	ui.Base
	// Value is the code typed so far, at most six characters, an input method's
	// composition included; change it with SetValue.
	Value string
	// Label names the field for assistive technologies.
	Label string
	// OnChange is told the code after every edit the user makes.
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

// codeField is the `<input id="codeInp">` of the row: a text field that draws
// nothing (`opacity:0`) over the whole row.
type codeField struct {
	headless.TextField

	row *CodeBoxes
}

var (
	_ ui.Labeler = (*codeField)(nil)
	_ ui.Labeler = (*CodeBoxes)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewCodeBoxes returns an empty row with the label of its field.
func NewCodeBoxes(label string) *CodeBoxes { return &CodeBoxes{Label: label} }

// SetValue replaces the code, filtered as typing is; OnChange is not told.
func (c *CodeBoxes) SetValue(s string) {
	v := FilterCode(s)
	if v == c.Value && c.field.Text() == v {
		return
	}
	c.Value = v
	c.field.SetText(v)
	c.Relayout()
}

// SetError turns the error state on or off; the same state changes nothing.
func (c *CodeBoxes) SetError(e bool) {
	if e != c.Error {
		c.Error = e
		c.Relayout()
	}
}

// Field returns the hidden text field, to focus it or read the caret.
func (c *CodeBoxes) Field() *headless.TextField { return &c.field.TextField }

// FocusedBox returns the index of the box that shows the focus border:
// `min(len, 5)`, the characters being those shown, a composition's included.
func (c *CodeBoxes) FocusedBox() int { return min(len(c.shown()), CodeLength-1) }

// AccessibleLabel returns the field's label.
func (c *CodeBoxes) AccessibleLabel() string { return c.Label }

// AccessibleLabel returns the row's label.
func (f *codeField) AccessibleLabel() string { return f.row.Label }

// DragExempt tells that a press on the field never starts the drag of a sheet
// around it.
func (f *codeField) DragExempt() bool { return true }

// Paint draws nothing: the field is `opacity:0`; the boxes show its text.
func (f *codeField) Paint(*ui.Context, *ui.Painter) {}

// HandlePointer focuses the field without placing the caret or selecting: the
// caret stays at the end of the code (see CodeBoxes).
func (f *codeField) HandlePointer(_ *ui.Context, e *ui.PointerEvent) {
	if e.Kind != ui.PointerDown || (e.Button != platform.ButtonPrimary && e.Button != platform.ButtonNone) {
		return // no drag selection, no primary selection, no word or line select
	}
	e.Handle()
	f.pin()
	f.row.Relayout()
}

// HandleKey edits at the end of the code: a key that moves the caret or selects
// leaves it there, Backspace deletes the last character.
func (f *codeField) HandleKey(c *ui.Context, e *ui.KeyEvent) {
	f.pin()
	f.TextField.HandleKey(c, e)
	f.pin()
	f.row.Relayout()
	f.row.follow()
}

// HandleText takes typed text and the corrections of an input method at the end
// of the code (a correction names its own range), and shows a composition in the
// boxes.
func (f *codeField) HandleText(c *ui.Context, e *ui.TextEvent) {
	if !e.Replacing {
		f.pin()
	}
	f.TextField.HandleText(c, e)
	f.pin()
	f.row.Relayout()
	f.row.follow()
}

// FocusChanged commits the composition of an input method when the field loses
// the focus (a text field drops it): the characters are in the boxes and in
// Value, and the keyboard going away is what ends them.
func (f *codeField) FocusChanged(c *ui.Context, focused bool) {
	if !focused {
		if s := f.Composing(); s != "" {
			f.Compose("", 0)
			f.pin()
			if f.Type(s) {
				f.OnChange(f.Text())
			}
			f.row.follow()
		}
	}
	f.TextField.FocusChanged(c, focused)
}

// SelectText ignores an input method's move of the caret or selection (the
// system keyboard follows a tap or a loupe that way): the caret stays at the end.
func (f *codeField) SelectText(platform.TextRange) { f.pin() }

// FilterCode returns the pairing code s makes: upper-cased, only 0-9 and A-Z
// kept, at most six characters (`toUpperCase().replace(/[^0-9A-Z]/g,”)
// .slice(0,6)`, index.html:570).
func FilterCode(s string) string {
	if cleanCode(s) {
		return s // the common case, a code typed a character at a time, costs nothing
	}
	var b strings.Builder
	n := 0
	for _, r := range strings.ToUpper(s) {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
			if n++; n == CodeLength {
				break
			}
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
	shown := c.shown()
	focus := min(len(shown), CodeLength-1)
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

// shown returns the characters the boxes show: the code, and the composition of
// an input method after it, filtered as the code is. Without a composition it is
// the field's text, which costs nothing.
func (c *CodeBoxes) shown() string {
	if !c.attached {
		return c.Value
	}
	if c.field.Composing() == "" {
		return FilterCode(c.field.Text())
	}
	s, _, _, _ := c.field.Display()

	return FilterCode(s)
}

// follow makes Value what the boxes show, as a composition changes it, and tells
// OnChange: a composition is an edit of the user as a typed character is.
func (c *CodeBoxes) follow() {
	if v := c.shown(); v != c.Value {
		c.Error = false
		c.Value = v
		c.Relayout()
		if c.OnChange != nil {
			c.OnChange(v)
		}
	}
}

// pin puts the caret at the end of the code and drops any selection, unless an
// input method composes (its composition sits at the caret and ends first).
func (f *codeField) pin() {
	if f.Composing() != "" {
		return
	}
	if n := f.Len(); f.Caret() != n || f.HasSelection() {
		f.Select(n, n)
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
	c.field.Keyboard = platform.KeyboardText
	c.field.OnChange = c.changed
	c.field.SetText(c.Value)
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

// changed filters an edit of the user: it keeps only what a code can hold,
// upper-cased, takes back the error and tells OnChange.
func (c *CodeBoxes) changed(s string) {
	c.Error = false
	if v := FilterCode(s); v != s {
		c.field.SetText(v)
	}
	c.Value = c.shown()
	c.Relayout()
	if c.OnChange != nil {
		c.OnChange(c.Value)
	}
}
