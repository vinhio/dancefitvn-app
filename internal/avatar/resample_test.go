package avatar

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestResampleKeepsTheMean shrinks a one-pixel checkerboard: every output
// pixel averages as much black as white, the mean grey.
func TestResampleKeepsTheMean(t *testing.T) {
	t.Parallel()

	for _, shrink := range []int{2, 3, 4} {
		const side = 96
		src := image.NewRGBA(image.Rect(0, 0, side, side))
		for y := range side {
			for x := range side {
				c := black
				if (x+y)%2 == 0 {
					c = white
				}
				src.SetRGBA(x, y, c)
			}
		}
		dst := image.NewRGBA(image.Rect(0, 0, side/shrink, side/shrink))
		whole := Rect{0, 0, side, side}
		resample(dst, src, whole, whole, red)
		grey := color.RGBA{R: 128, G: 128, B: 128, A: 255}
		for y := range dst.Rect.Dy() {
			for x := range dst.Rect.Dx() {
				// An odd shrink covers one pixel more of one colour.
				d := 2
				if shrink%2 == 1 {
					d = 255/(shrink*shrink) + 1
				}
				if !near(dst.RGBAAt(x, y), grey, d) {
					t.Fatalf("÷%d at %d,%d: %v", shrink, x, y, dst.RGBAAt(x, y))
				}
			}
		}
	}
}

// TestResampleIdentity checks that the same size copies the picture.
func TestResampleIdentity(t *testing.T) {
	t.Parallel()

	src := halves(10, 6, red, blue)
	dst := image.NewRGBA(src.Rect)
	whole := Rect{0, 0, 10, 6}
	resample(dst, src, whole, whole, green)
	assert.Equal(t, src.Pix, dst.Pix)
}

// TestResampleFill checks what the rect covers past the clip: the fill,
// in whole pixels and in shares.
func TestResampleFill(t *testing.T) {
	t.Parallel()

	src := solid(32, 32, red)
	whole := Rect{0, 0, 32, 32}

	dst := image.NewRGBA(image.Rect(0, 0, 32, 32))
	resample(dst, src, Rect{-64, 0, -32, 32}, whole, green)
	for y := range 32 {
		for x := range 32 {
			assert.Equal(t, green, dst.RGBAAt(x, y))
		}
	}

	// Half past the left edge: left half fill, right half picture.
	resample(dst, src, Rect{-16, 0, 16, 32}, whole, green)
	assert.Equal(t, green, dst.RGBAAt(0, 5))
	assert.Equal(t, green, dst.RGBAAt(15, 5))
	assert.Equal(t, red, dst.RGBAAt(16, 5))
	assert.Equal(t, red, dst.RGBAAt(31, 5))

	// A clip narrower than the picture: the square of a landscape.
	wide := solid(64, 32, red)
	resample(dst, wide, Rect{0, 0, 64, 32}, Rect{16, 0, 48, 32}, green)
	assert.Equal(t, green, dst.RGBAAt(7, 3), "left of the square")
	assert.Equal(t, red, dst.RGBAAt(16, 3), "in the square")
	assert.Equal(t, green, dst.RGBAAt(24, 3), "right of the square")

	// A pixel half in: half and half.
	dst4 := image.NewRGBA(image.Rect(0, 0, 4, 4))
	resample(dst4, solid(8, 8, white), Rect{-1, 0, 7, 8}, Rect{0, 0, 8, 8}, black)
	assert.True(t, near(dst4.RGBAAt(0, 0), color.RGBA{R: 128, G: 128, B: 128, A: 255}, 1), "%v", dst4.RGBAAt(0, 0))
	assert.Equal(t, white, dst4.RGBAAt(1, 0))
}

// TestResampleGrows checks a blow-up: a linear blend between the pixel
// centres, the edges held.
func TestResampleGrows(t *testing.T) {
	t.Parallel()

	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.SetRGBA(0, 0, black)
	src.SetRGBA(1, 0, white)
	dst := image.NewRGBA(image.Rect(0, 0, 8, 1))
	whole := Rect{0, 0, 2, 1}
	resample(dst, src, whole, whole, red)
	prev := -1
	for x := range 8 {
		c := dst.RGBAAt(x, 0)
		assert.GreaterOrEqual(t, int(c.R), prev, "x %d", x)
		assert.Equal(t, c.R, c.G)
		assert.Equal(t, uint8(255), c.A, "no fill inside the picture")
		prev = int(c.R)
	}
	assert.Equal(t, black, dst.RGBAAt(0, 0))
	assert.Equal(t, white, dst.RGBAAt(7, 0))
	assert.True(t, near(dst.RGBAAt(4, 0), color.RGBA{R: 159, G: 159, B: 159, A: 255}, 1), "%v", dst.RGBAAt(4, 0))
}

// TestResampleReadsAnyPicture checks the rows of pictures that are not
// RGBA: YCbCr (a JPEG), NRGBA (a PNG with alpha), Gray.
func TestResampleReadsAnyPicture(t *testing.T) {
	t.Parallel()

	ycc := image.NewYCbCr(image.Rect(0, 0, 16, 16), image.YCbCrSubsampleRatio420)
	for i := range ycc.Y {
		ycc.Y[i] = 200
	}
	for i := range ycc.Cb {
		ycc.Cb[i], ycc.Cr[i] = 128, 128
	}
	nrgba := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for i := 0; i < len(nrgba.Pix); i += 4 {
		copy(nrgba.Pix[i:], []byte{200, 200, 200, 128})
	}
	gray := image.NewGray(image.Rect(0, 0, 16, 16))
	for i := range gray.Pix {
		gray.Pix[i] = 200
	}
	tests := []struct {
		name string
		src  image.Image
		want color.RGBA
	}{
		{"ycbcr", ycc, color.RGBA{R: 200, G: 200, B: 200, A: 255}},
		{"nrgba", nrgba, color.RGBA{R: 100, G: 100, B: 100, A: 128}},
		{"gray", gray, color.RGBA{R: 200, G: 200, B: 200, A: 255}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dst := image.NewRGBA(image.Rect(0, 0, 5, 5))
			whole := Rect{0, 0, 16, 16}
			resample(dst, tc.src, whole, whole, red)
			for y := range 5 {
				for x := range 5 {
					assert.True(t, near(dst.RGBAAt(x, y), tc.want, 1), "%d,%d: %v", x, y, dst.RGBAAt(x, y))
				}
			}
		})
	}
}
