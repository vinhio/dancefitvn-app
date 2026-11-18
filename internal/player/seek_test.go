package player_test

import (
	"context"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/progress"
	"dancefitvn-app/internal/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserSeeksAreToldToTheSaverFirst checks K38: Seek and SeekBy tell a SeekNoter
// the position they move to before the engine reports it, so that the Saver can
// hold back a jump into the watched part of the lesson; the loop's own jumps and a
// seek with no lesson are not the user's.
func TestUserSeeksAreToldToTheSaverFirst(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.SetRate(1)
	r.sav.order = nil

	r.p.Seek(150)
	require.Len(t, r.sav.seeked, 1)
	assert.InDelta(t, 150, r.sav.seeked[0].pos, 0)
	assert.InDelta(t, 220, r.sav.seeked[0].dur, 0)
	assert.Equal(t, "l_1", r.sav.seeked[0].lesson)
	assert.Equal(t, "seek", r.sav.order[0], "told before the position the engine reports")

	r.p.SeekBy(10)
	require.Len(t, r.sav.seeked, 2)
	assert.InDelta(t, 160, r.sav.seeked[1].pos, 0)
	r.p.SeekBy(1000)
	assert.InDelta(t, 219, r.sav.seeked[2].pos, 0, "the buttons stop a second short of the end")
	r.p.Seek(999)
	assert.InDelta(t, 220, r.sav.seeked[3].pos, 0, "kept in the media")

	// the loop's wrap is playback, not a seek by hand
	r.p.Seek(0)
	r.p.SetLoop(true)
	n := len(r.sav.seeked)
	r.tick(4 * 31)
	assert.Equal(t, n, len(r.sav.seeked))
	assert.Less(t, r.p.Pos(), 30.0, "it wrapped")
}

// TestSeekNearTheEndIsNotTheEnd checks that a seek to the last second does not end
// the lesson or ask for the next one: only playback reaching the end does.
func TestSeekNearTheEndIsNotTheEnd(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.SetRate(1)
	r.p.SeekBy(1000) // 219 of 220
	r.settle()
	assert.False(t, r.has(player.ChangeEnded))
	assert.True(t, r.p.Playing())
	requireState(t, r.p, player.StatePlaying)
	r.tick(2)
	assert.False(t, r.has(player.ChangeEnded), "219.5")
	r.tick(4)
	assert.True(t, r.has(player.ChangeEnded), "it reached the end by itself")
}

// TestWatchedAfterASeekFollowsPlaybackNotTheJump runs the real Tracker under the
// Player: a jump to the last second marks nothing watched and saves a position
// under the threshold, until the end plays.
func TestWatchedAfterASeekFollowsPlaybackNotTheJump(t *testing.T) {
	t.Parallel()

	var won []string
	tr, snd := newTracker(t, func(id string) { won = append(won, id) })
	r := newRig(t, rigOpts{eng: fullLength().eng, saver: tr})
	r.playVideo(0)
	r.p.SetRate(1)
	r.tick(8) // 2 s of playing
	require.Empty(t, won)

	r.p.Seek(215) // into the part past 90 % of 220 s (198)
	r.settle()
	assert.Empty(t, won, "the jump does not mark it watched")
	r.p.Pause()
	r.settle()
	require.Eventually(t, func() bool { return snd.last() > 0 }, time.Second, time.Millisecond)
	assert.Less(t, snd.last(), 198, "and what is saved is below the threshold")

	r.p.Play()
	r.tick(4 * 6) // 6 s on: 221, the end
	r.settle()
	assert.True(t, r.has(player.ChangeEnded) || len(won) > 0, "playback got to the end")
	assert.Equal(t, []string{"l_1"}, won, "watched at the end")
}

// stubSender saves nothing and remembers the last position sent.
type stubSender struct {
	pos chan int
	cur int
}

func (s *stubSender) SaveProgress(_ context.Context, id string, r model.ProgressRequest) (model.ProgressResult, error) {
	select {
	case s.pos <- r.PositionSec:
	default:
	}

	return model.ProgressResult{LessonID: id, PositionSec: r.PositionSec}, nil
}

func (s *stubSender) SendProgressBatch(context.Context, model.BatchRequest) (model.BatchResult, error) {
	return model.BatchResult{}, api.ErrOffline
}

// last returns the last position sent, 0 before any.
func (s *stubSender) last() int {
	for {
		select {
		case p := <-s.pos:
			s.cur = p
		default:
			return s.cur
		}
	}
}

// newTracker makes a progress.Tracker on a stub sender.
func newTracker(t *testing.T, watched func(string)) (*progress.Tracker, *stubSender) {
	t.Helper()
	snd := &stubSender{pos: make(chan int, 64)}
	q, err := progress.NewQueue(store.NewMemory(), snd)
	require.NoError(t, err)
	tr := progress.NewTracker(progress.Options{
		Sender: snd, Queue: q, Clock: clock.System(nil), Dispatch: func(func()) {}, OnWatched: watched,
	})
	t.Cleanup(tr.Close)

	return tr, snd
}
