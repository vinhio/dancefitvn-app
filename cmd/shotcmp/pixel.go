package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Thresholds of 07 section 3.3 for the pixel comparison.
const (
	channelTol = 32    // a channel differs when it is off by more than 32/255
	pixelMax   = 0.015 // share of differing pixels allowed
)

// Rect is a rectangle in layout units of the prototype.
type Rect struct{ X, Y, W, H float64 }

// Masks lists the parts of the prototype's screen not compared (07
// section 3.2: status bar, device frame corners, video area, QR, clock...).
type Masks struct {
	Rects     []Rect         // explicit rectangles
	StatusBar float64        // height of the band at the top
	CornerR   float64        // radius of the rounded corners of the root
	Names     *regexp.Regexp // prototype elements whose boxes are masked
}

// PixelRow is the pixel verdict on one element crop or on the whole screen.
type PixelRow struct {
	Compared int     `json:"compared"` // unmasked pixels
	Bad      int     `json:"bad"`
	Share    float64 `json:"share"`
	Status   string  `json:"status"` // ok, FAIL, masked, skipped
	Note     string  `json:"note,omitempty"`
}

// Picture is a decoded screenshot with the scale to its layout units.
type Picture struct {
	Img   image.Image
	Scale float64
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// maskRects returns every rectangle to mask for the prototype layout.
func (m Masks) maskRects(proto Layout) []Rect {
	out := append([]Rect(nil), m.Rects...)
	if m.StatusBar > 0 {
		out = append(out, Rect{0, 0, proto.Screen.W, m.StatusBar})
	}
	if m.Names != nil {
		for _, e := range proto.Elements {
			if m.Names.MatchString(e.Name) {
				out = append(out, Rect{e.X, e.Y, e.W, e.H})
			}
		}
	}
	return out
}

// maskedAt tells whether the device pixel (x, y) of a w x h picture at
// scale s is masked.
func maskedAt(x, y, w, h int, s float64, rects []Rect, corner float64) bool {
	fx, fy := (float64(x)+0.5)/s, (float64(y)+0.5)/s
	for _, r := range rects {
		if fx >= r.X && fx < r.X+r.W && fy >= r.Y && fy < r.Y+r.H {
			return true
		}
	}
	return corner > 0 && cornerMasked(x, y, w, h, corner*s)
}

// ComparePixels compares the area of the prototype picture inside box pb
// with the area of the app picture inside ab (app layout units), after
// resampling the app's to the prototype's pixel size. A pixel is bad when a
// channel differs by more than 32/255; the area fails above limit (1.5 %
// by default) bad pixels. mask, when not nil, is set to true for every masked pixel of the
// whole prototype picture and the bad pixels are painted into bad.
func ComparePixels(pp, ap Picture, pb, ab Box, rects []Rect, corner float64, bad *image.Gray, limit float64) PixelRow {
	pw, ph := pp.Img.Bounds().Dx(), pp.Img.Bounds().Dy()
	x0, y0 := int(math.Round(pb.X*pp.Scale)), int(math.Round(pb.Y*pp.Scale))
	x1, y1 := int(math.Round((pb.X+pb.W)*pp.Scale)), int(math.Round((pb.Y+pb.H)*pp.Scale))
	x0, y0, x1, y1 = max(x0, 0), max(y0, 0), min(x1, pw), min(y1, ph)
	if x1-x0 < 1 || y1-y0 < 1 {
		return PixelRow{Status: "skipped", Note: "empty crop"}
	}
	crop := subCopy(ap.Img, int(math.Round(ab.X*ap.Scale)), int(math.Round(ab.Y*ap.Scale)),
		int(math.Round((ab.X+ab.W)*ap.Scale)), int(math.Round((ab.Y+ab.H)*ap.Scale)))
	if crop == nil {
		return PixelRow{Status: "skipped", Note: "app box outside the picture"}
	}
	app := resample(crop, x1-x0, y1-y0)
	var r PixelRow
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if maskedAt(x, y, pw, ph, pp.Scale, rects, corner) {
				continue
			}
			r.Compared++
			if pixelBad(pp.Img.At(pp.Img.Bounds().Min.X+x, pp.Img.Bounds().Min.Y+y), app.At(x-x0, y-y0)) {
				r.Bad++
				if bad != nil {
					bad.SetGray(x, y, color.Gray{Y: 255})
				}
			}
		}
	}
	if r.Compared == 0 {
		r.Status = "masked"
		return r
	}
	r.Share = float64(r.Bad) / float64(r.Compared)
	r.Status = "ok"
	if r.Share > limit {
		r.Status = "FAIL"
	}
	return r
}

// pixelBad tells whether two pixels differ by more than 32/255 in a channel.
func pixelBad(a, b color.Color) bool {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	d := func(x, y uint32) bool {
		if x > y {
			return (x-y)>>8 > channelTol
		}
		return (y-x)>>8 > channelTol
	}
	return d(ar, br) || d(ag, bg) || d(ab, bb)
}

// subCopy returns the pixels [x0,x1) x [y0,y1) of im as a new image, or nil
// when the rectangle is empty or lies outside im.
func subCopy(im image.Image, x0, y0, x1, y1 int) *image.RGBA {
	b := im.Bounds()
	if x0 < 0 || y0 < 0 || x1 > b.Dx() || y1 > b.Dy() || x1 <= x0 || y1 <= y0 {
		return nil
	}
	out := image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			out.Set(x-x0, y-y0, im.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return out
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// Origin is the pixel of a PNG where the layout's (0, 0) lies: the
// prototype's mobile shots carry a 1 px frame column, so the 393 wide screen
// sits in a 394 wide PNG at x = 1. Auto derives it from the picture size.
type Origin struct {
	Auto bool
	X, Y int
}

// maxAutoOrigin is the largest frame the auto detection takes for an origin;
// a bigger size difference is a different scale, not a frame.
const maxAutoOrigin = 2

// parseOrigin reads "auto" (or empty) or "x,y" in device pixels.
func parseOrigin(s string) (Origin, error) {
	if s == "" || s == "auto" {
		return Origin{Auto: true}, nil
	}
	var o Origin
	if _, err := fmt.Sscanf(s, "%d,%d", &o.X, &o.Y); err != nil || o.X < 0 || o.Y < 0 {
		return Origin{}, fmt.Errorf("origin %q: want auto or x,y in pixels", s)
	}
	return o, nil
}

// readPicture decodes a PNG; the scale is its width over the layout's
// screen width (flag > 0 wins). The picture is cut to the screen: the frame
// before the origin is dropped, so layout (0, 0) is pixel (0, 0).
func readPicture(path string, screen Screen, flag float64, org Origin) (Picture, error) {
	f, err := os.Open(path)
	if err != nil {
		return Picture{}, err
	}
	defer func() { _ = f.Close() }() // read-only: a close error loses nothing
	im, err := png.Decode(f)
	if err != nil {
		return Picture{}, err
	}
	return alignPicture(im, screen, flag, org), nil
}

// alignPicture detects the scale and cuts the frame off im (see Origin).
func alignPicture(im image.Image, screen Screen, flag float64, org Origin) Picture {
	b := im.Bounds()
	scale := detectScale(b.Dx(), screen.W, flag)
	if org.Auto {
		org.X = b.Dx() - int(math.Round(screen.W*scale))
		org.Y = b.Dy() - int(math.Round(screen.H*scale))
		limit := max(maxAutoOrigin, int(math.Ceil(scale)))
		if org.X < 0 || org.X > limit {
			org.X = 0
		}
		if org.Y < 0 || org.Y > limit {
			org.Y = 0
		}
	}
	if org.X == 0 && org.Y == 0 {
		return Picture{Img: im, Scale: scale}
	}
	if cut := subCopy(im, org.X, org.Y, b.Dx(), b.Dy()); cut != nil {
		im = cut
	}
	return Picture{Img: im, Scale: scale}
}

// detectScale is the pixels to the unit of a picture w pixels wide that
// shows screenW units; flag > 0 wins. Adapted from jplayer-mobile.
func detectScale(w int, screenW, flag float64) float64 {
	switch {
	case flag > 0:
		return flag
	case screenW <= 0 || w <= 0:
		return 1
	default:
		// a whole scale whose picture is at most one scale unit wider than the
		// screen (the frame column) wins over the raw ratio: 1182/393 is 3
		if r := math.Round(float64(w) / screenW); r >= 1 {
			if extra := float64(w) - r*screenW; extra >= 0 && extra <= math.Max(maxAutoOrigin, r) {
				return r
			}
		}
		return math.Round(float64(w)/screenW*100) / 100
	}
}

// resample returns im at w x h pixels (area average); im itself when it has
// that size. Adapted from jplayer-mobile.
func resample(im image.Image, w, h int) image.Image {
	b := im.Bounds()
	if b.Dx() == w && b.Dy() == h {
		return im
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0, y1 := y*b.Dy()/h, max((y+1)*b.Dy()/h, y*b.Dy()/h+1)
		for x := 0; x < w; x++ {
			x0, x1 := x*b.Dx()/w, max((x+1)*b.Dx()/w, x*b.Dx()/w+1)
			var r, g, bl, a, n uint64
			for sy := y0; sy < min(y1, b.Dy()); sy++ {
				for sx := x0; sx < min(x1, b.Dx()); sx++ {
					cr, cg, cb, ca := im.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
					r, g, bl, a, n = r+uint64(cr), g+uint64(cg), bl+uint64(cb), a+uint64(ca), n+1
				}
			}
			if n == 0 {
				continue
			}
			i := out.PixOffset(x, y)
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = uint8(r/n>>8), uint8(g/n>>8), uint8(bl/n>>8), uint8(a/n>>8)
		}
	}
	return out
}

// cornerMasked tells whether device pixel (x, y) of a w x h picture lies
// outside the rounded corners of radius r (device pixels). Adapted from
// jplayer-mobile.
func cornerMasked(x, y, w, h int, r float64) bool {
	fx, fy := float64(x)+0.5, float64(y)+0.5
	cx, cy := fx, fy
	switch {
	case fx < r:
		cx = r
	case fx > float64(w)-r:
		cx = float64(w) - r
	}
	switch {
	case fy < r:
		cy = r
	case fy > float64(h)-r:
		cy = float64(h) - r
	}
	if cx == fx || cy == fy {
		return false
	}
	return math.Hypot(fx-cx, fy-cy) > r-1
}

// writeTriptych writes prototype | app | difference side by side. The app
// picture is resampled to the prototype's size. The difference panel dims
// the prototype, paints the bad pixels white, the masked ones blue and
// outlines the failing boxes (device pixels of the prototype) red.
func writeTriptych(path string, pp, ap Picture, bad *image.Gray, rects []Rect, corner float64, fails []Box) error {
	pb := pp.Img.Bounds()
	w, h := pb.Dx(), pb.Dy()
	app := resample(ap.Img, w, h)
	out := image.NewRGBA(image.Rect(0, 0, 3*w+20, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out.Set(x, y, pp.Img.At(pb.Min.X+x, pb.Min.Y+y))
			out.Set(w+10+x, y, app.At(x, y))
			c := color.RGBA{255, 255, 255, 255}
			switch {
			case bad.GrayAt(x, y).Y > 0:
			case maskedAt(x, y, w, h, pp.Scale, rects, corner):
				c = color.RGBA{40, 60, 160, 255}
			default:
				r, g, b, _ := pp.Img.At(pb.Min.X+x, pb.Min.Y+y).RGBA()
				v := uint8((r + g + b) / 3 >> 8 / 4)
				c = color.RGBA{v, v, v, 255}
			}
			out.Set(2*w+20+x, y, c)
		}
	}
	red := color.RGBA{255, 0, 0, 255}
	for _, f := range fails {
		x0, y0 := int(f.X*pp.Scale), int(f.Y*pp.Scale)
		x1, y1 := int((f.X+f.W)*pp.Scale), int((f.Y+f.H)*pp.Scale)
		for x := x0; x <= x1; x++ {
			out.Set(2*w+20+x, y0, red)
			out.Set(2*w+20+x, y1, red)
		}
		for y := y0; y <= y1; y++ {
			out.Set(2*w+20+x0, y, red)
			out.Set(2*w+20+x1, y, red)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, out); err != nil {
		_ = f.Close() // the encode error is the one to report
		return err
	}
	return f.Close()
}
