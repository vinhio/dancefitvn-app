package kit

import (
	"dancefitvn-app/internal/ui/icon"
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

// The numbers of `.vside button` (index.html:157-159).
const (
	// sideDisc is `.vside button i{width:46px;height:46px}` (:158).
	sideDisc float32 = 46
	// sideGap is `.vside button{gap:4px}` (:157), between the disc and its label.
	sideGap float32 = 4
	// sideSize is `.vside button{font-size:10px;font-weight:700}` (:157).
	sideSize float32 = 10
	// SideHeight is the button's height: the disc, the gap and the 12 DIP line of
	// the 10 px label, 62 in layout.json.
	SideHeight float32 = 62
)

// sideBg is `.vside button i{background:rgba(0,0,0,.45)}` (:158); `.on i` is
// --pr with --on (:159).
var sideBg = color.RGBA(0, 0, 0, .45)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// SideButton is a button of the column on the right of screen 06 (`.vside
// button`): a 46 disc of black at .45 holding a 22 icon in white, over a 10/700
// white label, 4 apart; On makes the disc --pr and the icon --on. The button is
// as wide as its disc, or its label when that is wider. Set the Name to the data
// key (`mirror`, `sheet:options`).
type SideButton struct {
	ui.Pressable
	// Icon is the symbol in the disc.
	Icon icon.Name
	// Text is the label under it.
	Text string
	// On is `.on`.
	On bool
	// Label names the button for assistive technologies; empty reads as Text.
	Label string
	// Value is what the button holds, spoken after its label ("0.75x"); empty says
	// nothing.
	Value string
	// Toggle makes the button read as a toggle that On says is on (mirror, loop).
	Toggle bool

	glyph    Icon
	text     Label
	attached bool
}

var (
	_ ui.Labeler   = (*SideButton)(nil)
	_ ui.Describer = (*SideButton)(nil)
)

// NewSideButton returns a side button with the icon and the text.
func NewSideButton(glyph icon.Name, text string) *SideButton {
	return &SideButton{Icon: glyph, Text: text}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetOn turns the disc to --pr, or back; the same state changes nothing. The icon
// takes its ink at once (white, or --on over --pr): no layout follows a change
// that leaves the sizes alone, so one left to Layout would stay the old ink.
func (b *SideButton) SetOn(on bool) {
	if b.On != on {
		b.On = on
		b.glyph.Ink = b.glyphInk()
		b.Repaint()
	}
}

// SetText changes the label; the same text changes nothing.
func (b *SideButton) SetText(s string) {
	if b.Text != s {
		b.Text = s
		b.Relayout()
	}
}

// AccessibleLabel returns what the button is named by.
func (b *SideButton) AccessibleLabel() string {
	if b.Label != "" {
		return b.Label
	}

	return b.Text
}

// Describe describes the button as a button named by its Label or its text, valued
// by Value; as a toggle, on or off, when Toggle says so.
func (b *SideButton) Describe(c *ui.Context, n *platform.SemanticNode) {
	b.Pressable.Describe(c, n)
	n.Value = b.Value
	if b.Toggle {
		n.Toggle = platform.ToggledOff
		if b.On {
			n.Toggle = platform.ToggledOn
		}
	}
}

// Layout makes the button: the disc 46 wide, or the label's width when wider,
// and the icon in the middle of the disc.
func (b *SideButton) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !b.attached {
		b.attached = true
		b.Look = noLook
		b.glyph.Name = iconName
		b.Add(&b.glyph, &b.text)
	}
	b.glyph.Glyph, b.glyph.Ink = b.Icon, b.glyphInk()
	labelStyle{Role: theme.RoleBody, Size: sideSize, Weight: 700, Ink: theme.InkWhite}.apply(&b.text)
	b.text.SetText(b.Text)
	ls := c.Layout(&b.text, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	size := cs.Constrain(geom.Sz(max(sideDisc, ls.W), SideHeight))
	gs := c.Layout(&b.glyph, ui.Loose(geom.Sz(sideDisc, sideDisc)))
	c.Place(&b.glyph, geom.Pt(centeredY(size.W, gs.W), centeredY(sideDisc, gs.H)))
	c.Place(&b.text, geom.Pt(centeredY(size.W, ls.W), sideDisc+sideGap))

	return size
}

// glyphInk is the ink of the icon: white, or --on when the disc is --pr.
func (b *SideButton) glyphInk() theme.Ink {
	if b.On {
		return theme.InkOnPrimary
	}

	return theme.InkWhite
}

// Paint draws the disc under the icon and the label.
func (b *SideButton) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	r := b.Bounds()
	disc := geom.XYWH((r.W-sideDisc)/2, 0, sideDisc, sideDisc)
	bg := sideBg
	if b.On {
		bg = look.T.Primary
	}
	p.Fill(disc, scene.Solid(bg), sideDisc/2)
	if b.FocusVisible() {
		focusRing(p, disc, sideDisc/2, look.T.Primary)
	}
	b.Base.Paint(c, p)
}
