//go:build !tvos

// Adapted from dancefitvn-app/internal/app/splash.go (the column of the brand,
// the country and the motto, which P2.2 matched to M01) and extended with the
// background, the dancer, the progress bar and the caption of MS.splash.

package screens

import (
	"time"

	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The splash's measures, in DIPs (index.html:328-335, MS.splash).
const (
	// splashDancerH is `height:190px` of the dancer, and splashDancerPull its
	// `margin-bottom:-120px`: it takes 190 - 120 of the column and paints over
	// the brand under it.
	splashDancerH    float32 = 190
	splashDancerPull float32 = 120
	// splashDancerOpacity is `opacity:.12`, splashDancerStroke the 9 of
	// `DANCER('#FFD028',false,9)` (:331).
	splashDancerOpacity float32 = .12
	splashDancerStroke  float32 = 9
	// brandSize, brandSpacing: `.d.acc` at font-size:72px;font-weight:800;
	// letter-spacing:2px (the .d rule adds line-height:1 and upper case).
	brandSize    float32 = 72
	brandSpacing float32 = 2
	// countrySize, countrySpacing, countryTop: `font-weight:700;
	// letter-spacing:8px;font-size:16px;margin-top:10px`.
	countrySize    float32 = 16
	countrySpacing float32 = 8
	countryTop     float32 = 10
	// mottoSize, mottoSpacing, mottoTop: `.eb` (11px, 700, upper case, --tx2)
	// with `margin-top:30px;letter-spacing:3px`.
	mottoSize    float32 = 11
	mottoSpacing float32 = 3
	mottoTop     float32 = 30
	// splashBarW, splashBarBottom: `.prog` at `left:50%;bottom:120px;width:120px;
	// margin-left:-60px` (:334).
	splashBarW      float32 = 120
	splashBarBottom float32 = 120
	// splashBarFrom, splashBarTime: the bar's fill starts at `width:45%` and
	// `animation:ld 1.2s linear forwards` takes it to 100% (:334-335).
	splashBarFrom float32 = .45
	splashBarTime         = 1200 * time.Millisecond
	// splashCaptionSize, splashCaptionBottom: `position:absolute;bottom:84px;
	// font-size:12px;color:var(--tx3)` (:335).
	splashCaptionSize   float32 = 12
	splashCaptionBottom float32 = 84
)

// The names of the splash's widgets: the keys of M01.layout.json (`div.d.acc`,
// `div.eb`, `div.prog`); the country line and the caption carry no class, so the
// capture does not record them.
const (
	splashBrandName   = "div.d.acc"
	splashCountryName = "splash.country"
	splashMottoName   = "div.eb"
	splashCaptionName = "splash.tagline"
)

// Splash is MS.splash (screen 01): the Night Stage glow, the dancer faint
// behind "DANCEFIT", "VIỆT NAM", the motto, a bar that fills in 1.2 s and the
// caption. It has no tab bar and no scroll. It does nothing else: the Shell's
// boot decides when it goes.
type Splash struct {
	ui.Base

	bg      radialBG
	dancer  dancerPlate
	group   *ui.Flex
	bar     *kit.Progress
	caption *kit.Label
	fill    ui.Animated[float32]
	hold    bool
}

// dancerPlate is the dancer in the column of the splash: it takes
// splashDancerH - splashDancerPull of the column and draws splashDancerH, at
// splashDancerOpacity.
type dancerPlate struct {
	ui.Base
}

var (
	_ ui.Mounter = (*Splash)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewSplash returns the splash. With hold the bar stands at the end of its
// travel and does not move: M01 as the prototype's capture shows it (animations
// are off there, F-11).
func NewSplash(e *env.Env, hold bool) *Splash {
	s := &Splash{hold: hold}
	// radial-gradient(ellipse at 50% 45%, rgba(229,56,59,.45), transparent 60%) (:328)
	s.bg.layers = []kit.Radial{{Shape: kit.RadialEllipse, CX: .5, CY: .45, Color: color.RGBA(229, 56, 59, .45), End: .6}}
	brand := &kit.Label{
		Text: e.T("splash.brand"), Role: theme.RoleDisplay, Size: brandSize, Weight: 800,
		Spacing: brandSpacing, LineHeight: 1, Upper: true, Ink: theme.InkPrimary,
	}
	brand.Name = splashBrandName
	country := &kit.Label{
		Text: e.T("splash.country"), Size: countrySize, Weight: 700, Spacing: countrySpacing, Ink: theme.InkText,
	}
	country.Name = splashCountryName
	motto := &kit.Label{
		Text: e.T("splash.motto"), Size: mottoSize, Weight: 700, Spacing: mottoSpacing, Upper: true, Ink: theme.InkText2,
	}
	motto.Name = splashMottoName

	s.group = ui.Column(0,
		&s.dancer,
		brand,
		ui.With(&ui.Box{Padding: geom.Insets{Top: countryTop}}, country),
		ui.With(&ui.Box{Padding: geom.Insets{Top: mottoTop}}, motto),
	)
	s.group.Align = ui.CrossCenter
	s.group.Hug = true

	s.bar = &kit.Progress{Value: splashBarFrom}
	s.caption = &kit.Label{Text: e.T("splash.tagline"), Size: splashCaptionSize, Ink: theme.InkText3, Align: ui.TextAlignCenter}
	s.caption.Name = splashCaptionName

	s.fill.Ease, s.fill.Duration = ui.Linear, splashBarTime
	s.fill.Set(splashBarFrom)
	if hold {
		s.fill.Set(1)
		s.bar.Value = 1
	}
	s.Add(&s.bg, s.group, s.bar, s.caption)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Mounted starts the bar: 45 % to 100 % in 1200 ms, linear.
func (s *Splash) Mounted(*ui.Context) {
	if !s.hold {
		s.fill.To(s, 1)
	}
}

// Progress returns how far the bar is, 0 to 1.
func (s *Splash) Progress() float32 { return s.fill.Value() }

// Layout fills the window: the group in the middle (`inset:0` flex centred), the
// bar and the caption from the bottom.
func (s *Splash) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	full := ui.Tight(size)
	c.Layout(&s.bg, full)
	c.Place(&s.bg, geom.Point{})
	gs := c.Layout(s.group, ui.Loose(size))
	c.Place(s.group, geom.Pt((size.W-gs.W)/2, (size.H-gs.H)/2))
	bs := c.Layout(s.bar, ui.Tight(geom.Sz(splashBarW, 5)))
	c.Place(s.bar, geom.Pt((size.W-bs.W)/2, size.H-splashBarBottom-bs.H))
	cp := c.Layout(s.caption, ui.Constraints{Min: geom.Sz(size.W, 0), Max: geom.Sz(size.W, ui.Unbounded)})
	c.Place(s.caption, geom.Pt(0, size.H-splashCaptionBottom-cp.H))

	return size
}

// Paint moves the bar to where the animation is, then draws the layers.
func (s *Splash) Paint(c *ui.Context, p *ui.Painter) {
	s.bar.Value = s.fill.Value()
	s.Base.Paint(c, p)
}

// Layout takes the column's room: the dancer's width, and its height less the
// pull.
func (d *dancerPlate) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Sz(splashDancerH, splashDancerH-splashDancerPull))
}

// Paint draws the dancer, the full 190 high, faint.
func (d *dancerPlate) Paint(_ *ui.Context, p *ui.Painter) {
	p.Layer(splashDancerOpacity, 0, func() {
		poster.DrawDancer(p, geom.XYWH(0, 0, splashDancerH, splashDancerH), tokens.Dark.Primary, splashDancerStroke)
	})
}
