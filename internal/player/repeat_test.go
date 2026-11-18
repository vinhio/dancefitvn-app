package player_test

import (
	"testing"

	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/player"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCycleRepeatGoesOffOneABOff checks K38: the audio button's order, the A-B loop
// beginning at the position, Repeat One having no band, and a change told each time.
func TestCycleRepeatGoesOffOneABOff(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playAudio(0)
	r.p.Seek(72)
	require.Equal(t, player.RepeatOff, r.p.Repeat())
	r.got = nil

	r.p.CycleRepeat()
	assert.Equal(t, player.RepeatOne, r.p.Repeat())
	assert.False(t, r.p.Loop(), "Repeat One is not the A-B loop")
	assert.True(t, r.has(player.ChangeOptions))
	r.p.CycleRepeat()
	assert.Equal(t, player.RepeatAB, r.p.Repeat())
	assert.True(t, r.p.Loop())
	assert.InDelta(t, 72, r.p.A(), 0)
	assert.InDelta(t, 102, r.p.B(), 0)
	r.p.CycleRepeat()
	assert.Equal(t, player.RepeatOff, r.p.Repeat())
	assert.False(t, r.p.Loop())
	r.p.CycleRepeat()
	assert.Equal(t, player.RepeatOne, r.p.Repeat(), "and round again")

	// one at a time, however they are set
	r.p.SetLoop(true)
	assert.Equal(t, player.RepeatAB, r.p.Repeat())
	r.p.SetRepeat(player.RepeatOne)
	assert.False(t, r.p.Loop())
	assert.Equal(t, player.RepeatOne, r.p.Repeat())
	r.p.SetLoop(false) // the options sheet's switch is for the A-B loop alone
	assert.Equal(t, player.RepeatOne, r.p.Repeat(), "turning the A-B loop off leaves Repeat One")
	r.p.SetRepeat(player.RepeatUnknown)
	assert.Equal(t, player.RepeatOff, r.p.Repeat(), "the zero reads as off")
	r.got = nil
	r.p.SetRepeat(player.RepeatOff)
	assert.False(t, r.has(player.ChangeOptions), "off to off changes nothing")
}

// TestRepeatOneStartsAgainAtTheEnd checks that a lesson that repeats begins again at 0 when
// it ends, playing on, with no end of lesson, no state Ended and no next lesson, for a
// sound and for a video.
func TestRepeatOneStartsAgainAtTheEnd(t *testing.T) {
	t.Parallel()

	for _, audio := range []bool{true, false} {
		r := newRig(t, rigOpts{eng: fake.Options{VideoDuration: 12, AudioDuration: 12}})
		if audio {
			r.playAudio(1)
		} else {
			r.playVideo(1)
		}
		r.p.SetRate(1)
		r.tick(2)
		r.p.SetRepeat(player.RepeatOne)
		r.got = nil
		r.sav.updates, r.sav.pauses = nil, 0

		r.eng.Finish()
		r.settle()

		assert.False(t, r.has(player.ChangeEnded), "audio=%v: no end of lesson", audio)
		assert.True(t, r.p.Playing(), "audio=%v", audio)
		requireState(t, r.p, player.StatePlaying)
		assert.InDelta(t, 0, r.p.Pos(), .3, "audio=%v: from the start", audio)
		require.NotEmpty(t, r.sav.updates)
		assert.InDelta(t, 12, r.sav.updates[0].pos, 0, "the end was saved first: it counts")
		assert.GreaterOrEqual(t, r.sav.pauses, 1, "and saved at once")
		r.tick(8) // 2 s on
		assert.Greater(t, r.p.Pos(), 1.0)
		assert.False(t, r.has(player.ChangeEnded))

		// and again at the next end
		r.eng.Finish()
		r.settle()
		assert.False(t, r.has(player.ChangeEnded), "audio=%v: a second round", audio)
		assert.True(t, r.p.Playing())
	}
}

// TestRepeatOneCountsWatchedOnce runs the real Tracker under it: the first end marks the
// lesson watched, the second does not mark it again.
func TestRepeatOneCountsWatchedOnce(t *testing.T) {
	t.Parallel()

	var won []string
	tr, _ := newTracker(t, func(id string) { won = append(won, id) })
	r := newRig(t, rigOpts{eng: fake.Options{AudioDuration: 12, VideoDuration: 12}, saver: tr})
	r.playAudio(0)
	r.p.SetRate(1)
	r.p.SetRepeat(player.RepeatOne)
	r.eng.Finish()
	r.settle()
	assert.Equal(t, []string{"l_1"}, won, "the first completion counts")
	r.tick(8)
	r.eng.Finish()
	r.settle()
	r.tick(8)
	r.eng.Finish()
	r.settle()
	assert.Equal(t, []string{"l_1"}, won, "once per visit")
	assert.True(t, r.p.Playing())
}

// TestRepeatOffEndsAsBefore checks that with no repeat the end is the end: the state and
// the change that the shell turns into the end screen and the next lesson.
func TestRepeatOffEndsAsBefore(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOpts{eng: fake.Options{VideoDuration: 12, AudioDuration: 12}})
	r.playAudio(1)
	r.p.SetRepeat(player.RepeatOne)
	r.p.SetRepeat(player.RepeatOff)
	r.eng.Finish()
	r.settle()
	assert.True(t, r.has(player.ChangeEnded))
	requireState(t, r.p, player.StateEnded)
}

// TestReloadLoadsAgainFromWhereItIs checks the call the shell makes when the file a sound
// plays from is deleted: the same lesson is asked for again and loaded at the position, playing
// as it was; nothing happens without a lesson.
func TestReloadLoadsAgainFromWhereItIs(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.p.Reload() // no lesson: nothing
	assert.Empty(t, r.loads())
	r.playAudio(0)
	r.p.SetRate(1)
	r.p.Seek(100)
	loads := len(r.loads())
	require.Equal(t, player.StatePlaying, r.p.State())

	r.p.Reload()
	r.settle()
	assert.Len(t, r.loads(), loads+1, "loaded again")
	last := r.loads()[len(r.loads())-1]
	assert.InDelta(t, 100, last.Load.Start, 1, "from where it was")
	assert.Equal(t, 2, r.res.calls(), "the address was asked for again")
	assert.True(t, r.p.Playing(), "playing as it was")

	r.p.Pause()
	r.p.Reload()
	r.settle()
	assert.False(t, r.p.Playing(), "paused as it was")
}
