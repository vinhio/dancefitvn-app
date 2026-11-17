package avatar

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestJPEGOrientation checks the tag read from a camera's EXIF block, in
// both byte orders, and the 1 of a JPEG without one or with a broken one.
func TestJPEGOrientation(t *testing.T) {
	t.Parallel()

	plain := encodeJPEG(t, solid(8, 4, red))
	for _, order := range []string{"II", "MM"} {
		for o := 1; o <= 8; o++ {
			assert.Equal(t, o, jpegOrientation(withEXIF(plain, order, o)), "%s %d", order, o)
		}
		assert.Equal(t, 1, jpegOrientation(withEXIF(plain, order, 9)), "%s out of range", order)
		assert.Equal(t, 1, jpegOrientation(withEXIF(plain, order, 0)), "%s zero", order)
	}
	broken := withEXIF(plain, "II", 6)
	broken[2+4+6] = 'X' // the byte order mark
	tests := []struct {
		name string
		data []byte
	}{
		{"no EXIF", plain},
		{"bad byte order", broken},
		{"not a JPEG", encodePNG(t, solid(2, 2, red))},
		{"empty", nil},
		{"SOI only", []byte{0xFF, 0xD8}},
		{"cut segment", withEXIF(plain, "MM", 6)[:12]},
	}
	for _, tc := range tests {
		assert.Equal(t, 1, jpegOrientation(tc.data), tc.name)
	}
}

// TestOrient turns a 3 × 2 picture of six colours by each orientation and
// checks every pixel against the EXIF definition of what shows upright:
// for display pixel (x, y), the stored pixel it comes from.
func TestOrient(t *testing.T) {
	t.Parallel()

	const w, h = 3, 2
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	at := func(x, y int) color.RGBA { return color.RGBA{R: uint8(10 + x), G: uint8(20 + y), B: 7, A: 255} }
	for y := range h {
		for x := range w {
			src.SetRGBA(x, y, at(x, y))
		}
	}
	tests := []struct {
		o      int
		dw, dh int
		from   func(x, y int) (int, int) // stored pixel of display (x, y)
	}{
		{1, w, h, func(x, y int) (int, int) { return x, y }},
		{2, w, h, func(x, y int) (int, int) { return w - 1 - x, y }},
		{3, w, h, func(x, y int) (int, int) { return w - 1 - x, h - 1 - y }},
		{4, w, h, func(x, y int) (int, int) { return x, h - 1 - y }},
		{5, h, w, func(x, y int) (int, int) { return y, x }},
		{6, h, w, func(x, y int) (int, int) { return y, h - 1 - x }},
		{7, h, w, func(x, y int) (int, int) { return w - 1 - y, h - 1 - x }},
		{8, h, w, func(x, y int) (int, int) { return w - 1 - y, x }},
		{9, w, h, func(x, y int) (int, int) { return x, y }},
	}
	for _, tc := range tests {
		got := orient(src, tc.o)
		require.Equal(t, image.Rect(0, 0, tc.dw, tc.dh), got.Bounds(), "orientation %d", tc.o)
		for y := range tc.dh {
			for x := range tc.dw {
				sx, sy := tc.from(x, y)
				assert.Equal(t, at(sx, sy), got.RGBAAt(x, y), "orientation %d at %d,%d", tc.o, x, y)
			}
		}
	}
	assert.Same(t, src, orient(src, 1), "upright: the same picture")
}

// TestOrientSubImage checks a picture whose bounds do not start at 0.
func TestOrientSubImage(t *testing.T) {
	t.Parallel()

	img := halves(4, 2, red, blue)
	sub, ok := img.SubImage(image.Rect(1, 0, 3, 2)).(*image.RGBA)
	require.True(t, ok)
	got := orient(sub, 2)
	assert.Equal(t, blue, got.RGBAAt(0, 0))
	assert.Equal(t, red, got.RGBAAt(1, 0))
}
