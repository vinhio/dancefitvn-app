// jPlayer Mobile's kit/badge.go is a small tag (`.badge` of its CSS), not this
// card of a medal over a caption, so nothing is copied: the badge is written
// from index.html:174-175, :466 and its medal from medal() (:307-310).

package kit

import (
	"math"

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

// The numbers of .badge (index.html:174-175) and medal() (:307-310).
const (
	// badgeWidth is the width of a badge given no width limit: three of them
	// fill the 353 of a screen with 10 between (:466).
	badgeWidth float32 = 111
	// badgeRadius is `.badge{border-radius:16px}`.
	badgeRadius float32 = 16
	// badgePadTop, badgePadSide, badgePadBottom are `.badge{padding:10px 6px
	// 12px}`; badgeGap is its `gap:8px`.
	badgePadTop, badgePadSide, badgePadBottom, badgeGap float32 = 10, 6, 12, 8
	// badgeSize, badgeSpacing are `.badge b{font-size:10px;font-weight:800;
	// letter-spacing:.6px}`.
	badgeSize, badgeSpacing float32 = 10, .6
	// medalSize is `medal(color,sub,s=72)`.
	medalSize float32 = 72
	// medalBox is the medal's viewBox, 100.
	medalBox float32 = 100
)

// The literals of medal() (index.html:307-310), not tokens.
var (
	medalRibbon    = color.RGB(0x2a, 0x2d, 0x33) // <path d="M32 0 L44 30 L56 30 L68 0Z">
	medalOuter     = color.RGB(0xd5, 0xd8, 0xde) // outer hexagon fill
	medalInner     = color.RGB(0xee, 0xf0, 0xf3) // inner hexagon fill
	medalInnerLine = color.RGB(0xb9, 0xbd, 0xc5) // inner hexagon stroke
	medalMarkInk   = color.RGB(0x8a, 0x8f, 0x98) // "HDV"
)

// medalMark is the text on the medal, "HDV" (index.html:309): an abbreviation,
// the same in every language.
const medalMark = "HDV"

// The names layout.json gives a badge and its caption.
const (
	nameBadge      = "div.badge"
	nameBadgeLabel = "b"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Badge is `.badge` (index.html:174-175, :466): a --sf card of 16 radius, as wide
// as the room it gets (`flex:1`) and 114 tall at 111 wide, holding a 72 medal over
// a 10/800 caption with .6 px spacing, centered, padded 10 6 12 and 8 apart.
//
// The medal is the badge's Image, the picture the API gives; with none it is
// drawn as the prototype's medal(): a ribbon, two hexagons in Color and the text
// "HDV" and Sub in Barlow Condensed 800 (Q-P2-6; the prototype's own text falls
// back to a serif).
type Badge struct {
	ui.Base
	// Decor leaves the medal's own texts ("HDV", "DANCEFIT") out of a reading: the
	// badge is one text, its caption.
	Decor
	// Image is the picture of the medal; nil draws the fallback.
	Image *scene.Image
	// Color is the medal's accent: its ribbon stripes, rim and Sub.
	Color color.Color
	// Sub is the small text on the fallback medal ("DANCEFIT").
	Sub string
	// Label is the caption, upper-cased.
	Label string

	medal    badgeMedal
	caption  Label
	probe    Label // measures the caption and the sub text, in no tree
	attached bool
}

// badgeMedal is the 72 x 72 medal.
type badgeMedal struct {
	ui.Base

	b            *Badge
	ribbon       scene.Path
	outer, inner scene.Path
	mark, sub    Label
	built        bool
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewBadge returns a badge with a caption and the fallback medal's accent and
// text.
func NewBadge(label string, accent color.Color, sub string) *Badge {
	return &Badge{Label: label, Color: accent, Sub: sub}
}

// Describe describes the badge as text, its caption: the medal is a picture.
func (b *Badge) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label = platform.RoleText, MedalText(b.Label)
}

// SetImage shows img as the medal (nil: the fallback).
func (b *Badge) SetImage(img *scene.Image) {
	if b.Image != img {
		flips := (b.Image == nil) != (img == nil)
		b.Image = img
		if flips {
			b.medal.Relayout() // the medal's Layout hides or shows its two texts
		}
		b.Repaint()
	}
}

// Layout takes the width it gets and stacks the medal and the caption.
func (b *Badge) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !b.attached {
		b.attached = true
		b.Name = nameBadge
		b.caption.Name = nameBadgeLabel
		b.medal.b = b
		b.Add(&b.medal, &b.caption)
	}
	w := badgeWidth
	if cs.Max.W < ui.Unbounded {
		w = cs.Max.W
	}
	room := max(w-2*badgePadSide, 0)
	// A name wraps between words; one word too wide for the room is made smaller
	// until it fits, not broken inside ("FOUNDE" over "R"). "_" of a code are spaces.
	caption := MedalText(b.Label)
	b.probe.Role, b.probe.Weight, b.probe.Spacing, b.probe.Upper, b.probe.Ink = theme.RoleBody, 800, badgeSpacing, true, theme.InkText
	b.probe.SetText(caption)
	size := WordSize(c, &b.probe, badgeSize, MedalCaptionFloor, room)
	labelStyle{Role: theme.RoleBody, Size: size, Weight: 800, Spacing: badgeSpacing, Upper: true, Ink: theme.InkText, Wrap: true}.apply(&b.caption)
	b.caption.Align = ui.TextAlignCenter
	b.caption.SetText(caption)
	ms := c.Layout(&b.medal, ui.Tight(geom.Sz(medalSize, medalSize)))
	c.Place(&b.medal, geom.Pt((w-ms.W)/2, badgePadTop))
	ls := c.Layout(&b.caption, ui.Loose(geom.Sz(room, ui.Unbounded)))
	c.Place(&b.caption, geom.Pt((w-ls.W)/2, badgePadTop+ms.H+badgeGap))

	return cs.Constrain(geom.Sz(w, badgePadTop+ms.H+badgeGap+ls.H+badgePadBottom))
}

// Paint draws the card, then the medal and the caption.
func (b *Badge) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(b.Bounds(), scene.Solid(theme.From(c).T.Surface), badgeRadius)
	b.Base.Paint(c, p)
}

// Layout makes the medal's paths the first time and puts its two texts where
// the SVG's baselines are.
func (m *badgeMedal) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(medalSize, medalSize))
	if !m.built {
		m.built = true
		m.build()
		m.Add(&m.mark, &m.sub)
	}
	k := size.W / medalBox
	// <text x="50" y="64" font-size="16" fill="#8A8F98">HDV</text>
	labelStyle{Role: theme.RoleDisplay, Size: 16 * k, Weight: 800}.apply(&m.mark)
	m.mark.Color = medalMarkInk
	m.mark.SetText(medalMark)
	// <text x="50" y="77" font-size="9" fill="${color}">${sub}</text>
	m.sub.Color = m.b.Color
	// The sub text is one line inside the hexagon: shrunk to fit it, then cut.
	sub := MedalText(m.b.Sub)
	m.b.probe.Role, m.b.probe.Weight, m.b.probe.Spacing, m.b.probe.Upper, m.b.probe.Ink = theme.RoleDisplay, 800, 0, false, theme.InkText
	m.b.probe.SetText(sub)
	subSize := LineSize(c, &m.b.probe, 9*k, MedalSubFloor, MedalSubWidth*k)
	labelStyle{Role: theme.RoleDisplay, Size: subSize, Weight: 800, Ellipsis: true}.apply(&m.sub)
	m.sub.SetText(sub)
	m.mark.Hidden = m.b.Image != nil
	m.sub.Hidden = m.b.Image != nil
	ms := c.Layout(&m.mark, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&m.mark, geom.Pt(50*k-ms.W/2, 64*k-m.mark.Baseline()))
	ss := c.Layout(&m.sub, ui.Loose(geom.Sz(MedalSubWidth*k, ui.Unbounded)))
	c.Place(&m.sub, geom.Pt(50*k-ss.W/2, 77*k-m.sub.Baseline()))

	return size
}

// Paint draws the picture of the API, or the fallback medal under its texts.
func (m *badgeMedal) Paint(c *ui.Context, p *ui.Painter) {
	r := m.Bounds()
	if img := m.b.Image; img != nil && img.Width > 0 && img.Height > 0 {
		p.Image(img, containRect(r, img), 1)

		return
	}
	k := r.W / medalBox
	accent := scene.Solid(m.b.Color)
	p.FillPath(&m.ribbon, scene.Solid(medalRibbon), scene.NonZero)
	// <rect x="40" y="0" width="4" height="26"/> and <rect x="56" ...>
	p.Fill(geom.XYWH(40*k, 0, 4*k, 26*k), accent, 0)
	p.Fill(geom.XYWH(56*k, 0, 4*k, 26*k), accent, 0)
	p.FillPath(&m.outer, scene.Solid(medalOuter), scene.NonZero)
	p.StrokePath(&m.outer, scene.Stroke{Width: 5 * k, Join: scene.JoinRound}, accent)
	p.FillPath(&m.inner, scene.Solid(medalInner), scene.NonZero)
	p.StrokePath(&m.inner, scene.Stroke{Width: k}, scene.Solid(medalInnerLine))
	m.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// build makes the paths of the medal at its 72 DIPs.
func (m *badgeMedal) build() {
	k := medalSize / medalBox
	m.ribbon.MoveTo(32*k, 0)
	m.ribbon.LineTo(44*k, 30*k)
	m.ribbon.LineTo(56*k, 30*k)
	m.ribbon.LineTo(68*k, 0)
	m.ribbon.Close()
	hexagon(&m.outer, 36*k, k)
	hexagon(&m.inner, 28*k, k)
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

// containRect returns where img is drawn to fit whole in r, centered.
func containRect(r geom.Rect, img *scene.Image) geom.Rect {
	k := min(r.W/float32(img.Width), r.H/float32(img.Height))
	w, h := float32(img.Width)*k, float32(img.Height)*k

	return geom.XYWH(r.X+(r.W-w)/2, r.Y+(r.H-h)/2, w, h)
}
