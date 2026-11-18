package progress_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// savedAt tells the position the n-th save carried.
func savedAt(t *testing.T, r *rig, n int) int {
	t.Helper()
	r.tr.Close()
	require.Greater(t, len(r.api.saves), n)

	return r.api.saves[n].req.PositionSec
}

// TestSeekIntoTheWatchedPartIsNotWatching checks K38: a jump from 20 s to 215 s
// of 220 (the threshold is 198) counts the lesson watched only when playback gets
// to the end, and meanwhile what is saved stays under the threshold.
func TestSeekIntoTheWatchedPartIsNotWatching(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.tr.Update("l_3503", "set-35", 20, 220, true)
	r.tr.Seeked("l_3503", 215, 220)
	r.tr.Update("l_3503", "set-35", 215, 220, true) // the engine reports the new position at once
	assert.Empty(t, r.won, "a jump is not watching")

	r.tr.Update("l_3503", "set-35", 217.75, 220, true)
	assert.Empty(t, r.won, "playing on inside the part is not yet the end")
	r.tr.Pause()
	assert.Equal(t, 197, savedAt(t, r, 0), "the position saved is the last whole second below the threshold (198 s)")
}

// TestSeekIntoTheWatchedPartIsWatchedAtTheEnd checks that playback reaching the end
// after the jump counts, and the true position is saved again.
func TestSeekIntoTheWatchedPartIsWatchedAtTheEnd(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.tr.Update("l_3503", "set-35", 20, 220, true)
	r.tr.Seeked("l_3503", 219, 220) // the bar stops one second short of the end
	r.tr.Update("l_3503", "set-35", 219, 220, true)
	require.Empty(t, r.won)
	r.tr.Update("l_3503", "set-35", 219.75, 220, true) // within half a second of the end
	assert.Equal(t, []string{"l_3503"}, r.won)
	r.tr.Update("l_3503", "set-35", 220, 220, false)
	r.tr.Pause()
	assert.Equal(t, 220, savedAt(t, r, 0))
}

// TestSeekBackOutOfTheWatchedPartLiftsTheHold checks that a later seek to before the
// threshold plays as ever: watched at the threshold.
func TestSeekBackOutOfTheWatchedPartLiftsTheHold(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.tr.Update("l_3503", "set-35", 20, 220, true)
	r.tr.Seeked("l_3503", 215, 220)
	r.tr.Update("l_3503", "set-35", 215, 220, true)
	r.tr.Seeked("l_3503", 100, 220)
	r.tr.Update("l_3503", "set-35", 100, 220, true)
	assert.Empty(t, r.won)
	r.tr.Update("l_3503", "set-35", 198, 220, true)
	assert.Equal(t, []string{"l_3503"}, r.won, "watched at 90 % by playing, as before")
}

// TestSeekInsideTheBodyChangesNothing checks that a seek below the threshold, and a
// seek in a lesson already counted, keep the true positions and the watched rule.
func TestSeekInsideTheBodyChangesNothing(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.tr.Update("l_3503", "set-35", 20, 220, true)
	r.tr.Seeked("l_3503", 150, 220)
	r.tr.Update("l_3503", "set-35", 150, 220, true)
	r.tr.Pause()
	assert.Equal(t, 150, savedAt(t, r, 0))

	r = newRig(t)
	r.tr.Update("l_3503", "set-35", 199, 220, true)
	require.Equal(t, []string{"l_3503"}, r.won)
	r.tr.Seeked("l_3503", 210, 220) // counted in this visit already
	r.tr.Update("l_3503", "set-35", 210, 220, true)
	r.tr.Pause()
	assert.Equal(t, 210, savedAt(t, r, 0), "the true position is saved")
}

// TestSeekedIsPerLesson checks that a hold made in one lesson is not carried to
// another.
func TestSeekedIsPerLesson(t *testing.T) {
	t.Parallel()

	r := newRig(t)
	r.tr.Update("l_3503", "set-35", 20, 220, true)
	r.tr.Seeked("l_3503", 215, 220)
	r.tr.Update("l_3504", "set-35", 199, 220, true)
	assert.Equal(t, []string{"l_3504"}, r.won)
}

// TestSeekedDoesNotAllocate checks that the hold costs a tick nothing.
func TestSeekedDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	r := newRig(t)
	r.tr.Update("l_3503", "set-35", 20, 220, false)
	r.tr.Seeked("l_3503", 215, 220)
	r.tr.Update("l_3503", "set-35", 215, 220, false)
	allocs := testing.AllocsPerRun(200, func() {
		r.tr.Seeked("l_3503", 215, 220)
		r.tr.Update("l_3503", "set-35", 216, 220, false)
	})
	assert.Zero(t, allocs)
}
