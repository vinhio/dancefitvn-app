package player_test

import (
	"dancefitvn-app/internal/media"
	"testing"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/safe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResolverPanicIsAFailedLoad makes the Resolver panic (it parses the server's
// answer for the address): the player shows the error state, as for any failed
// load, instead of staying in loading for ever or ending the app, and a press on
// play asks again and plays.
// (docs/plan/03 section 9: "Không phát được bài này. Thử lại sau")
func TestResolverPanicIsAFailedLoad(t *testing.T) {
	t.Parallel()
	safe.Expect("player.resolve")

	tests := []struct {
		name  string
		start func(r *rig)
	}{
		{"video", func(r *rig) { r.playVideo(0) }},
		{"audio", func(r *rig) { r.playAudio(0) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res := &resolver{panicAt: map[int]bool{1: true}}
			r := newRig(t, rigOpts{eng: fullLength().eng, res: res})
			tc.start(r)
			requireState(t, r.p, player.StateError)
			require.ErrorIs(t, r.p.Err(), safe.ErrPanic)

			r.p.Toggle()
			r.settle()
			requireState(t, r.p, player.StatePlaying)
			assert.NoError(t, r.p.Err())
		})
	}
}

// TestNextAfterAFailedLessonPlaysTheNext checks the recovery that needs no retry:
// a lesson whose file the system cannot decode (a codec it lacks, a damaged file)
// shows the error state, and Next loads the lesson after it.
// (docs/plan/03 section 9)
func TestNextAfterAFailedLessonPlaysTheNext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"unsupported codec", media.ErrDecode},
		{"source cannot be opened", media.ErrSource},
		{"file missing", media.ErrMissingFile},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			eo := fullLength().eng
			eo.Fail = map[string]error{videoURL("l_1"): tc.err}
			r := newRig(t, rigOpts{eng: eo})
			r.playVideo(0)
			requireState(t, r.p, player.StateError)
			require.ErrorIs(t, r.p.Err(), tc.err)

			r.p.Next() // a failed lesson wants nothing: the next one loads ready, as after a pause
			r.settle()
			requireState(t, r.p, player.StatePaused)
			assert.Equal(t, "l_2", r.p.Lesson().ID)
			assert.NoError(t, r.p.Err())

			r.p.Toggle()
			r.settle()
			requireState(t, r.p, player.StatePlaying)
		})
	}
}
