// Adapted from jplayer-mobile/internal/sources/coverimg.go at 49de3c2
// (jpegOrientation, tiffOrientation, orient). Changed: orient turns an
// *image.RGBA by copying its pixels' bytes, not through At and Set, so
// a working picture of 3 MP turns in milliseconds without an allocation
// a pixel.

package avatar

import (
	"bytes"
	"encoding/binary"
	"image"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// jpegOrientation reads the Orientation tag (0x0112) of the EXIF block of a
// JPEG; 1 when there is none or it is not readable.
func jpegOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			return 1
		}
		marker := b[i+1]
		if marker == 0xFF { // fill byte
			i++

			continue
		}
		if marker == 0xDA || marker == 0xD9 { // picture data, or the end
			return 1
		}
		n := int(binary.BigEndian.Uint16(b[i+2:]))
		if n < 2 || i+2+n > len(b) {
			return 1
		}
		seg := b[i+4 : i+2+n]
		if marker == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
			return tiffOrientation(seg[6:])
		}
		i += 2 + n
	}

	return 1
}

// tiffOrientation finds the Orientation in the first IFD of a TIFF block.
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	if bo.Uint16(t[2:]) != 42 {
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off < 8 || off+2 > len(t) {
		return 1
	}
	count := int(bo.Uint16(t[off:]))
	for k := range count {
		e := off + 2 + 12*k
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			if v := int(bo.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}

			return 1
		}
	}

	return 1
}

// orient returns img turned the way EXIF orientation o (1 to 8) says, so
// that it shows upright: img itself for 1 or a value out of 1–8, a new
// picture otherwise.
func orient(img *image.RGBA, o int) *image.RGBA {
	if o <= 1 || o > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	nw, nh := w, h
	if o >= 5 {
		nw, nh = h, w
	}
	out := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := range h {
		start := img.PixOffset(b.Min.X, b.Min.Y+y)
		row := img.Pix[start : start+4*w]
		for x := range w {
			var dx, dy int
			switch o {
			case 2: // mirrored horizontally
				dx, dy = w-1-x, y
			case 3: // rotated 180
				dx, dy = w-1-x, h-1-y
			case 4: // mirrored vertically
				dx, dy = x, h-1-y
			case 5: // transposed
				dx, dy = y, x
			case 6: // rotated 90 clockwise
				dx, dy = h-1-y, x
			case 7: // transversed
				dx, dy = h-1-y, w-1-x
			default: // 8: rotated 90 counter-clockwise
				dx, dy = y, w-1-x
			}
			copy(out.Pix[dy*out.Stride+4*dx:dy*out.Stride+4*dx+4], row[4*x:4*x+4])
		}
	}

	return out
}
