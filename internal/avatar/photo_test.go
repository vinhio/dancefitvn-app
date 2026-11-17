package avatar

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestDecodeTurnsUpright decodes a camera's JPEG stored on its side
// (left half red, right half blue) under each orientation and checks
// where the red half shows once upright.
func TestDecodeTurnsUpright(t *testing.T) {
	t.Parallel()

	plain := encodeJPEG(t, halves(80, 40, red, blue))
	tests := []struct {
		o      int
		w, h   int
		redAt  image.Point // a point inside the red half once upright
		blueAt image.Point
	}{
		{1, 80, 40, image.Pt(10, 20), image.Pt(70, 20)},
		{2, 80, 40, image.Pt(70, 20), image.Pt(10, 20)},
		{3, 80, 40, image.Pt(70, 20), image.Pt(10, 20)},
		{4, 80, 40, image.Pt(10, 20), image.Pt(70, 20)},
		{5, 40, 80, image.Pt(20, 10), image.Pt(20, 70)},
		{6, 40, 80, image.Pt(20, 10), image.Pt(20, 70)}, // the phone held upright: the left edge is the top
		{7, 40, 80, image.Pt(20, 70), image.Pt(20, 10)},
		{8, 40, 80, image.Pt(20, 70), image.Pt(20, 10)},
	}
	for _, tc := range tests {
		for _, order := range []string{"II", "MM"} {
			img, err := Decode(withEXIF(plain, order, tc.o))
			require.NoError(t, err)
			require.Equal(t, image.Rect(0, 0, tc.w, tc.h), img.Bounds(), "%s %d", order, tc.o)
			assert.True(t, near(img.RGBAAt(tc.redAt.X, tc.redAt.Y), red, 12), "%s %d red: %v", order, tc.o, img.RGBAAt(tc.redAt.X, tc.redAt.Y))
			assert.True(t, near(img.RGBAAt(tc.blueAt.X, tc.blueAt.Y), blue, 12), "%s %d blue: %v", order, tc.o, img.RGBAAt(tc.blueAt.X, tc.blueAt.Y))
		}
	}
}

// TestDecodeReduces checks the working picture: a shorter side past 1560
// comes down to 1560 with the aspect kept; a smaller one stays.
func TestDecodeReduces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"small", 600, 400, 600, 400},
		{"at the limit", 2000, 1560, 2000, 1560},
		{"12 MP landscape", 4032, 3024, 2080, 1560},
		{"tall", 1600, 3200, 1560, 3120},
		{"4096 square", 4096, 4096, 1560, 1560},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// A Gray PNG compresses to little, whatever its size.
			g := image.NewGray(image.Rect(0, 0, tc.w, tc.h))
			for i := range g.Pix {
				g.Pix[i] = 90
			}
			img, err := Decode(encodePNG(t, g))
			require.NoError(t, err)
			assert.Equal(t, image.Rect(0, 0, tc.wantW, tc.wantH), img.Bounds())
			mid := img.RGBAAt(tc.wantW/2, tc.wantH/2)
			assert.True(t, near(mid, color.RGBA{R: 90, G: 90, B: 90, A: 255}, 1), "%v", mid)
		})
	}
}

// TestDecodeRefuses checks what Decode does not take.
func TestDecodeRefuses(t *testing.T) {
	t.Parallel()

	jpg := encodeJPEG(t, solid(64, 64, red))
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"past 4096 wide", encodePNG(t, image.NewGray(image.Rect(0, 0, MaxSide+1, 1))), core.ErrInvalid},
		{"past 4096 tall", encodePNG(t, image.NewGray(image.Rect(0, 0, 1, MaxSide+1))), core.ErrInvalid},
		{"past 64 MB", make([]byte, MaxBytes+1), core.ErrInvalid},
		{"cut JPEG", jpg[:len(jpg)/2], core.ErrInvalid},
		{"GIF", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), core.ErrUnsupported},
		{"HEIC", append([]byte("\x00\x00\x00\x18ftypheic"), make([]byte, 64)...), core.ErrUnsupported},
		{"nothing", nil, core.ErrUnsupported},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			img, err := Decode(tc.data)
			require.ErrorIs(t, err, tc.want)
			assert.Nil(t, img)
		})
	}
}

// TestDecodeTransparentPNG checks that a PNG's alpha is kept,
// premultiplied, for Export to lay over the frame's colour.
func TestDecodeTransparentPNG(t *testing.T) {
	t.Parallel()

	n := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, n))
	img, err := Decode(buf.Bytes())
	require.NoError(t, err)
	assert.Equal(t, color.RGBA{}, img.RGBAAt(5, 5))
}

// BenchmarkDecode12MP measures the working picture of a 12 MP JPEG.
func BenchmarkDecode12MP(b *testing.B) {
	data := encodeJPEG(b, halves(4032, 3024, red, blue))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Decode(withEXIF(data, "II", 6)); err != nil {
			b.Fatal(err)
		}
	}
}
