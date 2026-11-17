// Adapted from jplayer-tv/internal/ui/kit/glow.go at 781bee5.

package kit

import (
	"math"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/scene"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// RadialShape is the shape of a CSS radial gradient's ending shape.
type RadialShape uint8

// The shapes. RadialUnknown reads as RadialEllipse, the CSS default.
const (
	RadialUnknown RadialShape = iota
	// RadialEllipse is `radial-gradient(ellipse ...)`: an ellipse with the
	// aspect of the box.
	RadialEllipse
	// RadialCircle is `radial-gradient(circle ...)`.
	RadialCircle
)

// Radial is one layer of a stack of CSS radial gradients,
//
//	radial-gradient(<ellipse|circle> at CX CY, <Color>, transparent <End>)
//
// sized `farthest-corner`, the CSS default: the gradient ray reaches the
// corner of the box farthest from the centre (an ellipse keeps the aspect of
// the farthest-side ellipse). The colour fades linearly, in premultiplied
// space, from Color at the centre to transparent at End of the ray, which for a
// colour fading to transparent keeps its hue and ramps its alpha; past End
// there is nothing. gDesk has no radial gradient (GT13 of jPlayer TV), so a
// stack is rendered once on the CPU into a small image and drawn stretched.
//
// The prototype's uses: the splash `radial-gradient(ellipse at 50% 45%,
// rgba(229,56,59,.45), transparent 60%)` (index.html:328), the login
// `circle at 85% 5%, rgba(255,208,40,.3), transparent 45%` (:339), and the
// video stage `ellipse at 50% 35%, rgba(255,208,40,.32), transparent 60%`
// (:140).
type Radial struct {
	// Shape is the ending shape.
	Shape RadialShape
	// CX and CY are the centre as shares of the box's width and height: 50%
	// is .5.
	CX, CY float64
	// Color is the colour at the centre, its alpha included
	// (`rgba(229,56,59,.45)`).
	Color color.Color
	// End is the share of the ray where the colour has faded out
	// (`transparent 60%` is .6); 0 reads as 1.
	End float64
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// RenderRadials renders the layers over the opaque background bg at w x h
// pixels, premultiplied and opaque, into dst when it is a w x h image (its
// Version is bumped, nothing is allocated), else into a new one. It returns
// nil when w or h is not positive.
//
// The layers are composited as CSS stacks them: the last one lowest, the first
// (topmost) over all. The geometry is the image's own w x h, so a circle is
// round when the image has the box's aspect: render at the box's size divided
// by a whole number.
func RenderRadials(bg color.Color, layers []Radial, w, h int, dst *scene.Image) *scene.Image {
	dst = radialImage(w, h, dst)
	if dst == nil {
		return nil
	}
	for y := range h {
		for x := range w {
			r, g, b := RadialAt(bg, layers, w, h, float64(x)+0.5, float64(y)+0.5)
			o := y*dst.Stride + x*4
			dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2], dst.Pix[o+3] = quantizeRadial(r), quantizeRadial(g), quantizeRadial(b), 255
		}
	}
	dst.Opaque = true
	dst.Version++

	return dst
}

// RenderRadialsOver renders the layers on a transparent ground, for a gradient
// laid over something else (the video stage's `:before`): premultiplied RGBA,
// the alpha being what the layers cover. Otherwise as RenderRadials.
func RenderRadialsOver(layers []Radial, w, h int, dst *scene.Image) *scene.Image {
	dst = radialImage(w, h, dst)
	if dst == nil {
		return nil
	}
	for y := range h {
		for x := range w {
			r, g, b, a := RadialOverAt(layers, w, h, float64(x)+0.5, float64(y)+0.5)
			o := y*dst.Stride + x*4
			dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2], dst.Pix[o+3] = quantizeRadial(r*255), quantizeRadial(g*255), quantizeRadial(b*255), quantizeRadial(a*255)
		}
	}
	dst.Opaque = false
	dst.Version++

	return dst
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// RadialAt evaluates the stack over bg at the point (x, y) of a w x h box, in
// pixels, as float channels from 0 to 255: the CSS gradients over bg.
func RadialAt(bg color.Color, layers []Radial, w, h int, x, y float64) (r, g, b float64) {
	r, g, b = float64(bg.R), float64(bg.G), float64(bg.B)
	for i := len(layers) - 1; i >= 0; i-- { // from the bottom layer up
		l := &layers[i]
		a := l.alphaAt(w, h, x, y)
		if a <= 0 {
			continue
		}
		r += (float64(l.Color.R) - r) * a
		g += (float64(l.Color.G) - g) * a
		b += (float64(l.Color.B) - b) * a
	}

	return r, g, b
}

// RadialOverAt evaluates the stack on a transparent ground at (x, y): the
// premultiplied channels and the alpha, each from 0 to 1.
func RadialOverAt(layers []Radial, w, h int, x, y float64) (r, g, b, a float64) {
	for i := len(layers) - 1; i >= 0; i-- {
		l := &layers[i]
		sa := l.alphaAt(w, h, x, y)
		if sa <= 0 {
			continue
		}
		keep := 1 - sa
		r = float64(l.Color.R)/255*sa + r*keep
		g = float64(l.Color.G)/255*sa + g*keep
		b = float64(l.Color.B)/255*sa + b*keep
		a = sa + a*keep
	}

	return r, g, b, a
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// alphaAt returns the opacity of the layer at (x, y) of a w x h box: its
// colour's alpha, faded linearly to 0 at End of the gradient ray.
func (l *Radial) alphaAt(w, h int, x, y float64) float64 {
	fw, fh := float64(w), float64(h)
	cx, cy := l.CX*fw, l.CY*fh
	// The farthest side, and from it the farthest corner (CSS Images 3,
	// section 3.2.2): an ellipse keeps the sides' ratio and passes through the
	// corner (a factor of the square root of two); a circle reaches it.
	sx, sy := math.Max(cx, fw-cx), math.Max(cy, fh-cy)
	var d float64
	switch l.Shape {
	case RadialCircle:
		ray := math.Hypot(sx, sy)
		if ray == 0 {
			return 0
		}
		d = math.Hypot(x-cx, y-cy) / ray
	case RadialEllipse, RadialUnknown:
		rx, ry := sx*math.Sqrt2, sy*math.Sqrt2
		if rx == 0 || ry == 0 {
			return 0
		}
		d = math.Hypot((x-cx)/rx, (y-cy)/ry)
	default:
		return 0
	}
	end := l.End
	if end <= 0 {
		end = 1
	}

	return float64(l.Color.A) / 255 * (1 - math.Min(d/end, 1))
}

// radialImage returns dst when it is a w x h image of packed rows, else a new
// one; nil when the size is not positive.
func radialImage(w, h int, dst *scene.Image) *scene.Image {
	if dst != nil && dst.Width == w && dst.Height == h && dst.Stride == w*4 {
		return dst
	}
	img, err := scene.NewImage(w, h)
	if err != nil {
		return nil
	}

	return img
}

// quantizeRadial rounds a channel to a byte.
func quantizeRadial(v float64) uint8 { return uint8(math.Min(255, math.Max(0, v+0.5))) }
