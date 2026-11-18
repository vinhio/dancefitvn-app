package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/player"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestHomeMark checks the mark of the first Home frame: nothing while Home
// is not on show, one line at the first frame that shows it, none after;
// off a phone the time counts from the app package's start.
func TestHomeMark(t *testing.T) {
	t.Parallel()

	l := &testLog{}
	shown := false
	m := newHomeMark(func() bool { return shown }, l.add)
	assert.Equal(t, "init", m.from, "a desktop reads no process start")
	m.start = time.UnixMilli(1_000)
	m.now = func() time.Time { return time.UnixMilli(2_234) }
	m.after()
	assert.Empty(t, l.all())
	shown = true
	m.after()
	m.after()
	require.Len(t, l.all(), 1)
	assert.Equal(t, "dancefitvn: mark home 1234 t=2234 from=init", l.all()[0])
}

// TestHomeMarkDoesNotAllocate checks the mark's AfterFrame, run after
// every frame: nothing allocated before Home shows nor once marked.
func TestHomeMarkDoesNotAllocate(t *testing.T) {
	shown := false
	m := newHomeMark(func() bool { return shown }, func(string, ...any) {})
	assert.Zero(t, testing.AllocsPerRun(100, m.after), "before Home")
	shown = true
	m.after()
	assert.Zero(t, testing.AllocsPerRun(100, m.after), "once marked")
}

// BenchmarkHomeMarkAfter times the mark's AfterFrame once marked.
func BenchmarkHomeMarkAfter(b *testing.B) {
	m := newHomeMark(func() bool { return true }, func(string, ...any) {})
	m.after()
	b.ReportAllocs()
	for b.Loop() {
		m.after()
	}
}

// TestChainFrames checks the AfterFrame of several: nil for none, the one
// itself, each in order otherwise.
func TestChainFrames(t *testing.T) {
	t.Parallel()

	assert.Nil(t, chainFrames(nil, nil))
	var got []string
	a := func() { got = append(got, "a") }
	b := func() { got = append(got, "b") }
	one := chainFrames(nil, a)
	require.NotNil(t, one)
	one()
	assert.Equal(t, []string{"a"}, got)
	got = nil
	chainFrames(a, nil, b)()
	assert.Equal(t, []string{"a", "b"}, got)
}

// TestEnvCount checks the developer's counts: the default in a build that
// reads no variable (the tests').
func TestEnvCount(t *testing.T) {
	t.Parallel()
	if devBuild {
		t.Skip("a developer's build reads the variable")
	}

	assert.Equal(t, 7, envCount(perfFlingsEnv, 7))
}

// TestSayPerfVideo checks the line of PERF-VIDEO10: pos is the engine's
// own reading even when the ticks stopped telling it (the Android run's
// pos=0.00, local/p8/perf/report.md note 14), tick the stale position of
// the last time event, the media named when given.
func TestSayPerfVideo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		play      bool
		media     string
		pos, tick float64
		suffix    string
	}{
		{"ticks stopped, phone", true, "amp", 9.75, 1.75, " media=amp"},
		{"ticks stopped, TV", true, "", 9.75, 1.75, ""},
		{"nothing loaded", false, "amp", 0, 0, " media=amp"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			k := newUIClock(func() time.Time { return t0 })
			tt := newBlankTester(t)
			k.Bind(tt.View())
			eng := fake.New(media.Config{Dispatch: func(fn func()) { fn() }, Clock: k}, fake.Options{VideoDuration: 12})
			p := player.New(player.Options{
				Engine: eng, Resolver: perfResolver{}, Dispatch: func(fn func()) { fn() },
				Go: func(work func(context.Context)) { work(t.Context()) },
			})
			t.Cleanup(p.Close)
			if tc.play {
				l := player.Lesson{ID: "l_1", CourseID: "c", DurationSec: 12}
				p.PlayVideo(l, []player.Lesson{l})
				tt.Elapse(2 * time.Second) // 2 s of media at the fresh 1x
				require.Equal(t, player.StatePlaying, p.State())
				require.InDelta(t, 1.75, p.Pos(), 0.2)
				eng.SetBackground(true) // the engine plays on and tells no time
				tt.Elapse(8 * time.Second)
				require.InDelta(t, 1.75, p.Pos(), 0.2, "no tick moved it")
			}
			l := &testLog{}
			sayPerfVideo(l.add, p, "at", tc.media, time.UnixMilli(1_234))

			require.Len(t, l.all(), 1)
			var what string
			var at int64
			var pos, dur, tick float64
			var state int
			var loop bool
			n, err := fmt.Sscanf(l.all()[0], "dfv: perf video %s t=%d pos=%f dur=%f state=%d loop=%t tick=%f",
				&what, &at, &pos, &dur, &state, &loop, &tick)
			require.NoError(t, err, l.all()[0])
			require.Equal(t, 7, n)
			assert.Equal(t, "at", what)
			assert.Equal(t, int64(1_234), at)
			assert.InDelta(t, tc.pos, pos, 0.2, "the engine's reading")
			assert.InDelta(t, tc.tick, tick, 0.2, "the last tick's")
			assert.InDelta(t, tc.pos, p.Pos(), 0.2, "kept as Pos")
			assert.True(t, strings.HasSuffix(l.all()[0], fmt.Sprintf("tick=%.2f", tick)+tc.suffix), l.all()[0])
		})
	}
}

// perfResolver gives every lesson an address (urlResolver of late_test.go
// is the phone's only).
type perfResolver struct{}

// VideoURL is an address of the lesson.
func (perfResolver) VideoURL(_ context.Context, id string) (string, error) {
	return "https://example.invalid/" + id + ".m3u8", nil
}

// AudioURL is an address of the lesson.
func (perfResolver) AudioURL(_ context.Context, id string) (string, error) {
	return "https://example.invalid/" + id + ".mp3", nil
}

// hasLine tells whether one of lines begins with prefix.
func hasLine(lines []string, prefix string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}

	return false
}
