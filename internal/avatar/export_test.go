package avatar

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// quadrants returns a side × side picture of four colours: red top left,
// blue top right, green bottom left, white bottom right.
func quadrants(side int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	for y := range side {
		for x := range side {
			c := red
			switch {
			case x >= side/2 && y < side/2:
				c = blue
			case x < side/2 && y >= side/2:
				c = green
			case x >= side/2 && y >= side/2:
				c = white
			}
			img.SetRGBA(x, y, c)
		}
	}

	return img
}

// TestExport checks the uploaded JPEG: 512 × 512, at most 2 MB, of the
// part of the picture under the ring.
func TestExport(t *testing.T) {
	t.Parallel()

	data, err := Export(quadrants(1040), Initial)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(data), MaxExportBytes)
	img, err := jpeg.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, image.Rect(0, 0, ExportSide, ExportSide), img.Bounds())
	at := func(x, y int) color.RGBA {
		r, g, b, a := img.At(x, y).RGBA()

		return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	}
	assert.True(t, near(at(100, 100), red, 8), "%v", at(100, 100))
	assert.True(t, near(at(400, 100), blue, 8), "%v", at(400, 100))
	assert.True(t, near(at(100, 400), green, 8), "%v", at(100, 400))
	assert.True(t, near(at(400, 400), white, 8), "%v", at(400, 400))
}

// TestCrop checks the square under the ring against the prototype's
// transform, before the JPEG: moved, zoomed, past the box, on a
// landscape (whose sides past the square the box never shows) and on a
// transparent picture.
func TestCrop(t *testing.T) {
	t.Parallel()

	q := quadrants(260) // 1 px a DIP at zoom 1
	tests := []struct {
		name string
		img  image.Image
		v    View
		at   []image.Point
		want []color.RGBA
	}{
		// zoom 1: the ring keeps [30, 230]: the four colours meet in the
		// middle.
		{"zoom 1", q, Picked, []image.Point{{10, 10}, {500, 10}, {10, 500}, {500, 500}}, []color.RGBA{red, blue, green, white}},
		// moved 90 DIP left: the ring sees [120, 320] across: blue up to
		// x 140, then the frame.
		{"moved left", q, View{Zoom: 1, X: -90}, []image.Point{{10, 10}, {200, 10}, {400, 10}, {500, 300}}, []color.RGBA{red, blue, Fill, Fill}},
		// zoomed 3 about the centre: ±33 px around (130, 130).
		{"zoom 3", q, View{Zoom: 3}, []image.Point{{200, 200}, {300, 200}, {200, 300}, {300, 300}}, []color.RGBA{red, blue, green, white}},
		// zoom 3 moved down 300 DIP: ±33 px around (130, 30): top colours.
		{"zoom 3 moved down", q, View{Zoom: 3, Y: 300}, []image.Point{{100, 256}, {400, 256}}, []color.RGBA{red, blue}},
		// far away: the frame only.
		{"past the box", q, View{Zoom: 1, X: 1000, Y: -1000}, []image.Point{{0, 0}, {256, 256}, {511, 511}}, []color.RGBA{Fill, Fill, Fill}},
		// zoom out is not offered (slider ≥ 1), but a zoom of .5 shows the
		// frame round the box: the box spans ±65 DIP of the ring's ±100.
		{"zoom .5", q, View{Zoom: 0.5}, []image.Point{{40, 350}, {350, 350}, {470, 350}}, []color.RGBA{Fill, white, Fill}},
		// a landscape 520 × 260 whose square is the middle half: what lies
		// left and right of it (green) never shows, even moved there.
		{"landscape sides hidden", landscape(), View{Zoom: 1, X: 150}, []image.Point{{40, 256}, {400, 256}}, []color.RGBA{Fill, red}},
		// transparent: the frame's colour.
		{"transparent", image.NewRGBA(image.Rect(0, 0, 300, 300)), Picked, []image.Point{{256, 256}}, []color.RGBA{Fill}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out := Crop(tc.img, tc.v)
			require.Equal(t, image.Rect(0, 0, ExportSide, ExportSide), out.Bounds())
			for i, p := range tc.at {
				assert.True(t, near(out.RGBAAt(p.X, p.Y), tc.want[i], 1), "%v: %v, want %v", p, out.RGBAAt(p.X, p.Y), tc.want[i])
			}
		})
	}
}

// landscape returns 520 × 260: green outside the centred square, red in it.
func landscape() *image.RGBA {
	img := solid(520, 260, green)
	for y := range 260 {
		for x := 130; x < 390; x++ {
			img.SetRGBA(x, y, red)
		}
	}

	return img
}

// TestExportRefuses checks the pictures and views Export does not take.
func TestExportRefuses(t *testing.T) {
	t.Parallel()

	_, err := Export(image.NewRGBA(image.Rectangle{}), Picked)
	require.ErrorIs(t, err, core.ErrInvalid)
	_, err = Export(solid(10, 10, red), View{})
	require.ErrorIs(t, err, core.ErrInvalid)
}

// TestDecodeThenExport goes the way of a picked photo: a camera JPEG
// stored on its side (EXIF 6), its top half red once upright, decoded and
// cropped at the sheet's first view: red above, blue below.
func TestDecodeThenExport(t *testing.T) {
	t.Parallel()

	data := withEXIF(encodeJPEG(t, halves(1600, 1200, red, blue)), "MM", 6)
	work, err := Decode(data)
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, 1200, 1600), work.Bounds())
	out := Crop(work, Picked)
	assert.True(t, near(out.RGBAAt(256, 100), red, 10), "%v", out.RGBAAt(256, 100))
	assert.True(t, near(out.RGBAAt(256, 400), blue, 10), "%v", out.RGBAAt(256, 400))
	jpg, err := Export(work, Picked)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(jpg), MaxExportBytes)
}

// BenchmarkExport measures the avatar of a working picture.
func BenchmarkExport(b *testing.B) {
	work := quadrants(1560)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Export(work, Initial); err != nil {
			b.Fatal(err)
		}
	}
}
