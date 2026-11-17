package kit_test

import (
	"math"
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWaveLitBars checks the rule `i / 70 <= pos / dur`: bar i is heard when i <=
// share x 70, so the first floor(share x 70) + 1 of them.
func TestWaveLitBars(t *testing.T) {
	t.Parallel()

	w := kit.NewWave("")
	assert.Equal(t, 1, w.Lit(), "bar 0 at position 0 (0 <= 0)")
	for _, c := range []struct {
		share float32
		want  int
	}{{0, 1}, {72.0 / 220, 23}, {.5, 36}, {40.0 / 220, 13}, {1, 70}, {2, 70}, {-1, 1}} {
		w.SetPos(c.share)
		assert.Equal(t, c.want, w.Lit(), "share %v", c.share)
	}
	assert.Equal(t, "div.wave", w.Name)
}

// TestWavePaintsAndSeeks checks the bars in pixels (heard in --pr, the rest in --sf2,
// heights from the formula, centered on the 40 DIP row) and that a press asks for
// the share of the width it fell on, once, with no drag.
func TestWavePaintsAndSeeks(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var got []float32
		w := kit.NewWave("")
		w.OnSeek = func(share float32) { got = append(got, share) }
		tt := mountStage(t, ph, placeAt(w, 20, 555, 353))
		f := frameOf(w)
		assert.Equal(t, geom.Sz(353, 40), f.Size())

		w.SetPos(72.0 / 220)
		tt.Frame()
		bar := float32(353-69*2) / 70
		centre := func(i int) geom.Point { return geom.Pt(f.X+float32(i)*(bar+2)+bar/2, f.Y+20) }
		heard := vvPixel(tt, centre(5))
		assert.Equal(t, uint8(255), heard.R, "bar 5 is --pr")
		assert.Equal(t, uint8(0xd0), heard.G)
		rest := vvPixel(tt, centre(40))
		assert.Equal(t, uint8(0x2a), rest.R, "bar 40 is --sf2")
		assert.Equal(t, uint8(0x2d), rest.G)
		// bar 0 is 25 + 75 |sin 0 cos 1| = 25 %: 10 of 40, from 15 to 25
		low := vvPixel(tt, geom.Pt(f.X+1, f.Y+5))
		assert.NotEqual(t, uint8(255), low.R, "above the short first bar")

		tt.ClickAt(geom.Pt(f.X+f.W/2, f.Y+20))
		require.Len(t, got, 1)
		assert.InDelta(t, .5, got[0], 1e-3)
		got = got[:0]
		tt.Drag(geom.Pt(f.X+20, f.Y+20), geom.Pt(f.X+200, f.Y+20))
		assert.Len(t, got, 1, "a drag is one press")
	})
}

// TestWaveSetPosDoesNotAllocate checks that moving the heard bars and painting does
// not allocate.
func TestWaveSetPosDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	w := kit.NewWave("")
	tt := mountStage(t, kittest.Phones[0], placeAt(w, 20, 555, 353))
	p := float32(0)
	step := func() {
		p += .004
		if p > 1 {
			p = 0
		}
		w.SetPos(p)
		tt.Frame()
	}
	for range 10 {
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(100, step))
}

// TestRingWedge checks the ring of MS.done: 64 square, the wedge from the top
// clockwise in --pr over --sf2, the 48 DIP hole in --sf, the text in the middle.
func TestRingWedge(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := kit.NewRing("2/10")
		tt := mountStage(t, ph, placeAt(r, 100, 300, 64))
		f := frameOf(r)
		assert.Equal(t, geom.Sz(64, 64), f.Size())
		assert.Equal(t, "2/10", r.Text())
		// at samples the ring's band (radius 28, between the hole's 24 and the edge's
		// 32) at deg degrees clockwise from the top
		at := func(deg float64) uint8 {
			rad := deg * math.Pi / 180
			return vvPixel(tt, geom.Pt(f.X+32+float32(28*math.Sin(rad)), f.Y+32-float32(28*math.Cos(rad)))).R
		}

		r.SetShare(.25) // the first quarter: from 12 to 3 o'clock
		tt.Frame()
		assert.Equal(t, uint8(255), at(45), "1:30 is --pr")
		assert.Equal(t, uint8(255), at(2), "just right of 12 is --pr")
		assert.Equal(t, uint8(255), at(88), "just over 3 o'clock is --pr")
		for _, deg := range []float64{92, 135, 180, 225, 270, 315, 358} {
			assert.Equal(t, uint8(0x2a), at(deg), "%v degrees is --sf2", deg)
		}
		assert.Equal(t, uint8(0x1f), vvPixel(tt, geom.Pt(f.X+32, f.Y+52)).R, "the hole is --sf")
		r.SetShare(.2) // 72 degrees
		tt.Frame()
		assert.Equal(t, uint8(255), at(70))
		assert.Equal(t, uint8(0x2a), at(74))
		r.SetShare(.75)
		tt.Frame()
		assert.Equal(t, uint8(255), at(268))
		assert.Equal(t, uint8(0x2a), at(272))
		r.SetShare(1)
		tt.Frame()
		for _, deg := range []float64{1, 91, 181, 271, 359} {
			assert.Equal(t, uint8(255), at(deg), "all of it")
		}
		r.SetShare(0)
		tt.Frame()
		assert.Equal(t, uint8(0x2a), at(45), "none of it: --sf2")
	})
}

// TestRingSetShareDoesNotAllocate checks that a new share and the paint of the wedge
// allocate nothing once the path has its memory.
func TestRingSetShareDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	r := kit.NewRing("2/10")
	tt := mountStage(t, kittest.Phones[0], placeAt(r, 100, 300, 64))
	s := float32(0)
	step := func() {
		s += .01
		if s > 1 {
			s = 0
		}
		r.SetShare(s)
		tt.Frame()
	}
	for range 120 {
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(100, step))
}
