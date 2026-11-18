package kit_test

import (
	"image/color"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
)

// topOf returns the pixel at the top of a ring of side 88 placed at (20, 20): the
// middle of the line, 4 under the edge.
func topOf(tt interface{ RGBAAt(x, y int) color.RGBA }) color.RGBA { return tt.RGBAAt(20+44, 20+4) }

// TestSpinnerSizeAndArc: a ring is the side it is given, 18 by default, with the arc
// in --pr at the top when it rests (motion reduced) and gone from there a quarter
// turn on (K47).
func TestSpinnerSizeAndArc(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		def := kit.NewSpinner(0, 0)
		big := kit.NewSpinner(88, 8)
		tt := kittest.Mount(t, ph, newStage(placeAt(def, 100, 400, 0), placeAt(big, 20, 20, 0)))
		assert.Equal(t, geom.Sz(18, 18), def.Frame().Size())
		assert.Equal(t, geom.Sz(88, 88), big.Frame().Size())

		rest := topOf(tt.Snapshot()) // the motion is reduced: the arc stands at the top
		assert.Greater(t, rest.R, uint8(240), "--pr's red")
		assert.Greater(t, rest.G, uint8(190), "--pr's green")
		assert.Less(t, rest.B, uint8(80), "--pr's blue")

		kittest.PinMotion(tt)
		tt.Repaint()
		tt.Elapse(kit.SpinnerPeriod / 2)
		later := topOf(tt.Snapshot())
		assert.NotEqual(t, rest, later, "the arc has left the top")
		assert.Less(t, later.R, uint8(120), "and the track, white at .22, shows there")
	})
}

// TestSpinnerDoesNotAllocate: a frame of the turning ring allocates nothing (K47).
func TestSpinnerDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	tt := kittest.Mount(t, kittest.Phones[0], newStage(placeAt(kit.NewSpinner(88, 8), 20, 20, 0)))
	kittest.PinMotion(tt)
	step := func() { tt.Elapse(time.Second / 60) }
	for range 3 {
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(100, step), "a frame of the ring, over more than a turn")
}
