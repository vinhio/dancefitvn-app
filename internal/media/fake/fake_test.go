// Adapted from jplayer-mobile/internal/media/fake/fake_test.go at 49de3c2.

package fake_test

import (
	"context"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var t0 = time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)

// rig is an engine, its clock and the events it published.
type rig struct {
	clk *clock.Fake
	eng *fake.Engine
	got []media.Event
}

func newRig(t testing.TB, o fake.Options) *rig {
	t.Helper()
	r := &rig{clk: clock.NewFake(t0)}
	r.eng = fake.New(media.Config{Clock: r.clk}, o)
	r.eng.Events().Subscribe(func(ev media.Event) { r.got = append(r.got, ev) })

	return r
}

// load loads a link and lets the engine finish.
func (r *rig) load(t testing.TB, src media.Source, o media.LoadOptions) uint64 {
	t.Helper()
	tok, err := r.eng.Load(context.Background(), src, o)
	require.NoError(t, err)
	r.clk.Advance(0)

	return tok
}

// kinds lists the kinds of the events so far, and clears them.
func (r *rig) kinds() []media.EventKind {
	var k []media.EventKind
	for _, ev := range r.got {
		k = append(k, ev.Kind)
	}
	r.got = nil

	return k
}

var (
	audio = media.Source{URL: "https://x.test/tone.mp3", AudioOnly: true}
	video = media.Source{URL: "https://x.test/clip.mp4"}
)

func TestLoad(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		src       media.Source
		opts      media.LoadOptions
		o         fake.Options
		wantKinds []media.EventKind
		wantState media.PlayState
		wantDur   float64
		wantPos   float64
	}{
		{
			name: "audio plays", src: audio, opts: media.LoadOptions{Auto: true},
			wantKinds: []media.EventKind{media.EventDuration, media.EventTime, media.EventState},
			wantState: media.StatePlaying, wantDur: 40,
		},
		{
			name: "audio paused", src: audio,
			wantKinds: []media.EventKind{media.EventDuration, media.EventTime, media.EventState},
			wantState: media.StatePaused, wantDur: 40,
		},
		{
			name: "video also gives its size", src: video, opts: media.LoadOptions{Auto: true},
			wantKinds: []media.EventKind{media.EventDuration, media.EventVideoSize, media.EventTime, media.EventState},
			wantState: media.StatePlaying, wantDur: 20,
		},
		{
			name: "start", src: audio, opts: media.LoadOptions{Start: 12, Auto: true},
			wantKinds: []media.EventKind{media.EventDuration, media.EventTime, media.EventState},
			wantState: media.StatePlaying, wantDur: 40, wantPos: 12,
		},
		{
			name: "durations by URL", src: audio, o: fake.Options{Durations: map[string]float64{audio.URL: 95}},
			wantKinds: []media.EventKind{media.EventDuration, media.EventTime, media.EventState},
			wantState: media.StatePaused, wantDur: 95,
		},
		{
			name: "default lengths", src: video, o: fake.Options{VideoDuration: 7},
			wantKinds: []media.EventKind{media.EventDuration, media.EventVideoSize, media.EventTime, media.EventState},
			wantState: media.StatePaused, wantDur: 7,
		},
		{
			name: "failure", src: audio, opts: media.LoadOptions{Auto: true},
			o:         fake.Options{Fail: map[string]error{audio.URL: media.ErrNetwork}},
			wantKinds: []media.EventKind{media.EventError}, wantState: media.StateError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, tt.o)
			tok := r.load(t, tt.src, tt.opts)

			assert.Equal(t, tt.wantKinds, r.kinds())
			assert.Equal(t, tt.wantState, r.eng.State())
			assert.Equal(t, tt.wantPos, r.eng.Position())
			assert.InDelta(t, tt.wantDur, r.eng.Duration(), 0)
			assert.Equal(t, uint64(1), tok)
		})
	}
}

func TestLoadDelay(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{LoadDelay: 500 * time.Millisecond})
	_, err := r.eng.Load(context.Background(), audio, media.LoadOptions{Auto: true})
	require.NoError(t, err)
	r.clk.Advance(499 * time.Millisecond)
	assert.Empty(t, r.got)
	assert.Equal(t, media.StateLoading, r.eng.State())

	r.clk.Advance(time.Millisecond)
	assert.Equal(t, media.StatePlaying, r.eng.State())
}

func TestTokens(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	first := r.load(t, audio, media.LoadOptions{Auto: true})
	r.got = nil
	_, err := r.eng.Load(context.Background(), audio, media.LoadOptions{Auto: true})
	require.NoError(t, err)
	r.clk.Advance(0)
	r.clk.Advance(250 * time.Millisecond)

	require.NotEmpty(t, r.got)
	for _, ev := range r.got {
		assert.Equal(t, first+1, ev.Token, "the earlier load's events are gone")
	}
}

func TestPlayback(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.load(t, audio, media.LoadOptions{Auto: true})
	r.kinds()
	r.clk.Advance(time.Second)

	assert.InDelta(t, 1, r.eng.Position(), 1e-9)
	assert.Equal(t, []media.EventKind{media.EventTime, media.EventTime, media.EventTime, media.EventTime}, r.kinds())
}

func TestPlaybackTimes(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{Tick: 100 * time.Millisecond})
	require.NoError(t, r.eng.SetRate(2))
	r.load(t, audio, media.LoadOptions{Auto: true})
	r.got = nil
	r.clk.Advance(300 * time.Millisecond)

	var times []float64
	for _, ev := range r.got {
		times = append(times, ev.Time)
	}
	assert.InDeltaSlice(t, []float64{0.2, 0.4, 0.6}, times, 1e-9, "100 ms ticks at double speed")
}

func TestEnds(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{AudioDuration: 1})
	r.load(t, audio, media.LoadOptions{Auto: true})
	r.got = nil
	r.clk.Advance(10 * time.Second)

	assert.Equal(t, []media.EventKind{media.EventTime, media.EventTime, media.EventTime, media.EventTime, media.EventEnded}, r.kinds())
	assert.Equal(t, media.StateEnded, r.eng.State())
	assert.InDelta(t, 1, r.eng.Position(), 0)
	assert.Zero(t, r.clk.Pending(), "the tick stopped")

	require.NoError(t, r.eng.Play())
	assert.Equal(t, media.StatePlaying, r.eng.State())
	assert.Zero(t, r.eng.Position(), "play at the end starts over")
}

func TestFinish(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.load(t, audio, media.LoadOptions{Auto: true})
	r.got = nil
	r.eng.Finish()

	assert.Equal(t, []media.EventKind{media.EventTime, media.EventEnded}, r.kinds())
	assert.InDelta(t, 40, r.eng.Position(), 0)
	r.eng.Finish()
	assert.Equal(t, []media.EventKind{media.EventTime, media.EventEnded}, r.kinds(), "once more, as asked")
}

func TestFinishBeforeLoad(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.eng.Finish()
	r.eng.Fail(errors.New("x"))
	r.eng.Stall()
	r.eng.Unstall()

	assert.Empty(t, r.got)
}

func TestPauseAndPlay(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.load(t, audio, media.LoadOptions{Auto: true})
	r.clk.Advance(time.Second)
	r.got = nil

	require.NoError(t, r.eng.Pause())
	assert.Equal(t, []media.EventKind{media.EventState}, r.kinds())
	r.clk.Advance(time.Second)
	assert.InDelta(t, 1, r.eng.Position(), 1e-9, "no time while paused")
	require.NoError(t, r.eng.Pause())
	assert.Empty(t, r.got, "pausing twice says nothing")

	require.NoError(t, r.eng.Play())
	r.clk.Advance(time.Second)
	assert.InDelta(t, 2, r.eng.Position(), 1e-9)
}

func TestPlayBeforeReady(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		do   func(e *fake.Engine)
		want media.PlayState
	}{
		{name: "play makes it start", do: func(e *fake.Engine) { _ = e.Play() }, want: media.StatePlaying},
		{name: "pause keeps it paused", do: func(e *fake.Engine) { _ = e.Pause() }, want: media.StatePaused},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, fake.Options{})
			_, err := r.eng.Load(context.Background(), audio, media.LoadOptions{Auto: tt.want == media.StatePaused})
			require.NoError(t, err)
			tt.do(r.eng)
			r.clk.Advance(0)

			assert.Equal(t, tt.want, r.eng.State())
		})
	}
}

func TestSeek(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		to   float64
		want float64
	}{
		{name: "inside", to: 12.5, want: 12.5},
		{name: "past the end", to: 99, want: 40},
		{name: "before the start", to: -4, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, fake.Options{})
			r.load(t, audio, media.LoadOptions{})
			r.got = nil
			require.NoError(t, r.eng.Seek(tt.to))

			assert.InDelta(t, tt.want, r.eng.Position(), 0)
			require.Len(t, r.got, 1)
			assert.Equal(t, media.EventTime, r.got[0].Kind)
			assert.InDelta(t, tt.want, r.got[0].Time, 0)
		})
	}
}

func TestStallAndFail(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.load(t, audio, media.LoadOptions{Auto: true})
	r.clk.Advance(time.Second)
	r.got = nil

	r.eng.Stall()
	assert.Equal(t, media.StateLoading, r.eng.State())
	r.clk.Advance(time.Second)
	assert.InDelta(t, 1, r.eng.Position(), 1e-9)
	r.eng.Unstall()
	r.clk.Advance(250 * time.Millisecond)
	assert.InDelta(t, 1.25, r.eng.Position(), 1e-9)

	r.got = nil
	r.eng.Fail(media.ErrDecode)
	require.Len(t, r.got, 1)
	assert.ErrorIs(t, r.got[0].Err, media.ErrDecode)
	assert.Equal(t, media.StateError, r.eng.State())
	r.clk.Advance(time.Second)
	assert.InDelta(t, 1.25, r.eng.Position(), 1e-9)
	assert.ErrorIs(t, r.eng.Play(), media.ErrDecode)
}

func TestBufferAndDuration(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.load(t, audio, media.LoadOptions{})
	r.got = nil
	r.eng.Buffer(9)
	r.eng.SetDuration(44)

	require.Len(t, r.got, 2)
	assert.InDelta(t, 9, r.got[0].Buffered, 0)
	assert.InDelta(t, 44, r.got[1].Duration, 0)
	assert.InDelta(t, 44, r.eng.Duration(), 0)
}

func TestEmitKeepsTheToken(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.eng.Emit(media.Event{Kind: media.EventTime, Token: 77})

	require.Len(t, r.got, 1)
	assert.Equal(t, uint64(77), r.got[0].Token)
}

func TestStop(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	tok := r.load(t, audio, media.LoadOptions{Auto: true})
	require.NoError(t, r.eng.Stop())
	r.got = nil
	r.clk.Advance(time.Second)

	assert.Empty(t, r.got)
	assert.Zero(t, r.clk.Pending())
	assert.Greater(t, r.eng.Token(), tok)
	assert.ErrorIs(t, r.eng.Play(), fake.ErrNotLoaded)
	assert.ErrorIs(t, r.eng.Pause(), fake.ErrNotLoaded)
	assert.ErrorIs(t, r.eng.Seek(1), fake.ErrNotLoaded)
}

func TestSettersAndCalls(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	require.NoError(t, r.eng.SetVolume(0.3))
	require.NoError(t, r.eng.SetMuted(true))
	require.NoError(t, r.eng.SetRate(1.5))
	r.load(t, audio, media.LoadOptions{Start: 3})
	require.NoError(t, r.eng.Close())

	assert.InDelta(t, 0.3, r.eng.Volume(), 0)
	assert.True(t, r.eng.Muted())
	assert.InDelta(t, 1.5, r.eng.Rate(), 0)
	calls := r.eng.Calls()
	require.Len(t, calls, 5)
	assert.Equal(t, []fake.Op{fake.OpVolume, fake.OpMuted, fake.OpRate, fake.OpLoad, fake.OpClose}, []fake.Op{calls[0].Op, calls[1].Op, calls[2].Op, calls[3].Op, calls[4].Op})
	assert.Equal(t, audio, calls[3].Src)
	assert.InDelta(t, 3, calls[3].Load.Start, 0)
	assert.True(t, calls[1].Flag)

	r.eng.ClearCalls()
	assert.Empty(t, r.eng.Calls())
}

func TestCallsAreBounded(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	for i := range 5000 {
		_ = r.eng.SetVolume(float64(i))
	}
	calls := r.eng.Calls()

	assert.LessOrEqual(t, len(calls), 4096)
	assert.InDelta(t, 4999, calls[len(calls)-1].Value, 0, "the newest are kept")
}

func TestVideoFrames(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.load(t, video, media.LoadOptions{Auto: true})
	img, seq := r.eng.Video().Frame(0)
	require.NotNil(t, img, "a first frame at load")
	assert.Equal(t, fake.FrameWidth, img.Width)
	assert.Equal(t, fake.FrameHeight, img.Height)
	assert.True(t, img.BGRA)
	first := img.Pix[0]

	r.clk.Advance(time.Second)
	img2, seq2 := r.eng.Video().Frame(0)
	assert.Equal(t, seq+4, seq2, "a frame per tick")
	assert.NotEqual(t, first, img2.Pix[0], "and a new colour")
	for i := 3; i < len(img2.Pix); i += 4 {
		require.Equal(t, uint8(0xff), img2.Pix[i], "opaque")
	}

	var size media.Event
	for _, ev := range r.got {
		if ev.Kind == media.EventVideoSize {
			size = ev
		}
	}
	assert.Equal(t, fake.FrameWidth, size.Width)
}

func TestAudioHasNoFrames(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.load(t, audio, media.LoadOptions{Auto: true})
	r.clk.Advance(time.Second)
	img, seq := r.eng.Video().Frame(0)

	assert.Nil(t, img)
	assert.Zero(t, seq)
}

func TestUnknownLengthNeverEnds(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{Durations: map[string]float64{audio.URL: 0}})
	r.load(t, audio, media.LoadOptions{Auto: true})
	r.clk.Advance(10 * time.Minute)

	assert.Equal(t, media.StatePlaying, r.eng.State())
	assert.InDelta(t, 600, r.eng.Position(), 1e-6)
}

func TestFakeTickDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	for _, src := range []media.Source{audio, video} {
		r := &rig{clk: clock.NewFake(t0)}
		r.eng = fake.New(media.Config{Clock: r.clk}, fake.Options{AudioDuration: 1e9, VideoDuration: 1e9})
		r.eng.Events().Subscribe(func(media.Event) {})
		r.load(t, src, media.LoadOptions{Auto: true})
		r.clk.Advance(time.Second)

		assert.Zero(t, testing.AllocsPerRun(100, func() { r.clk.Advance(250 * time.Millisecond) }), src.URL)
	}
}

func TestSession(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	s := &fake.Session{Err: boom}
	var got []media.SessionEvent
	s.Events().Subscribe(func(ev media.SessionEvent) { got = append(got, ev) })

	assert.ErrorIs(t, s.Activate(), boom)
	assert.Equal(t, 1, s.Activations)
	s.Emit(media.SessionEvent{Kind: media.InterruptEnd, ShouldResume: true})
	assert.Equal(t, []media.SessionEvent{{Kind: media.InterruptEnd, ShouldResume: true}}, got)
}

func TestImplementsContracts(t *testing.T) {
	t.Parallel()
	var _ media.Engine = (*fake.Engine)(nil)
	var _ media.Session = (*fake.Session)(nil)
	var _ media.Backgrounder = (*fake.Engine)(nil)
	var _ media.TimeReader = (*fake.Engine)(nil)
}

func TestFakeBackground(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.load(t, video, media.LoadOptions{Auto: true, Start: 2})
	r.clk.Advance(500 * time.Millisecond)
	r.kinds()
	_, seqBefore := r.eng.Video().Frame(0)

	r.eng.SetBackground(true)
	assert.True(t, r.eng.Backgrounded())
	r.clk.Advance(5 * time.Second)
	assert.Empty(t, r.kinds(), "no time in the background")
	img, seq := r.eng.Video().Frame(0)
	assert.Nil(t, img, "no frames either: the images are freed")
	assert.Zero(t, seq)
	assert.InDelta(t, 7.5, r.eng.Position(), 1e-9, "it plays on, untold, as audio does")
	assert.Equal(t, media.StatePlaying, r.eng.State())

	r.eng.SetBackground(true) // again: nothing
	r.eng.SetBackground(false)
	assert.Equal(t, []media.EventKind{media.EventTime}, r.kinds(), "the time at once")
	_, seq = r.eng.Video().Frame(0)
	assert.Greater(t, seq, seqBefore, "and a frame")
	r.clk.Advance(250 * time.Millisecond)
	assert.Equal(t, []media.EventKind{media.EventTime}, r.kinds(), "then it ticks again")

	calls := r.eng.Calls()
	assert.Equal(t, fake.OpSetBackground, calls[len(calls)-1].Op)
	assert.False(t, calls[len(calls)-1].Flag)
}

// TestFakeBackgroundVideoOff follows a video through the background as
// the systems' engines take it (F2, F3 of M11.P): the picture is not
// decoded there and the images are freed, once (the surface's OnFree),
// while the sound plays on and keeps its place; back in the foreground
// the picture is decoded again and the first frame comes at once, into
// images made again; an audio load frees them too, and an audio media in
// the background has no picture to turn off.
func TestFakeBackgroundVideoOff(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	freed := 0
	r.eng.Video().SetOnFree(func() { freed++ })
	r.load(t, video, media.LoadOptions{Auto: true})
	r.clk.Advance(time.Second)
	require.True(t, r.eng.Video().Holds())
	assert.False(t, r.eng.VideoOff())

	r.eng.SetBackground(true)
	assert.True(t, r.eng.VideoOff(), "the picture off in the background")
	assert.False(t, r.eng.Video().Holds(), "the images freed")
	assert.Equal(t, 1, freed)
	r.clk.Advance(4 * time.Second)
	assert.False(t, r.eng.Video().Holds(), "and not made again there")
	assert.InDelta(t, 5, r.eng.Position(), 1e-9, "the sound plays on")
	r.eng.SetBackground(true)
	assert.Equal(t, 1, freed, "told again: nothing more to free")

	r.kinds()
	r.eng.SetBackground(false)
	assert.False(t, r.eng.VideoOff())
	img, seq := r.eng.Video().Frame(0)
	require.NotNil(t, img, "a frame at once")
	assert.Positive(t, seq)
	assert.Equal(t, [2]int{fake.FrameWidth, fake.FrameHeight}, [2]int{img.Width, img.Height})
	assert.Equal(t, []media.Event{{Kind: media.EventTime, Token: r.eng.Token(), Time: 5}}, r.got, "at its place")

	r.load(t, audio, media.LoadOptions{Auto: true})
	assert.False(t, r.eng.Video().Holds(), "an audio load frees the images")
	assert.Equal(t, 2, freed)
	r.eng.SetBackground(true)
	assert.False(t, r.eng.VideoOff(), "audio has no picture")
	assert.Equal(t, 2, freed, "nothing to free")
}

func TestFakeBackgroundPaused(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.load(t, audio, media.LoadOptions{})
	r.kinds()
	r.eng.SetBackground(true)
	r.eng.SetBackground(false)
	assert.Equal(t, []media.EventKind{media.EventTime}, r.kinds())
	r.clk.Advance(time.Second)
	assert.Empty(t, r.kinds(), "paused: no tick after the return")

	// A play in the background starts the clock only on return.
	r.eng.SetBackground(true)
	require.NoError(t, r.eng.Play())
	r.kinds()
	r.clk.Advance(time.Second)
	assert.Empty(t, r.kinds())
	r.eng.SetBackground(false)
	r.kinds()
	r.clk.Advance(250 * time.Millisecond)
	assert.Equal(t, []media.EventKind{media.EventTime}, r.kinds())
}

func TestFakeReadTime(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	tok, err := r.eng.Load(context.Background(), audio, media.LoadOptions{Auto: true, Start: 3})
	require.NoError(t, err)
	_, ok := r.eng.ReadTime(tok)
	assert.False(t, ok, "not ready")
	r.clk.Advance(0)
	got, ok := r.eng.ReadTime(tok)
	require.True(t, ok)
	assert.InDelta(t, 3.0, got, 1e-9)
	_, ok = r.eng.ReadTime(tok - 1)
	assert.False(t, ok, "another load's")

	// In the background: no event, the position moves at the speed.
	r.clk.Advance(time.Second)
	r.kinds()
	r.eng.SetBackground(true)
	r.clk.Advance(10 * time.Second)
	require.NoError(t, r.eng.SetRate(2))
	r.clk.Advance(5 * time.Second)
	assert.Empty(t, r.kinds())
	got, ok = r.eng.ReadTime(tok)
	require.True(t, ok)
	assert.InDelta(t, 4+10+10, got, 1e-9)
	require.NoError(t, r.eng.Seek(30))
	r.kinds()
	r.clk.Advance(2 * time.Second)
	require.NoError(t, r.eng.Pause())
	assert.InDelta(t, 34.0, r.eng.Position(), 1e-9)
	r.clk.Advance(time.Minute)
	got, _ = r.eng.ReadTime(tok)
	assert.InDelta(t, 34.0, got, 1e-9, "paused: it stays")

	require.NoError(t, r.eng.Stop())
	_, ok = r.eng.ReadTime(tok)
	assert.False(t, ok, "unloaded")
}

func TestFakeBackgroundEnds(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.load(t, audio, media.LoadOptions{Auto: true, Start: 30})
	r.eng.SetBackground(true)
	r.kinds()
	r.clk.Advance(9 * time.Second)
	assert.Empty(t, r.kinds())
	r.clk.Advance(time.Second)
	assert.Equal(t, []media.EventKind{media.EventTime, media.EventEnded}, r.kinds(),
		"the end, as the system tells it, without a tick")
	assert.Equal(t, media.StateEnded, r.eng.State())
	assert.InDelta(t, 40.0, r.eng.Position(), 1e-9)

	// A media loaded in the background plays there, to its end too.
	tok := r.load(t, audio, media.LoadOptions{Auto: true})
	r.kinds()
	r.clk.Advance(17 * time.Second)
	got, ok := r.eng.ReadTime(tok)
	require.True(t, ok)
	assert.InDelta(t, 17.0, got, 1e-9)
	r.eng.SetBackground(false)
	assert.Equal(t, []media.EventKind{media.EventTime}, r.kinds())
	assert.InDelta(t, 17.0, r.eng.Position(), 1e-9, "the foreground goes on from there")
	r.clk.Advance(250 * time.Millisecond)
	assert.InDelta(t, 17.25, r.eng.Position(), 1e-9)
}

func TestFakeBackgroundBeforeLoad(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.eng.SetBackground(true)
	r.eng.SetBackground(false)
	assert.Empty(t, r.kinds())
}

func TestFakeNotifiesFrames(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	var seqs []uint64
	r.eng.Video().Frames().Subscribe(func(n uint64) { seqs = append(seqs, n) })
	r.load(t, video, media.LoadOptions{Auto: true})
	assert.Equal(t, []uint64{1}, seqs, "the first frame at load")
	r.clk.Advance(time.Second)
	assert.Equal(t, []uint64{1, 2, 3, 4, 5}, seqs, "one per frame")
	assert.True(t, r.eng.Video().Active(), "frames are coming while it plays")

	require.NoError(t, r.eng.Pause())
	assert.False(t, r.eng.Video().Active())
}

func TestFakeSurfaceLifecycle(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{})
	r.load(t, video, media.LoadOptions{Auto: true})
	img, _ := r.eng.Video().Frame(0)
	require.NotNil(t, img)

	// A new load, even of audio, shows no picture of the last media.
	_, err := r.eng.Load(context.Background(), audio, media.LoadOptions{})
	require.NoError(t, err)
	img, _ = r.eng.Video().Frame(0)
	assert.Nil(t, img, "Load resets the surface")
	assert.False(t, r.eng.Video().Active())
	r.clk.Advance(0)

	r.load(t, video, media.LoadOptions{Auto: true})
	img, _ = r.eng.Video().Frame(0)
	require.NotNil(t, img)
	require.NoError(t, r.eng.Stop())
	img, _ = r.eng.Video().Frame(0)
	assert.Nil(t, img, "Stop frees it")

	r.load(t, video, media.LoadOptions{})
	require.NoError(t, r.eng.Close())
	img, _ = r.eng.Video().Frame(0)
	assert.Nil(t, img, "Close too")
}

func TestFrequencies(t *testing.T) {
	t.Parallel()

	frame := time.Second / 60
	audio := media.Source{URL: "/data/k.mp3", AudioOnly: true, Analyse: true}
	tests := []struct {
		name   string
		src    media.Source
		load   bool
		play   bool
		want   bool
		sound  bool
		bounce bool
	}{
		{"not loaded", audio, false, false, false, false, false},
		{"playing", audio, true, true, true, true, false},
		{"paused is silence", audio, true, false, true, false, false},
		{"not analysed", media.Source{URL: "https://x.test/a.mp3", AudioOnly: true}, true, true, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, fake.Options{})
			if tt.load {
				r.load(t, tt.src, media.LoadOptions{Auto: tt.play})
				r.clk.Advance(time.Second)
			}
			var b [media.FreqBins]uint8
			b[5] = 77
			var got bool
			for i := 0; i < 20; i++ {
				got = r.eng.Frequencies(&b, frame)
			}
			require.Equal(t, tt.want, got)
			if !tt.want {
				assert.Equal(t, uint8(77), b[5], "left alone")

				return
			}
			if !tt.sound {
				assert.Equal(t, [media.FreqBins]uint8{}, b)

				return
			}
			assert.Contains(t, []int{1, 2, 3}, peakOf(&b), "the tones saturate the bins around them")
			assert.GreaterOrEqual(t, b[2], uint8(150))
			assert.GreaterOrEqual(t, b[3], uint8(150))
			assert.LessOrEqual(t, b[40], uint8(60))
		})
	}
}

// TestFrequenciesLoadingIsSilence checks that a media analysed answers
// silence from its Load on, before it is ready (as avf's tap before its
// first buffer: the prototype's analyser is real from the load).
func TestFrequenciesLoadingIsSilence(t *testing.T) {
	t.Parallel()

	r := newRig(t, fake.Options{LoadDelay: time.Second})
	r.load(t, media.Source{URL: "/k.mp3", AudioOnly: true, Analyse: true}, media.LoadOptions{Auto: true})
	b := [media.FreqBins]uint8{5: 77}
	require.True(t, r.eng.Frequencies(&b, time.Second/60))
	assert.Equal(t, [media.FreqBins]uint8{}, b)
}

func peakOf(b *[media.FreqBins]uint8) int {
	p := 0
	for i, v := range b {
		if v > b[p] {
			p = i
		}
	}

	return p
}

func TestFrequenciesPauseDecaysAndNewLoadResets(t *testing.T) {
	t.Parallel()

	frame := time.Second / 60
	r := newRig(t, fake.Options{})
	r.load(t, media.Source{URL: "/k.mp3", AudioOnly: true, Analyse: true}, media.LoadOptions{Auto: true})
	r.clk.Advance(time.Second)
	var b [media.FreqBins]uint8
	for i := 0; i < 20; i++ {
		r.eng.Frequencies(&b, frame)
	}
	loud := b[2]
	require.NotZero(t, loud)
	require.NoError(t, r.eng.Pause())
	r.eng.Frequencies(&b, frame)
	assert.Less(t, b[2], loud)
	for i := 0; i < 600 && b != [media.FreqBins]uint8{}; i++ {
		r.eng.Frequencies(&b, frame)
	}
	assert.Equal(t, [media.FreqBins]uint8{}, b)

	require.NoError(t, r.eng.Play())
	for i := 0; i < 20; i++ {
		r.eng.Frequencies(&b, frame)
	}
	require.NotZero(t, b[2])
	r.load(t, media.Source{URL: "/k2.mp3", AudioOnly: true, Analyse: true}, media.LoadOptions{})
	r.clk.Advance(time.Second)
	r.eng.Frequencies(&b, frame)
	assert.Equal(t, [media.FreqBins]uint8{}, b, "smoothing forgotten, new load paused")
}

func TestFrequenciesDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	r := newRig(t, fake.Options{})
	r.load(t, media.Source{URL: "/k.mp3", AudioOnly: true, Analyse: true}, media.LoadOptions{Auto: true})
	r.clk.Advance(time.Second)
	var b [media.FreqBins]uint8
	r.eng.Frequencies(&b, time.Second/60)
	assert.Zero(t, testing.AllocsPerRun(100, func() { r.eng.Frequencies(&b, time.Second/60) }))
}

func TestLiveSource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  media.Source
	}{
		{"audio", media.Source{URL: "https://x.test/radio.m3u8", AudioOnly: true, Live: true}},
		{"video", media.Source{URL: "https://x.test/tv.m3u8", Live: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, fake.Options{})
			r.load(t, tt.src, media.LoadOptions{Auto: true})
			assert.Zero(t, r.eng.Duration(), "no length")
			r.clk.Advance(10 * time.Minute) // a media of 40 s would have ended
			assert.Equal(t, media.StatePlaying, r.eng.State())
			pos := r.eng.Position()
			assert.Greater(t, pos, 599.0)
			require.NoError(t, r.eng.Seek(3))
			assert.InDelta(t, pos, r.eng.Position(), 1e-9, "the fake has no live window to seek in")
			for _, ev := range r.got {
				assert.NotEqual(t, media.EventEnded, ev.Kind)
			}
		})
	}
}

func TestStartAfterDuration(t *testing.T) {
	t.Parallel()
	const none = -1.0
	tests := []struct {
		name string
		// before is a Seek before ready, answer one in answer to
		// EventDuration; none for no Seek.
		before, answer float64
		wantStart      float64
		wantEnded      bool
	}{
		{name: "start kept within the length", before: none, answer: none, wantStart: 12, wantEnded: true},
		{name: "start inside the length", before: 5, answer: none, wantStart: 5},
		{name: "a seek in answer to the length wins", before: none, answer: 0, wantStart: 0},
		{name: "it wins over a seek before ready", before: 5, answer: 2, wantStart: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, fake.Options{VideoDuration: 12, StartAfterDuration: true})
			var timeAtDuration float64
			r.eng.Events().Subscribe(func(ev media.Event) {
				if ev.Kind != media.EventDuration {
					return
				}
				timeAtDuration = r.eng.Position()
				if tt.answer != none {
					require.NoError(t, r.eng.Seek(tt.answer))
				}
			})
			_, err := r.eng.Load(context.Background(), video, media.LoadOptions{Start: 72, Auto: true})
			require.NoError(t, err)
			if tt.before != none {
				require.NoError(t, r.eng.Seek(tt.before))
			}
			r.clk.Advance(0)

			assert.InDelta(t, 0, timeAtDuration, 0, "the start is not applied before the length is told")
			var times []float64
			for _, ev := range r.got {
				if ev.Kind == media.EventTime {
					times = append(times, ev.Time)
				}
			}
			assert.Equal(t, []float64{tt.wantStart}, times, "one time, the start, once ready")
			r.clk.Advance(fake.DefaultTick)
			assert.Equal(t, tt.wantEnded, r.eng.State() == media.StateEnded, "a start at the end ends at the first tick")
		})
	}
}

func TestReadyStopsWhenReloadedInAnswer(t *testing.T) {
	t.Parallel()
	r := newRig(t, fake.Options{LoadDelay: time.Second})
	reloaded := false
	r.eng.Events().Subscribe(func(ev media.Event) {
		if ev.Kind == media.EventDuration && !reloaded {
			reloaded = true
			_, err := r.eng.Load(context.Background(), audio, media.LoadOptions{})
			require.NoError(t, err)
		}
	})
	r.load(t, video, media.LoadOptions{Auto: true})
	r.clk.Advance(time.Second)

	assert.Equal(t, []media.EventKind{media.EventDuration}, r.kinds(), "nothing more of the load replaced in answer to its length")
	assert.Equal(t, media.StateLoading, r.eng.State(), "the new load waits for its own ready")
}
