// Adapted from jplayer-mobile/internal/media/enginekit/reattach_test.go at 49de3c2.

package enginekit_test

import (
	"dancefitvn-app/internal/media/enginekit"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestReattachSeek checks when the engine seeks again after giving a
// player its video Surface back: only when the guarded seek's completion
// finds the player away from its target, beyond what it played since
// (Samsung's NuPlayer went to a key frame, 2026-11-09), once, and never
// over a seek of the user's.
func TestReattachSeek(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 9, 6, 0, 0, 0, time.UTC)
	type check struct {
		done, asked int
		pos         float64
		after       time.Duration // since the seek was asked
		at          float64
		again       bool
	}
	tests := []struct {
		name   string
		arm    bool
		rate   float64
		checks []check
	}{
		{name: "not armed", checks: []check{{done: 1, asked: 1, pos: 2}}},
		{
			name: "playing, went back to the key frame",
			arm:  true, rate: 1,
			checks: []check{
				{done: 3, asked: 3, pos: 10.426, after: 230 * time.Millisecond, at: 11.485, again: true},
				{done: 4, asked: 4, pos: 10.426}, // once only
			},
		},
		{
			name: "playing, went back to a key frame just before",
			arm:  true, rate: 1,
			checks: []check{{done: 3, asked: 3, pos: 11.337, after: 200 * time.Millisecond, at: 11.485, again: true}},
		},
		{name: "landed exactly", arm: true, rate: 1, checks: []check{{done: 3, asked: 3, pos: 11.485}}},
		{name: "landed within the slack", arm: true, rate: 1, checks: []check{{done: 3, asked: 3, pos: 11.465}}},
		{
			name: "playing, played on since",
			arm:  true, rate: 1,
			checks: []check{{done: 3, asked: 3, pos: 11.7, after: 300 * time.Millisecond}},
		},
		{
			name: "playing twice as fast, played on since",
			arm:  true, rate: 2,
			checks: []check{{done: 3, asked: 3, pos: 12, after: 300 * time.Millisecond}},
		},
		{
			name: "playing, went on to the next key frame",
			arm:  true, rate: 1,
			checks: []check{{done: 3, asked: 3, pos: 14.583, after: 300 * time.Millisecond, at: 11.485, again: true}},
		},
		{
			name:   "paused, went on to the next key frame",
			arm:    true,
			checks: []check{{done: 3, asked: 3, pos: 14.583, after: 300 * time.Millisecond, at: 11.485, again: true}},
		},
		{
			name:   "paused, landed exactly",
			arm:    true,
			checks: []check{{done: 3, asked: 3, pos: 11.485, after: time.Second}},
		},
		{
			name: "an earlier seek done first",
			arm:  true, rate: 1,
			checks: []check{
				{done: 2, asked: 3, pos: 5},
				{done: 3, asked: 3, pos: 10.426, at: 11.485, again: true},
			},
		},
		{
			name: "the user seeked after it",
			arm:  true, rate: 1,
			checks: []check{
				{done: 3, asked: 4, pos: 10.426},
				{done: 4, asked: 4, pos: 1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var r enginekit.ReattachSeek
			if tt.arm {
				r.Arm(3, 11.485, tt.rate, t0)
			}
			for i, c := range tt.checks {
				at, again := r.Check(c.done, c.asked, c.pos, t0.Add(c.after))
				assert.Equal(t, c.again, again, "check %d", i)
				assert.InDelta(t, c.at, at, 1e-9, "check %d", i)
			}
		})
	}
}

// TestReattachSeekDisarm checks that a disarmed guard seeks nothing.
func TestReattachSeekDisarm(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 9, 6, 0, 0, 0, time.UTC)
	var r enginekit.ReattachSeek
	r.Arm(1, 30, 1, t0)
	r.Disarm()
	_, again := r.Check(1, 1, 20, t0)
	assert.False(t, again)
}

// TestReattachSeekCheckDoesNotAllocate checks the seek completion's
// check allocates nothing.
func TestReattachSeekCheckDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	t0 := time.Date(2026, 11, 9, 6, 0, 0, 0, time.UTC)
	var r enginekit.ReattachSeek
	allocs := testing.AllocsPerRun(100, func() {
		r.Arm(1, 30, 1, t0)
		_, _ = r.Check(1, 1, 20, t0.Add(time.Second))
	})
	assert.Zero(t, allocs)
}
