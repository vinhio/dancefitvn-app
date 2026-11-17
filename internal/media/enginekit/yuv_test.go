// Adapted from jplayer-mobile/internal/media/enginekit/yuv_test.go at 49de3c2.

package enginekit

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refRGB is the float conversion the fixed-point code approximates.
func refRGB(y, u, v byte, bt709 bool) [3]float64 {
	yy := 1.164383 * (float64(y) - 16)
	cb, cr := float64(u)-128, float64(v)-128
	if bt709 {
		return [3]float64{yy + 1.792741*cr, yy - 0.213249*cb - 0.532909*cr, yy + 2.112402*cb}
	}

	return [3]float64{yy + 1.596027*cr, yy - 0.391762*cb - 0.812968*cr, yy + 2.017232*cb}
}

func clampF(f float64) float64 { return math.Max(0, math.Min(255, f)) }

func TestYUVGolden(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		y, u, v byte
		bt709   bool
		want    [3]byte
		slack   int
	}{
		{"black", 16, 128, 128, false, [3]byte{0, 0, 0}, 0},
		{"white", 235, 128, 128, false, [3]byte{255, 255, 255}, 0},
		{"mid grey", 126, 128, 128, false, [3]byte{128, 128, 128}, 1},
		{"below black clamps", 0, 128, 128, false, [3]byte{0, 0, 0}, 0},
		{"above white clamps", 255, 128, 128, true, [3]byte{255, 255, 255}, 0},
		{"red 601", 81, 90, 240, false, [3]byte{255, 0, 0}, 2},
		{"green 601", 145, 54, 34, false, [3]byte{0, 255, 0}, 2},
		{"blue 601", 41, 240, 110, false, [3]byte{0, 0, 255}, 2},
		{"red 709", 63, 102, 240, true, [3]byte{255, 0, 0}, 2},
		{"green 709", 173, 42, 26, true, [3]byte{0, 255, 0}, 2},
		{"blue 709", 32, 240, 118, true, [3]byte{0, 0, 255}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dst := make([]byte, 2*2*4)
			y := []byte{tt.y, tt.y, tt.y, tt.y}
			YUV420ToRGBA(dst, 8, 2, 2, y, 2, []byte{tt.u}, []byte{tt.v}, 1, 1, tt.bt709)
			for p := range 4 {
				for c := range 3 {
					assert.InDelta(t, tt.want[c], dst[p*4+c], float64(tt.slack), "pixel %d channel %d", p, c)
				}
				assert.EqualValues(t, 0xff, dst[p*4+3], "opaque")
			}
		})
	}
}

// TestYUVHandWorked checks a 4 by 4 picture whose four chroma blocks differ,
// pixel by pixel against the float formula.
func TestYUVHandWorked(t *testing.T) {
	t.Parallel()

	y := []byte{
		16, 60, 100, 140,
		180, 220, 235, 120,
		30, 70, 110, 150,
		190, 230, 50, 90,
	}
	u := []byte{128, 90, 240, 60}
	v := []byte{128, 240, 110, 200}
	for _, bt709 := range []bool{false, true} {
		dst := make([]byte, 4*4*4)
		YUV420ToRGBA(dst, 16, 4, 4, y, 4, u, v, 2, 1, bt709)
		for row := range 4 {
			for x := range 4 {
				ci := (row/2)*2 + x/2
				want := refRGB(y[row*4+x], u[ci], v[ci], bt709)
				for c := range 3 {
					assert.InDelta(t, clampF(want[c]), dst[(row*4+x)*4+c], 1.01, "709=%v (%d,%d) channel %d", bt709, x, row, c)
				}
			}
		}
	}
	dst := make([]byte, 4*4*4)
	YUV420ToRGBA(dst, 16, 4, 4, y, 4, u, v, 2, 1, false)
	assert.Equal(t, dst[(0*4+0)*4:(0*4+0)*4+3], []byte{0, 0, 0}, "Y 16, U and V 128 is black")
	assert.Greater(t, dst[(0*4+3)*4], dst[(0*4+3)*4+2], "U 90 and V 240 make the top right block red")
}

// TestYUVLayouts checks that planar, NV12 and NV21 buffers, with row
// padding, give the same picture, odd sizes included.
func TestYUVLayouts(t *testing.T) {
	t.Parallel()

	sizes := [][2]int{{4, 4}, {5, 3}, {1, 1}, {7, 6}, {2, 5}}
	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		cw, ch := (w+1)/2, (h+1)/2
		yPlane := make([]byte, w*h)
		uPlane := make([]byte, cw*ch)
		vPlane := make([]byte, cw*ch)
		for i := range yPlane {
			yPlane[i] = byte(16 + i*7%220)
		}
		for i := range uPlane {
			uPlane[i], vPlane[i] = byte(16+i*37%225), byte(16+i*53%225)
		}
		want := make([]byte, 4*w*h)
		YUV420ToRGBA(want, 4*w, w, h, yPlane, w, uPlane, vPlane, cw, 1, false)

		// Padded luma rows and padded destination.
		yPad := make([]byte, (w+3)*h)
		for r := range h {
			copy(yPad[r*(w+3):], yPlane[r*w:(r+1)*w])
		}
		// NV12: UVUV...; NV21: VUVU...
		nv12 := make([]byte, (2*cw+2)*ch)
		nv21 := make([]byte, (2*cw+2)*ch)
		for r := range ch {
			for c := range cw {
				nv12[r*(2*cw+2)+2*c], nv12[r*(2*cw+2)+2*c+1] = uPlane[r*cw+c], vPlane[r*cw+c]
				nv21[r*(2*cw+2)+2*c], nv21[r*(2*cw+2)+2*c+1] = vPlane[r*cw+c], uPlane[r*cw+c]
			}
		}
		for name, run := range map[string]func(dst []byte, stride int){
			"padded luma": func(dst []byte, stride int) {
				YUV420ToRGBA(dst, stride, w, h, yPad, w+3, uPlane, vPlane, cw, 1, false)
			},
			"nv12": func(dst []byte, stride int) {
				YUV420ToRGBA(dst, stride, w, h, yPlane, w, nv12, nv12[1:], 2*cw+2, 2, false)
			},
			"nv21": func(dst []byte, stride int) {
				YUV420ToRGBA(dst, stride, w, h, yPlane, w, nv21[1:], nv21, 2*cw+2, 2, false)
			},
		} {
			stride := 4*w + 8
			dst := make([]byte, stride*h)
			run(dst, stride)
			for r := range h {
				require.Equal(t, want[r*4*w:(r+1)*4*w], dst[r*stride:r*stride+4*w], "%s %dx%d row %d", name, w, h, r)
			}
		}
	}
}

func TestYUVBadInput(t *testing.T) {
	t.Parallel()

	good := func() (dst, y, u, v []byte) {
		return make([]byte, 4*4*4), make([]byte, 16), make([]byte, 4), make([]byte, 4)
	}
	tests := []struct {
		name string
		call func(dst, y, u, v []byte)
	}{
		{"no width", func(dst, y, u, v []byte) { YUV420ToRGBA(dst, 16, 0, 4, y, 4, u, v, 2, 1, false) }},
		{"no height", func(dst, y, u, v []byte) { YUV420ToRGBA(dst, 16, 4, 0, y, 4, u, v, 2, 1, false) }},
		{"short dst", func(dst, y, u, v []byte) { YUV420ToRGBA(dst[:60], 16, 4, 4, y, 4, u, v, 2, 1, false) }},
		{"short y", func(dst, y, u, v []byte) { YUV420ToRGBA(dst, 16, 4, 4, y[:15], 4, u, v, 2, 1, false) }},
		{"short u", func(dst, y, u, v []byte) { YUV420ToRGBA(dst, 16, 4, 4, y, 4, u[:3], v, 2, 1, false) }},
		{"short v", func(dst, y, u, v []byte) { YUV420ToRGBA(dst, 16, 4, 4, y, 4, u, v[:3], 2, 1, false) }},
		{"stride too small", func(dst, y, u, v []byte) { YUV420ToRGBA(dst, 8, 4, 4, y, 4, u, v, 2, 1, false) }},
		{"no pixel stride", func(dst, y, u, v []byte) { YUV420ToRGBA(dst, 16, 4, 4, y, 4, u, v, 2, 0, false) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dst, y, u, v := good()
			assert.NotPanics(t, func() { tt.call(dst, y, u, v) })
			assert.Equal(t, make([]byte, 64), dst, "left alone")
		})
	}
}

func TestBT709For(t *testing.T) {
	t.Parallel()

	assert.False(t, BT709For(576))
	assert.False(t, BT709For(719))
	assert.True(t, BT709For(720))
	assert.True(t, BT709For(1080))
}

func TestYUVDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	const w, h = 320, 180
	dst := make([]byte, 4*w*h)
	y := make([]byte, w*h)
	uv := make([]byte, w*h/2)
	allocs := testing.AllocsPerRun(10, func() {
		YUV420ToRGBA(dst, 4*w, w, h, y, w, uv, uv[1:], w, 2, true)
	})
	assert.Zero(t, allocs)
}

func benchmarkYUV(b *testing.B, w, h int) {
	dst := make([]byte, 4*w*h)
	y := make([]byte, w*h)
	for i := range y {
		y[i] = byte(i)
	}
	uv := make([]byte, w*h/2)
	b.ReportAllocs()
	b.SetBytes(int64(4 * w * h))
	for b.Loop() {
		YUV420ToRGBA(dst, 4*w, w, h, y, w, uv, uv[1:], w, 2, true)
	}
}

func BenchmarkYUV640x360(b *testing.B)  { benchmarkYUV(b, 640, 360) }
func BenchmarkYUV1280x720(b *testing.B) { benchmarkYUV(b, 1280, 720) }
