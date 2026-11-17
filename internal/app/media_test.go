//go:build !tvos

// TestProcessSession is adapted from
// jplayer-mobile/internal/app/media_test.go at 49de3c2; the warm-up tests
// are new.

package app

import (
	"errors"
	"sync"
	"testing"

	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// procTestMu keeps the tests of the process's session one at a time: they
// share it.
var procTestMu sync.Mutex

// resetProc empties the process's session for a test and puts it back
// after.
func resetProc(t *testing.T) {
	t.Helper()
	procTestMu.Lock()
	proc.mu.Lock()
	saved := proc.sess
	proc.sess, proc.err, proc.warming = nil, nil, nil
	proc.mu.Unlock()
	t.Cleanup(func() {
		proc.mu.Lock()
		proc.sess, proc.err, proc.warming = saved, nil, nil
		proc.mu.Unlock()
		procTestMu.Unlock()
	})
}

// TestProcessSession checks the session of the process is made once, and
// made again only after a failure.
func TestProcessSession(t *testing.T) { //nolint:paralleltest // the process's session
	resetProc(t)
	made := 0
	fail := true
	newSession := func(media.Config) (media.Session, error) {
		made++
		if fail {
			return nil, core.ErrUnsupported
		}

		return &fake.Session{}, nil
	}
	_, err := processSession(media.Config{}, newSession, t.Logf)
	require.ErrorIs(t, err, core.ErrUnsupported)
	fail = false
	s1, err := processSession(media.Config{}, newSession, t.Logf)
	require.NoError(t, err)
	s2, err := processSession(media.Config{}, newSession, t.Logf)
	require.NoError(t, err)
	assert.Same(t, s1, s2)
	assert.Equal(t, 2, made)
}

// TestWarmSession checks the warm-up makes the session off the caller's
// goroutine, once, tells ready on the dispatch, and that processSession
// waits for it and returns its session; a warm-up after it only tells
// ready.
func TestWarmSession(t *testing.T) { //nolint:paralleltest // the process's session
	resetProc(t)
	gate := make(chan struct{})
	made := 0
	want := &fake.Session{}
	newSession := func(media.Config) (media.Session, error) {
		<-gate
		made++

		return want, nil
	}
	q := &uiQueue{}
	cfg := media.Config{Dispatch: func(fn func()) { q.enqueue(fn) }}
	readies := 0
	ready := func() { readies++ }
	warmSession(cfg, newSession, ready, t.Logf)
	warmSession(cfg, newSession, ready, t.Logf) // under way: told once done, not made again
	assert.False(t, sessionWarm(), "under way")
	close(gate)
	got, err := processSession(cfg, func(media.Config) (media.Session, error) {
		return nil, errors.New("not called: the warm-up's is waited for")
	}, t.Logf)
	require.NoError(t, err)
	assert.Same(t, want, got)
	assert.True(t, sessionWarm())
	for readies < 2 {
		q.drain()
	}
	warmSession(cfg, newSession, ready, t.Logf)
	q.drain()
	assert.Equal(t, 3, readies)
	assert.Equal(t, 1, made)
}

// TestWarmSessionFailure checks a warm-up that fails hands its error to
// the next processSession once, which a later one tries again.
func TestWarmSessionFailure(t *testing.T) { //nolint:paralleltest // the process's session
	resetProc(t)
	q := &uiQueue{}
	cfg := media.Config{Dispatch: func(fn func()) { q.enqueue(fn) }}
	done := make(chan struct{})
	warmSession(cfg, func(media.Config) (media.Session, error) { return nil, core.ErrUnsupported }, func() { close(done) }, t.Logf)
	for {
		q.drain()
		select {
		case <-done:
		default:
			continue
		}

		break
	}
	_, err := processSession(cfg, func(media.Config) (media.Session, error) { return &fake.Session{}, nil }, t.Logf)
	require.ErrorIs(t, err, core.ErrUnsupported)
	s, err := processSession(cfg, func(media.Config) (media.Session, error) { return &fake.Session{}, nil }, t.Logf)
	require.NoError(t, err)
	assert.NotNil(t, s)
}

// TestMakeMediaFallsBack checks a system without an engine plays on the
// fake, without a log, and an engine that fails is logged and replaced,
// its session kept.
func TestMakeMediaFallsBack(t *testing.T) {
	t.Parallel()

	cfg := media.Config{Dispatch: func(fn func()) { fn() }, Clock: clock.NewFake(t0)}
	tests := []struct {
		name   string
		newErr error
		logs   int
	}{
		{"no engine on this system", errNoEngine, 0},
		{"engine failed", core.ErrClosed, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var lines []string
			sess := &fake.Session{}
			pm := makeMedia(cfg, func(media.Config) (platformMedia, error) {
				return platformMedia{sess: sess}, tc.newErr
			}, func(f string, args ...any) { lines = append(lines, f) })
			assert.Equal(t, "fake", pm.name)
			assert.IsType(t, &fake.Engine{}, pm.eng)
			assert.Same(t, sess, pm.sess)
			assert.Len(t, lines, tc.logs)
		})
	}
}

// TestMaxResolution checks the limits of the qualities: the portrait
// variants of the lessons, none for auto.
func TestMaxResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		q    store.Quality
		w, h int
	}{
		{store.QualityAuto, 0, 0},
		{store.QualityUnknown, 0, 0},
		{store.QualityP1080, 1080, 1920},
		{store.QualityP720, 720, 1280},
		{store.QualityP480, 480, 854},
		{store.Quality(99), 0, 0},
	}
	for _, tc := range tests {
		w, h := maxResolution(tc.q)
		assert.Equal(t, [2]int{tc.w, tc.h}, [2]int{w, h}, "quality %d", tc.q)
	}
	assert.False(t, limitResolution(fake.New(media.Config{Clock: clock.NewFake(t0)}, fake.Options{}), 480, 854),
		"only the iPhone's engine is limited")
}
