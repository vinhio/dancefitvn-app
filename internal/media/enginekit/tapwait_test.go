// Adapted from jplayer-mobile/internal/media/enginekit/tapwait_test.go at 49de3c2.

package enginekit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTapWait(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	at := func(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }
	type step struct {
		ms      int
		playing bool
	}
	tests := []struct {
		name  string
		steps []step
		now   int
		want  bool
	}{
		{"never played", nil, 60_000, false},
		{"paused a long time, never played", []step{{0, false}, {10_000, false}}, 60_000, false},
		{"playing, under the limit", []step{{0, true}}, 1499, false},
		{"playing, at the limit", []step{{0, true}}, 1500, true},
		{"played under the limit, paused long", []step{{0, true}, {1000, false}}, 60_000, false},
		{"played in two spans, the sum at the limit", []step{{0, true}, {1000, false}, {5000, true}}, 5500, true},
		{"played in two spans, the sum under it", []step{{0, true}, {1000, false}, {5000, true}}, 5400, false},
		{"playing told twice counts once", []step{{0, true}, {1000, true}}, 1500, true},
		{"paused told twice counts once", []step{{0, true}, {1000, false}, {2000, false}}, 60_000, false},
		{"a clock gone back counts nothing", []step{{5000, true}, {0, false}}, 60_000, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var w TapWait
			for _, s := range tt.steps {
				w.SetPlaying(s.playing, at(s.ms))
			}
			assert.Equal(t, tt.want, w.Starved(at(tt.now)))
		})
	}
}

func TestTapAnswers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		ready, attached bool
		written         uint64
		starved         bool
		want            bool
	}{
		{"loading: silence until the tap", false, false, 0, false, true},
		{"loading, whatever the wait says", false, false, 0, true, true},
		{"ready, no audio track: synthetic", true, false, 0, false, false},
		{"ready, paused, nothing yet: silence", true, true, 0, false, true},
		{"playing, first buffer not yet: silence", true, true, 0, false, true},
		{"played the limit, no buffer (AirPlay): synthetic", true, true, 0, true, false},
		{"gave samples: real", true, true, 4096, false, true},
		{"gave samples, then starved: still real", true, true, 4096, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, TapAnswers(tt.ready, tt.attached, tt.written, tt.starved))
		})
	}
}

func TestTapWaitDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	var w TapWait
	t0 := time.Now()
	assert.Zero(t, testing.AllocsPerRun(100, func() {
		w.SetPlaying(true, t0)
		_ = TapAnswers(true, true, 0, w.Starved(t0.Add(time.Second)))
		w.SetPlaying(false, t0.Add(time.Millisecond))
	}))
}
