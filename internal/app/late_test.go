//go:build !tvos

// Adapted from jplayer-mobile/internal/app/late_test.go at 49de3c2
// (TestOpenMakesMediaLate is TestWireMakesMediaLate here,
// player_mobile_test.go; TestLatePlayerPlaysThrough plays a lesson).

package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/player"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// urlResolver answers every lesson with an address the fake engine takes.
type urlResolver struct{}

// VideoURL is an address of the lesson.
func (urlResolver) VideoURL(_ context.Context, id string) (string, error) {
	return "https://example.invalid/" + id + ".m3u8", nil
}

// AudioURL is an address of the lesson.
func (urlResolver) AudioURL(_ context.Context, id string) (string, error) {
	return "https://example.invalid/" + id + ".mp3", nil
}

// lateRig is a lateMedia over the fake engine and session on a fake
// clock, with what the engine published to the player.
type lateRig struct {
	m     *lateMedia
	clk   *clock.Fake
	eng   *fake.Engine
	sess  *fake.Session
	made  int
	got   []media.Event
	unsub func()
}

// newLateRig returns a rig whose media are not made yet.
func newLateRig(t *testing.T) *lateRig {
	t.Helper()
	r := &lateRig{clk: clock.NewFake(t0)}
	newMedia := func(cfg media.Config) (platformMedia, error) {
		r.made++
		r.eng = fake.New(cfg, fake.Options{})
		r.sess = &fake.Session{}

		return platformMedia{name: "fake", eng: r.eng, sess: r.sess}, nil
	}
	r.m = newLateMedia(media.Config{Dispatch: func(fn func()) { fn() }, Clock: r.clk}, newMedia, t.Logf)
	r.unsub = r.m.Events().Subscribe(func(ev media.Event) { r.got = append(r.got, ev) })
	t.Cleanup(func() {
		r.unsub()
		require.NoError(t, r.m.Close())
	})

	return r
}

// tokens returns the tokens of the events of kind k the player got.
func (r *lateRig) tokens(k media.EventKind) []uint64 {
	var out []uint64
	for _, ev := range r.got {
		if ev.Kind == k {
			out = append(out, ev.Token)
		}
	}

	return out
}

// TestLateMediaReplaysLoad loads twice and sets the volume, the mute and
// the speed before the media are made: only the last load reaches the
// engine, with the settings after it, and the engine's events reach the
// player under the token the last Load returned; a load after make goes
// straight through, its token following on.
func TestLateMediaReplaysLoad(t *testing.T) {
	t.Parallel()

	r := newLateRig(t)
	a, err := r.m.Load(context.Background(), media.Source{URL: "https://x.test/a.mp3"}, media.LoadOptions{})
	require.NoError(t, err)
	b, err := r.m.Load(context.Background(), media.Source{URL: "https://x.test/b.mp3"}, media.LoadOptions{Start: 3})
	require.NoError(t, err)
	require.NoError(t, r.m.SetRate(1.5))
	require.NoError(t, r.m.SetVolume(0.5))
	require.NoError(t, r.m.SetMuted(true))
	r.m.SetBackground(true)
	assert.Zero(t, r.made, "made before make")
	assert.Greater(t, b, a)

	r.m.make()
	r.m.make()
	require.Equal(t, 1, r.made)
	calls := r.eng.Calls()
	require.Len(t, calls, 5)
	assert.Equal(t, fake.OpLoad, calls[0].Op)
	assert.Equal(t, "https://x.test/b.mp3", calls[0].Src.URL)
	assert.InDelta(t, 3, calls[0].Load.Start, 0)
	assert.Equal(t, []fake.Op{fake.OpRate, fake.OpVolume, fake.OpMuted, fake.OpSetBackground},
		[]fake.Op{calls[1].Op, calls[2].Op, calls[3].Op, calls[4].Op})
	r.clk.Advance(time.Second)
	require.NotEmpty(t, r.got)
	for _, ev := range r.got {
		assert.Equal(t, b, ev.Token, "event %d", ev.Kind)
	}

	c, err := r.m.Load(context.Background(), media.Source{URL: "https://x.test/c.mp3"}, media.LoadOptions{})
	require.NoError(t, err)
	assert.Equal(t, b+1, c)
	r.got = nil
	r.clk.Advance(time.Second)
	assert.Equal(t, []uint64{c}, r.tokens(media.EventDuration))
}

// TestLateMediaReadsTime checks the stand-in reads the engine's position
// with the player's tokens (media.TimeReader, F1 of M9): nothing before
// make, the load replayed and later ones after, nothing of a stale token.
func TestLateMediaReadsTime(t *testing.T) {
	t.Parallel()

	r := newLateRig(t)
	var _ media.TimeReader = r.m
	a, err := r.m.Load(context.Background(), media.Source{URL: "https://x.test/a.mp3"}, media.LoadOptions{})
	require.NoError(t, err)
	b, err := r.m.Load(context.Background(), media.Source{URL: "https://x.test/b.mp3"}, media.LoadOptions{Auto: true, Start: 3})
	require.NoError(t, err)
	_, ok := r.m.ReadTime(b)
	assert.False(t, ok, "not made")

	r.m.make()
	r.clk.Advance(0)
	got, ok := r.m.ReadTime(b)
	require.True(t, ok)
	assert.InDelta(t, 3, got, 1e-9)
	r.m.SetBackground(true)
	r.clk.Advance(5 * time.Second)
	got, ok = r.m.ReadTime(b)
	require.True(t, ok)
	assert.InDelta(t, 8, got, 1e-9, "played on in the background")
	_, ok = r.m.ReadTime(a)
	assert.False(t, ok, "the load make dropped")

	c, err := r.m.Load(context.Background(), media.Source{URL: "https://x.test/c.mp3"}, media.LoadOptions{})
	require.NoError(t, err)
	r.clk.Advance(0)
	_, ok = r.m.ReadTime(b)
	assert.False(t, ok, "the last load's")
	got, ok = r.m.ReadTime(c)
	require.True(t, ok)
	assert.Zero(t, got)
}

// TestLateMediaStopBeforeMake stops before the media are made: nothing is
// loaded when they are, and a load after has a token of its own.
func TestLateMediaStopBeforeMake(t *testing.T) {
	t.Parallel()

	r := newLateRig(t)
	a, err := r.m.Load(context.Background(), media.Source{URL: "https://x.test/a.mp3"}, media.LoadOptions{})
	require.NoError(t, err)
	require.NoError(t, r.m.Stop())
	r.m.make()
	assert.Empty(t, r.eng.Calls())
	b, err := r.m.Load(context.Background(), media.Source{URL: "https://x.test/b.mp3"}, media.LoadOptions{})
	require.NoError(t, err)
	assert.Greater(t, b, a+1)
	r.clk.Advance(time.Second)
	assert.Equal(t, []uint64{b}, r.tokens(media.EventDuration))
}

// TestLateMediaMissingFile loads a file that is not there before the media
// are made: media.ErrMissingFile at once, as the engines tell it, and
// nothing waits for make.
func TestLateMediaMissingFile(t *testing.T) {
	t.Parallel()

	r := newLateRig(t)
	_, err := r.m.Load(context.Background(), media.Source{URL: filepath.Join(t.TempDir(), "gone.mp3")}, media.LoadOptions{})
	require.ErrorIs(t, err, media.ErrMissingFile)
	assert.Zero(t, r.made)
	r.m.make()
	assert.Empty(t, r.eng.Calls())
}

// TestLateMediaMadeOnDemand checks what cannot wait makes the media at
// once: Play (after the waiting Load), Pause, Seek, the Surface, the
// session's Activate.
func TestLateMediaMadeOnDemand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		do   func(r *lateRig) error
		op   fake.Op
	}{
		{"play", func(r *lateRig) error { return r.m.Play() }, fake.OpPlay},
		{"pause", func(r *lateRig) error { return r.m.Pause() }, fake.OpPause},
		{"seek", func(r *lateRig) error { return r.m.Seek(4) }, fake.OpSeek},
		{"video", func(r *lateRig) error {
			if r.m.Video() == nil {
				return assert.AnError
			}

			return nil
		}, ""},
		{"activate", func(r *lateRig) error { return lateSession{m: r.m}.Activate() }, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newLateRig(t)
			_, err := r.m.Load(context.Background(), media.Source{URL: "https://x.test/a.mp3"}, media.LoadOptions{})
			require.NoError(t, err)
			require.NoError(t, tc.do(r))
			require.Equal(t, 1, r.made)
			calls := r.eng.Calls()
			require.NotEmpty(t, calls)
			assert.Equal(t, fake.OpLoad, calls[0].Op)
			if tc.op != "" {
				assert.Equal(t, tc.op, calls[len(calls)-1].Op)
			}
			if tc.name == "activate" {
				assert.Equal(t, 1, r.sess.Activations)
			}
		})
	}
}

// TestLateSessionForwards checks the system session's events reach the
// player through the stand-in once the media are made.
func TestLateSessionForwards(t *testing.T) {
	t.Parallel()

	r := newLateRig(t)
	var got []media.SessionEvent
	unsub := lateSession{m: r.m}.Events().Subscribe(func(ev media.SessionEvent) { got = append(got, ev) })
	t.Cleanup(unsub)
	r.m.make()
	r.sess.Emit(media.SessionEvent{Kind: media.InterruptBegin})
	assert.Equal(t, []media.SessionEvent{{Kind: media.InterruptBegin}}, got)
}

// TestLateMediaCloseBeforeMake closes before the media are made: they never
// are, and the calls after do nothing.
func TestLateMediaCloseBeforeMake(t *testing.T) {
	t.Parallel()

	r := newLateRig(t)
	require.NoError(t, r.m.Close())
	r.m.make()
	require.NoError(t, r.m.Play())
	assert.Nil(t, r.m.Video())
	assert.Zero(t, r.made)
}

// TestLatePlayerPlaysThrough plays a lesson through the stand-in with the
// player: the address resolved, the load waits for make; Play makes the
// media, which plays, and the player follows its events and its
// session's.
func TestLatePlayerPlaysThrough(t *testing.T) {
	t.Parallel()

	k := newUIClock(func() time.Time { return t0 })
	tt := newBlankTester(t)
	k.Bind(tt.View())
	var eng *fake.Engine
	var sess *fake.Session
	m := newLateMedia(media.Config{Dispatch: func(fn func()) { fn() }, Clock: k}, func(cfg media.Config) (platformMedia, error) {
		eng, sess = fake.New(cfg, fake.Options{}), &fake.Session{}

		return platformMedia{name: "fake", eng: eng, sess: sess}, nil
	}, t.Logf)
	p := player.New(player.Options{
		Engine: m, Session: lateSession{m: m}, Resolver: urlResolver{},
		Dispatch: func(fn func()) { fn() }, Go: func(work func(context.Context)) { work(t.Context()) },
	})
	t.Cleanup(func() {
		p.Close()
		require.NoError(t, m.Close())
	})
	l := player.Lesson{ID: "l_1", CourseID: "c", DurationSec: 20}
	p.PlayAudio(l, []player.Lesson{l})
	require.NotNil(t, eng, "the session's Activate makes the media")
	tt.Elapse(time.Second)
	assert.Equal(t, player.StatePlaying, p.State())
	assert.Positive(t, p.Pos())
	sess.Emit(media.SessionEvent{Kind: media.InterruptBegin})
	tt.Elapse(time.Second / 4)
	assert.Equal(t, player.StatePaused, p.State(), "a call pauses")
}

// TestLateForwardDoesNotAllocate checks an event of the engine reaches
// the player through the stand-in without allocating: a tick goes
// through it.
func TestLateForwardDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	r := newLateRig(t)
	r.m.make()
	r.got = make([]media.Event, 0, 1024)
	ev := media.Event{Kind: media.EventTime, Time: 1}
	allocs := testing.AllocsPerRun(200, func() {
		r.got = r.got[:0]
		r.m.forward(ev)
	})
	assert.Zero(t, allocs)
}

// BenchmarkLateForward measures an event of the engine reaching the
// player through the stand-in: every tick.
func BenchmarkLateForward(b *testing.B) {
	r := &lateRig{clk: clock.NewFake(t0)}
	r.m = newLateMedia(media.Config{Dispatch: func(fn func()) { fn() }, Clock: r.clk}, fakeMediaOf(fake.Options{}), b.Logf)
	r.m.make()
	defer func() { _ = r.m.Close() }()
	ev := media.Event{Kind: media.EventTime, Time: 1}
	b.ReportAllocs()
	for b.Loop() {
		r.m.forward(ev)
	}
}
