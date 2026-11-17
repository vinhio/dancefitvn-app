//go:build tvos

// Adapted from dancefitvn-app/internal/ui/kit/toast.go (itself from
// jplayer-mobile/internal/ui/sheets/toast.go at 49de3c2): the host that is
// hidden without a message, the announcement, the timer. Changed to the TV's
// `.ttoast` (index.html:200, :732).

package tvkit

import (
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of `.ttoast` (index.html:200).
const (
	// ToastTime is `setTimeout(..., 2400)` of tvToast (:732).
	ToastTime = 2400 * time.Millisecond
	// ToastFade is `animation:fi .2s` (:200): CSS `ease`.
	ToastFade = 200 * time.Millisecond

	// toastBottom is `bottom:60px`.
	toastBottom float32 = 60
	// toastRadius, toastPadV, toastPadH are `border-radius:24px;
	// padding:22px 34px`.
	toastRadius, toastPadV, toastPadH float32 = 24, 22, 34
	// toastSize, toastWeight are `font-size:28px;font-weight:700`.
	toastSize   float32     = 28
	toastWeight text.Weight = 700
)

// nameToast is the key layout.json gives the bubble.
const nameToast = "div.ttoast"

// Toast is the TV's toast host: `.ttoast` (index.html:200, :732) as an overlay
// as large as its room, hidden while there is no message. Put it over the
// screen (z-index 90).
//
// Show puts a message in the bubble: centered (`left:50%;translateX(-50%)`), 60
// above the bottom, #F4F4F2 with #141518 text in 28/700 on one line, 22 and 34
// of padding, 24 rounded, fading in over 200 ms when it opens. A message shown
// while one is up replaces its text and starts the 2400 ms again without
// replaying the fade (K17); there is no fade out, the bubble is gone at the end.
// The message is announced to a screen reader.
type Toast struct {
	ui.Base

	bubble toastBubble

	alpha   ui.Animated[float32]
	timer   ui.Timer
	hideFn  func()
	pending bool // Show was called before the view existed
}

// toastBubble is `div.ttoast`.
type toastBubble struct {
	ui.Base
	// Decor: the bubble is one status, its message, and nothing in it is read
	// twice.
	kit.Decor

	msg kit.Label

	// dimmed, c and p paint the bubble under its layer while it fades in: made
	// once, so that a frame allocates nothing.
	dimmed func()
	c      *ui.Context
	p      *ui.Painter
	t      *Toast
}

var _ ui.Describer = (*toastBubble)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewToast returns the host with no message.
func NewToast() *Toast {
	t := &Toast{}
	t.Hidden = true
	t.hideFn = t.hide
	t.bubble.t = t
	t.bubble.Name = nameToast
	t.bubble.Add(&t.bubble.msg)
	t.Add(&t.bubble)

	return t
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Show shows msg for 2400 ms from now, replacing the message showing.
func (t *Toast) Show(msg string) {
	t.bubble.msg.SetText(msg)
	t.timer.Stop()
	open := t.Hidden
	kit.SetHidden(t, false)
	if v := t.View(); v != nil {
		t.start(v.Context(), open)

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

// Bubble returns the bubble's frame in the toast's coordinates, as of the last
// layout.
func (t *Toast) Bubble() geom.Rect { return t.bubble.Frame() }

// Layout takes the whole room and puts the bubble in the middle, 60 above the
// bottom.
func (t *Toast) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if t.pending {
		t.pending = false
		t.start(c, true)
	}
	room := cs.Max
	look := theme.From(c)
	m := &t.bubble.msg
	m.Role, m.Weight, m.Ink = theme.RoleBody, toastWeight, theme.InkText
	m.Color = look.C.ToastText
	if m.Size != toastSize {
		m.Size = toastSize
		m.Relayout()
	}
	bs := c.Layout(&t.bubble, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&t.bubble, geom.Pt((room.W-bs.W)/2, room.H-toastBottom-bs.H))

	return room
}

// Describe describes the bubble as a status, a message told without asking for
// attention; it is announced as it shows (start). It covers nothing: the screen
// under it stays read and pressed. Adapted from kit.Toast's bubble.
func (b *toastBubble) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label = platform.RoleStatus, b.msg.Text
}

// Layout sizes the bubble around its one line of text.
func (b *toastBubble) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	ms := c.Layout(&b.msg, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&b.msg, geom.Pt(toastPadH, toastPadV))

	return cs.Constrain(geom.Sz(ms.W+2*toastPadH, ms.H+2*toastPadV))
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

// LayoutStyle is `div.ttoast`: the text style, the fill and the radius.
func (b *toastBubble) LayoutStyle(c *ui.Context) (kit.CSS, string) {
	look := theme.From(c)
	s, txt := b.msg.LayoutStyle(c)
	s.Background = kit.CSSColor(look.C.ToastBg)
	s.Radius = "24px"

	return s, txt
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// paint draws the surface and the text; `.ttoast` has no shadow.
func (b *toastBubble) paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	p.Fill(b.Bounds(), scene.Solid(look.C.ToastBg), toastRadius)
	b.Base.Paint(c, p)
}

// start arms the timer that takes the bubble away, fades it in when it opens,
// and announces the message.
func (t *Toast) start(c *ui.Context, open bool) {
	if open {
		t.alpha.Set(0)
		t.alpha.Duration, t.alpha.Ease = ToastFade, theme.CSSEase
		t.alpha.To(t, 1)
	}
	t.timer = c.AfterFunc(ToastTime, t.hideFn)
	c.Announce(t.bubble.msg.Text)
}

// hide takes the bubble away.
func (t *Toast) hide() {
	kit.SetHidden(t, true)
	t.alpha.Set(0)
}
