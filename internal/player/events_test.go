package player_test

import (
	"context"
	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/player"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		audio bool
	}{
		{"video", false},
		{"audio", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			if tc.audio {
				r.playAudio(1)
			} else {
				r.playVideo(1)
			}
			r.sav.pauses = 0
			r.eng.Finish()

			requireState(t, r.p, player.StateEnded)
			assert.True(t, r.has(player.ChangeEnded))
			assert.InDelta(t, 220, r.p.Pos(), 0)
			assert.False(t, r.p.Playing())
			assert.Equal(t, 1, r.sav.pauses, "the end is saved")
			last := r.sav.updates[len(r.sav.updates)-1]
			assert.InDelta(t, 220, last.pos, 0)
			assert.False(t, last.playing)
			assert.Equal(t, "l_2", last.lesson)
			assert.Equal(t, "set-35", last.course)
			// the engine's pause at the end is not a state change
			requireState(t, r.p, player.StateEnded)
		})
	}
}

func TestAutoAdvance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		autoOn bool
		from   int
		moved  bool
		wantID string
	}{
		{"on: next lesson plays", true, 0, true, "l_2"},
		{"off: stays on the end", false, 0, false, "l_1"},
		{"on: last lesson has no next", true, 4, false, "l_5"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, rigOpts{eng: fullLength().eng, autoOff: !tc.autoOn})
			r.playVideo(tc.from)
			r.p.SetA(10)
			r.eng.Finish()
			requireState(t, r.p, player.StateEnded)
			before := len(r.loads())

			moved := r.p.AutoAdvance()
			r.settle()

			assert.Equal(t, tc.moved, moved)
			assert.Equal(t, tc.wantID, r.p.Lesson().ID)
			if !tc.moved {
				assert.Len(t, r.loads(), before)
				requireState(t, r.p, player.StateEnded)

				return
			}
			require.Len(t, r.loads(), before+1)
			assert.True(t, r.loads()[before].Load.Auto)
			assert.InDelta(t, 0, r.loads()[before].Load.Start, 0)
			requireState(t, r.p, player.StatePlaying)
			assert.InDelta(t, 10, r.p.A(), 0, "A and B stay")
		})
	}
}

func TestAutoAdvanceWithoutPreferenceIsOn(t *testing.T) {
	t.Parallel()

	clk := clock.NewFake(t0)
	eng := fake.New(media.Config{Clock: clk}, fullLength().eng)
	var q []func()
	p := player.New(player.Options{
		Engine: eng, Resolver: &resolver{},
		Dispatch: func(f func()) { q = append(q, f) },
		Go:       func(work func(context.Context)) { work(context.Background()) },
	})
	t.Cleanup(p.Close)
	l := course()
	p.PlayVideo(l[0], l)
	assert.True(t, p.AutoAdvance())
	assert.Equal(t, "l_2", p.Lesson().ID)
}

func TestStaleTokenDropped(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	old := r.eng.Token()
	r.p.Next()
	r.settle()
	cur := r.eng.Token()
	require.NotEqual(t, old, cur)
	requireState(t, r.p, player.StatePlaying)

	tests := []struct {
		name string
		ev   media.Event
	}{
		{"time", media.Event{Kind: media.EventTime, Time: 99}},
		{"duration", media.Event{Kind: media.EventDuration, Duration: 5}},
		{"state", media.Event{Kind: media.EventState, State: media.StatePaused}},
		{"ended", media.Event{Kind: media.EventEnded}},
		{"error", media.Event{Kind: media.EventError, Err: media.ErrDecode}},
	}
	for _, tc := range tests {
		tc.ev.Token = old
		r.eng.Emit(tc.ev)
		assert.InDelta(t, 0, r.p.Pos(), 0, tc.name)
		assert.InDelta(t, 220, r.p.Dur(), 0, tc.name)
		requireState(t, r.p, player.StatePlaying)
	}

	r.eng.Emit(media.Event{Kind: media.EventTime, Token: cur, Time: 5})
	assert.InDelta(t, 5, r.p.Pos(), 0)
}

func TestEventsBeforeTheLoadAreDropped(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	l := course()
	r.p.PlayVideo(l[0], l)
	r.eng.Emit(media.Event{Kind: media.EventTime, Token: 0, Time: 50})

	assert.InDelta(t, 0, r.p.Pos(), 0)
	requireState(t, r.p, player.StateLoading)
}

func TestStaleAddressDropped(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	l := course()
	r.p.PlayVideo(l[0], l)
	r.p.PlayVideo(l[1], l) // the answer for l_1 is still queued
	r.settle()

	require.Len(t, r.loads(), 1, "only the answer of the last request loads")
	assert.Equal(t, videoURL("l_2"), r.loads()[0].Src.URL)
}

func TestErrors(t *testing.T) {
	t.Parallel()

	notEnrolled := &api.Error{Status: 403, Code: "not_enrolled"}
	otherForbidden := &api.Error{Status: 403, Code: "forbidden"}
	tests := []struct {
		name string
		// engine fails the loads of these addresses
		fail map[string]error
		// resolver errors by call number
		resErr map[int]error
		// resolver addresses by call number
		urls map[int]string
		// after the first play, the engine fails playing with this error
		failWhilePlaying error
		wantCalls        int
		wantLoads        int
		wantState        player.State
		wantErr          error
		wantErrNot       error
	}{
		{
			name:      "network error while loading: one retry, then the error",
			fail:      map[string]error{videoURL("l_1"): media.ErrNetwork},
			wantCalls: 2, wantLoads: 2, wantState: player.StateError, wantErr: media.ErrNetwork,
		},
		{
			name:      "source error is retried too",
			fail:      map[string]error{videoURL("l_1"): media.ErrSource},
			wantCalls: 2, wantLoads: 2, wantState: player.StateError, wantErr: media.ErrSource,
		},
		{
			name:      "expired address: the new one plays",
			fail:      map[string]error{videoURL("l_1"): media.ErrNetwork},
			urls:      map[int]string{2: "https://x.test/video/l_1-new.m3u8"},
			wantCalls: 2, wantLoads: 2, wantState: player.StatePlaying,
		},
		{
			name:             "network error while playing: one retry, plays on",
			failWhilePlaying: media.ErrNetwork,
			wantCalls:        2, wantLoads: 2, wantState: player.StatePlaying,
		},
		{
			name:             "decode error is not retried",
			failWhilePlaying: media.ErrDecode,
			wantCalls:        1, wantLoads: 1, wantState: player.StateError, wantErr: media.ErrDecode,
		},
		{
			name:      "not enrolled",
			resErr:    map[int]error{1: notEnrolled},
			wantCalls: 1, wantLoads: 0, wantState: player.StateError, wantErr: player.ErrNotEnrolled,
		},
		{
			name:      "resolver offline",
			resErr:    map[int]error{1: api.ErrOffline},
			wantCalls: 1, wantLoads: 0, wantState: player.StateError, wantErr: api.ErrOffline, wantErrNot: player.ErrNotEnrolled,
		},
		{
			name:      "other 403 is not not_enrolled",
			resErr:    map[int]error{1: otherForbidden},
			wantCalls: 1, wantLoads: 0, wantState: player.StateError, wantErr: otherForbidden, wantErrNot: player.ErrNotEnrolled,
		},
		{
			name:      "the retry's resolver fails",
			fail:      map[string]error{videoURL("l_1"): media.ErrNetwork},
			resErr:    map[int]error{2: api.ErrOffline},
			wantCalls: 2, wantLoads: 1, wantState: player.StateError, wantErr: api.ErrOffline,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			eo := fullLength().eng
			eo.Fail = tc.fail
			res := &resolver{err: tc.resErr, urls: tc.urls}
			r := newRig(t, rigOpts{eng: eo, res: res})
			r.playVideo(0)
			if tc.failWhilePlaying != nil {
				r.p.SetRate(1)
				r.tick(8) // 2 s
				r.eng.Fail(tc.failWhilePlaying)
				r.settle()
			}

			assert.Equal(t, tc.wantCalls, res.calls())
			assert.Len(t, r.loads(), tc.wantLoads)
			requireState(t, r.p, tc.wantState)
			if tc.wantErr != nil {
				require.Error(t, r.p.Err())
				assert.ErrorIs(t, r.p.Err(), tc.wantErr)
				assert.True(t, r.has(player.ChangeError))
				var got error
				for _, c := range r.got {
					if c.Kind == player.ChangeError {
						got = c.Err
					}
				}
				assert.ErrorIs(t, got, tc.wantErr)
			} else {
				assert.NoError(t, r.p.Err())
			}
			if tc.wantErrNot != nil {
				assert.NotErrorIs(t, r.p.Err(), tc.wantErrNot)
			}
		})
	}
}

func TestRetryResumesAtThePosition(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.SetRate(1)
	r.tick(8)
	r.eng.Fail(media.ErrNetwork)
	requireState(t, r.p, player.StateLoading)
	r.settle()

	require.Len(t, r.loads(), 2)
	assert.InDelta(t, 2, r.loads()[1].Load.Start, 0)
	assert.True(t, r.loads()[1].Load.Auto, "it played, so it plays again")
	assert.Equal(t, []string{"l_1", "l_1"}, r.res.videos)
	requireState(t, r.p, player.StatePlaying)
}

func TestRetryOfAPausedMediaStaysPaused(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.Pause()
	r.eng.Fail(media.ErrNetwork)
	r.settle()

	require.Len(t, r.loads(), 2)
	assert.False(t, r.loads()[1].Load.Auto)
	requireState(t, r.p, player.StatePaused)
}

func TestRetryIsOncePerFailureNotOncePerLesson(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.SetRate(1)
	for i := range 3 { // an address that expires again and again while the lesson plays
		r.tick(4)
		r.eng.Fail(media.ErrNetwork)
		r.settle()
		requireState(t, r.p, player.StatePlaying)
		assert.Equal(t, i+2, r.res.calls())
	}
}

func TestDuplicateErrorIsIgnored(t *testing.T) {
	t.Parallel()

	eo := fullLength().eng
	eo.Fail = map[string]error{videoURL("l_1"): media.ErrDecode}
	r := newRig(t, rigOpts{eng: eo})
	r.playVideo(0)
	requireState(t, r.p, player.StateError)
	count := func() int {
		n := 0
		for _, c := range r.got {
			if c.Kind == player.ChangeError {
				n++
			}
		}

		return n
	}
	require.Equal(t, 1, count())
	r.eng.Emit(media.Event{Kind: media.EventError, Token: r.eng.Token(), Err: media.ErrDecode})
	assert.Equal(t, 1, count())
}

func TestPlayAfterAnErrorLoadsAgain(t *testing.T) {
	t.Parallel()

	res := &resolver{err: map[int]error{1: api.ErrOffline}}
	r := newRig(t, rigOpts{eng: fullLength().eng, res: res})
	r.playVideo(0)
	requireState(t, r.p, player.StateError)

	r.p.Toggle()
	requireState(t, r.p, player.StateLoading)
	r.settle()

	requireState(t, r.p, player.StatePlaying)
	assert.NoError(t, r.p.Err())
	assert.Equal(t, 2, res.calls())
}

func TestSessionEvents(t *testing.T) {
	t.Parallel()

	begin := media.SessionEvent{Kind: media.InterruptBegin}
	endYes := media.SessionEvent{Kind: media.InterruptEnd, ShouldResume: true}
	endNo := media.SessionEvent{Kind: media.InterruptEnd}
	tests := []struct {
		name   string
		paused bool // the user paused before the events
		events []media.SessionEvent
		user   func(p *player.Player) // the user acts between the events
		want   player.State
	}{
		{"begin pauses", false, []media.SessionEvent{begin}, nil, player.StatePaused},
		{"begin then end with resume plays", false, []media.SessionEvent{begin, endYes}, nil, player.StatePlaying},
		{"begin then end without resume stays paused", false, []media.SessionEvent{begin, endNo}, nil, player.StatePaused},
		{"end with resume of a user pause stays paused", true, []media.SessionEvent{begin, endYes}, nil, player.StatePaused},
		{"user pause during the interruption cancels the resume", false, []media.SessionEvent{begin, endYes}, func(p *player.Player) { p.Pause() }, player.StatePaused},
		{"end alone does nothing", false, []media.SessionEvent{endYes}, nil, player.StatePlaying},
		{"noisy pauses", false, []media.SessionEvent{{Kind: media.Noisy}}, nil, player.StatePaused},
		{"noisy then end with resume stays paused", false, []media.SessionEvent{{Kind: media.Noisy}, endYes}, nil, player.StatePaused},
		{"duck changes nothing", false, []media.SessionEvent{{Kind: media.Duck}, {Kind: media.Unduck}}, nil, player.StatePlaying},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			r.playVideo(0)
			assert.GreaterOrEqual(t, r.ses.Activations, 1)
			if tc.paused {
				r.p.Pause()
			}
			for i, ev := range tc.events {
				r.ses.Emit(ev)
				if tc.user != nil && i == 0 {
					tc.user(r.p)
				}
			}

			requireState(t, r.p, tc.want)
			if tc.want == player.StatePlaying {
				assert.True(t, r.p.Playing())
			}
		})
	}
}

func TestMediaResetReloadsAtThePosition(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.SetRate(1)
	r.tick(8)
	calls := r.res.calls()
	r.ses.Emit(media.SessionEvent{Kind: media.MediaReset})
	r.settle()

	require.Len(t, r.loads(), 2)
	assert.InDelta(t, 2, r.loads()[1].Load.Start, 0)
	assert.False(t, r.loads()[1].Load.Auto)
	assert.Equal(t, videoURL("l_1"), r.loads()[1].Src.URL)
	assert.Equal(t, calls, r.res.calls(), "the address is kept")
	requireState(t, r.p, player.StatePaused)
}

func TestCloseStopsListening(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.Close()
	pauses := r.count(fake.OpPause)
	r.ses.Emit(media.SessionEvent{Kind: media.Noisy})
	assert.Equal(t, pauses, r.count(fake.OpPause))
	assert.Zero(t, r.ses.Events().Len())
	assert.Zero(t, r.eng.Events().Len())
}

func TestDefaultGoResolvesAwayFromTheUIThread(t *testing.T) {
	t.Parallel()

	clk := clock.NewFake(t0)
	eng := fake.New(media.Config{Clock: clk}, fullLength().eng)
	answers := make(chan func(), 4)
	entered := make(chan string, 4)
	cancelled := make(chan string, 4)
	res := &blockingResolver{entered: entered, cancelled: cancelled, block: map[string]bool{"l_1": true}}
	p := player.New(player.Options{
		Engine: eng, Resolver: res,
		Dispatch: func(f func()) { answers <- f },
	})
	l := course()

	// the first lesson's resolver blocks; asking for the second cancels it
	p.PlayVideo(l[0], l)
	require.Equal(t, "l_1", <-entered)
	p.PlayVideo(l[1], l)
	require.Equal(t, "l_1", <-cancelled)
	(<-answers)() // l_1's answer, stale
	(<-answers)() // l_2's
	clk.Advance(0)
	assert.Equal(t, "l_2", p.Lesson().ID)
	requireState(t, p, player.StatePlaying)
	require.NotEmpty(t, eng.Calls())
	assert.Equal(t, fake.OpLoad, eng.Calls()[0].Op)
	assert.Equal(t, videoURL("l_2"), eng.Calls()[0].Src.URL)

	// Close cancels a resolver under way and waits for it
	p.PlayVideo(l[0], l)
	require.Equal(t, "l_1", <-entered)
	p.Close()
	assert.Equal(t, "l_1", <-cancelled)
}

// blockingResolver blocks the lessons of block until its context ends.
type blockingResolver struct {
	entered, cancelled chan string
	block              map[string]bool
}

func (b *blockingResolver) VideoURL(ctx context.Context, id string) (string, error) {
	if b.block[id] {
		b.entered <- id
		<-ctx.Done()
		b.cancelled <- id

		return "", errors.Join(ctx.Err())
	}

	return videoURL(id), nil
}

func (b *blockingResolver) AudioURL(ctx context.Context, id string) (string, error) {
	return b.VideoURL(ctx, id)
}

func TestEndedKeepsPosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		act  func(r *rig)
	}{
		{"leave", func(r *rig) { r.p.Leave() }},
		{"background", func(r *rig) { r.p.Background(true) }},
		{"pause", func(r *rig) { r.p.Pause() }},
		{"sync", func(r *rig) { r.p.SyncPos() }},
		{"close", func(r *rig) { r.p.Close() }},
		{"leave then background", func(r *rig) { r.p.Leave(); r.p.Background(true) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			r.playVideo(1)
			r.eng.Finish()
			requireState(t, r.p, player.StateEnded)
			// the engine reads a little short of the length, and ticks late
			require.NoError(t, r.eng.Seek(219.7))
			r.tick(2)
			tc.act(r)

			require.NotEmpty(t, r.sav.updates)
			assert.InDelta(t, 220, r.p.Pos(), 0)
			var seenEnd bool
			for _, u := range r.sav.updates {
				if u.pos >= 220 {
					seenEnd = true
				} else if seenEnd {
					t.Fatalf("a save of %v follows the end: %+v", u.pos, r.sav.updates)
				}
			}
			assert.True(t, seenEnd)
			assert.InDelta(t, 220, r.sav.updates[len(r.sav.updates)-1].pos, 0)
		})
	}
}

func TestEndedWithLoop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		audio     bool
		loop      bool
		b         float64 // 0 keeps the B the loop begins with (A+30 within 12 s = 12)
		wantEnded bool
	}{
		{"video loop on, B at the end", false, true, 0, false},
		{"video loop on, B within a tick of the end", false, true, 11.8, false},
		{"video loop off", false, false, 0, true},
		{"audio loop on loops too (K38)", true, true, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, rigOpts{eng: fake.Options{VideoDuration: 12, AudioDuration: 12}})
			if tc.audio {
				r.playAudio(1)
			} else {
				r.playVideo(1)
			}
			r.tick(1)
			if tc.loop {
				r.p.SetLoop(true)
			}
			if tc.b > 0 {
				r.p.SetB(tc.b)
			}
			r.sav.pauses = 0
			r.got = nil
			r.eng.Finish()
			r.settle()

			assert.Equal(t, tc.wantEnded, r.has(player.ChangeEnded))
			if tc.wantEnded {
				requireState(t, r.p, player.StateEnded)
				assert.InDelta(t, 12, r.p.Pos(), 0)

				return
			}
			assert.InDelta(t, r.p.A(), r.p.Pos(), 0.3)
			assert.True(t, r.p.Playing())
			assert.Equal(t, 0, r.sav.pauses, "a wrap is no end")
			for _, u := range r.sav.updates {
				assert.Less(t, u.pos, 11.0, "the wrap does not save the end")
			}
			r.tick(4)
			assert.Greater(t, r.p.Pos(), r.p.A())
			assert.False(t, r.has(player.ChangeEnded))
		})
	}
}

func TestLoopBelowTheEndStillWrapsOnce(t *testing.T) {
	t.Parallel()

	r := newRig(t, rigOpts{eng: fake.Options{VideoDuration: 12}})
	r.playVideo(1)
	r.p.SetLoop(true)
	r.p.SetAB(2, 6)
	r.tick(40) // 10 s: past B twice, never at the end
	assert.False(t, r.has(player.ChangeEnded))
	assert.True(t, r.p.Playing())
	assert.Less(t, r.p.Pos(), 6.5)
}
