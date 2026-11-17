// Adapted from jplayer-mobile/internal/media/enginekit/size_test.go at 49de3c2.

package enginekit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		w, h, limit  int
		wantW, wantH int
	}{
		{"full hd", 1920, 1080, MaxEdge, 1280, 720},
		{"small is kept", 640, 360, MaxEdge, 640, 360},
		{"exactly the limit", 1280, 720, MaxEdge, 1280, 720},
		{"portrait", 1080, 1920, MaxEdge, 720, 1280},
		{"odd sides become even", 641, 361, MaxEdge, 640, 360},
		{"odd after scaling", 1921, 1081, MaxEdge, 1280, 720},
		{"4k", 3840, 2160, MaxEdge, 1280, 720},
		{"square", 2000, 2000, MaxEdge, 1280, 1280},
		{"anamorphic ultra wide", 5000, 100, MaxEdge, 1280, 26},
		{"a thin one stays two", 5000, 1, MaxEdge, 1280, 2},
		{"no limit", 4000, 3000, 0, 4000, 3000},
		{"zero width", 0, 100, MaxEdge, 0, 0},
		{"negative height", 100, -1, MaxEdge, 0, 0},
		{"the 1179 by 663 of M0.4", 1179, 663, MaxEdge, 1178, 662},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w, h := Fit(tt.w, tt.h, tt.limit)
			assert.Equal(t, tt.wantW, w)
			assert.Equal(t, tt.wantH, h)
		})
	}
}

func TestProbeAt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dur  float64
		want float64
	}{
		{"unknown", 0, 1},
		{"negative", -5, 1},
		{"short is not the fallback: 0.6 is truthy", 4, 0.6},
		{"forty seconds", 40, 6},
		{"twenty", 20, 3},
		{"cap", 600, 12},
		{"just at the cap", 80, 12},
		{"just under", 79, 11.85},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.InDelta(t, tt.want, ProbeAt(tt.dur), 1e-9)
		})
	}
}

func TestThumbSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		vw, vh int
		wantH  int
	}{
		{"16 by 9", 1920, 1080, 180},
		{"640 by 360", 640, 360, 180},
		{"4 by 3", 640, 480, 240},
		{"portrait", 1080, 1920, 569}, // 568.89
		{"rounds half up", 640, 361, 181},
		{"square", 100, 100, 320},
		{"unknown", 0, 0, 180},
		{"no height", 100, 0, 180},
		{"vanishing height rounds to 0", 100000, 1, 180},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w, h := ThumbSize(tt.vw, tt.vh)
			assert.Equal(t, 320, w)
			assert.Equal(t, tt.wantH, h)
		})
	}
}

func TestRotationOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		a, b, c, d float64
		want       int
	}{
		{"identity", 1, 0, 0, 1, 0},
		{"portrait iPhone, 90 clockwise", 0, 1, -1, 0, 90},
		{"upside down", -1, 0, 0, -1, 180},
		{"270", 0, -1, 1, 0, 270},
		{"slightly off 90", 0.0001, 1, -1, 0.0001, 90},
		{"scaled rotation", 0, 2, -2, 0, 90},
		{"flip is no turn", -1, 0, 0, 1, 180},
		{"zero matrix", 0, 0, 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, RotationOf(tt.a, tt.b, tt.c, tt.d))
		})
	}
}

func TestTurned(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		w, h, deg    int
		wantW, wantH int
	}{
		{"0", 640, 360, 0, 640, 360},
		{"90", 640, 360, 90, 360, 640},
		{"180", 640, 360, 180, 640, 360},
		{"270", 640, 360, 270, 360, 640},
		{"360", 640, 360, 360, 640, 360},
		{"minus 90", 640, 360, -90, 360, 640},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w, h := Turned(tt.w, tt.h, tt.deg)
			assert.Equal(t, tt.wantW, w)
			assert.Equal(t, tt.wantH, h)
		})
	}
}
