// Adapted from jplayer-mobile/internal/ui/np/video_test.go at 49de3c2
// (TestFitRect, TestFitRectEmpty, TestTurnMapsUnturnedToFitted), with the
// mirror.

package kit

import (
	"testing"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
)

func TestFitRect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		w, h, rot    int
		box          geom.Rect
		fit          Fit
		x, y, rw, rh float32
	}{
		{"contain wide in square", 160, 90, 0, geom.XYWH(0, 0, 300, 300), FitContain, 0, 65.625, 300, 168.75},
		{"cover wide in square", 160, 90, 0, geom.XYWH(0, 0, 300, 300), FitCover, -116.6667, 0, 533.3333, 300},
		{"contain 9:16 in the 393 by 852 screen: bars above and below", 480, 854, 0, geom.XYWH(0, 0, 393, 852), FitContain, 0, 76.4, 393, 699.2},
		{"contain a wide picture in the 393 by 852 screen", 854, 480, 0, geom.XYWH(0, 0, 393, 852), FitContain, 0, 315.555, 393, 220.89},
		{"box offset", 160, 90, 0, geom.XYWH(10, 20, 300, 300), FitContain, 10, 85.625, 300, 168.75},
		{"contain turned 90", 160, 90, 90, geom.XYWH(0, 0, 200, 200), FitContain, 43.75, 0, 112.5, 200},
		{"contain turned 270", 160, 90, 270, geom.XYWH(0, 0, 200, 200), FitContain, 43.75, 0, 112.5, 200},
		{"contain turned 180", 160, 90, 180, geom.XYWH(0, 0, 300, 300), FitContain, 0, 65.625, 300, 168.75},
		{"cover turned 90", 160, 90, 90, geom.XYWH(0, 0, 200, 200), FitCover, 0, -77.7778, 200, 355.5556},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fitRect(tt.w, tt.h, tt.rot, tt.box, tt.fit)
			assert.InDelta(t, tt.x, got.X, .1)
			assert.InDelta(t, tt.y, got.Y, .1)
			assert.InDelta(t, tt.rw, got.W, .1)
			assert.InDelta(t, tt.rh, got.H, .1)
		})
	}
}

func TestFitRectEmpty(t *testing.T) {
	t.Parallel()

	box := geom.XYWH(0, 0, 100, 100)
	assert.True(t, fitRect(0, 90, 0, box, FitContain).Empty())
	assert.True(t, fitRect(160, 0, 0, box, FitContain).Empty())
	assert.True(t, fitRect(160, 90, 0, geom.Rect{}, FitCover).Empty())
	assert.True(t, fitRect(160, 90, 0, box, FitUnknown).Empty())
	assert.True(t, fitRect(160, 90, 0, box, Fit(99)).Empty())
}

func TestTurnMapsUnturnedToFitted(t *testing.T) {
	t.Parallel()

	dst := geom.XYWH(10, 20, 112, 200)
	c := geom.Pt(dst.X+dst.W/2, dst.Y+dst.H/2)
	for _, rot := range []int{0, 90, 180, 270} {
		pre := unturned(dst, rot)
		got := turn(rot, c).Bounds(pre)
		assert.InDelta(t, dst.X, got.X, .001, "%d", rot)
		assert.InDelta(t, dst.Y, got.Y, .001, "%d", rot)
		assert.InDelta(t, dst.W, got.W, .001, "%d", rot)
		assert.InDelta(t, dst.H, got.H, .001, "%d", rot)
	}
	assert.Equal(t, turn(0, c), turn(45, c), "a bad angle is no turn")
}

func TestMirrorAbout(t *testing.T) {
	t.Parallel()

	m := mirrorAbout(100)
	assert.Equal(t, geom.Pt(170, 33), m.Apply(geom.Pt(30, 33)), "a point left of the line goes right of it")
	assert.Equal(t, geom.Pt(100, 5), m.Apply(geom.Pt(100, 5)), "the line stays")
	assert.Equal(t, geom.Pt(30, 33), m.Apply(m.Apply(geom.Pt(30, 33))), "twice is no flip")
}
