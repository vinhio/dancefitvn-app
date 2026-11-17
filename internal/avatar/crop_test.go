package avatar

import (
	"image"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestSquare checks the part of a picture the box shows: its centred
// square, the extra pixel of an odd difference right or below.
func TestSquare(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		w, h int
		want image.Rectangle
	}{
		{"landscape", 400, 300, image.Rect(50, 0, 350, 300)},
		{"portrait", 300, 400, image.Rect(0, 50, 300, 350)},
		{"square", 300, 300, image.Rect(0, 0, 300, 300)},
		{"odd difference", 301, 300, image.Rect(0, 0, 300, 300)},
		{"odd difference tall", 300, 303, image.Rect(0, 1, 300, 301)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, Square(tc.w, tc.h))
		})
	}
}

// TestSourceRect checks the part of the picture the ring keeps, against
// the prototype's transform worked by hand: the box (260 DIP) shows the
// square of side m at k = 260 / m DIP a pixel, moved by (x, y) and zoomed
// about its centre; the ring's square is ±100 DIP about the frame's
// centre.
func TestSourceRect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		w, h int
		v    View
		want Rect
	}{
		// 260 px square: 1 px a DIP; the ring keeps ±100 about the centre.
		{"square zoom 1", 260, 260, Picked, Rect{30, 30, 230, 230}},
		// zoom 2: ±50 px.
		{"square zoom 2", 260, 260, View{Zoom: 2}, Rect{80, 80, 180, 180}},
		// zoom 3, the slider's top: ±33.3 px.
		{"square zoom 3", 260, 260, View{Zoom: MaxZoom}, Rect{130 - 100.0/3, 130 - 100.0/3, 130 + 100.0/3, 130 + 100.0/3}},
		// the sheet's first view: ±83.3 px.
		{"square zoom 1.2", 260, 260, Initial, Rect{130 - 100/1.2, 130 - 100/1.2, 130 + 100/1.2, 130 + 100/1.2}},
		// box moved 20 DIP right and 10 up: the ring sees 20 px further
		// left and 10 further down of the picture.
		{"moved", 260, 260, View{Zoom: 1, X: 20, Y: -10}, Rect{10, 40, 210, 240}},
		// moved and zoomed: the offset is in DIP, divided by the zoom.
		{"moved zoom 2", 260, 260, View{Zoom: 2, X: 20, Y: -10}, Rect{70, 85, 170, 185}},
		// landscape 520 × 260: square [130, 390], 1 px a DIP.
		{"landscape", 520, 260, Picked, Rect{160, 30, 360, 230}},
		// portrait 1300 × 2600: square side 1300, 5 px a DIP, centre (650, 1300).
		{"portrait", 1300, 2600, Picked, Rect{150, 800, 1150, 1800}},
		// moved past the box: the rect leaves the square.
		{"past the box", 260, 260, View{Zoom: 1, X: 300}, Rect{-270, 30, -70, 230}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := SourceRect(tc.w, tc.h, tc.v)
			assert.InDelta(t, tc.want.X0, got.X0, 1e-9, "x0")
			assert.InDelta(t, tc.want.Y0, got.Y0, 1e-9, "y0")
			assert.InDelta(t, tc.want.X1, got.X1, 1e-9, "x1")
			assert.InDelta(t, tc.want.Y1, got.Y1, 1e-9, "y1")
		})
	}
}

// TestPlacement checks where the box is drawn: its centre at the offset,
// its side 260 × zoom.
func TestPlacement(t *testing.T) {
	t.Parallel()

	cx, cy, side := Placement(View{Zoom: 1.5, X: -12, Y: 7})
	assert.InDelta(t, -12.0, cx, 0)
	assert.InDelta(t, 7.0, cy, 0)
	assert.InDelta(t, 390.0, side, 1e-9)
	assert.InDelta(t, 1.2, Initial.Zoom, 0, "the sheet opens at 1.2 (index.html:284)")
	assert.Equal(t, View{Zoom: 1}, Picked, "a new picture resets (index.html:628)")
}

// TestViewValid checks the views Export refuses.
func TestViewValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    View
		want bool
	}{
		{"picked", Picked, true},
		{"far and zoomed", View{Zoom: 3, X: -1e6, Y: 1e6}, true},
		{"zero zoom", View{}, false},
		{"negative zoom", View{Zoom: -1}, false},
		{"NaN zoom", View{Zoom: math.NaN()}, false},
		{"infinite zoom", View{Zoom: math.Inf(1)}, false},
		{"NaN x", View{Zoom: 1, X: math.NaN()}, false},
		{"infinite y", View{Zoom: 1, Y: math.Inf(-1)}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, tc.v.Valid())
		})
	}
}

// TestSourceRectDoesNotAllocate checks the crop's math allocates nothing:
// the sheet may ask it on every drag move.
func TestSourceRectDoesNotAllocate(t *testing.T) {
	v := View{Zoom: 1.7, X: 12, Y: -30}
	var sink Rect
	n := testing.AllocsPerRun(100, func() {
		sink = SourceRect(4032, 3024, v)
		_, _, _ = Placement(v)
		_ = Square(4032, 3024)
	})
	assert.Zero(t, n)
	assert.Positive(t, sink.X1)
}

// BenchmarkSourceRect measures the crop's math.
func BenchmarkSourceRect(b *testing.B) {
	v := View{Zoom: 1.7, X: 12, Y: -30}
	b.ReportAllocs()
	for b.Loop() {
		_ = SourceRect(4032, 3024, v)
	}
}
