// Adapted from jplayer-mobile/internal/ui/np/fit.go at 49de3c2 (Fit, fitRect,
// unturned, turn). Added: mirrorAbout, the flip about the middle of the box that
// the prototype's `.vstage.mir .who{transform:scaleX(-1)}` (index.html:144)
// asks of the picture.

package kit

import (
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Fit is how a video's picture meets the box of its view: object-fit.
type Fit uint8

// The values of object-fit the app uses.
const (
	FitUnknown Fit = iota
	// FitContain shows the whole picture, in bars where the box is wider or
	// taller: screen 06 (Q-P4-2, no part of the dancer is cut).
	FitContain
	// FitCover fills the box and cuts what falls outside it.
	FitCover
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// fitRect returns where a picture of w by h pixels, turned clockwise by rot
// degrees (0, 90, 180 or 270) to stand upright, lies in box under fit,
// centered, as object-fit places it. The result is the upright picture's
// rectangle: for 90 and 270 its width follows h. It may reach beyond box for
// FitCover. A picture or a box without area, or a fit not known, gives the
// empty rectangle.
func fitRect(w, h, rot int, box geom.Rect, fit Fit) geom.Rect {
	if w <= 0 || h <= 0 || box.W <= 0 || box.H <= 0 {
		return geom.Rect{}
	}
	dw, dh := float32(w), float32(h)
	if rot == 90 || rot == 270 {
		dw, dh = dh, dw
	}
	sx, sy := box.W/dw, box.H/dh
	var s float32
	switch fit {
	case FitContain:
		s = min(sx, sy)
	case FitCover:
		s = max(sx, sy)
	case FitUnknown:
		return geom.Rect{}
	default:
		return geom.Rect{}
	}
	rw, rh := dw*s, dh*s

	return geom.XYWH(box.X+(box.W-rw)/2, box.Y+(box.H-rh)/2, rw, rh)
}

// unturned returns the rectangle the picture is drawn in before turn maps it
// to dst: the same center, and for 90 and 270 degrees the width and height
// the other way round.
func unturned(dst geom.Rect, rot int) geom.Rect {
	if rot != 90 && rot != 270 {
		return dst
	}
	cx, cy := dst.X+dst.W/2, dst.Y+dst.H/2

	return geom.XYWH(cx-dst.H/2, cy-dst.W/2, dst.H, dst.W)
}

// turn returns the map that turns the plane clockwise by rot degrees about
// c, exact for the four right angles. Other angles give the identity.
func turn(rot int, c geom.Point) scene.Affine {
	switch rot {
	case 90:
		// (x, y) -> (c.x - (y - c.y), c.y + (x - c.x))
		return scene.Affine{XY: -1, X0: c.X + c.Y, YX: 1, Y0: c.Y - c.X}
	case 180:
		return scene.Affine{XX: -1, X0: 2 * c.X, YY: -1, Y0: 2 * c.Y}
	case 270:
		// (x, y) -> (c.x + (y - c.y), c.y - (x - c.x))
		return scene.Affine{XY: 1, X0: c.X - c.Y, YX: -1, Y0: c.Y + c.X}
	default:
		return scene.Identity
	}
}

// mirrorAbout returns the map that flips the plane left to right about the
// vertical line x = cx: scaleX(-1) about the middle of the box.
func mirrorAbout(cx float32) scene.Affine {
	return scene.Affine{XX: -1, YY: 1, X0: 2 * cx}
}
