package avatar

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // registers the JPEG decoder of Decode
	_ "image/png"  // and PNG's
	"math"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

const (
	// MaxBytes bounds the bytes Decode takes, as the picker bounds what it
	// reads (internal/sys maxImageBytes).
	MaxBytes = 64 << 20
	// MaxSide is the longest side Decode takes: the picker hands Go
	// nothing larger (internal/sys maxGoSide); a 4096 × 4096 PNG is 64 MB
	// decoded.
	MaxSide = 4096
	// WorkSide is the shorter side a working picture keeps at most: twice
	// the box's 260 DIP at 3 px a DIP (Q-P6-10), so that the crop zoomed
	// ×2 on a 3× screen still has a pixel for each of the screen's.
	WorkSide = 1560
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Decode decodes a picked picture (a JPEG or a PNG, what sys.PickImage
// hands over), turns it upright by its EXIF orientation (a JPEG's tag
// 0x0112, 1 to 8) and reduces it, by an area average, so that its
// shorter side is at most WorkSide: the working picture of sheet 15D,
// premultiplied RGBA of origin (0, 0). The decoded original is dropped
// before it returns, so a 12 MP photo holds about 18 MB (YCbCr) plus the
// 13 MB of the working picture for a moment. Run it off the UI thread.
//
// More than MaxBytes, a side past MaxSide or a broken picture is
// core.ErrInvalid; a format other than JPEG and PNG core.ErrUnsupported.
func Decode(data []byte) (*image.RGBA, error) {
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("avatar: decode: larger than %d bytes: %w", MaxBytes, core.ErrInvalid)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, decodeError(err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || max(cfg.Width, cfg.Height) > MaxSide {
		return nil, fmt.Errorf("avatar: decode: %d×%d px, at most %d a side: %w", cfg.Width, cfg.Height, MaxSide, core.ErrInvalid)
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, decodeError(err)
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if m := min(w, h); m > WorkSide {
		k := float64(WorkSide) / float64(m)
		w, h = max(int(math.Round(float64(w)*k)), 1), max(int(math.Round(float64(h)*k)), 1)
	}
	work := image.NewRGBA(image.Rect(0, 0, w, h))
	whole := Rect{X0: float64(b.Min.X), Y0: float64(b.Min.Y), X1: float64(b.Max.X), Y1: float64(b.Max.Y)}
	resample(work, src, whole, whole, color.RGBA{})
	if format == "jpeg" {
		work = orient(work, jpegOrientation(data))
	}

	return work, nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// decodeError wraps a decoder's error: an unknown format is
// core.ErrUnsupported, anything else core.ErrInvalid.
func decodeError(err error) error {
	if errors.Is(err, image.ErrFormat) {
		return fmt.Errorf("avatar: decode: not a JPEG or PNG: %w", core.ErrUnsupported)
	}

	return fmt.Errorf("avatar: decode: %w: %w", core.ErrInvalid, err)
}
