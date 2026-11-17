package avatar

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// solid returns a w × h picture of c.
func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		copy(img.Pix[i:i+4], []byte{c.R, c.G, c.B, c.A})
	}

	return img
}

// halves returns a w × h picture whose left half is a and right half b.
func halves(w, h int, a, b color.RGBA) *image.RGBA {
	img := solid(w, h, a)
	for y := range h {
		for x := w / 2; x < w; x++ {
			img.SetRGBA(x, y, b)
		}
	}

	return img
}

// encodeJPEG returns img as a JPEG of quality 95.
func encodeJPEG(t testing.TB, img image.Image) []byte {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}))

	return buf.Bytes()
}

// encodePNG returns img as a PNG.
func encodePNG(t testing.TB, img image.Image) []byte {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	return buf.Bytes()
}

// withEXIF returns the JPEG j with an APP1 EXIF segment, of byte order
// "II" or "MM", whose first IFD holds Orientation o, right after its SOI:
// what a camera writes.
func withEXIF(j []byte, order string, o int) []byte {
	var bo binary.AppendByteOrder = binary.LittleEndian
	if order == "MM" {
		bo = binary.BigEndian
	}
	tiff := []byte(order)
	tiff = bo.AppendUint16(tiff, 42)
	tiff = bo.AppendUint32(tiff, 8) // the first IFD right after the header
	tiff = bo.AppendUint16(tiff, 1) // one entry
	tiff = bo.AppendUint16(tiff, 0x0112)
	tiff = bo.AppendUint16(tiff, 3) // SHORT
	tiff = bo.AppendUint32(tiff, 1)
	tiff = bo.AppendUint16(tiff, uint16(o))
	tiff = bo.AppendUint16(tiff, 0)
	tiff = bo.AppendUint32(tiff, 0) // no next IFD
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1}
	seg = binary.BigEndian.AppendUint16(seg, uint16(2+len(payload)))
	seg = append(seg, payload...)
	out := append([]byte{}, j[:2]...)
	out = append(out, seg...)

	return append(out, j[2:]...)
}

// near tells whether two colours differ by at most d in each channel.
func near(a, b color.RGBA, d int) bool {
	diff := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }

	return diff(a.R, b.R) <= d && diff(a.G, b.G) <= d && diff(a.B, b.B) <= d && diff(a.A, b.A) <= d
}

// The test colours.
var (
	red   = color.RGBA{R: 220, G: 30, B: 40, A: 255}
	blue  = color.RGBA{R: 30, G: 60, B: 220, A: 255}
	green = color.RGBA{R: 40, G: 200, B: 60, A: 255}
	white = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black = color.RGBA{A: 255}
)
