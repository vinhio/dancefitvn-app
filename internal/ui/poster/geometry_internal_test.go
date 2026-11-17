package poster

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGeometryFormulas checks the numbers of poster() on the sizes the
// prototype uses, against values worked out by hand from index.html:291-299.
func TestGeometryFormulas(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                   string
		w, h, ts               float32
		noTitle                bool
		wantTS, wantFig        float32
		wantFigX, wantFigY     float32
		wantRingX, wantRingY   float32
		wantRingD, wantRingBdr float32
	}{
		// 156x100: ts 20 given; h<130 so fh = min(100*.62, 156*.3) = 46.8;
		// the group is 1.75*46.8 = 81.9 wide: left 156-6.24-81.9 = 67.86 (the
		// rect Chrome reports for the dancers' box is 67.859); ring 110 at
		// right -25, top -20, border 7 (computed style 7px).
		{"156x100", 156, 100, 20, false, 20, 46.8, 67.86, 47.2, 71, -20, 110, 7},
		// 353x232 without title: fh = min(232*.72, 353*.42) = 148.26; left
		// 353-14.12-259.455 = 79.425 (Chrome: 79.453); ring 255.2 at x 155.8,
		// top -46.4 (Chrome: 155.813, -46.391).
		{"353x232", 353, 232, 0, true, 32.48, 148.26, 79.43, 69.84, 155.8, -46.4, 255.2, 16.24},
		// 540x304 (TV), ts 54: fh = min(304*.72, 540*.3) = 162.
		{"540x304", 540, 304, 54, false, 54, 162, 540 - 21.6 - 283.5, 304 - 18.24 - 162, 540 + 76 - 334.4, -60.8, 334.4, 21.28},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := newGeo(tt.w, tt.h, tt.ts, tt.noTitle)
			if tt.ts == 0 {
				tt.wantTS = min(tt.h*.2, tt.w*.14)
			}
			assert.InDelta(t, tt.wantTS, g.ts, .01, "ts")
			assert.InDelta(t, tt.wantFig, g.figH, .01, "fh")
			assert.InDelta(t, tt.wantFigX, g.figX, .05, "group left")
			assert.InDelta(t, tt.wantFigY, g.figY, .05, "group top")
			assert.InDelta(t, tt.wantRingX, g.ring.X, .05, "ring left")
			assert.InDelta(t, tt.wantRingY, g.ring.Y, .05, "ring top")
			assert.InDelta(t, tt.wantRingD, g.ring.W, .05, "ring D")
			assert.InDelta(t, tt.wantRingBdr, g.ringWidth, .01, "ring border")
		})
	}
}

// TestGeometryTallPosterFigure checks the h>=130 branch: fh = h*.72.
func TestGeometryTallPosterFigure(t *testing.T) {
	t.Parallel()
	g := newGeo(1000, 200, 0, false)
	assert.InDelta(t, 144, g.figH, .001)
	g = newGeo(1000, 129, 0, false)
	assert.InDelta(t, 129*.62, g.figH, .001)
}

// TestGeometryVinyl checks the disc of Hot Extra: d = .7 m centered at
// (.62 w, .42 h).
func TestGeometryVinyl(t *testing.T) {
	t.Parallel()
	g := newGeo(156, 100, 0, false)
	assert.InDelta(t, 70, g.vinyl.W, .001)
	assert.InDelta(t, 156*.62-35, g.vinyl.X, .001)
	assert.InDelta(t, 100*.42-35, g.vinyl.Y, .001)
}

// TestCSSLine135 checks the gradient line of 135 degrees: from the top left
// corner to the bottom right one for a square, by the CSS length for others.
func TestCSSLine135(t *testing.T) {
	t.Parallel()
	g := newGeo(100, 100, 0, false)
	assert.InDelta(t, 0, g.from.X, .01)
	assert.InDelta(t, 0, g.from.Y, .01)
	assert.InDelta(t, 100, g.to.X, .01)
	assert.InDelta(t, 100, g.to.Y, .01)
	// 200x100: the line is centered, its half length (200+100)*.7071/2.
	g = newGeo(200, 100, 0, false)
	half := (200*.70710678 + 100*.70710678) / 2
	assert.InDelta(t, 100-half*.70710678, g.from.X, .01)
	assert.InDelta(t, 50-half*.70710678, g.from.Y, .01)
}
