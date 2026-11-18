// Adapted from jplayer-mobile/internal/ui/kit/button.go at 49de3c2.

package kit

import (
	"dancefitvn-app/internal/ui/icon"
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

// ButtonKind is the look of a Button: the class after `.btn`.
type ButtonKind uint8

// The kinds of button. ButtonKindUnknown reads as ButtonP.
const (
	ButtonKindUnknown ButtonKind = iota
	// ButtonP is `.btn.p`: --on on --pr, 15/800 with 1 px spacing, upper case.
	ButtonP
	// ButtonG is `.btn.g`: --tx on a 1.2 px --bd outline, 15/600.
	ButtonG
	// ButtonS is `.btn.s`: --tx on --sf2, 15/600.
	ButtonS
	// ButtonR is `.btn.r`: #fff on --dg, 15/800 with 1 px spacing, upper case.
	ButtonR
	// ButtonT is `.btn.t`: --pr text alone, 15/700.
	ButtonT
)

// The numbers of .btn (index.html:57-63).
const (
	// btnHeight, btnRadius, btnGap, btnSize are `.btn{height:54px;
	// border-radius:14px;gap:8px;font-size:15px}` (:57).
	btnHeight, btnRadius, btnGap, btnSize float32 = 54, 14, 8, 15
	// btnIcon is the size `ic('trash',20)`, `ic('dl',20)` and the others the
	// buttons hold are drawn at (:357, :503, :516, :519).
	btnIcon float32 = 20
	// btnSpacing is `.btn{letter-spacing:1px}` (:57), which `.g`, `.s` and `.t`
	// reset to 0 (:59, :60, :62).
	btnSpacing float32 = 1
	// btnOutline is `.btn.g{border:1.2px solid var(--bd)}` (:59).
	btnOutline float32 = 1.2
	// btnDisabled is the `opacity:.45` of a button that does nothing yet
	// (`linkTV` with a short code, :502).
	btnDisabled float32 = .45
)

// The names layout.json gives a button without a data attribute: tag and
// classes.
const (
	nameBtnP = "button.btn.p"
	nameBtnG = "button.btn.g"
	nameBtnS = "button.btn.s"
	nameBtnR = "button.btn.r"
	nameBtnT = "button.btn.t"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Button is `.btn` (index.html:57-63): a 54 tall, 14 rounded button as wide as
// the room it gets, its optional 20 DIP icon and its text centered with 8
// between. While a finger is on it it shrinks to .98 at once (:63). A disabled
// button is drawn at .45 and takes no press.
//
// Put the action in the Pressable's OnPress. Set the Name to the data key
// (`bioLogin`, `closeSheet`) the prototype's layout.json knows it by; left
// empty it is `button.btn.<kind>`.
type Button struct {
	ui.Pressable
	// Kind picks the look.
	Kind ButtonKind
	// Text is the label.
	Text string
	// Speak, when set, is what a screen reader says in place of Text ("Lùi điểm B
	// 30 giây" for the button that reads "−30s").
	Speak string
	// Icon is the symbol before the text; icon.NameUnknown for none.
	Icon icon.Name
	// IconSize is the symbol's size; 0 reads as 20.
	IconSize float32
	// IconStroke is the symbol's stroke (`ic('check',20,2.4)`); 0 is the
	// default.
	IconStroke float32
	// Height is the button's height; 0 reads as 54 (the Huỷ button of a sheet
	// is 48, index.html:519).
	Height float32
	// Ink, when not unknown, colors the text and the icon in place of the
	// kind's (`style="color:var(--dg)"`, :474).
	Ink theme.Ink
	// Wrap lets the button be as wide as its content when the room has no
	// width limit; the default is the room (`width:100%`).
	Wrap bool
	// Underline draws the default underline of an `<a class="btn">` under the
	// text (the enroll button of 05B): a 1 DIP rule, as wide as the label, one
	// below its baseline, in the text's color. Chrome draws it 3 device px thick
	// at 3x on the row after the baseline (M05B, rows 2382-2384).
	Underline bool

	label    Label
	attached bool
	press    pressFX
	scale    float32
	// downAt is where the finger came down, in window coordinates, while it is on
	// the button (see HandlePointer).
	downAt  geom.Point
	holding bool
	iconAt  geom.Point // where the icon's top-left is, from the last layout
	// dimmed, c and p paint a disabled button under its layer: made once, so
	// that a frame allocates nothing.
	dimmed func()
	c      *ui.Context
	p      *ui.Painter
}

var _ ui.Labeler = (*Button)(nil)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// HandlePointer is the Pressable's, with one rule more: a tap is a tap when the
// finger did not move, even if the button moved from under it. A press on a button
// takes the focus from the text field that had it, the on-screen keyboard goes down
// and the sheet or page that rose above it comes down with it; the system tells so
// while the finger is still on the glass, and the release would fall off the button
// (the first tap on "Lưu mật khẩu" only put the keyboard away). A finger that moved
// more than the touch slop is a drag and presses nothing, as before.
func (b *Button) HandlePointer(c *ui.Context, e *ui.PointerEvent) {
	switch e.Kind {
	case ui.PointerDown:
		b.downAt, b.holding = e.Pos.Add(b.WindowFrame().Min()), true
	case ui.PointerUp:
		if b.holding && !e.Inside && !b.Disabled {
			d := e.Pos.Add(b.WindowFrame().Min()).Sub(b.downAt)
			e.Inside = d.X*d.X+d.Y*d.Y < ui.TouchSlop*ui.TouchSlop
		}
		b.holding = false
	case ui.PointerCancel:
		b.holding = false
	case ui.PointerUnknown, ui.PointerMove, ui.PointerEnter, ui.PointerLeave, ui.PointerScroll:
	default:
	}
	b.Pressable.HandlePointer(c, e)
}

// NewButton returns a button of kind k with text and the symbol icn (or
// icon.NameUnknown).
func NewButton(k ButtonKind, text string, icn icon.Name) *Button {
	return &Button{Kind: k, Text: text, Icon: icn}
}

// SetText changes the label, laying the button out again; the same text
// changes nothing.
func (b *Button) SetText(s string) {
	if s != b.Text {
		b.Text = s
		b.Relayout()
	}
}

// SetDisabled makes the button take no press and look as a disabled one, or
// take them again; the same state changes nothing.
func (b *Button) SetDisabled(d bool) {
	if d != b.Disabled {
		b.Disabled = d
		b.Repaint()
	}
}

// Scale returns the scale the button was drawn at last: 1 at rest, .98 while
// pressed.
func (b *Button) Scale() float32 { return b.scale }

// IconRect returns where the icon is, in the button's coordinates, as of the
// last layout; the zero rectangle when there is none.
func (b *Button) IconRect() geom.Rect {
	if b.Icon == icon.NameUnknown {
		return geom.Rect{}
	}
	isz := b.iconSize()

	return geom.XYWH(b.iconAt.X, b.iconAt.Y, isz, isz)
}

// AccessibleLabel returns what a screen reader says: Speak when set, else the
// button's text.
func (b *Button) AccessibleLabel() string {
	if b.Speak != "" {
		return b.Speak
	}

	return b.Text
}

// Layout sizes the button to the room it gets (`width:100%`), or around its
// content without a width limit, and centers the icon and the text.
func (b *Button) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !b.attached {
		b.attached = true
		b.Look = noLook
		b.Add(&b.label)
	}
	if b.Name == "" {
		b.Name = b.kindName()
	}
	spec := b.spec()
	labelStyle{Role: theme.RoleBody, Size: btnSize, Weight: spec.weight, Spacing: spec.spacing, Upper: spec.upper, Ink: b.textInk()}.apply(&b.label)
	b.label.SetText(b.Text)
	var ts geom.Size
	content := float32(0)
	if b.Text != "" {
		ts = c.Layout(&b.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
		content = ts.W
	}
	isz := b.iconSize()
	if b.Icon != icon.NameUnknown {
		content += isz
		if b.Text != "" {
			content += btnGap
		}
	}
	h := b.Height
	if h <= 0 {
		h = btnHeight
	}
	w := content
	if !b.Wrap && cs.Max.W < ui.Unbounded {
		w = cs.Max.W
	}
	size := cs.Constrain(geom.Sz(w, h))
	x := (size.W - content) / 2
	if b.Icon != icon.NameUnknown {
		b.iconAt = geom.Pt(x, centeredY(size.H, isz))
		x += isz
		if b.Text != "" {
			x += btnGap
		}
	}
	if b.Text != "" {
		c.Place(&b.label, geom.Pt(x, centeredY(size.H, ts.H)))
	}

	return size
}

// Paint draws the button, scaled about the middle while pressed, at .45
// opacity while disabled.
func (b *Button) Paint(c *ui.Context, p *ui.Painter) {
	b.scale = b.press.track(b, b.Interaction().Pressed && !b.Disabled, &pressButton)
	if !b.Disabled {
		b.paint(c, p)

		return
	}
	if b.dimmed == nil {
		b.dimmed = func() { b.paint(b.c, b.p) }
	}
	b.c, b.p = c, p
	p.Layer(btnDisabled, 0, b.dimmed)
	b.c, b.p = nil, nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// btnSpec is what a kind sets besides its colors.
type btnSpec struct {
	weight  text.Weight
	spacing float32
	upper   bool
}

// spec is the weight, spacing and case of the kind: `.btn` 800/1px/upper; `.g`
// and `.s` 600 with none; `.t` 700 with none.
func (b *Button) spec() btnSpec {
	switch b.Kind {
	case ButtonG, ButtonS:
		return btnSpec{weight: 600}
	case ButtonT:
		return btnSpec{weight: 700}
	case ButtonKindUnknown, ButtonP, ButtonR:
		return btnSpec{weight: 800, spacing: btnSpacing, upper: true}
	default:
		return btnSpec{weight: 800, spacing: btnSpacing, upper: true}
	}
}

// kindName is the name layout.json gives the kind.
func (b *Button) kindName() string {
	switch b.Kind {
	case ButtonG:
		return nameBtnG
	case ButtonS:
		return nameBtnS
	case ButtonR:
		return nameBtnR
	case ButtonT:
		return nameBtnT
	case ButtonKindUnknown, ButtonP:
		return nameBtnP
	default:
		return nameBtnP
	}
}

// textInk is the color of the text and the icon: --on for `.p`, #fff for `.r`,
// --pr for `.t`, --tx for the others, or the Ink the caller set.
func (b *Button) textInk() theme.Ink {
	if b.Ink != theme.InkUnknown {
		return b.Ink
	}
	switch b.Kind {
	case ButtonP, ButtonKindUnknown:
		return theme.InkOnPrimary
	case ButtonR:
		return theme.InkWhite
	case ButtonT:
		return theme.InkPrimary
	case ButtonG, ButtonS:
		return theme.InkText
	default:
		return theme.InkText
	}
}

// iconSize is the size of the symbol: 20 unless set.
func (b *Button) iconSize() float32 {
	if b.IconSize > 0 {
		return b.IconSize
	}

	return btnIcon
}

// fill is the background: --pr, --sf2 or --dg; none for `.g` and `.t`.
func (b *Button) fill(look *theme.Look) color.Color {
	switch b.Kind {
	case ButtonP, ButtonKindUnknown:
		return look.T.Primary
	case ButtonS:
		return look.T.Surface2
	case ButtonR:
		return look.T.Danger
	case ButtonG, ButtonT:
		return color.Color{}
	default:
		return color.Color{}
	}
}

// paint draws the button at its scale, without the opacity of a disabled one.
func (b *Button) paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	r := SnapBox(p, b.Bounds()) // `button.btn`: boxes snap to whole CSS px (Chrome)
	scaled(p, r, b.scale, func() {
		radius := capRadius(r, btnRadius)
		if f := b.fill(look); f.A > 0 {
			p.Fill(r, scene.Solid(f), radius)
		}
		if b.Kind == ButtonG {
			p.Border(r, geom.Uniform(btnOutline), look.T.Border, radius)
		}
		if b.Icon != icon.NameUnknown {
			isz := b.iconSize()
			icon.Draw(p, b.Icon, snapOrigin(p, geom.XYWH(b.iconAt.X, b.iconAt.Y, isz, isz)), // `svg.i`: whole-CSS-px origin (Chrome)
				b.textInk().Of(look), b.IconStroke)
		}
		if b.FocusVisible() {
			focusRing(p, r, radius, look.T.Primary)
		}
		b.Base.Paint(c, p)
		if b.Underline && b.Text != "" {
			b.paintUnderline(p, b.textInk().Of(look))
		}
	})
}

// paintUnderline draws the rule of Underline under the label.
func (b *Button) paintUnderline(p *ui.Painter, col color.Color) {
	f := b.label.Frame()
	off := p.Offset()
	base := snapAxis(off.Y+f.Y+b.label.Baseline(), iconSnapUnit)
	rule := SnapBox(p, geom.XYWH(f.X, base+1-off.Y, f.W, 1))
	p.Fill(rule, scene.Solid(col), 0)
}
