// Adapted from jplayer-mobile/internal/media/enginekit/tapwindow_test.go at 49de3c2.

package enginekit

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTapWindow(t *testing.T) {
	t.Parallel()

	const step = 1.0 / 44100
	tests := []struct {
		name     string
		count    uint64
		end, pos float64
		step     float64
		want     uint64
		ok       bool
	}{
		{"too few samples", 100, math.NaN(), 0, 0, 0, false},
		{"no time: the newest", 1000, math.NaN(), 5, 0, 744, true},
		{"no step: the newest", 1000, 2, 1, 0, 744, true},
		{"at the newest", 1000, 2, 2, step, 744, true},
		{"a little behind the newest", 1000, 2, 2.05, step, 744, true},
		{"far behind the newest", 1000, 2, 2.5, step, 0, false},
		{"ahead by 500 samples", 1000, 2, 2 - 500*step, step, 244, true},
		{"ahead by more than written", 1000, 2, 1, step, 0, false},
		{"window would start before 0", 1000, 2, 2 - 900*step, step, 0, false},
		{"gone from the ring", 100000, 2, 2 - 16300*step, step, 0, false},
		{"just within the ring", 100000, 2, 2 - 16000*step, step, 83744, true},
		{"rate 2: two media samples a sample", 4000, 2, 2 - 1000*2*step, 2 * step, 2744, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := TapWindow(tt.count, tt.end, tt.step, tt.pos, SpectrumSize, 16384)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTapWindowBadSizes(t *testing.T) {
	t.Parallel()

	_, ok := TapWindow(1000, math.NaN(), 0, 0, 0, 16384)
	assert.False(t, ok)
	_, ok = TapWindow(100000, math.NaN(), 0, 0, 512, 256)
	assert.False(t, ok)
}

func TestTapWindowDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	assert.Zero(t, testing.AllocsPerRun(100, func() {
		_, _ = TapWindow(100000, 2, 1.0/44100, 1.99, SpectrumSize, 16384)
	}))
}
