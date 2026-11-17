//go:build tvos

// Adapted from jplayer-tv/internal/ui/kit/item.go at 781bee5. Changed: the
// transitions are the prototype's 150 ms CSS `ease` (`#tv .f`, index.html:180)
// on the scale and on the shadow, the fill and the ink switch at once; the
// pressed scale, the pulse timer and the channel are dropped (the prototype's
// TV has no pressed look); the scale is reported to layoutdump (F-6).

package tvkit

import (
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/focus"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Item is the base of every focusable control of the TV: a `.f` element
// (`data-fid`). It is a focus.Item (Key is the `data-fid` the focus heals by),
// a focus.Activator (OnSelect is the `data-tact`, OnHold a hold of Select), a
// focus.Presser that shows nothing, and a ui.FocusHandler that animates the
// scale and the shadow of `#tv .f.focus`.
//
// A concrete control embeds an Item, sets FocusScale from its CSS rule, and
// paints with Focus, Scale and Blend inside Scaled. The zero Item takes the
// focus, does nothing and does not move.
type Item struct {
	ui.Base
	// Key is the stable key that finds the control again after its screen is
	// rebuilt: `data-fid`. It is also the Name layout.json knows the control by.
	Key string
	// IsDefault makes the control the one Initial prefers. The prototype has
	// no such mark (the first `.f` that is not in the nav bar wins, :723), so
	// screens leave it unset.
	IsDefault bool
	// Label names the control for tests and assistive technologies.
	Label string
	// Detail is what assistive technologies read after the label.
	Detail string
	// Describing, when set, refines the control's semantic node when a screen
	// reader reads the tree.
	Describing func(n *platform.SemanticNode)
	// OnSelect runs on Select: the `data-tact` action.
	OnSelect func()
	// OnHold runs when Select is held for 550 ms. A control without it runs
	// OnSelect at 550 ms, once (K27).
	OnHold func()
	// OnMove sees an arrow before the focus moves and returns true when it
	// used it.
	OnMove func(focus.Dir) bool

	// FocusScale is the scale the control grows to when focused; 1 when zero.
	FocusScale float32

	scale    ui.Animated[float32] // the transform's scale; read through Scale
	blend    ui.Animated[float32] // 0 at rest, 1 focused: the shadow's progress
	scaleSet bool                 // scale holds a value, not the zero of an unused Animated
	focused  bool
	pressing bool

	// run, c, p and body paint a control under its scale: made once, so that a
	// frame allocates nothing.
	run  func()
	c    *ui.Context
	p    *ui.Painter
	body func(c *ui.Context, p *ui.Painter)
}

var (
	_ focus.Item       = (*Item)(nil)
	_ focus.Activator  = (*Item)(nil)
	_ focus.Presser    = (*Item)(nil)
	_ ui.KeyHandler    = (*Item)(nil)
	_ ui.FocusHandler  = (*Item)(nil)
	_ ui.Labeler       = (*Item)(nil)
	_ ui.Describer     = (*Item)(nil)
	_ ui.SemanticActor = (*Item)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AcceptsFocus tells that an item takes the focus unless it is hidden or
// disabled: every control is reached by direction.
func (i *Item) AcceptsFocus() bool { return !i.Hidden && !i.Disabled }

// FocusKey returns `data-fid`.
func (i *Item) FocusKey() string { return i.Key }

// FocusDefault tells that the item is the one Initial prefers.
func (i *Item) FocusDefault() bool { return i.IsDefault }

// AccessibleLabel returns the control's label.
func (i *Item) AccessibleLabel() string { return i.Label }

// Describe describes the control as a button named by its label, with Detail
// as its value. It runs only when a screen reader reads the tree.
func (i *Item) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label, n.Value = platform.RoleButton, i.Label, i.Detail
	n.Actions |= platform.ActPress
	if i.Describing != nil {
		i.Describing(n)
	}
}

// PerformAction takes a screen reader's press as Select. Other actions go to
// the view, which focuses the control for ActFocus.
func (i *Item) PerformAction(_ *ui.Context, a platform.SemanticAction) bool {
	if a != platform.ActPress {
		return false
	}

	return i.Activate(false)
}

// Activate runs the control's action: for a hold, OnHold, and false when there
// is none so that the focus Manager runs the short press instead; for a short
// press, OnSelect. It reports whether it ran something.
func (i *Item) Activate(long bool) bool {
	switch {
	case long && i.OnHold != nil:
		i.OnHold()

		return true
	case long:
		return false
	case i.OnSelect != nil:
		i.OnSelect()

		return true
	default:
		return false
	}
}

// SetPressing remembers that Select is down. The prototype's TV draws no
// pressed look, so nothing else changes.
func (i *Item) SetPressing(on bool) { i.pressing = on }

// Pulse does nothing: the prototype's TV has no `.pressed` look.
func (i *Item) Pulse() {}

// IsFocused tells whether the control has the focus, as the engine last said.
// It is the `.focus` class: the fill and the ink follow it at once.
func (i *Item) IsFocused() bool { return i.focused }

// Pressing tells whether Select is down on the control.
func (i *Item) Pressing() bool { return i.pressing }

// Scale returns the scale to paint the control at, 1 before it was focused.
func (i *Item) Scale() float32 {
	if !i.scaleSet {
		return 1
	}

	return i.scale.Value()
}

// Blend returns the progress of the shadow's transition, 0 to 1.
func (i *Item) Blend() float32 { return i.blend.Value() }

// HandleKey gives the control's OnMove the arrow; when it returns true the
// press is used and the focus does not move.
func (i *Item) HandleKey(_ *ui.Context, e *ui.KeyEvent) {
	if !e.Down || i.OnMove == nil {
		return
	}
	d := focus.ButtonOf(e.Key).Dir()
	if d != focus.DirUnknown && i.OnMove(d) {
		e.Handle()
	}
}

// FocusChanged starts the focus transition: the scale and the shadow grow
// towards the focused look, or back, in 150 ms; the fill switches at once.
func (i *Item) FocusChanged(c *ui.Context, focused bool) {
	i.focused = focused
	i.retarget(c)
	i.Repaint()
}

// PaintTransform is layoutdump's Transformer: the control is painted scaled
// about its centre, and the prototype's layout.json records the scaled box
// (F-6).
func (i *Item) PaintTransform() (scene.Affine, bool) {
	s := i.Scale()
	if s == 1 {
		return scene.Identity, false
	}
	b := i.Bounds()

	return scaleAbout(s, geom.Pt(b.W/2, b.H/2)), true
}

// Scaled calls body with the control's scale about its centre applied,
// `transform: scale(s)` with the default transform-origin 50% 50%; at scale 1
// it only calls body. body must be a function value made once (a method value
// taken at each call allocates).
func (i *Item) Scaled(c *ui.Context, p *ui.Painter, body func(c *ui.Context, p *ui.Painter)) {
	s := i.Scale()
	if s == 1 {
		body(c, p)

		return
	}
	if i.run == nil {
		i.run = func() { i.body(i.c, i.p) }
	}
	b := i.Bounds()
	i.c, i.p, i.body = c, p, body
	p.Transform(scaleAbout(s, geom.Pt(b.W/2, b.H/2)), i.run)
	i.c, i.p, i.body = nil, nil, nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// retarget sets the animations' targets for the control's state. A nil context
// is the view's own.
func (i *Item) retarget(c *ui.Context) {
	w := i.Self()
	if w == nil {
		return
	}
	v := w.Node().View()
	if v == nil {
		return
	}
	if c == nil {
		c = v.Context()
	}
	focusScale := i.FocusScale
	if focusScale == 0 {
		focusScale = 1
	}
	target, mix := float32(1), float32(0)
	if i.focused {
		mix = 1
		target = focusScale
	}
	if !i.scaleSet {
		i.scale.Set(1)
		i.scaleSet = true
	}
	i.scale.Ease, i.scale.Duration = theme.CSSEase, Transition
	i.scale.To(w, target)
	i.blend.Ease, i.blend.Duration = theme.CSSEase, Transition
	i.blend.To(w, mix)
}

// scaleAbout returns the map scaling by s about the point at.
func scaleAbout(s float32, at geom.Point) scene.Affine {
	return scene.Affine{XX: s, YY: s, X0: at.X * (1 - s), Y0: at.Y * (1 - s)}
}
