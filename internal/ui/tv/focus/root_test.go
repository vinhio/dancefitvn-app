//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/root_test.go at 781bee5.

package focus_test

import (
	"testing"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/ui/tv/focus"
)

func TestRootMapsKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		key  keys.Key
		mods keys.Modifiers
		// wantUsed is whether Tester.Key reports the press as used.
		wantUsed bool
		menus    int
		plays    int
	}{
		{"escape is menu", keys.KeyEscape, 0, true, 1, 0},
		{"backspace is menu", keys.KeyBackspace, 0, true, 1, 0},
		{"space is play/pause", keys.KeySpace, 0, true, 0, 1},
		{"p is play/pause", keys.KeyP, 0, true, 0, 1},
		{"media play/pause", keys.KeyMediaPlayPause, 0, true, 0, 1},
		{"shift+p is still p", keys.KeyP, keys.ModShift, true, 0, 1},
		{"h is the system's", keys.KeyH, 0, false, 0, 0},
		{"shift+s is not Siri", keys.KeyS, keys.ModShift, false, 0, 0},
		{"ctrl+p is not play/pause", keys.KeyP, keys.ModCtrl, false, 0, 0},
		{"cmd+escape is not menu", keys.KeyEscape, keys.ModSuper, false, 0, 0},
		{"alt+backspace is not menu", keys.KeyBackspace, keys.ModAlt, false, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			menus, plays := 0, 0
			e.m.OnMenu(func() bool { menus++; return true })
			e.m.OnPlayPause(func() { plays++ })

			assert.Equal(t, tt.wantUsed, e.tt.Key(tt.key, tt.mods))

			assert.Equal(t, tt.menus, menus)
			assert.Equal(t, tt.plays, plays)
		})
	}
}

func TestRootReleasesAreNotTaken(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	menus := 0
	e.m.OnMenu(func() bool { menus++; return true })

	assert.False(t, e.up(keys.KeyEscape), "the release of Escape is unhandled")
	assert.False(t, e.up(keys.KeyLeft))
	assert.False(t, e.up(keys.KeySpace))
	assert.Zero(t, menus)
	assert.True(t, e.down(keys.KeyEscape))
	assert.Equal(t, 1, menus)
}

func TestRootMenuUnhandledPassesOn(t *testing.T) {
	t.Parallel()
	e := newEnv(t)

	assert.False(t, e.tt.Key(keys.KeyEscape, 0), "Menu goes to the system")
}

func TestRootArrowsAreLeftToGDeskUnlessPreTakesThem(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.focusOn("a1")

	assert.False(t, e.tt.Key(keys.KeyRight, 0), "no Pre: the focused widget and gDesk move it")
	assert.Zero(t, e.eng.moves, "the Root does not move the focus itself")

	var seen []focus.Button
	e.m.SetPre(func(b focus.Button) bool { seen = append(seen, b); return b == focus.ButtonLeft })
	assert.True(t, e.tt.Key(keys.KeyLeft, 0), "Pre took it")
	assert.False(t, e.tt.Key(keys.KeyDown, 0))
	assert.Equal(t, []focus.Button{focus.ButtonLeft, focus.ButtonDown}, seen)
}

// TestRootArrowRepeatReachesPreUnlessItTookThePress: the repeats of an arrow
// go to Pre, but not those of a press Pre took: the press is over for
// the app, and a release or a new press ends it.
func TestRootArrowRepeatReachesPreUnlessItTookThePress(t *testing.T) {
	t.Parallel()

	t.Run("declined", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		calls := 0
		e.m.SetPre(func(focus.Button) bool { calls++; return false })

		assert.False(t, e.down(keys.KeyRight))
		assert.False(t, e.repeat(keys.KeyRight))
		assert.False(t, e.repeat(keys.KeyRight))

		assert.Equal(t, 3, calls, "every repeat of a press Pre did not take is asked")
	})
	t.Run("taken", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		calls := 0
		e.m.SetPre(func(focus.Button) bool { calls++; return true })

		assert.True(t, e.down(keys.KeyRight))
		assert.True(t, e.repeat(keys.KeyRight), "swallowed")
		assert.True(t, e.repeat(keys.KeyRight), "swallowed")
		assert.Equal(t, 1, calls, "Pre was asked once")

		assert.False(t, e.up(keys.KeyRight), "a release is not the Root's")
		assert.True(t, e.down(keys.KeyRight), "a new press is asked")
		assert.Equal(t, 2, calls)
		assert.True(t, e.down(keys.KeyLeft), "another button is asked")
		assert.True(t, e.repeat(keys.KeyRight), "a repeat of the earlier press than the last down is asked again")
		assert.Equal(t, 4, calls)
	})
	t.Run("menu", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		menus, calls := 0, 0
		e.m.OnMenu(func() bool { menus++; return true })
		e.m.SetPre(func(focus.Button) bool { calls++; return true })

		assert.True(t, e.down(keys.KeyEscape))
		assert.True(t, e.repeat(keys.KeyEscape))

		assert.Equal(t, 1, calls)
		assert.Zero(t, menus, "taken by Pre")
	})
}

func TestRootArrowHealsFocus(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	require.Nil(t, e.eng.focused)

	e.tt.Key(keys.KeyDown, 0)

	assert.Same(t, e.it["a1"], e.eng.focused)
}

func TestRootSelectHoldAndRepeat(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a1 := e.focusOn("a1")
	a1.holdHandled = true

	assert.True(t, e.down(keys.KeyEnter))
	assert.True(t, e.repeat(keys.KeyEnter), "repeats are swallowed")
	e.tt.Elapse(focus.HoldDuration)
	assert.True(t, e.up(keys.KeyEnter))

	assert.Equal(t, []bool{true}, a1.acts)
	assert.Equal(t, []bool{true, false}, a1.pressing)
}

func TestRootSelectShortPress(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a1 := e.focusOn("a1")

	assert.True(t, e.tt.Key(keys.KeyEnter, 0))

	assert.Equal(t, []bool{false}, a1.acts)
}

// TestRootPreSeesSelectDown: Pre is asked when Select goes down, once;
// a Select it takes does not select, hold or show .pressing.
func TestRootPreSeesSelectDown(t *testing.T) {
	t.Parallel()

	t.Run("declined", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.focusOn("a1")
		var seen []focus.Button
		e.m.SetPre(func(b focus.Button) bool { seen = append(seen, b); return false })

		e.tt.Key(keys.KeyEnter, 0)

		assert.Equal(t, []focus.Button{focus.ButtonSelect}, seen)
		assert.Equal(t, []bool{false}, e.it["a1"].acts)
	})
	t.Run("taken", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		a1 := e.focusOn("a1")
		a1.holdHandled = true
		calls := 0
		e.m.SetPre(func(focus.Button) bool { calls++; return true })

		assert.True(t, e.down(keys.KeyEnter))
		assert.True(t, e.repeat(keys.KeyEnter))
		e.tt.Elapse(2 * focus.HoldDuration)
		assert.True(t, e.up(keys.KeyEnter))

		assert.Equal(t, 1, calls, "asked once for the whole press")
		assert.Empty(t, a1.acts, "neither the hold nor the select")
		assert.Empty(t, a1.pressing, "no .pressing")
		assert.Zero(t, a1.pulses)
	})
}

func TestRootFillsAndLaysOutContent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)

	e.tt.Frame()

	assert.Equal(t, float32(640), e.root.Frame().W)
	assert.Equal(t, float32(360), e.root.Frame().H)
	assert.Same(t, e.m, e.root.Manager())
}

func TestRootNilContent(t *testing.T) {
	t.Parallel()
	r := focus.NewRoot(focus.NewManager(&fakeEngine{}, nil), nil)

	assert.Zero(t, r.ChildCount())
}

func TestRootKeyDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	e := newEnv(t)
	e.focusOn("a1")
	e.m.SetPre(func(focus.Button) bool { return false })
	ev := ui.KeyEvent{Key: keys.KeyRight, Down: true}
	menu := ui.KeyEvent{Key: keys.KeyEscape, Down: true}
	e.m.OnMenu(func() bool { return true })
	e.root.InterceptKey(nil, &ev) // warm

	arrow := testing.AllocsPerRun(100, func() { e.root.InterceptKey(nil, &ev) })
	esc := testing.AllocsPerRun(100, func() { e.root.InterceptKey(nil, &menu) })

	assert.Zero(t, arrow, "arrow")
	assert.Zero(t, esc, "menu")
}
