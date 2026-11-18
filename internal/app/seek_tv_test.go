//go:build tvos

package app

import (
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	tvenv "dancefitvn-app/internal/ui/tv/env"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTVSeekMovesThePlayer: the scrub of the position bar (K43) asks the TV's
// Playback for a position; the player moves there (the engine is told, and the
// progress tracker as a seek by hand through the saver's SeekNoter), and the
// clock goes on from it.
func TestTVSeekMovesThePlayer(t *testing.T) {
	t.Parallel()
	r := signedInTV(t)
	c, err := r.w.Course(t.Context(), tvDemoCourse)
	require.NoError(t, err)
	list := tvenv.PlayerLessons(c)
	i := tvenv.LessonIndex(list, tvDemoLesson)
	pb := r.w.Playback()
	pb.PlayVideo(list[i], list)
	r.drive("playing", func() bool {
		if pb.State() == player.StateLoading {
			r.media.Advance(time.Millisecond)
		}

		return pb.State() == player.StatePlaying
	})
	var told []float64
	unsub := pb.Changes().Subscribe(func(ch player.Change) {
		if ch.Kind == player.ChangeSeek {
			told = append(told, pb.Pos())
		}
	})
	defer unsub()
	pb.Seek(150)
	assert.Equal(t, []float64{150}, told, "the screens hear one jump")
	assert.InDelta(t, 150, pb.Pos(), 0.5)
	r.media.Advance(2 * time.Second)
	r.q.drain()
	assert.InDelta(t, 150+2*pb.Rate(), pb.Pos(), 1, "it plays on from there")
	pb.Seek(1e6)
	assert.InDelta(t, pb.Dur(), pb.Pos(), 1, "kept inside the media")
}
