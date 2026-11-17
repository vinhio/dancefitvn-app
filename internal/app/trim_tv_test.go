//go:build tvos

package app

import (
	"testing"

	tvenv "dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/shell"
	"dancefitvn-app/internal/ui/tv/tvtest"
	"dancefitvn-app/locales"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestTVMemoryGivesBackWhenSomethingGoes: a screen left, the panel closed
// and a toast gone each give the memory back, off the UI thread; what
// stays on screen does not.
func TestTVMemoryGivesBackWhenSomethingGoes(t *testing.T) {
	t.Parallel()
	r := signedInTV(t)
	m, done := watchedMemory(r)
	m.after()
	m.after()
	assert.Zero(t, m.frees.Load(), "the first screen and frames that change nothing")

	steps := []struct {
		name string
		act  func()
	}{
		{"a screen left", func() { r.sh.Go(nav.Route{Screen: tvenv.RouteCourses}, false) }},
		{"the panel closed", func() {
			r.sh.Stack().SetPanel(true)
			m.after()
			r.sh.Stack().SetPanel(false)
		}},
		{"a toast gone", func() {
			r.sh.Toast("toast")
			m.after()
			r.sh.ToastHost().Hide()
		}},
		{"back to the screen before", func() { r.sh.Back() }},
	}
	for i, s := range steps {
		s.act()
		m.after()
		<-done
		assert.Equal(t, int64(i+1), m.frees.Load(), s.name)
		m.after()
		assert.Equal(t, int64(i+1), m.frees.Load(), "%s: once", s.name)
	}
}

// TestTVMemoryOneAtATime: what goes while the memory is being given back
// starts no second one.
func TestTVMemoryOneAtATime(t *testing.T) {
	t.Parallel()
	r := signedInTV(t)
	m := newTVMemory(r.sh)
	release, done := make(chan struct{}), make(chan struct{})
	m.free = func() {
		<-release
		done <- struct{}{}
	}
	m.after()
	r.sh.Go(nav.Route{Screen: tvenv.RouteCourses}, true)
	m.after()
	r.sh.Go(nav.Route{Screen: tvenv.RouteSettings}, true)
	m.after()
	assert.Equal(t, int64(1), m.frees.Load(), "one in flight")
	close(release)
	<-done
	for m.freeing.Load() { // the goroutine's defer, right after done
		r.q.drain()
	}
	r.sh.Go(nav.Route{Screen: tvenv.RouteHome}, true)
	m.after()
	<-done
	assert.Equal(t, int64(2), m.frees.Load(), "the next one once it is over")
}

// TestTVMemoryAfterFrameDoesNotAllocate: the look after each frame
// allocates nothing.
func TestTVMemoryAfterFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	r := signedInTV(t)
	m, _ := watchedMemory(r)
	m.after()
	allocs := testing.AllocsPerRun(1000, m.after)
	assert.Zero(t, allocs)
}

// BenchmarkTVMemoryAfterFrame measures the look after each frame.
func BenchmarkTVMemoryAfterFrame(b *testing.B) {
	sh := shell.New(tvtest.NewServices(), locales.MustTexts())
	m := newTVMemory(sh)
	m.free = func() {}
	b.ReportAllocs()
	for b.Loop() {
		m.after()
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// watchedMemory returns the watcher of r's shell, its giving back told on
// done.
func watchedMemory(r *tvRig) (*tvMemory, chan struct{}) {
	m := newTVMemory(r.sh)
	done := make(chan struct{}, 1)
	m.free = func() { done <- struct{}{} }
	require.NotNil(r.t, m.sh)

	return m, done
}
