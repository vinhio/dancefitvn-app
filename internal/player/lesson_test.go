package player_test

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/player"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fullLength() rigOpts {
	return rigOpts{eng: fake.Options{VideoDuration: 220, AudioDuration: 220}}
}

func TestStartPosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		resume  int
		watched bool
		kind    player.Kind
		want    float64
	}{
		{"no progress", 0, false, player.KindVideo, 0},
		{"72 of 220 resumes", 72, false, player.KindVideo, 72},
		{"197 is under 90 percent", 197, false, player.KindVideo, 197},
		{"198 is 90 percent", 198, false, player.KindVideo, 0},
		{"200 of 220 starts over", 200, false, player.KindVideo, 0},
		{"watched starts over", 72, true, player.KindVideo, 0},
		{"audio starts at 0", 72, false, player.KindAudio, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			l := course()
			l[0].ResumeSec, l[0].Watched = tc.resume, tc.watched
			if tc.kind == player.KindVideo {
				r.p.PlayVideo(l[0], l)
			} else {
				r.p.PlayAudio(l[0], l)
			}
			r.settle()

			require.Len(t, r.loads(), 1)
			assert.InDelta(t, tc.want, r.loads()[0].Load.Start, 0)
			assert.True(t, r.loads()[0].Load.Auto)
			assert.InDelta(t, tc.want, r.p.Pos(), 0)
			assert.Equal(t, tc.kind == player.KindAudio, r.loads()[0].Src.AudioOnly)
			assert.Equal(t, 0, r.count(fake.OpSeek), "a resume that fits the length is not seeked")
			requireState(t, r.p, player.StatePlaying)
		})
	}
}

func TestPlayVideoSetsAB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		dur      int
		wantA    float64
		wantB    float64
		wantKind player.Kind
	}{
		{"220 s", 220, 73, 101, player.KindVideo},
		{"100 s", 100, 33, 61, player.KindVideo},
		{"half rounds up (150 s)", 150, 50, 78, player.KindVideo}, // 49.5 -> 50 as Math.round
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, rigOpts{eng: fake.Options{VideoDuration: float64(tc.dur)}})
			l := course()
			l[0].DurationSec = tc.dur
			r.p.PlayVideo(l[0], l)
			r.settle()

			assert.InDelta(t, tc.wantA, r.p.A(), 0)
			assert.InDelta(t, tc.wantB, r.p.B(), 0)
			assert.Equal(t, tc.wantKind, r.p.Kind())
		})
	}
}

func TestSameLessonKeepsPositionAndAB(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.SetRate(1)
	r.p.SetA(10)
	r.p.SetB(20)
	r.tick(8) // 2 s
	require.InDelta(t, 2, r.p.Pos(), 0)

	l := course()
	r.p.PlayVideo(l[0], l)
	r.settle()

	assert.Len(t, r.loads(), 1, "the same lesson is not loaded again")
	assert.InDelta(t, 2, r.p.Pos(), 0)
	assert.InDelta(t, 10, r.p.A(), 0)
	assert.InDelta(t, 20, r.p.B(), 0)
	requireState(t, r.p, player.StatePlaying)

	// paused: the same call resumes
	r.p.Pause()
	requireState(t, r.p, player.StatePaused)
	r.p.PlayVideo(l[0], l)
	requireState(t, r.p, player.StatePlaying)
	assert.Len(t, r.loads(), 1)

	// another lesson: A and B are made again
	r.p.PlayVideo(l[2], l)
	r.settle()
	assert.Len(t, r.loads(), 2)
	assert.InDelta(t, 73, r.p.A(), 0)
	assert.InDelta(t, 101, r.p.B(), 0)
	assert.InDelta(t, 0, r.p.Pos(), 0)
}

func TestSameLessonOtherKindKeepsPosition(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.SetRate(1)
	r.tick(40) // 10 s
	l := course()
	r.p.PlayAudio(l[0], l)
	r.settle()

	require.Len(t, r.loads(), 2)
	assert.True(t, r.loads()[1].Src.AudioOnly)
	assert.InDelta(t, 10, r.loads()[1].Load.Start, 0)
	assert.Equal(t, player.KindAudio, r.p.Kind())
	assert.Equal(t, []string{"l_1"}, r.res.videos)
	assert.Equal(t, []string{"l_1"}, r.res.audios)
}

func TestInitialAndPersistentOptions(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	assert.InDelta(t, 0.75, r.p.Rate(), 0)
	assert.True(t, r.p.Mirror())
	assert.True(t, r.p.Beat())
	assert.False(t, r.p.Loop())
	requireState(t, r.p, player.StateIdle)

	r.playVideo(0)
	assert.InDelta(t, 0.75, r.eng.Rate(), 0, "the engine gets the speed after the load")

	r.p.SetRate(1.25)
	r.p.SetMirror(false)
	r.p.SetBeat(false)
	r.p.SetLoop(true)
	l := course()
	r.p.PlayVideo(l[3], l)
	r.settle()

	assert.InDelta(t, 1.25, r.p.Rate(), 0)
	assert.InDelta(t, 1.25, r.eng.Rate(), 0)
	assert.False(t, r.p.Mirror())
	assert.False(t, r.p.Beat())
	assert.True(t, r.p.Loop())
	assert.Equal(t, "l_4", r.p.Lesson().ID)
}

func TestPrevNext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		from    int
		next    bool
		playing bool
		moved   bool
		wantID  string
	}{
		{"next playing", 1, true, true, true, "l_3"},
		{"next paused stays paused", 1, true, false, true, "l_3"},
		{"prev playing", 2, false, true, true, "l_2"},
		{"prev paused stays paused", 2, false, false, true, "l_2"},
		{"prev at the first lesson", 0, false, true, false, "l_1"},
		{"next at the last lesson", 4, true, true, false, "l_5"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			r.playVideo(tc.from)
			r.p.SetRate(1)
			r.tick(20)
			if !tc.playing {
				r.p.Pause()
			}
			r.p.SetA(10)
			r.p.SetB(20)
			r.p.SetLoop(true)
			before := len(r.loads())
			if tc.next {
				assert.Equal(t, tc.moved, r.p.HasNext())
				r.p.Next()
			} else {
				assert.Equal(t, tc.moved, r.p.HasPrev())
				r.p.Prev()
			}
			r.settle()

			assert.Equal(t, tc.wantID, r.p.Lesson().ID)
			assert.InDelta(t, 10, r.p.A(), 0, "A stays (Q-P4-4)")
			assert.InDelta(t, 20, r.p.B(), 0)
			assert.True(t, r.p.Loop())
			if !tc.moved {
				assert.Len(t, r.loads(), before)

				return
			}
			require.Len(t, r.loads(), before+1)
			last := r.loads()[before]
			assert.InDelta(t, 0, last.Load.Start, 0)
			assert.Equal(t, tc.playing, last.Load.Auto)
			assert.False(t, last.Src.AudioOnly)
			if tc.playing {
				requireState(t, r.p, player.StatePlaying)
			} else {
				requireState(t, r.p, player.StatePaused)
			}
			assert.InDelta(t, 0, r.p.Pos(), 0)
		})
	}
}

func TestPrevNextOfAudioLoadsAudio(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playAudio(1)
	r.p.Next()
	r.settle()

	require.Len(t, r.loads(), 2)
	assert.True(t, r.loads()[1].Src.AudioOnly)
	assert.Equal(t, audioURL("l_3"), r.loads()[1].Src.URL)
	assert.Equal(t, player.KindAudio, r.p.Kind())
}

func TestAudioKeepsABOfEarlierLesson(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.SetA(10)
	r.p.SetB(20)
	l := course()
	r.p.PlayAudio(l[1], l)
	r.settle()

	assert.InDelta(t, 10, r.p.A(), 0)
	assert.InDelta(t, 20, r.p.B(), 0)
}

func TestAudioFirstGetsDefaultAB(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playAudio(0)

	assert.InDelta(t, 73, r.p.A(), 0)
	assert.InDelta(t, 101, r.p.B(), 0)
}

func TestEngineDurationReplacesAPIDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		engineDur float64
		resume    int
		wantDur   float64
		wantSeeks int
		wantPos   float64
		wantA     float64
		wantB     float64
	}{
		{"mock: 12 s against 220, resume 72", 12, 72, 12, 1, 0, 4, 12},
		{"engine agrees", 220, 72, 220, 0, 72, 73, 101},
		{"engine 70 s, resume 72 beyond", 70, 72, 70, 1, 0, 23, 51},
		{"engine 100 s, resume 72 fits", 100, 72, 100, 0, 72, 33, 61},
		{"engine a bit shorter, resume fits", 210, 72, 210, 0, 72, 73, 101},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, rigOpts{eng: fake.Options{VideoDuration: tc.engineDur}})
			l := course()
			l[0].ResumeSec = tc.resume
			r.p.PlayVideo(l[0], l)
			assert.InDelta(t, 220, r.p.Dur(), 0, "the API's length until the engine tells its own")
			r.settle()

			assert.InDelta(t, tc.wantDur, r.p.Dur(), 0)
			assert.Equal(t, tc.wantSeeks, r.count(fake.OpSeek))
			assert.InDelta(t, tc.wantPos, r.p.Pos(), 0)
			assert.InDelta(t, tc.wantA, r.p.A(), 0)
			assert.InDelta(t, tc.wantB, r.p.B(), 0)

			// the same length told again does not seek again
			r.eng.SetDuration(tc.engineDur)
			assert.Equal(t, tc.wantSeeks, r.count(fake.OpSeek))
		})
	}
}

func TestDurationOfEngineIgnoresNonsense(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.eng.SetDuration(0)
	assert.InDelta(t, 220, r.p.Dur(), 0)
	r.eng.Emit(media.Event{Kind: media.EventDuration, Token: r.eng.Token(), Duration: -3})
	assert.InDelta(t, 220, r.p.Dur(), 0)
}

func TestResumePastEngineLength(t *testing.T) {
	t.Parallel()

	const (
		ended = iota
		duration
	)
	tests := []struct {
		name      string
		order     []int
		auto      bool
		wantEnded bool
		wantPlay  int
	}{
		{"end before duration plays from 0", []int{ended, duration}, true, false, 1},
		{"end before duration stays paused", []int{ended, duration}, false, false, 0},
		{"end alone starts over", []int{ended}, true, false, 1},
		{"duration before end seeks at duration", []int{duration}, true, false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, rigOpts{eng: fake.Options{VideoDuration: 12, LoadDelay: 3600e9}})
			l := course()
			l[0].ResumeSec = 72
			l[0].DurationSec = 220
			if !tc.auto {
				r.p.PlayVideo(l[0], l)
				r.p.Pause()
			} else {
				r.p.PlayVideo(l[0], l)
			}
			r.settle()
			require.Len(t, r.loads(), 1)
			tok := r.eng.Token()
			r.eng.ClearCalls()

			for _, o := range tc.order {
				if o == ended {
					r.eng.Emit(media.Event{Kind: media.EventEnded, Token: tok})
				} else {
					r.eng.Emit(media.Event{Kind: media.EventDuration, Duration: 12, Token: tok})
				}
			}

			assert.Equal(t, tc.wantEnded, r.p.State() == player.StateEnded)
			assert.Equal(t, tc.wantEnded, r.has(player.ChangeEnded))
			if !tc.wantEnded {
				assert.InDelta(t, 0, r.p.Pos(), 0)
				assert.Equal(t, 1, r.count(fake.OpSeek))
			}
			assert.Equal(t, tc.wantPlay, r.count(fake.OpPlay))
		})
	}
}

// TestResumePastEngineLengthInEngineOrder runs the fake in avf's order: the
// length is told once ready, before the load goes to its start (72 s, past
// a media of 12). The player's seek to 0 in answer to the length wins, so
// the lesson plays from 0 and stays on 06; only a real end, 12 s later,
// ends it (P4.9 D2: the start applied after the seek ended it at once).
func TestResumePastEngineLengthInEngineOrder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		auto      bool
		wantState player.State
	}{
		{"playing", true, player.StatePlaying},
		{"paused", false, player.StatePaused},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, rigOpts{eng: fake.Options{VideoDuration: 12, StartAfterDuration: true}, autoOff: true})
			l := course()
			l[0].ResumeSec = 72
			l[0].DurationSec = 220
			r.p.PlayVideo(l[0], l)
			if !tc.auto {
				r.p.Pause()
			}
			r.settle()
			require.Len(t, r.loads(), 1)
			require.InDelta(t, 72, r.loads()[0].Load.Start, 0, "the load asks for the resume position")
			r.tick(1)

			assert.InDelta(t, 12, r.p.Dur(), 0)
			assert.Equal(t, tc.wantState, r.p.State())
			assert.False(t, r.has(player.ChangeEnded), "06 stays")
			assert.Less(t, r.eng.Position(), 1.0, "the media plays from its start")
			assert.Less(t, r.p.Pos(), 1.0)
			if !tc.auto {
				return
			}

			// a real end: the media played through, at the lesson's speed
			r.tick(int(math.Ceil(12/r.eng.Rate()/fake.DefaultTick.Seconds())) + 1)
			assert.Equal(t, player.StateEnded, r.p.State())
			assert.True(t, r.has(player.ChangeEnded))
		})
	}
}
