package avatar

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

const (
	// ExportSide is the side of the uploaded avatar in pixels (03 §6).
	ExportSide = 512
	// ExportQuality is the JPEG quality the avatar is made at first.
	ExportQuality = 90
	// MaxExportBytes bounds the uploaded avatar (03 §6: ≤ 2 MB); past it
	// the quality drops by 5 until it fits.
	MaxExportBytes = 2 << 20
)

// Fill is the colour of the crop's frame (.crop background #2A2D33):
// what the ring shows where the box does not reach, and what shows
// through a transparent picture.
var Fill = color.RGBA{R: 0x2A, G: 0x2D, B: 0x33, A: 0xFF}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Export makes the avatar of img as v shows it: the square under the
// ring (SourceRect), resampled to ExportSide × ExportSide, Fill where the
// box does not reach and under what is transparent, as a JPEG of quality
// ExportQuality (lower in steps of 5 while past MaxExportBytes). img is
// the working picture of Decode, upright. Run it off the UI thread.
//
// An empty picture or a view that is not Valid is core.ErrInvalid.
func Export(img image.Image, v View) ([]byte, error) {
	b := img.Bounds()
	if b.Empty() {
		return nil, fmt.Errorf("avatar: export: empty picture: %w", core.ErrInvalid)
	}
	if !v.Valid() {
		return nil, fmt.Errorf("avatar: export: view %+v: %w", v, core.ErrInvalid)
	}
	out := Crop(img, v)
	var buf bytes.Buffer
	for q := ExportQuality; ; q -= 5 {
		buf.Reset()
		if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: q}); err != nil {
			return nil, fmt.Errorf("avatar: export: %w", err)
		}
		if buf.Len() <= MaxExportBytes {
			return buf.Bytes(), nil
		}
		if q <= 5 {
			return nil, fmt.Errorf("avatar: export: %d bytes at quality %d: %w", buf.Len(), q, core.ErrInvalid)
		}
	}
}

// Crop returns the square under the ring at v, ExportSide pixels a side,
// opaque: what Export encodes. v must be Valid.
func Crop(img image.Image, v View) *image.RGBA {
	b := img.Bounds()
	mx, my := float64(b.Min.X), float64(b.Min.Y)
	r := SourceRect(b.Dx(), b.Dy(), v)
	sq := Square(b.Dx(), b.Dy())
	r = Rect{X0: r.X0 + mx, Y0: r.Y0 + my, X1: r.X1 + mx, Y1: r.Y1 + my}
	clip := Rect{X0: float64(sq.Min.X) + mx, Y0: float64(sq.Min.Y) + my, X1: float64(sq.Max.X) + mx, Y1: float64(sq.Max.Y) + my}
	out := image.NewRGBA(image.Rect(0, 0, ExportSide, ExportSide))
	resample(out, img, r, clip, Fill)
	overFill(out)

	return out
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// overFill lays the premultiplied pixels of img over Fill, making them
// opaque: a transparent PNG shows the frame's colour, as the box does.
func overFill(img *image.RGBA) {
	p := img.Pix
	for i := 0; i+3 < len(p); i += 4 {
		a := p[i+3]
		if a == 0xFF {
			continue
		}
		rest := uint32(0xFF - a)
		p[i] = uint8(min(uint32(p[i])+(uint32(Fill.R)*rest+127)/0xFF, 0xFF))
		p[i+1] = uint8(min(uint32(p[i+1])+(uint32(Fill.G)*rest+127)/0xFF, 0xFF))
		p[i+2] = uint8(min(uint32(p[i+2])+(uint32(Fill.B)*rest+127)/0xFF, 0xFF))
		p[i+3] = 0xFF
	}
}
