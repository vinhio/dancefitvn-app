//go:build tvos

package screens_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/screens"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// socket is a bare container a test puts a screen in and takes it out of, to mount
// and detach it without the Shell.
type socket struct {
	ui.Base
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newSocket mounts an empty socket in a Tester.
func newSocket(t testing.TB) (*socket, *ui.Tester) {
	t.Helper()
	s := &socket{}

	return s, tvtest.Mount(t, s)
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout gives every child the whole socket.
func (s *socket) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	for w := range s.Children() {
		c.Layout(w, ui.Tight(size))
		c.Place(w, geom.Point{})
	}

	return size
}

// TestScreensReloadAfterALoadThatRanDetached: Mounted asks for its read with the
// next turn (AfterFunc); a screen taken out of the tree before that turn comes
// has its read start outside a view, where ui.Go starts nothing and returns a
// cancel that does nothing. The next Mounted must read then, whatever the cancel
// looks like: the screens once tested "cancel != nil" and never read again.
func TestScreensReloadAfterALoadThatRanDetached(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		build  func(e *env.Env) ui.Widget
		read   string
		loaded func(w ui.Widget) bool
	}{
		{"detail", func(e *env.Env) ui.Widget { return screens.NewDetail(e, "set-35") }, "Course",
			func(w ui.Widget) bool { return w.(*screens.Detail).Loaded() }},
		{"home", func(e *env.Env) ui.Widget { return screens.NewHome(e) }, "Home",
			func(w ui.Widget) bool { return w.(*screens.Home).Loaded() }},
		{"courses", func(e *env.Env) ui.Widget { return screens.NewCourses(e) }, "Courses",
			func(w ui.Widget) bool { return w.(*screens.Courses).Loaded() }},
		{"settings", func(e *env.Env) ui.Widget { return screens.NewSettings(e) }, "Me",
			func(w ui.Widget) bool { return w.(*screens.Settings).Loaded() }},
		{"music", func(e *env.Env) ui.Widget { return screens.NewMusic(e) }, "Home",
			func(w ui.Widget) bool { return w.(*screens.Music).Loaded() }},
		{"error", func(e *env.Env) ui.Widget { return screens.NewError(e) }, "Config",
			func(w ui.Widget) bool { return strings.Contains(w.(*screens.Error).CodeText(), "036 800 8268") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := tvtest.NewPlayerServices(t)
			if tc.name == "music" { // nothing loaded: the home's lesson is what it reads
				svc.PB = &tvtest.Playback{}
			}
			sock, tt := newSocket(t)
			w := tc.build(env.New(svc, texts()))

			sock.Add(w) // Mounted asks for the read with the next turn
			sock.Remove(w)
			tt.Elapse(tvtest.Frame) // the turn comes with no view
			tt.RunUntilIdle()
			assert.Zero(t, svc.Reads(tc.read), "a detached screen reads nothing")
			assert.False(t, tc.loaded(w))

			sock.Add(w)
			tt.Elapse(tvtest.Frame)
			tt.RunUntilIdle()
			tt.Elapse(tvtest.Frame)
			assert.Equal(t, 1, svc.Reads(tc.read), "mounted again, it reads")
			assert.True(t, tc.loaded(w))

			// and a screen that has its content reads no more
			sock.Remove(w)
			sock.Add(w)
			tt.Elapse(tvtest.Frame)
			tt.RunUntilIdle()
			assert.Equal(t, 1, svc.Reads(tc.read), "a loaded screen mounted again reads nothing")
			require.Empty(t, tt.Leaks())
		})
	}
}

// TestScreenLeftWhileReadingReadsAgain: leaving a screen with its read in flight
// cancels it; the screen mounted again reads again, once.
func TestScreenLeftWhileReadingReadsAgain(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewSignedServices(t)
	sock, tt := newSocket(t)
	w := screens.NewDetail(env.New(svc, texts()), "set-35")

	sock.Add(w)
	tt.Elapse(tvtest.Frame) // the read starts off the UI thread
	sock.Remove(w)          // its answer is dropped
	tt.RunUntilIdle()
	assert.False(t, w.Loaded())

	sock.Add(w)
	tt.Elapse(tvtest.Frame)
	tt.RunUntilIdle()
	tt.Elapse(tvtest.Frame)
	assert.True(t, w.Loaded())
	assert.Equal(t, 2, svc.Reads("Course"), "the dropped read and the new one")
}
