// Adapted from jplayer-tv/internal/ui/cover/cover.go at 781bee5 (cssLine).

package poster

import (
	"math"

	"github.com/gdeskdev/desk/foundation/core/geom"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of poster() (index.html:291-299) and its CSS (:65-71), each
// with the line it is copied from.
const (
	// DefaultRadius is `border-radius:${o.r??16}px` (:298) and `.poster{border-radius:16px}`
	// (:65).
	DefaultRadius float32 = 16
	// RadiusDefault, as Spec.Radius, asks for DefaultRadius: a zero radius is
	// a real one (the detail hero, `r:0`).
	RadiusDefault float32 = -1

	// gradientDeg is `linear-gradient(135deg, ...)` (:298).
	gradientDeg = 135
	// tsHeight and tsWidth are `ts=o.ts||Math.min(h*.2,w*.14)` (:292).
	tsHeight, tsWidth float32 = .2, .14

	// ringSize, ringRight, ringTop and ringBorder are the ring's
	// `width:${m*1.1}px; right:${-m*.25}px; top:${-m*.2}px;
	// border-width:${m*.07}px` (:297), of the smaller side m.
	ringSize, ringRight, ringTop, ringBorder float32 = 1.1, -.25, -.2, .07

	// vinylSize, vinylX and vinylY are `r=m*.7`, `left:${w*.62-r/2}px`,
	// `top:${h*.42-r/2}px` (:294).
	vinylSize, vinylX, vinylY float32 = .7, .62, .42
	// vinylHub is the radius of the orange hub in the radius R of the disc:
	// `radial-gradient(circle,#FF6A1A 0 14%,#1a1a1a 14.5% 100%)` (:71) has
	// farthest-corner sizing, a radius of r/sqrt(2) for the box r x r, so
	// the stops lie at .14 * sqrt(2) * R and .145 * sqrt(2) * R; the edge
	// between them is half a percent wide, drawn at its middle.
	vinylHub = (.14 + .145) / 2 * math.Sqrt2
	// vinylBands are the four `inset 0 0 0 Npx` shadows of `.vinyl` (:71),
	// listed first-on-top in the CSS: 6 #1a1a1a, 7 #3a3a3a, 16 #1a1a1a, 17
	// #3a3a3a. They are painted from the last, the one at the bottom.
	vinylBandDark1, vinylBandLight1, vinylBandDark2, vinylBandLight2 float32 = 6, 7, 16, 17

	// figShort and figTall are `h*(h<130?.62:.72)` (:295), figShortBelow the 130.
	figShort, figTall, figShortBelow float32 = .62, .72, 130
	// figWide and figWideNoTitle are `w*(o.title===false?.42:.3)` (:295).
	figWide, figWideNoTitle float32 = .3, .42
	// figRight and figBottom are `right:${w*.04}px; bottom:${h*.06}px` (:295).
	figRight, figBottom float32 = .04, .06
	// figOverlap is `margin-left:-${fh*.25}px` of the second dancer (:295).
	figOverlap float32 = .25
	// figOpacity is `opacity:.92` of the dancers' group (:295).
	figOpacity float32 = .92
	// figStroke is DANCER(c.ink,true,7): stroke-width 7 of the 100 unit box
	// (:295, :242).
	figStroke float32 = 7

	// titleLeft and titleBottom are `.poster .ttl{left:8%;bottom:8%}` (:67).
	titleLeft, titleBottom float32 = .08, .08
	// brandSize is `<s style="font-size:${ts*.42}px">` (:296).
	brandSize float32 = .42
	// brandSpacing is `.poster .ttl s{letter-spacing:1.5px}` (:69).
	brandSpacing float32 = 1.5
	// brandOpacity is `.poster .ttl s{opacity:.85}` (:69).
	brandOpacity float32 = .85
	// brandWeight and nameWeight are the font weights of `s` (:69) and `b`
	// (:68).
	brandWeight, nameWeight = 700, 800
	// nameLine is `.poster .ttl b{line-height:.95}` (:68).
	nameLine float32 = .95
)

// geo is what a poster of w x h is made of, in its own coordinates.
type geo struct {
	w, h float32
	m    float32 // the smaller side
	ts   float32 // the size of the short name

	ring       geom.Rect // the ring's border box
	ringWidth  float32   // its border
	ringRadius float32   // of the border box (50 %)

	vinyl geom.Rect // the record's box; the disc's diameter is vinyl.W

	figH float32 // height and width of one dancer
	figX float32 // left of the group
	figY float32 // top of the group

	titleX float32 // left of the title block
	titleB float32 // the bottom of the title block, from the top of the poster

	from, to geom.Point // the gradient line
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newGeo returns the numbers of a poster w x h whose title is titleSize
// (0: `min(h*.2, w*.14)`) and that shows no title when noTitle.
func newGeo(w, h, titleSize float32, noTitle bool) geo {
	g := geo{w: w, h: h, m: min(w, h)}
	g.ts = titleSize
	if g.ts <= 0 {
		g.ts = min(h*tsHeight, w*tsWidth)
	}

	d := g.m * ringSize
	g.ring = geom.XYWH(w-g.m*ringRight-d, g.m*ringTop, d, d)
	g.ringWidth = g.m * ringBorder
	g.ringRadius = d / 2

	r := g.m * vinylSize
	g.vinyl = geom.XYWH(w*vinylX-r/2, h*vinylY-r/2, r, r)

	k := figTall
	if h < figShortBelow {
		k = figShort
	}
	wide := figWide
	if noTitle {
		wide = figWideNoTitle
	}
	g.figH = min(h*k, w*wide)
	g.figX = w - w*figRight - g.figH*(2-figOverlap)
	g.figY = h - h*figBottom - g.figH

	g.titleX = w * titleLeft
	g.titleB = h - h*titleBottom

	g.from, g.to = cssLine(geom.XYWH(0, 0, w, h), gradientDeg)

	return g
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// cssLine returns the ends of the gradient line CSS draws for
// linear-gradient(deg, ...) over r: the direction is (sin a, -cos a), the
// length |w sin a| + |h cos a| and the line is centered on r, so that the
// corners of r get the first and last stop. 0 degrees runs from the bottom
// to the top, 90 from left to right.
func cssLine(r geom.Rect, deg float64) (from, to geom.Point) {
	a := deg * math.Pi / 180
	sin, cos := math.Sin(a), math.Cos(a)
	half := (math.Abs(float64(r.W)*sin) + math.Abs(float64(r.H)*cos)) / 2
	cx, cy := float64(r.X)+float64(r.W)/2, float64(r.Y)+float64(r.H)/2
	dx, dy := sin*half, -cos*half

	return geom.Pt(float32(cx-dx), float32(cy-dy)), geom.Pt(float32(cx+dx), float32(cy+dy))
}
