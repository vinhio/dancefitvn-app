// Adapted from jplayer-mobile/internal/ui/kit/seg.go at 49de3c2.

package kit

import (
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of .segc (index.html:121-123).
const (
	// segcRadius, segcPad, segcBorder are `.segc{border-radius:22px;
	// padding:3px;border:1px solid var(--bd)}` (:121).
	segcRadius, segcPad, segcBorder float32 = 22, 3, 1
	// segButton, segButtonRadius, segSize are `.segc button{height:34px;
	// border-radius:17px;font-size:13px}` (:122).
	segButton, segButtonRadius, segSize float32 = 34, 17, 13
	// segcWidth is the width of a control given no width limit.
	segcWidth float32 = 280
)

// nameSegc is the key layout.json gives the control: tag and class.
const nameSegc = "div.segc"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Segmented is `.segc` (index.html:121-123): equal buttons of 13 px text in a
// 22 rounded plate of --sf with a --bd border and 3 of padding. The chosen button
// is --pr with --on 13/700 text; the others are --tx2 13/500. Nothing
// transitions.
//
// Each button's Name is NamePrefix + Keys[i] (Options[i] without Keys): the
// prototype's `speed:0.75`, `filter:Thường`.
type Segmented struct {
	ui.Base
	// Options are the texts of the buttons, in order.
	Options []string
	// Keys are the data values the buttons are named by; nil for Options.
	Keys []string
	// NamePrefix starts the name of every button (`speed:`).
	NamePrefix string
	// Selected is the index of the chosen button.
	Selected int
	// OnSelect is called with the index of a button pressed that was not the
	// chosen one; Selected has changed by then.
	OnSelect func(i int)

	buttons []*segItem
	names   []string // names[i] is the name of button i, namePrefix + nameKeys[i]
	// nameKeys and namePrefix are what names was joined from: a layout joins again
	// only when a key or the prefix is not the one it was made for.
	nameKeys   []string
	namePrefix string
}

// segItem is one button of a Segmented.
type segItem struct {
	ui.Pressable
	seg   *Segmented
	index int
	label Label
	ready bool
}

var (
	_ ui.Labeler   = (*segItem)(nil)
	_ ui.Describer = (*segItem)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewSegmented returns a control of options with the first chosen.
func NewSegmented(options ...string) *Segmented { return &Segmented{Options: options} }

// Select chooses the button i without telling OnSelect.
func (s *Segmented) Select(i int) {
	if i != s.Selected {
		s.Selected = i
		s.Relayout()
	}
}

// Item returns the button i, nil past the options (after the first layout).
func (s *Segmented) Item(i int) ui.Widget {
	if i < 0 || i >= len(s.buttons) {
		return nil
	}

	return s.buttons[i]
}

// Layout divides the width between the buttons.
func (s *Segmented) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	s.sync()
	if s.Name == "" {
		s.Name = nameSegc
	}
	w := segcWidth
	if cs.Max.W < ui.Unbounded {
		w = cs.Max.W
	}
	inset := segcBorder + segcPad
	n := float32(max(len(s.buttons), 1))
	col := max(w-2*inset, 0) / n
	for i, b := range s.buttons {
		b.Name = s.nameOf(i)
		c.Layout(b, ui.Tight(geom.Sz(col, segButton)))
		c.Place(b, geom.Pt(inset+float32(i)*col, inset))
	}
	s.namePrefix = s.NamePrefix

	return cs.Constrain(geom.Sz(w, segButton+2*inset))
}

// Paint draws the plate; the buttons draw themselves.
func (s *Segmented) Paint(c *ui.Context, p *ui.Painter) {
	t := &theme.From(c).T
	r := s.Bounds()
	radius := capRadius(r, segcRadius)
	p.Fill(r, scene.Solid(t.Surface), radius)
	p.Border(r, geom.Uniform(segcBorder), t.Border, radius)
	s.Base.Paint(c, p)
}

// AccessibleLabel returns the button's text.
func (b *segItem) AccessibleLabel() string { return b.seg.Options[b.index] }

// Describe describes the button as a radio button named by its text, selected
// when it is the chosen one. It sets no Toggle: the iOS bridge turns a toggle
// into a "toggle button" that speaks 0 or 1 (P8.3d dump); a choice is a button
// that is selected.
func (b *segItem) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label = platform.RoleRadio, b.seg.Options[b.index]
	n.Actions |= platform.ActPress
	if b.seg.Selected == b.index {
		n.Flags |= platform.FlagSelected
	}
}

// Activate chooses the button's option.
func (b *segItem) Activate() {
	if b.seg.Selected == b.index {
		return
	}
	b.seg.Selected = b.index
	b.seg.Relayout()
	if b.seg.OnSelect != nil {
		b.seg.OnSelect(b.index)
	}
}

// Layout centers the text.
func (b *segItem) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !b.ready {
		b.ready = true
		b.Look = noLook
		b.Add(&b.label)
	}
	on := b.seg.Selected == b.index
	ls := labelStyle{Role: theme.RoleBody, Size: segSize, Weight: 500, Ink: theme.InkText2}
	if on {
		ls.Weight, ls.Ink = 700, theme.InkOnPrimary
	}
	ls.apply(&b.label)
	b.label.SetText(b.seg.Options[b.index])
	size := cs.Constrain(geom.Sz(cs.Max.W, segButton))
	s := c.Layout(&b.label, ui.Loose(geom.Sz(size.W, ui.Unbounded)))
	c.Place(&b.label, geom.Pt(centeredY(size.W, s.W), centeredY(size.H, s.H)))

	return size
}

// Paint draws the --pr pill of the chosen button.
func (b *segItem) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	r := b.Bounds()
	radius := capRadius(r, segButtonRadius)
	if b.seg.Selected == b.index {
		p.Fill(r, scene.Solid(look.T.Primary), radius)
	}
	if b.FocusVisible() {
		focusRing(p, r, radius, look.T.Primary)
	}
	b.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// sync makes the buttons match the options.
func (s *Segmented) sync() {
	for len(s.buttons) > len(s.Options) {
		last := len(s.buttons) - 1
		s.Remove(s.buttons[last])
		s.buttons = s.buttons[:last]
		s.names = s.names[:last]
		s.nameKeys = s.nameKeys[:last]
	}
	for len(s.buttons) < len(s.Options) {
		b := &segItem{seg: s, index: len(s.buttons)}
		s.buttons = append(s.buttons, b)
		s.names = append(s.names, "")
		s.nameKeys = append(s.nameKeys, "")
		s.Add(b)
	}
}

// nameOf returns the name of button i. It is joined from the prefix and the key
// when either has changed since the last time (the first layout, or a caller that
// set Keys, NamePrefix or an option): comparing strings allocates nothing, so a
// layout in which no key changed (a tap on the filter) allocates nothing either.
func (s *Segmented) nameOf(i int) string {
	key := s.Options[i]
	if i < len(s.Keys) {
		key = s.Keys[i]
	}
	if s.names[i] == "" || s.nameKeys[i] != key || s.namePrefix != s.NamePrefix {
		s.nameKeys[i] = key
		s.names[i] = s.NamePrefix + key
	}

	return s.names[i]
}
