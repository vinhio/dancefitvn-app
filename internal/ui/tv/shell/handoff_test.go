//go:build tvos

package shell_test

import (
	"testing"

	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/shell"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// looker is what an Item draws its focus look from: the flag the fill and the ink
// follow, and the scale and the shadow that grow over Transition.
type looker interface {
	IsFocused() bool
	Scale() float32
	Blend() float32
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestChangeOfScreenAsksForItsFrame: the focus of a new screen is given by the
// next Layout, and Go, Replace, Back and SwitchTab ask for a frame, so that a
// route change made by an event with no frame after it (the network flip that
// shows TV11, the end of a video) has its focus as soon as the display's next
// frame runs. The test runs only the frames that were asked for (Tester.Frame),
// never Elapse, which runs one at every refresh, with the motion reduced so that no
// animation of an earlier change has a frame pending that would stand for the
// one the change owes.
func TestChangeOfScreenAsksForItsFrame(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		from string
		act  func(r *rig)
		want string
	}{
		{"Go", env.RouteHome, func(r *rig) { r.sh.Go(nav.Route{Screen: env.RouteError}, false) }, "e1"},
		{"Go root", env.RouteHome, func(r *rig) { r.sh.Go(nav.Route{Screen: env.RouteCourses}, true) }, "cf0"},
		{"Replace", env.RoutePlayer, func(r *rig) { r.sh.Replace(nav.Route{Screen: env.RouteUpNext}) }, "u1"},
		{"SwitchTab", env.RouteHome, func(r *rig) { r.sh.SwitchTab(3) }, "nav3"},
		{"Back", env.RouteHome, func(r *rig) {
			r.sh.Go(nav.Route{Screen: env.RouteError}, false)
			r.tt.Frame()
			r.sh.Back()
			r.tt.Elapse(tvtest.Frame) // the screen left had its own frame pending (paced): it stands for the one Back owes
		}, "h1"},
		{"network flip on the player", env.RoutePlayer, func(r *rig) { r.svc.SetOnline(false) }, "e1"},
		{"video end", env.RoutePlayer, func(r *rig) {
			r.svc.PB.(*tvtest.Playback).End()
		}, "u1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := open(t, tc.from, false)
			r.tt.View().Context().Theme().ReduceMotion = true
			r.tt.Elapse(tvkit.Transition) // idle: nothing is due
			tc.act(r)
			r.tt.Frame() // what was asked for, no refresh passes
			assert.Equal(t, tc.want, r.key())
			require.NotNil(t, r.sh.Manager().Current())
		})
	}
}

// TestFocusLandsOnTheControlWhenTheDataComes: a screen shows the nav bar alone
// while its data is read, the focus on a stand-in (K12); when the data comes the
// focus goes to the control it stood for and shows its look, whenever the answer
// comes: at once, after seconds, or after the user moved to the nav bar (which
// then keeps it). Menu back to TV04 is the case the remote showed once with no look.
func TestFocusLandsOnTheControlWhenTheDataComes(t *testing.T) {
	t.Parallel()
	routes := []struct {
		name  string
		route nav.Route
		root  bool
		key   string
	}{
		{"home", nav.Route{Screen: env.RouteHome}, true, "h1"},
		{"detail", nav.Route{Screen: env.RouteDetail, Params: "set-35"}, false, "d1"},
		{"courses", nav.Route{Screen: env.RouteCourses}, true, "cf0"},
	}
	waits := []struct {
		name string
		wait func(r *rig)
	}{
		{"at once", func(r *rig) {}},
		{"one frame", func(r *rig) { r.tt.Elapse(tvtest.Frame) }},
		{"600 ms", func(r *rig) { r.tt.Elapse(600 * 1e6) }},
		{"2 s", func(r *rig) { r.tt.Elapse(2000 * 1e6) }},
	}
	for _, rt := range routes {
		for _, w := range waits {
			t.Run(rt.name+" after "+w.name, func(t *testing.T) {
				t.Parallel()
				r := open(t, env.RouteError, false) // something to go back to
				r.sh.Go(nav.Route{Screen: env.RouteSettings}, true)
				r.tt.Elapse(tvtest.Frame)
				release := r.svc.Hold()
				if rt.root {
					r.sh.Go(rt.route, true)
				} else {
					r.sh.Go(rt.route, false)
				}
				r.tt.Elapse(tvtest.Frame) // the stand-in takes the focus
				assert.Equal(t, rt.key, r.key(), "the stand-in has the key of the control")
				w.wait(r)
				release()
				r.tt.RunUntilIdle()
				r.tt.Elapse(tvtest.Frame)
				r.tt.Elapse(tvkit.Transition + tvtest.Frame)
				assertLook(t, r, rt.key)
			})
		}
	}
}

// TestMenuBackToHomeShowsTheFocusLook: TV04 -> TV06 -> Menu: the home is built
// again, reads its data and `h1` has its look 600 ms after, as on TV04 first shown.
func TestMenuBackToHomeShowsTheFocusLook(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteHome, false)
	require.Equal(t, "h1", r.key())
	r.sh.Go(nav.Route{Screen: env.RouteDetail, Params: "set-35"}, false)
	r.tt.Elapse(tvtest.Frame)
	r.tt.RunUntilIdle()
	r.tt.Elapse(tvkit.Transition)
	require.Equal(t, "d1", r.key())

	release := r.svc.Hold()
	assert.True(t, r.tt.Key(keys.KeyEscape, 0)) // Menu
	r.tt.Elapse(tvtest.Frame)
	r.tt.Elapse(300 * 1e6) // the data is slow
	release()
	r.tt.RunUntilIdle()
	r.tt.Elapse(300 * 1e6)
	assertLook(t, r, "h1")
}

// TestMovedFocusStaysWhenTheDataComes: Up from the stand-in goes to the nav bar,
// and the data coming after it leaves the focus there.
func TestMovedFocusStaysWhenTheDataComes(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteSettings, false)
	release := r.svc.Hold()
	r.sh.Go(nav.Route{Screen: env.RouteHome}, true)
	r.tt.Elapse(tvtest.Frame)
	require.Equal(t, "h1", r.key(), "the stand-in")
	r.press(keys.KeyUp)
	moved := r.key()
	require.Contains(t, moved, "nav", "Up from the stand-in goes to the nav bar")
	release()
	r.tt.RunUntilIdle()
	r.tt.Elapse(tvkit.Transition + tvtest.Frame)
	assertLook(t, r, moved)
	assert.Positive(t, r.svc.Reads("Home"))
}

// TestShellNewWithoutAViewAsksForNothing: Go before the view exists (the first
// route, shown before the app has a window) does not need a frame: the mount asks
// for the first.
func TestShellNewWithoutAViewAsksForNothing(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	sh := shell.New(svc, texts())
	require.NotPanics(t, func() { sh.Go(nav.Route{Screen: env.RouteHome}, true) })
	tt := tvtest.Mount(t, sh)
	tt.Frame()
	require.NotNil(t, sh.Manager().Current())
	assert.Equal(t, "h1", sh.Manager().Current().FocusKey())
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// assertLook asserts that the focus is on the control with key, drawn with its
// focused look: the flag, the grown scale and the shadow, on a real box.
func assertLook(t *testing.T, r *rig, key string) {
	t.Helper()
	it := r.sh.Manager().Current()
	require.NotNil(t, it, "something has the focus")
	require.Equal(t, key, it.FocusKey())
	lk, ok := it.(looker)
	require.True(t, ok)
	assert.True(t, lk.IsFocused())
	assert.Greater(t, lk.Scale(), float32(1.01), "the scale grew")
	assert.InDelta(t, 1, lk.Blend(), 0.001, "the shadow is in")
	f := it.(ui.Widget).Node().WindowFrame()
	assert.Greater(t, f.W, float32(1), "on the control, not on a stand-in")
	assert.False(t, f.Intersect(geom.XYWH(0, 0, tvtest.Width, tvtest.Height)).Empty())
	assert.Empty(t, r.tt.Leaks())
}
