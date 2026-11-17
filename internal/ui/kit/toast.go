// Adapted from jplayer-mobile/internal/ui/sheets/toast.go at 49de3c2 (the
// host that takes no touch outside the bubble, the announcement, the timer); its
// spring, stack of two, action button and fade out are not in the prototype.

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

// The numbers of .toast (index.html:131).
const (
	// toastSide is `.toast{left:16px;right:16px}`.
	toastSide float32 = 16
	// toastBottom is `.toast{bottom:100px}`: over the 84 tall tab bar.
	toastBottom float32 = 100
	// toastRadius, toastPadV, toastPadH, toastGap are `.toast{border-radius:14px;
	// padding:13px 16px;gap:10px}`.
	toastRadius, toastPadV, toastPadH, toastGap float32 = 14, 13, 16, 10
	// toastSize is `.toast{font-size:13px;font-weight:600}`.
	toastSize float32 = 13
	// toastIcon is `ic('info',18)` (:534).
	toastIcon float32 = 18
	// toastShadowY, toastShadowBlur are `box-shadow:0 10px 30px rgba(0,0,0,.4)`;
	// toastShadowAlpha is its .4.
	toastShadowY, toastShadowBlur, toastShadowAlpha float32 = 10, 30, .4
)

// nameToast is the key layout.json gives the bubble.
const nameToast = "div.toast"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Toast is the toast host: `.toast` (index.html:131, :534, :552) as an overlay
// as large as its room, hidden while there is no message so that it takes no
// touch, and with a bubble that takes the touches over it. Put it over the Sheet
// (z-index 90 over 71).
//
// Show puts a message in the bubble: 16 from the sides, 100 above the bottom,
// #F4F4F2 with #141518 text in 13/600, an info icon (18) before it, a soft
// shadow, fading in over 200 ms (CSS `ease`). A new message replaces the one
// showing and starts the 2200 ms again; there is no fade out, the bubble is
// gone at the end (`setTimeout`, :552). The message is announced to a screen
// reader.
type Toast struct {
	ui.Base

	bubble toastBubble

	alpha   ui.Animated[float32]
	timer   ui.Timer
	hideFn  func()
	pending bool // Show was called before the view existed
}

// toastBubble is `div.toast`: the icon and the message in a row.
type toastBubble struct {
	ui.Base
	// Decor: the bubble is one status, its message, and nothing in it is read
	// twice.
	Decor

	icon Icon
	msg  Label

	// dimmed, c and p paint the bubble under its layer while it fades in: made
	// once, so that a frame allocates nothing.
	dimmed func()
	c      *ui.Context
	p      *ui.Painter
	t      *Toast
}

var (
	_ ui.HitTester = (*Toast)(nil)
	_ ui.Describer = (*toastBubble)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewToast returns the host with no message.
func NewToast() *Toast {
	t := &Toast{}
	t.Hidden = true
	t.hideFn = t.hide
	t.bubble.t = t
	t.bubble.Name = nameToast
	t.bubble.icon.Name = iconName
	t.bubble.icon.Glyph, t.bubble.icon.Size = icon.Info, toastIcon
	t.bubble.Add(&t.bubble.icon, &t.bubble.msg)
	t.Add(&t.bubble)

	return t
}

// Show shows msg, replacing the message showing, for 2200 ms from now.
func (t *Toast) Show(msg string) {
	t.bubble.msg.SetText(msg)
	t.Hidden = false
	t.timer.Stop()
	t.Relayout()
	if v := t.View(); v != nil {
		t.start(v.Context())

		return
	}
	t.pending = true
}

// Visible tells whether a message is showing.
func (t *Toast) Visible() bool { return !t.Hidden }

// Message returns the message showing; "" when none.
func (t *Toast) Message() string {
	if t.Hidden {
		return ""
	}

	return t.bubble.msg.Text
}

// Hide takes the message away at once.
func (t *Toast) Hide() {
	t.timer.Stop()
	t.hide()
}

// HitTest lets a touch through everywhere but the bubble.
func (t *Toast) HitTest(p geom.Point) bool { return t.bubble.Frame().Contains(p) }

// Layout takes the whole room and puts the bubble 16 from the sides and 100
// above the bottom (more where the safe area is deeper than the tab bar's 34).
func (t *Toast) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if t.pending {
		t.pending = false
		t.start(c)
	}
	room := cs.Max
	look := theme.From(c)
	labelStyle{Role: theme.RoleBody, Size: toastSize, Weight: 600, Ink: theme.InkText, Wrap: true}.apply(&t.bubble.msg)
	t.bubble.msg.Color = look.C.ToastText
	t.bubble.icon.Color = look.C.ToastText
	w := max(room.W-2*toastSide, 0)
	bs := c.Layout(&t.bubble, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(&t.bubble, geom.Pt(toastSide, room.H-toastBottom-max(c.SafeArea().Bottom-tabInsetFloor, 0)-bs.H))

	return room
}

// Describe describes the bubble as a status, a message told without asking for
// attention; it is also announced when it shows (start). It covers nothing: the
// screen under it stays read.
func (b *toastBubble) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label = platform.RoleStatus, b.msg.Text
}

// Layout lays the icon and the message in a row, centered on it.
func (b *toastBubble) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	is := c.Layout(&b.icon, ui.Tight(geom.Sz(toastIcon, toastIcon)))
	room := max(w-2*toastPadH-is.W-toastGap, 0)
	ms := c.Layout(&b.msg, ui.Loose(geom.Sz(room, ui.Unbounded)))
	h := max(is.H, ms.H)
	c.Place(&b.icon, geom.Pt(toastPadH, toastPadV+centeredY(h, is.H)))
	c.Place(&b.msg, geom.Pt(toastPadH+is.W+toastGap, toastPadV+centeredY(h, ms.H)))

	return cs.Constrain(geom.Sz(w, h+2*toastPadV))
}

// Paint draws the bubble, under a layer while it fades in.
func (b *toastBubble) Paint(c *ui.Context, p *ui.Painter) {
	k := b.t.alpha.Value()
	if k >= 1 || !b.t.alpha.Running() {
		b.paint(c, p)

		return
	}
	if b.dimmed == nil {
		b.dimmed = func() { b.paint(b.c, b.p) }
	}
	b.c, b.p = c, p
	p.Layer(k, 0, b.dimmed)
	b.c, b.p = nil, nil
}

// paint draws the shadow, the surface and the content.
func (b *toastBubble) paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	r := b.Bounds()
	p.Shadow(r, toastRadius, ui.Shadow{Offset: geom.Pt(0, toastShadowY), Blur: toastShadowBlur, Color: color.RGBA(0, 0, 0, toastShadowAlpha)})
	p.Fill(r, scene.Solid(look.C.ToastBg), toastRadius)
	b.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// start fades the bubble in, arms the timer that takes it away, and announces
// the message.
func (t *Toast) start(c *ui.Context) {
	t.alpha.Set(0)
	t.alpha.Duration, t.alpha.Ease = theme.FadeIn, theme.CSSEase
	t.alpha.To(t, 1)
	t.timer = c.AfterFunc(theme.ToastTime, t.hideFn)
	c.Announce(t.bubble.msg.Text)
}

// hide takes the bubble away.
func (t *Toast) hide() {
	t.Hidden = true
	t.alpha.Set(0)
	t.Relayout()
}
