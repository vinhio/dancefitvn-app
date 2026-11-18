//go:build tvos

package shell_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/focus"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/screens"
	"dancefitvn-app/internal/ui/tv/shell"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// rig is a Shell mounted in a Tester at 1920x1080.
type rig struct {
	t   testing.TB
	sh  *shell.Shell
	tt  *ui.Tester
	svc *tvtest.Services
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// open mounts a Shell and goes to route as a root, like the app does after
// the session is read.
func open(t testing.TB, route string, real bool) *rig {
	t.Helper()
	svc := tvtest.NewPlayerServices(t)
	sh := shell.New(svc, texts())
	sh.Go(nav.Route{Screen: route}, true)
	var tt *ui.Tester
	if real {
		tt = tvtest.MountReal(t, sh)
	} else {
		tt = tvtest.Mount(t, sh)
	}
	tvtest.PinMotion(tt)
	tt.Elapse(tvtest.Frame)
	tt.RunUntilIdle() // the reads of the screen
	tt.Elapse(tvtest.Frame)

	return &rig{t: t, sh: sh, tt: tt, svc: svc}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// key is the key of the focused item, "" when nothing is focused.
func (r *rig) key() string {
	r.t.Helper()
	if it := r.sh.Manager().Current(); it != nil {
		return it.FocusKey()
	}

	return ""
}

// press sends a key (press and release) and lets the frames run.
func (r *rig) press(k keys.Key) bool {
	r.t.Helper()
	used := r.tt.Key(k, 0)
	r.tt.Elapse(tvtest.Frame)

	return used
}

// welcome returns the welcome screen showing.
func (r *rig) welcome() *screens.Welcome {
	r.t.Helper()
	w, ok := r.sh.Screen().(*screens.Welcome)
	require.True(r.t, ok)

	return w
}

// navOn returns the item that is on in the nav bar of the screen showing, -1
// when the screen has no nav bar.
func (r *rig) navOn() int {
	r.t.Helper()
	switch sc := r.sh.Screen().(type) {
	case *screens.Home:
		return sc.Nav().On
	case *screens.Courses:
		return sc.Nav().On
	case *screens.Music:
		return sc.Nav().On
	case *screens.Settings:
		return sc.Nav().On
	}

	return -1
}

// TestDefaultFocusPerScreen: after a change of screen the focus is the first
// `.f` that is not in the nav bar, and `nav0` on the settings (index.html:723,
// F-7 of local/plans/P7.md).
func TestDefaultFocusPerScreen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		route string
		want  string
	}{
		{env.RouteWelcome, "w1"},
		{env.RouteCode, "c2"},
		{env.RouteHome, "h1"},
		{env.RouteCourses, "cf0"},
		{env.RouteDetail, "d1"},
		{env.RoutePlayer, "p1"},
		{env.RouteMusic, "mu0"},
		{env.RouteUpNext, "u1"},
		{env.RouteError, "e1"},
		{env.RouteSettings, "nav0"},
	}
	for _, tc := range tests {
		t.Run(tc.route, func(t *testing.T) {
			t.Parallel()
			r := open(t, tc.route, false)
			assert.Equal(t, tc.want, r.key())
			assert.Empty(t, r.tt.Leaks())
		})
	}
}

// TestNavFocusMoves walks the nav bar with the geometry of the real layout:
// Right goes from item to item, Down leaves the bar for the control under it
// and Up comes back to an item.
func TestNavFocusMoves(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteSettings, true)
	require.Equal(t, "nav0", r.key())
	for _, want := range []string{"nav1", "nav2", "nav3"} {
		assert.True(t, r.press(keys.KeyRight))
		assert.Equal(t, want, r.key())
	}
	for _, want := range []string{"nav2", "nav1", "nav0"} {
		assert.True(t, r.press(keys.KeyLeft))
		assert.Equal(t, want, r.key())
	}
	assert.True(t, r.press(keys.KeyDown))
	assert.Contains(t, []string{"s0", "s1", "s2", "s3", "s4", "s5"}, r.key())
	assert.True(t, r.press(keys.KeyUp))
	assert.Contains(t, []string{"nav0", "nav1", "nav2", "nav3"}, r.key())
}

// TestNavSelectSwitchesInPlace: Select on a nav item shows that screen with an
// empty stack and keeps the focus on the item (index.html:735-736).
func TestNavSelectSwitchesInPlace(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteHome, false)
	require.Equal(t, "h1", r.key())
	r.sh.Go(nav.Route{Screen: env.RouteDetail}, false)
	r.tt.Elapse(tvtest.Frame)
	require.Equal(t, 1, r.sh.Stack().Depth())
	r.sh.SwitchTab(0)
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
	assert.Zero(t, r.sh.Stack().Depth())

	assert.Equal(t, "nav0", r.key(), "the focus stays on the item that was selected")
	require.True(t, r.press(keys.KeyRight))
	require.Equal(t, "nav1", r.key())
	assert.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, env.RouteCourses, r.sh.Stack().Screen())
	assert.Zero(t, r.sh.Stack().Depth())
	assert.Equal(t, "nav1", r.key(), "the focus stays on the item")
	assert.Equal(t, 1, r.navOn())
	r.sh.SwitchTab(3)
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, "nav3", r.key())
	assert.Equal(t, 3, r.navOn())
	r.sh.SwitchTab(9) // out of range: nothing
	assert.Equal(t, env.RouteSettings, r.sh.Stack().Screen())
}

// TestMenuBackChain: Select on w1 opens the code screen (stack 1, focus c2);
// Menu goes back to the welcome; Menu at a root screen is not used, so the
// system takes it (the Apple TV's Home).
func TestMenuBackChain(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteWelcome, false).armed()
	require.Equal(t, "w1", r.key())
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, env.RouteCode, r.sh.Stack().Screen())
	assert.Equal(t, 1, r.sh.Stack().Depth())
	assert.Equal(t, "c2", r.key())

	assert.True(t, r.press(keys.KeyEscape))
	assert.Equal(t, env.RouteWelcome, r.sh.Stack().Screen())
	assert.Equal(t, "w1", r.key())
	assert.False(t, r.press(keys.KeyEscape), "Menu at a root screen is left to the system")
	assert.False(t, r.press(keys.KeyBackspace))
	assert.Equal(t, env.RouteWelcome, r.sh.Stack().Screen())
}

// TestMenuChainClosesPanelFirst: with the player's panel open Menu closes it
// and focuses p2, then goes back (index.html:731); the arrows never leave the
// panel while it is open.
func TestMenuChainClosesPanelFirst(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteHome, false)
	r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false)
	r.tt.Elapse(tvtest.Frame)
	require.Equal(t, "p1", r.key())
	require.True(t, r.press(keys.KeyRight))
	require.Equal(t, "p2", r.key())

	require.True(t, r.press(keys.KeyEnter))
	assert.True(t, r.sh.Stack().Panel())
	assert.Equal(t, "o0", r.key(), "the panel opens on its first item")
	for _, k := range []keys.Key{keys.KeyLeft, keys.KeyRight, keys.KeyUp, keys.KeyUp, keys.KeyDown, keys.KeyDown, keys.KeyDown, keys.KeyDown, keys.KeyDown, keys.KeyDown, keys.KeyDown, keys.KeyDown, keys.KeyDown} {
		r.press(k)
		k := r.key()
		assert.Contains(t, []string{"o0", "o1", "o2", "o3", "ot0", "ot1", "ot2", "oc"}, k, "the focus stays in the panel")
	}

	assert.True(t, r.press(keys.KeyEscape))
	assert.False(t, r.sh.Stack().Panel())
	assert.Equal(t, "p2", r.key(), "closing the panel focuses p2")
	assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen(), "the first Menu closed the panel only")
	assert.True(t, r.press(keys.KeyEscape))
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
	assert.False(t, r.press(keys.KeyEscape))
}

// TestPlayPauseOnlyWithAHandler: Play/Pause is used only while a handler is
// registered (Q-P7-18); the player and music stubs register one.
func TestPlayPauseOnlyWithAHandler(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteHome, false)
	assert.False(t, r.press(keys.KeyMediaPlayPause), "home: left to the system")

	calls := 0
	remove := r.sh.OnPlayPause(func() { calls++ })
	assert.True(t, r.press(keys.KeyMediaPlayPause))
	assert.True(t, r.press(keys.KeySpace))
	assert.Equal(t, 2, calls)
	remove()
	assert.False(t, r.press(keys.KeyMediaPlayPause))

	r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false)
	r.tt.Elapse(tvtest.Frame)
	assert.True(t, r.press(keys.KeyMediaPlayPause), "the player's stub has a handler")
	r.sh.Go(nav.Route{Screen: env.RouteDetail}, false)
	r.tt.Elapse(tvtest.Frame)
	assert.False(t, r.press(keys.KeyMediaPlayPause), "its handler left with it")
	assert.Empty(t, r.tt.Leaks())
}

// TestOnKeyHookTakesButtonsUntilRemoved: the hook sees every button before the focus
// does and a button it takes does nothing else; a stale remove leaves a newer hook
// alone (K46: a player opened over a player).
func TestOnKeyHookTakesButtonsUntilRemoved(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteHome, false)
	var seen []focus.Button
	remove := r.sh.OnKey(func(b focus.Button) bool { seen = append(seen, b); return b == focus.ButtonRight })
	before := r.key()
	assert.True(t, r.press(keys.KeyRight), "taken")
	assert.Equal(t, before, r.key(), "the focus did not move")
	r.press(keys.KeyDown)
	assert.Equal(t, focus.ButtonRight, seen[0], "Right came once and was taken")
	assert.Contains(t, seen[1:], focus.ButtonDown, "Down went on to the focus") // an untaken button is asked by the Shell and by the Root

	var second int
	remove2 := r.sh.OnKey(func(focus.Button) bool { second++; return false })
	remove() // the first screen leaving after the second took the hook
	r.press(keys.KeyDown)
	assert.Positive(t, second, "the newer hook stays")
	remove2()
	n := second
	r.press(keys.KeyDown)
	assert.Equal(t, n, second, "removed")
}

// TestSelectOnReleaseAndHold: a short press activates on release (K27); a hold
// activates once at 550 ms and the release then does nothing.
func TestSelectOnReleaseAndHold(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteWelcome, false).armed()
	w, ok := r.sh.Screen().(*screens.Welcome)
	require.True(t, ok)
	w1 := w.Item("w1")
	taps := 0
	w1.OnSelect = func() { taps++ }

	r.tt.KeyDown(keys.KeyEnter, 0)
	assert.Zero(t, taps, "nothing on the press")
	r.tt.Elapse(100 * time.Millisecond)
	r.tt.KeyUp(keys.KeyEnter, 0)
	assert.Equal(t, 1, taps, "activated on release")

	r.tt.KeyDown(keys.KeyEnter, 0)
	r.tt.Elapse(focus.HoldDuration - time.Millisecond)
	assert.Equal(t, 1, taps)
	r.tt.Elapse(time.Millisecond)
	assert.Equal(t, 2, taps, "the hold activates at 550 ms")
	r.tt.KeyRepeat(keys.KeyEnter, 0)
	r.tt.Elapse(time.Second)
	r.tt.KeyUp(keys.KeyEnter, 0)
	assert.Equal(t, 2, taps, "once: the repeats and the release do nothing")

	// a control with a hold of its own runs it, not the tap
	holds := 0
	w1.OnHold = func() { holds++ }
	r.tt.KeyDown(keys.KeyEnter, 0)
	r.tt.Elapse(focus.HoldDuration)
	r.tt.KeyUp(keys.KeyEnter, 0)
	assert.Equal(t, 1, holds)
	assert.Equal(t, 2, taps)
}

// TestToastTiming: a toast shows for 2400 ms with a 200 ms fade in, no fade
// out; a second message restarts the time without a new fade (K17).
func TestToastTiming(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteHome, false)
	toast := r.sh.ToastHost()
	assert.False(t, toast.Visible())

	r.sh.Toast("Đã đăng nhập bằng thaovy.hlv@gmail.com")
	assert.True(t, toast.Visible())
	assert.Equal(t, "Đã đăng nhập bằng thaovy.hlv@gmail.com", toast.Message())
	r.tt.Elapse(2399 * time.Millisecond)
	assert.True(t, toast.Visible(), "2399 ms")
	r.tt.Elapse(time.Millisecond)
	assert.False(t, toast.Visible(), "2400 ms: gone")

	r.sh.Toast("a")
	r.tt.Elapse(2000 * time.Millisecond)
	r.sh.Toast("b")
	assert.Equal(t, "b", toast.Message())
	r.tt.Elapse(2399 * time.Millisecond)
	assert.True(t, toast.Visible(), "2399 ms after the second")
	r.tt.Elapse(time.Millisecond)
	assert.False(t, toast.Visible())
	assert.Equal(t, []string{"Đã đăng nhập bằng thaovy.hlv@gmail.com", "a", "b"}, r.tt.Announcements())
	_ = tvkit.ToastTime
}

// TestGoPausesPlayback: leaving for a screen that is not the player or the
// music pauses, a Back always does (index.html:730-731).
func TestGoPausesPlayback(t *testing.T) {
	t.Parallel()
	r := open(t, env.RouteHome, false)
	pb := &tvtest.Playback{}
	r.svc.PB = pb
	pb.Start()
	r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false)
	assert.True(t, pb.Playing(), "the player keeps playing")
	r.sh.Go(nav.Route{Screen: env.RouteMusic}, false)
	assert.True(t, pb.Playing(), "so does the music")
	r.sh.Go(nav.Route{Screen: env.RouteDetail}, false)
	assert.False(t, pb.Playing(), "a course screen pauses")
	pb.Start()
	require.True(t, r.sh.Back())
	assert.False(t, pb.Playing(), "Back pauses")
	pb.Start()
	r.sh.SwitchTab(2) // music keeps playing
	assert.True(t, pb.Playing())
	r.sh.SwitchTab(0)
	assert.False(t, pb.Playing())
}

// TestUnknownRouteIsTheWelcome: a route nobody knows shows the welcome screen.
func TestUnknownRouteIsTheWelcome(t *testing.T) {
	t.Parallel()
	r := open(t, "nowhere", false)
	r.welcome()
	assert.Equal(t, "w1", r.key())
}

// TestShellSetsTheTextShiftAtOneX: mounting at the TV's 1x sets the text shift of
// the kit to the measured -0.1; the phones' default is -0.5.
func TestShellSetsTheTextShiftAtOneX(t *testing.T) {
	t.Parallel()
	assert.InDelta(t, -0.5, float64(kit.DefaultTextShift), 0)
	open(t, env.RouteWelcome, false)
	assert.InDelta(t, float64(shell.TextShift1x), float64(kit.TextShift()), 0)
	assert.InDelta(t, -0.1, float64(shell.TextShift1x), 1e-6)
}

// armed lets the welcome screen's guard of the first Select (450 ms) run out.
func (r *rig) armed() *rig {
	r.t.Helper()
	r.tt.Elapse(500 * time.Millisecond)

	return r
}
