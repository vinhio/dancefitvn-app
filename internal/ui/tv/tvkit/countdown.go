//go:build tvos

package tvkit

import (
	"strconv"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the up-next screen's countdown disc (index.html:703): `position:
// absolute;right:30px;top:30px;width:90px;height:90px;border-radius:50%;
// background:rgba(0,0,0,.6);display:flex;align-items:center;justify-content:
// center;font-family:var(--disp);font-size:46px;font-weight:800`.
const (
	// CountdownSize is the width and the height.
	CountdownSize float32 = 90
	// CountdownMax is the largest number the disc shows, `cdT=5`.
	CountdownMax = 5
	// countdownSize, countdownWeight are the number's `font-size:46px;
	// font-weight:800`.
	countdownSize   float32 = 46
	countdownWeight         = 800
)

// countdownBg is `background:rgba(0,0,0,.6)`.
var countdownBg = color.RGBA(0, 0, 0, .6)

// nameCountdown is the key layout.json gives the disc; its `data-live="tcd"` has
// no class, so the capture does not record it.
const nameCountdown = "tv.upnext.count"

// Countdown is the disc over the next lesson's picture, with the seconds left
// (5 to 0) in Barlow 46/800. The texts are made once, so that a tick allocates
// nothing.
type Countdown struct {
	ui.Base
	// Decor: the digit is not read; the line "tự phát sau 5 giây" next to the disc
	// says it, and a number that changes each second would speak over the focus.
	kit.Decor

	value  int
	label  kit.Label
	digits [CountdownMax + 1]string
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewCountdown returns the disc showing n seconds, 0 to 5.
func NewCountdown(n int) *Countdown {
	c := &Countdown{value: -1}
	c.Name = nameCountdown
	for i := range c.digits {
		c.digits[i] = strconv.Itoa(i)
	}
	c.label.Role, c.label.Size, c.label.Weight, c.label.Ink = theme.RoleDisplay, countdownSize, countdownWeight, theme.InkText
	c.Add(&c.label)
	c.Set(n)

	return c
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Set shows n seconds, clamped to 0 to 5; the same number changes nothing.
func (c *Countdown) Set(n int) {
	n = min(max(n, 0), CountdownMax)
	if n != c.value {
		c.value = n
		c.label.SetText(c.digits[n])
		c.Repaint()
	}
}

// Value returns the seconds shown.
func (c *Countdown) Value() int { return c.value }

// Layout is 90 by 90 with the number in the middle.
func (c *Countdown) Layout(ctx *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(CountdownSize, CountdownSize))
	ls := ctx.Layout(&c.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	ctx.Place(&c.label, geom.Pt((size.W-ls.W)/2, (size.H-ls.H)/2))

	return size
}

// Paint draws the disc and its number.
func (c *Countdown) Paint(ctx *ui.Context, p *ui.Painter) {
	r := c.Bounds()
	p.Fill(r, scene.Solid(countdownBg), r.W/2)
	c.Base.Paint(ctx, p)
}

// LayoutStyle is the disc's `div`: the number's style and the disc's fill.
func (c *Countdown) LayoutStyle(ctx *ui.Context) (kit.CSS, string) {
	s, txt := c.label.LayoutStyle(ctx)
	s.Background = kit.CSSColor(countdownBg)
	s.Radius = "50%"

	return s, txt
}
