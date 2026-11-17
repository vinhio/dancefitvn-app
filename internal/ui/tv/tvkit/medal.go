//go:build tvos

// Adapted from dancefitvn-app/internal/ui/kit/badge.go (the badge's medal: the
// picture of the API or the prototype's medal() drawn in code), at the size the
// TV's settings card gives it, 140: the mobile badge has a fixed 72.

package tvkit

import (
	"math"

	"dancefitvn-app/internal/ui/kit"
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

// The numbers of the settings card's badges (index.html:710): `medal(color,sub,
// 140)` over a label of `font-size:19px;font-weight:800;margin-top:8px` in a
// `text-align:center` block, in a row of `display:flex;justify-content:
// space-around;padding:0 30px`.
const (
	// MedalSize is `medal(b[1],b[2],140)`.
	MedalSize float32 = 140
	// medalBox is the medal's viewBox, 100.
	medalBox float32 = 100
	// medalLabelTop, medalLabelSize are `margin-top:8px` and `font-size:19px`.
	medalLabelTop, medalLabelSize float32 = 8, 19
	// medalMark is the text on the medal, "HDV": an abbreviation, the same in
	// every language.
	medalMark = "HDV"
)

// The literals of medal() (index.html:307-310), not tokens.
var (
	medalRibbon    = color.RGB(0x2a, 0x2d, 0x33) // <path d="M32 0 L44 30 L56 30 L68 0Z">
	medalOuter     = color.RGB(0xd5, 0xd8, 0xde) // outer hexagon fill
	medalInner     = color.RGB(0xee, 0xf0, 0xf3) // inner hexagon fill
	medalInnerLine = color.RGB(0xb9, 0xbd, 0xc5) // inner hexagon stroke
	medalMarkInk   = color.RGB(0x8a, 0x8f, 0x98) // "HDV"
)

// Medal is a badge of the settings card: the medal (the API's picture, or the
// prototype's figure drawn in code in Accent) over its upper-cased caption, in
// one centred column as wide as the medal or the caption, whichever is wider.
type Medal struct {
	ui.Base
	// Image is the picture of the medal; nil draws the figure.
	Image *scene.Image
	// Accent is the figure's ribbon stripes, rim and Sub.
	Accent color.Color
	// Sub is the small text on the figure ("DANCEFIT").
	Sub string
	// Caption is the label under the medal, upper-cased.
	Caption string
	// scale is the share of MedalSize the medal is drawn at; 0 is 1 (the
	// prototype's 140). A row with more medals than it has room for draws them
	// smaller (SetScale) instead of running over its card.
	scale float32

	art      medalArt
	label    kit.Label
	probe    kit.Label // measures the caption and the sub text, in no tree
	attached bool
}

// medalArt is the 140 by 140 medal.
type medalArt struct {
	ui.Base

	m            *Medal
	ribbon       scene.Path
	outer, inner scene.Path
	mark, sub    kit.Label
	built        bool
	k            float32 // DIPs per unit of the box the paths are built at
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewMedal returns a medal with a caption and the figure's accent and text.
func NewMedal(caption string, accent color.Color, sub string) *Medal {
	return &Medal{Caption: caption, Accent: accent, Sub: sub}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Describe describes the badge as one text, its caption ("HDV DanceFIT"): the
// medal's mark and sub ("HDV", "DANCEFIT") are the picture, not words to hear.
func (m *Medal) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label = platform.RoleText, kit.MedalText(m.Caption)
}

// SetScale draws the medal at s of its 140 DIPs (0 or 1: the prototype's size),
// its caption in proportion down to the caption's floor; it lays the medal out
// again when it changes.
func (m *Medal) SetScale(s float32) {
	if s <= 0 || s > 1 {
		s = 1
	}
	if m.scale != s {
		m.scale = s
		m.art.Relayout()
		m.Relayout()
	}
}

// Size returns the side of the medal's figure, MedalSize at scale 1.
func (m *Medal) Size() float32 {
	if m.scale <= 0 {
		return MedalSize
	}

	return MedalSize * m.scale
}

// SetImage shows img as the medal (nil: the figure).
func (m *Medal) SetImage(img *scene.Image) {
	if m.Image != img {
		flips := (m.Image == nil) != (img == nil)
		m.Image = img
		if flips {
			m.art.Relayout()
		}
		m.Repaint()
	}
}

// Layout stacks the medal and the caption, the narrower centred on the wider.
func (m *Medal) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !m.attached {
		m.attached = true
		m.art.m = m
		m.label.Role, m.label.Weight, m.label.Upper = theme.RoleBody, 800, true
		m.label.Ink, m.label.Align = theme.InkText, ui.TextAlignCenter
		m.Add(&m.art, &m.label)
	}
	// The caption is as wide as the room the row gives the medal, at most: it wraps
	// between words, and a word too wide for the room is made smaller, not broken
	// inside. "_" of a code are spaces. Given no room it is one line.
	caption := kit.MedalText(m.Caption)
	// the caption's base size follows the medal's scale down to the caption floor
	base := medalLabelSize * max(m.Size()/MedalSize, kit.MedalCaptionFloor)
	size := base
	room := float32(ui.Unbounded)
	if cs.Max.W < ui.Unbounded {
		room = max(cs.Max.W, 0)
		m.probe.Role, m.probe.Weight, m.probe.Upper, m.probe.Ink = theme.RoleBody, 800, true, theme.InkText
		m.probe.SetText(caption)
		size = kit.WordSize(c, &m.probe, base, kit.MedalCaptionFloor, room)
	}
	if m.label.Size != size || !m.label.Wrap {
		m.label.Size, m.label.Wrap = size, true
		m.label.Relayout()
	}
	m.label.SetText(caption)
	side := m.Size()
	as := c.Layout(&m.art, ui.Tight(geom.Sz(side, side)))
	ls := c.Layout(&m.label, ui.Loose(geom.Sz(room, ui.Unbounded)))
	w := max(as.W, ls.W)
	c.Place(&m.art, geom.Pt((w-as.W)/2, 0))
	c.Place(&m.label, geom.Pt((w-ls.W)/2, as.H+medalLabelTop))

	return cs.Constrain(geom.Sz(w, as.H+medalLabelTop+ls.H))
}

// Layout makes the paths the first time and puts the two texts where the SVG's
// baselines are.
func (a *medalArt) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(MedalSize, MedalSize))
	k := size.W / medalBox
	if !a.built || a.k != k {
		if !a.built {
			a.Add(&a.mark, &a.sub)
		}
		a.built, a.k = true, k
		a.build(k)
		a.mark.Role, a.mark.Weight = theme.RoleDisplay, 800
		a.sub.Role, a.sub.Weight = theme.RoleDisplay, 800
	}
	// <text x="50" y="64" font-size="16" fill="#8A8F98">HDV</text>
	a.mark.Size, a.mark.Color = 16*k, medalMarkInk
	a.mark.SetText(medalMark)
	// <text x="50" y="77" font-size="9" fill="${color}">${sub}</text>
	// The sub text is one line inside the hexagon: shrunk to fit it, then cut.
	sub := kit.MedalText(a.m.Sub)
	a.m.probe.Role, a.m.probe.Weight, a.m.probe.Spacing, a.m.probe.Upper, a.m.probe.Ink = theme.RoleDisplay, 800, 0, false, theme.InkText
	a.m.probe.SetText(sub)
	subSize := kit.LineSize(c, &a.m.probe, 9*k, kit.MedalSubFloor, kit.MedalSubWidth*k)
	if a.sub.Size != subSize || !a.sub.Ellipsis {
		a.sub.Size, a.sub.Ellipsis = subSize, true
		a.sub.Relayout()
	}
	a.sub.Color = a.m.Accent
	a.sub.SetText(sub)
	a.mark.Hidden = a.m.Image != nil
	a.sub.Hidden = a.m.Image != nil
	ms := c.Layout(&a.mark, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&a.mark, geom.Pt(50*k-ms.W/2, 64*k-a.mark.Baseline()))
	ss := c.Layout(&a.sub, ui.Loose(geom.Sz(kit.MedalSubWidth*k, ui.Unbounded)))
	c.Place(&a.sub, geom.Pt(50*k-ss.W/2, 77*k-a.sub.Baseline()))

	return size
}

// Paint draws the picture of the API, or the figure under its texts.
func (a *medalArt) Paint(c *ui.Context, p *ui.Painter) {
	r := a.Bounds()
	if img := a.m.Image; img != nil && img.Width > 0 && img.Height > 0 {
		k := min(r.W/float32(img.Width), r.H/float32(img.Height))
		w, h := float32(img.Width)*k, float32(img.Height)*k
		p.Image(img, geom.XYWH(r.X+(r.W-w)/2, r.Y+(r.H-h)/2, w, h), 1)

		return
	}
	k := r.W / medalBox
	accent := scene.Solid(a.m.Accent)
	p.FillPath(&a.ribbon, scene.Solid(medalRibbon), scene.NonZero)
	// <rect x="40" y="0" width="4" height="26"/> and <rect x="56" ...>
	p.Fill(geom.XYWH(40*k, 0, 4*k, 26*k), accent, 0)
	p.Fill(geom.XYWH(56*k, 0, 4*k, 26*k), accent, 0)
	p.FillPath(&a.outer, scene.Solid(medalOuter), scene.NonZero)
	p.StrokePath(&a.outer, scene.Stroke{Width: 5 * k, Join: scene.JoinRound}, accent)
	p.FillPath(&a.inner, scene.Solid(medalInner), scene.NonZero)
	p.StrokePath(&a.inner, scene.Stroke{Width: k}, scene.Solid(medalInnerLine))
	a.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// build makes the paths of the medal at its size, k DIPs per unit of the box.
func (a *medalArt) build(k float32) {
	a.ribbon.Reset()
	a.outer.Reset()
	a.inner.Reset()
	a.ribbon.MoveTo(32*k, 0)
	a.ribbon.LineTo(44*k, 30*k)
	a.ribbon.LineTo(56*k, 30*k)
	a.ribbon.LineTo(68*k, 0)
	a.ribbon.Close()
	hexagon(&a.outer, 36*k, k)
	hexagon(&a.inner, 28*k, k)
}

// hexagon makes the polygon pts(r) of medal(): six points on the circle of
// radius r (in DIPs) about (50, 62) of the box, from 30 degrees on.
func hexagon(p *scene.Path, r, k float32) {
	for i := range 6 {
		a := float64(math.Pi/6) + float64(i)*float64(math.Pi/3)
		x, y := 50*k+r*float32(math.Cos(a)), 62*k+r*float32(math.Sin(a))
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	p.Close()
}
