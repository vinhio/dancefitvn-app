//go:build tvos

package shell_test

import (
	"testing"

	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Flows of 07 section 2 (UI TV) that the per-screen tests in screens/ and
// shell_test.go leave open. Already covered elsewhere, not repeated here:
//   - default focus of TV01, TV02, TV04, TV05, TV06, TV07, TV09, TV10, TV11,
//     TV12 and of TV08 (`o0`): TestDefaultFocusPerScreen,
//     TestMenuChainClosesPanelFirst (shell_test.go);
//   - Menu back chain, the panel first, Menu left to the system at a root:
//     TestMenuBackChain, TestMenuChainClosesPanelFirst, screens
//     TestCodeDefaultFocusAndBack, TestDetailBack, TestCoursesMenuIsLeftToTheSystem,
//     TestWelcomeLoginAndMenu, TestPlayerPanelKeepsTheArrowsAndMenuClosesIt;
//   - Play/Pause only with a handler: TestPlayPauseOnlyWithAHandler;
//   - toast timing: TestToastTiming (shell_test.go), tvkit TestToastTimingAndFade;
//     toast texts: screens TestWelcomeMoreToast, TestCodeErrors,
//     TestCodeLiveConfirmed, TestErrorRetry.

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestMenuPerScreen: Menu goes back on a screen that was opened over another
// (the stack pops to TV04 with its default focus), and is left to the system on
// a root screen (index.html:731, tvBack).
func TestMenuPerScreen(t *testing.T) {
	t.Parallel()
	roots := []struct{ route, focus string }{
		{env.RouteWelcome, "w1"}, {env.RouteHome, "h1"}, {env.RouteCourses, "cf0"},
		{env.RouteMusic, "mu0"}, {env.RouteSettings, "nav0"},
	}
	for _, tc := range roots {
		t.Run("root "+tc.route, func(t *testing.T) {
			t.Parallel()
			r := open(t, tc.route, false)
			require.Equal(t, tc.focus, r.key())
			assert.False(t, r.press(keys.KeyEscape), "Menu at a root is left to the system")
			assert.Equal(t, tc.route, r.sh.Stack().Screen())
			assert.Equal(t, tc.focus, r.key())
		})
	}
	pushed := []nav.Route{
		{Screen: env.RouteCode}, {Screen: env.RouteDetail, Params: "set-35"}, {Screen: env.RoutePlayer},
		{Screen: env.RouteUpNext}, {Screen: env.RouteError},
	}
	for _, rt := range pushed {
		t.Run("pushed "+rt.Screen, func(t *testing.T) {
			t.Parallel()
			r := open(t, env.RouteHome, false)
			r.sh.Go(rt, false)
			r.tt.Elapse(tvtest.Frame)
			r.tt.RunUntilIdle()
			r.tt.Elapse(tvtest.Frame)
			require.Equal(t, rt.Screen, r.sh.Stack().Screen())
			require.Equal(t, 1, r.sh.Stack().Depth())
			assert.True(t, r.press(keys.KeyEscape))
			assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
			assert.Zero(t, r.sh.Stack().Depth())
			assert.Equal(t, "h1", r.key(), "the screen below gets its default focus: no memory")
			assert.False(t, r.press(keys.KeyEscape), "and is a root")
			assert.Empty(t, r.tt.Leaks())
		})
	}
}

// TestBackspaceIsMenu: Backspace goes back like Menu (index.html:766).
func TestBackspaceIsMenu(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteHome, false)
	r.sh.Go(nav.Route{Screen: env.RouteError}, false)
	r.tt.Elapse(tvtest.Frame)
	assert.True(t, r.press(keys.KeyBackspace))
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
}

// TestPlayPausePerScreen: Play/Pause toggles the playback on the player and the
// music screens and is left to the system on every other (index.html:765).
func TestPlayPausePerScreen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		route string
		used  bool
	}{
		{env.RouteWelcome, false}, {env.RouteCode, false}, {env.RouteHome, false}, {env.RouteCourses, false},
		{env.RouteDetail, false}, {env.RoutePlayer, true}, {env.RouteMusic, true}, {env.RouteUpNext, false},
		{env.RouteError, false}, {env.RouteSettings, false},
	}
	for _, tc := range tests {
		t.Run(tc.route, func(t *testing.T) {
			t.Parallel()
			r := open(t, env.RouteHome, false)
			pb := r.svc.PB.(*tvtest.Playback)
			pb.PlayingV = false
			r.sh.Go(nav.Route{Screen: tc.route, Params: "set-35"}, false)
			r.tt.Elapse(tvtest.Frame)
			r.tt.RunUntilIdle()
			r.tt.Elapse(tvtest.Frame)
			pb.PlayingV = false // leaving the screens that are not players paused it
			assert.Equal(t, tc.used, r.press(keys.KeyMediaPlayPause))
			assert.Equal(t, tc.used, pb.Playing(), "one press toggles the playback where it is used")
			assert.Equal(t, tc.used, r.press(keys.KeySpace))
			assert.False(t, pb.Playing(), "the second press toggles it back")
		})
	}
}

// TestToastKeepsFocusAndSurvivesAChangeOfScreen: a toast is no focus target and
// does not take the keys; a change of screen leaves it on until its time is up,
// as the prototype's S.tv.toast outlives renderTV (index.html:732).
func TestToastKeepsFocusAndSurvivesAChangeOfScreen(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteHome, false)
	require.Equal(t, "h1", r.key())
	r.sh.Toast("Đã đăng nhập bằng thaovy.hlv@gmail.com")
	assert.True(t, r.sh.ToastHost().Visible())
	assert.Equal(t, "h1", r.key(), "TV-toast is captured with h1 focused")
	assert.Equal(t, 0, countFocused(r.sh.ToastHost()), "the toast is never drawn focused")

	r.sh.Go(nav.Route{Screen: env.RouteError}, false)
	r.tt.Elapse(tvtest.Frame)
	assert.True(t, r.sh.ToastHost().Visible(), "the toast stays over the new screen")
	assert.True(t, r.press(keys.KeyEscape), "Menu works under a toast")
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
	assert.Equal(t, "h1", r.key())

	r.tt.Elapse(tvkit.ToastTime)
	assert.False(t, r.sh.ToastHost().Visible())
	assert.Empty(t, r.tt.Leaks())
}

// TestUpNextBackToTheCourseStacksARoute: "Về khoá học" of TV09 is `data-tgo=
// "detail"`, which tvGo (index.html:730) pushes on the stack like any other
// route: the up-next screen stays under the course, so each lap of detail,
// player, up-next, detail adds two routes and Menu unwinds them one at a time.
// The random walk counts these laps (walk_test.go: lapDepth).
func TestUpNextBackToTheCourseStacksARoute(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteHome, false)
	settle := func() {
		r.tt.Elapse(tvtest.Frame)
		r.tt.RunUntilIdle()
		r.tt.Elapse(tvtest.Frame)
	}
	r.sh.Go(nav.Route{Screen: env.RouteDetail, Params: "set-35"}, false)
	settle()
	for lap := 1; lap <= 3; lap++ {
		r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false)
		settle()
		require.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
		r.svc.PB.(*tvtest.Playback).End()
		settle()
		require.Equal(t, env.RouteUpNext, r.sh.Stack().Screen(), "the lesson's end takes the player's place")
		require.Equal(t, 2*lap, r.sh.Stack().Depth(), "lap %d: the up-next screen replaced the player", lap)
		r.press(keys.KeyDown) // u1 "Xem ngay" to u2 "Về khoá học"
		require.Equal(t, "u2", r.key())
		r.press(keys.KeyEnter)
		settle()
		require.Equal(t, env.RouteDetail, r.sh.Stack().Screen())
		assert.Equal(t, 2*lap+1, r.sh.Stack().Depth(), "the up-next screen was pushed under the course")
	}
	for r.sh.Stack().Depth() > 0 {
		before := r.sh.Stack().Depth()
		require.True(t, r.press(keys.KeyEscape))
		assert.Equal(t, before-1, r.sh.Stack().Depth(), "Menu pops one route")
	}
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
}

// TestLeftPlayerDoesNotHearTheNetwork: only the player on the screen shows TV11
// when the network goes away (`$P:812`); a player that was left stopped its watch
// (the walk once met a stack growing by a route per left player).
func TestLeftPlayerDoesNotHearTheNetwork(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteHome, false)
	for range 3 {
		r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false)
		r.tt.Elapse(tvtest.Frame)
		r.tt.RunUntilIdle()
		require.True(t, r.press(keys.KeyEscape))
	}
	require.Zero(t, r.sh.Stack().Depth())
	r.svc.SetOnline(false)
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen(), "no screen is left to answer")
	assert.Zero(t, r.sh.Stack().Depth())

	r.svc.SetOnline(true)
	r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false)
	r.tt.Elapse(tvtest.Frame)
	r.tt.RunUntilIdle()
	r.svc.SetOnline(false)
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, env.RouteError, r.sh.Stack().Screen())
	assert.Equal(t, 2, r.sh.Stack().Depth(), "one TV11 over the player")
}
