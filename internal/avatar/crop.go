package avatar

import (
	"image"
	"math"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The crop of sheet 15D in DIP (the prototype's CSS px, index.html:167-169):
// the picture box, centred in the frame, shows the picture cover-scaled
// (its centred square) and moves and zooms about its centre; the ring
// keeps the square of its diameter, which is exported.
const (
	// BoxSide is the side of the picture box (.crop .img, 260 × 260).
	BoxSide = 260.0
	// RingSide is the diameter of the ring (.crop .mask:after, 200), the
	// side of the square that is exported.
	RingSide = 200.0
)

// The zoom slider's range and step (input#zoom: min 1, max 3, step .01).
const (
	MinZoom  = 1.0
	MaxZoom  = 3.0
	ZoomStep = 0.01
)

// View is where the picture box is: its zoom about its centre and its
// offset from the frame's centre in DIP (the prototype's S.avatar zoom,
// x, y; transform: translate(x,y) scale(zoom)). The prototype clamps
// neither the offset nor, past the slider, the zoom (Q-P6-8).
type View struct {
	Zoom float64
	X, Y float64
}

// Initial is the view the sheet opens on before any pick (S.avatar of
// index.html:284); Picked the view of a new picture (#filePick,
// index.html:628).
var (
	Initial = View{Zoom: 1.2}
	Picked  = View{Zoom: 1}
)

// Rect is a rectangle of a picture in pixels, of float corners: the part
// of the source one output covers.
type Rect struct {
	X0, Y0, X1, Y1 float64
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Valid tells whether v can be drawn: a finite zoom above 0 and a finite
// offset.
func (v View) Valid() bool {
	return v.Zoom > 0 && !math.IsInf(v.Zoom, 0) && !math.IsNaN(v.X) && !math.IsInf(v.X, 0) &&
		!math.IsNaN(v.Y) && !math.IsInf(v.Y, 0)
}

// Square returns the centred square of a w × h picture: what the box
// shows of it (background-size: cover; background-position: center).
// Its side is the shorter side; an odd difference leaves the extra pixel
// on the right or at the bottom.
func Square(w, h int) image.Rectangle {
	m := min(w, h)
	x, y := (w-m)/2, (h-m)/2

	return image.Rect(x, y, x+m, y+m)
}

// Placement returns where the box is drawn, relative to the frame's
// centre in DIP: its centre (cx, cy) and its side. The box's picture is
// Square of the picture, scaled to side.
func Placement(v View) (cx, cy, side float64) {
	return v.X, v.Y, BoxSide * v.Zoom
}

// SourceRect returns the part of a w × h picture that the ring's square
// keeps at v, in the picture's pixels: a point p of the frame (DIP from
// its centre) shows the source point q = c + ((p − (x, y)) / zoom) / k,
// where c is the centre of Square and k = BoxSide / its side. The rect may
// reach past Square, where the box shows nothing (the frame's #2A2D33).
// It does not allocate.
func SourceRect(w, h int, v View) Rect {
	sq := Square(w, h)
	m := float64(sq.Dx())
	cx, cy := float64(sq.Min.X)+m/2, float64(sq.Min.Y)+m/2
	scale := m / BoxSide / v.Zoom // source px per frame DIP
	const r = RingSide / 2

	return Rect{
		X0: cx + (-r-v.X)*scale,
		Y0: cy + (-r-v.Y)*scale,
		X1: cx + (r-v.X)*scale,
		Y1: cy + (r-v.Y)*scale,
	}
}
