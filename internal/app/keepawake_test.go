//go:build !tvos

// Adapted from jplayer-mobile/internal/app (keepAwake's tests of
// m9_wire_test.go at 49de3c2): a recording sink in place of the system's.

package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/player"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAwake is an awakeSink that writes down what it was told.
type fakeAwake struct {
	mu       sync.Mutex
	sets     []bool
	reapply  int
	failWith error
}

// Set records on.
func (f *fakeAwake) Set(on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failWith != nil {
		return f.failWith
	}
	f.sets = append(f.sets, on)

	return nil
}

// Reapply counts.
func (f *fakeAwake) Reapply() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reapply++

	return f.failWith
}

// told returns the values set.
func (f *fakeAwake) told() []bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]bool(nil), f.sets...)
}

// TestKeepAwakeFollowsVideo checks the screen stays on while a video plays
// with the app in front, and only then: a pause, the background, an audio
// lesson let it go; the foreground keeps it again and reapplies it.
func TestKeepAwakeFollowsVideo(t *testing.T) {
	t.Parallel()

	k := newUIClock(func() time.Time { return t0 })
	tt := newBlankTester(t)
	k.Bind(tt.View())
	eng := fake.New(media.Config{Dispatch: func(fn func()) { fn() }, Clock: k}, fake.Options{})
	p := player.New(player.Options{
		Engine: eng, Resolver: urlResolver{}, Dispatch: func(fn func()) { fn() },
		Go: func(work func(context.Context)) { work(t.Context()) },
	})
	t.Cleanup(p.Close)
	sink := &fakeAwake{}
	a := newKeepAwake(sink, p, t.Logf)
	t.Cleanup(a.close)
	v := player.Lesson{ID: "l_1", CourseID: "c", DurationSec: 20}
	au := player.Lesson{ID: "l_2", CourseID: "c", DurationSec: 20}
	list := []player.Lesson{v, au}

	p.PlayVideo(v, list)
	tt.Elapse(time.Second)
	require.Equal(t, player.StatePlaying, p.State())
	assert.Equal(t, []bool{false, true}, sink.told(), "told off at the load, on as the video plays")

	p.Pause()
	tt.Elapse(time.Second / 4)
	assert.Equal(t, []bool{false, true, false}, sink.told(), "paused")

	p.Play()
	tt.Elapse(time.Second / 4)
	a.lifecycle(platform.LifecycleBackground)
	assert.Equal(t, []bool{false, true, false, true, false}, sink.told(), "the background lets go")
	a.lifecycle(platform.LifecycleActive)
	assert.Equal(t, []bool{false, true, false, true, false, true}, sink.told(), "in front again")
	assert.Equal(t, 1, sink.reapply)

	p.PlayAudio(au, list)
	tt.Elapse(time.Second)
	require.Equal(t, player.StatePlaying, p.State())
	assert.Equal(t, []bool{false, true, false, true, false, true, false}, sink.told(), "an audio lesson")
}

// TestKeepAwakeUnsupported checks a system that cannot keep the screen on
// is asked once, and a nil sink never.
func TestKeepAwakeUnsupported(t *testing.T) {
	t.Parallel()

	k := newUIClock(func() time.Time { return t0 })
	tt := newBlankTester(t)
	k.Bind(tt.View())
	eng := fake.New(media.Config{Dispatch: func(fn func()) { fn() }, Clock: k}, fake.Options{})
	p := player.New(player.Options{
		Engine: eng, Resolver: urlResolver{}, Dispatch: func(fn func()) { fn() },
		Go: func(work func(context.Context)) { work(t.Context()) },
	})
	t.Cleanup(p.Close)
	sink := &fakeAwake{failWith: core.ErrUnsupported}
	a := newKeepAwake(sink, p, t.Logf)
	none := newKeepAwake(nil, p, t.Logf)
	v := player.Lesson{ID: "l_1", CourseID: "c", DurationSec: 20}
	p.PlayVideo(v, []player.Lesson{v})
	tt.Elapse(time.Second)
	p.Pause()
	a.lifecycle(platform.LifecycleActive)
	none.lifecycle(platform.LifecycleActive)
	a.close()
	none.close()
	assert.Zero(t, sink.reapply, "turned off by the first refusal")
}
