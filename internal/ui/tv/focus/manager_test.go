//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/manager_test.go at 781bee5.

package focus_test

import (
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/ui/tv/focus"
)

func TestHealKeepsValidFocus(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a2 := e.focusOn("a2")
	var heard []string
	e.m.OnFocus(func(it focus.Item) { heard = append(heard, it.FocusKey()) })

	e.m.Heal()

	assert.Same(t, a2, e.eng.focused)
	assert.Empty(t, heard)
}

func TestHealFindsSameKey(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	old := e.focusOn("a2")
	var heard []string
	e.m.OnFocus(func(it focus.Item) { heard = append(heard, it.FocusKey()) })
	// The screen is rebuilt: the same key, a new widget.
	e.eng.remove(old)
	require.Nil(t, e.eng.focused)
	fresh := e.add(e.home, &item{key: "a2"})

	e.m.Heal()

	assert.Same(t, fresh, e.eng.focused)
	assert.Empty(t, heard, "heal sets silently")
}

func TestHealFallsBackToInitial(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.it["a3"].def = true
	e.focusOn("a2")
	e.eng.remove(e.it["a2"])

	e.m.Heal()

	assert.Same(t, e.it["a3"], e.eng.focused, "the default item of the screen")
}

func TestHealWhenNothingWasFocused(t *testing.T) {
	t.Parallel()
	e := newEnv(t)

	e.m.Heal()

	assert.Same(t, e.it["a1"], e.eng.focused)
}

func TestHealScopedToScreenTakesDefault(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.focusOn("a3")
	e.m.SetScope(e.home)
	e.eng.remove(e.it["a3"])
	// The screen is rebuilt without a3: nothing has its key, so Initial(scope)
	// runs and the default item wins; there is no memory of other screens.
	e.it["a2"].def = true

	e.m.Heal()

	assert.Equal(t, "a2", e.eng.focused.(*item).key)
}

func TestInitialPrefersKeyThenDefaultThenFirst(t *testing.T) {
	t.Parallel()
	t.Run("first", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		assert.Same(t, e.it["a1"], e.m.Initial(e.home, ""))
	})
	t.Run("default beats first", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.it["a2"].def = true
		assert.Same(t, e.it["a2"], e.m.Initial(e.home, ""))
	})
	t.Run("prefer beats default", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.it["a2"].def = true
		assert.Same(t, e.it["a3"], e.m.Initial(e.home, "a3"))
	})
	t.Run("a stale prefer falls to default", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.it["a2"].def = true
		assert.Same(t, e.it["a2"], e.m.Initial(e.home, "gone"))
	})
	t.Run("a prefer may name a nav item (settings takes nav0)", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		assert.Same(t, e.it["n1"], e.m.Initial(nil, "n1"))
	})
	t.Run("no memory: leaving a screen and coming back takes the default", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.m.Set(e.it["a3"], focus.SetOptions{Silent: true})
		e.m.Clear()
		e.it["a2"].def = true
		assert.Same(t, e.it["a2"], e.m.Initial(e.home, ""), "D8: the prototype has no per-screen memory")
	})
	t.Run("another screen", func(t *testing.T) {
		t.Parallel()
		e := newEnv(t)
		e.m.Set(e.it["a3"], focus.SetOptions{Silent: true})
		e.it["b2"].def = true
		assert.Same(t, e.it["b2"], e.m.Initial(e.tv, ""))
	})
}

func TestInitialSkipsChromeUnlessSidebar(t *testing.T) {
	t.Parallel()
	e := newEnv(t)

	assert.Same(t, e.it["a1"], e.m.Initial(nil, ""), "nil root: the default scope list includes chrome, which is skipped")

	got := e.m.Initial(e.side, "")
	assert.Same(t, e.it["n1"], got, "sidebar root keeps its items")

	got = e.m.Initial(e.play, "")
	assert.Same(t, e.it["p1"], got, "a bar that is not the sidebar: its items are skipped, but fs[0] is the last resort")
}

func TestInitialEmptyClears(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.focusOn("a1")
	empty := &screenBox{id: "empty"}
	e.home.Add(empty)

	got := e.m.Initial(empty, "")

	assert.Nil(t, got)
	assert.Nil(t, e.eng.focused)
}

func TestHasInitialChangesNothing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.focusOn("a2")
	empty := &screenBox{id: "empty"}
	e.home.Add(empty)

	assert.True(t, e.m.HasInitial(e.home), "a screen with items")
	assert.False(t, e.m.HasInitial(empty), "a screen with none")
	assert.False(t, e.m.HasInitial(e.play), "a bar that is not the sidebar: its items are skipped")
	assert.True(t, e.m.HasInitial(e.side), "the sidebar's own items count")
	assert.Same(t, e.it["a2"], e.m.Current(), "the focus is where it was")
}

func TestSetRevealsUnlessNoScroll(t *testing.T) {
	t.Parallel()
	e := newEnv(t)

	e.m.Set(e.it["a1"], focus.SetOptions{})
	e.m.Set(e.it["a1"], focus.SetOptions{}) // same item: reveal only
	e.m.Set(e.it["a2"], focus.SetOptions{NoScroll: true})

	assert.Equal(t, []ui.Widget{e.it["a1"], e.it["a1"]}, e.eng.reveals)
}

func TestSetRefusedByEngine(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.focusOn("a1")
	e.eng.refuse[e.it["a2"]] = true
	heard := 0
	e.m.OnFocus(func(focus.Item) { heard++ })

	e.m.Set(e.it["a2"], focus.SetOptions{})

	assert.Same(t, e.it["a1"], e.eng.focused)
	assert.Zero(t, heard)
}

func TestSilentSetDoesNotNotify(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	var heard []string
	remove := e.m.OnFocus(func(it focus.Item) { heard = append(heard, it.FocusKey()) })

	e.m.Set(e.it["a1"], focus.SetOptions{Silent: true})
	assert.Empty(t, heard)
	e.m.Set(e.it["a2"], focus.SetOptions{})
	assert.Equal(t, []string{"a2"}, heard)
	remove()
	e.m.Set(e.it["a3"], focus.SetOptions{})
	assert.Equal(t, []string{"a2"}, heard, "removed")
}

func TestEngineMoveNotifies(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.focusOn("a1")
	var heard []string
	e.m.OnFocus(func(it focus.Item) { heard = append(heard, it.FocusKey()) })

	assert.True(t, e.m.Move(focus.DirRight))
	assert.True(t, e.m.Move(focus.DirRight))
	assert.False(t, e.m.Move(focus.DirRight), "edge")

	assert.Equal(t, []string{"a2", "a3"}, heard)
	assert.Equal(t, "a3", e.m.Current().FocusKey())
}

func TestPreRunsBeforeArrowsMenuPlayPause(t *testing.T) {
	t.Parallel()
	for _, b := range []focus.Button{focus.ButtonLeft, focus.ButtonRight, focus.ButtonUp, focus.ButtonDown, focus.ButtonMenu, focus.ButtonPlayPause} {
		t.Run(b.String(), func(t *testing.T) {
			t.Parallel()
			e := newEnv(t)
			var seen []focus.Button
			var focusedInPre string
			e.m.SetPre(func(b focus.Button) bool {
				seen = append(seen, b)
				if c := e.m.Current(); c != nil {
					focusedInPre = c.FocusKey()
				}

				return true
			})
			menus, plays := 0, 0
			e.m.OnMenu(func() bool { menus++; return true })
			e.m.OnPlayPause(func() { plays++ })

			assert.True(t, e.m.Press(b), "Pre took it")

			assert.Equal(t, []focus.Button{b}, seen)
			assert.Equal(t, "a1", focusedInPre, "Heal ran before Pre")
			assert.Zero(t, menus+plays)
			assert.Zero(t, e.eng.moves)
		})
	}
}

func TestPressDispatchesWhenPreDeclines(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.m.SetPre(func(focus.Button) bool { return false })
	menus, plays := 0, 0
	e.m.OnMenu(func() bool { menus++; return true })
	e.m.OnPlayPause(func() { plays++ })
	e.focusOn("a1")

	assert.True(t, e.m.Press(focus.ButtonRight))
	assert.True(t, e.m.Press(focus.ButtonMenu))
	assert.True(t, e.m.Press(focus.ButtonPlayPause))
	assert.False(t, e.m.Press(focus.ButtonSelect), "Select has SelectDown and SelectUp")

	assert.Equal(t, "a2", e.m.Current().FocusKey())
	assert.Equal(t, 1, menus)
	assert.Equal(t, 1, plays)
}

func TestPreSeesSelectDownOnly(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	calls := 0
	e.m.SetPre(func(b focus.Button) bool { calls++; return b != focus.ButtonSelect })
	e.focusOn("a1")

	e.m.SelectDown()
	e.m.SelectDown() // a second down before the up is ignored
	e.m.SelectUp()

	assert.Equal(t, 1, calls, "asked on the first down, not on the up")
	assert.Equal(t, []bool{false}, e.it["a1"].acts)
}

func TestPreTakingSelectSwallowsThePress(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a1 := e.focusOn("a1")
	a1.holdHandled = true
	e.m.SetPre(func(focus.Button) bool { return true })

	e.m.SelectDown()
	e.tt.Elapse(2 * focus.HoldDuration)
	e.m.SelectUp()

	assert.Empty(t, a1.acts)
	assert.Empty(t, a1.pressing)
	assert.Zero(t, a1.pulses)

	e.m.SetPre(nil)
	e.m.SelectDown()
	e.m.SelectUp()
	assert.Equal(t, []bool{false}, a1.acts, "the next press is a press again")
}

func TestSelectShortPress(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a1 := e.focusOn("a1")

	e.m.SelectDown()
	assert.Equal(t, []bool{true}, a1.pressing)
	assert.Empty(t, a1.acts)
	e.tt.Elapse(300 * time.Millisecond)
	e.m.SelectUp()

	assert.Equal(t, []bool{true, false}, a1.pressing)
	assert.Equal(t, []bool{false}, a1.acts)
	assert.Equal(t, 1, a1.pulses)
	e.tt.Elapse(time.Second)
	assert.Equal(t, []bool{false}, a1.acts, "the cancelled timer does not fire")
}

func TestSelectHoldAt550(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a1 := e.focusOn("a1")
	a1.holdHandled = true

	e.m.SelectDown()
	e.tt.Elapse(focus.HoldDuration - time.Millisecond)
	assert.Empty(t, a1.acts, "549 ms: nothing yet")
	e.tt.Elapse(time.Millisecond)
	assert.Equal(t, []bool{true}, a1.acts, "550 ms: hold")
	assert.Equal(t, 1, a1.pulses)

	e.m.SelectUp()
	assert.Equal(t, []bool{true}, a1.acts, "release after a hold selects nothing")
	e.tt.Elapse(time.Second)
	assert.Equal(t, []bool{true}, a1.acts)
}

func TestHoldWithoutHandlerRunsSelect(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a1 := e.focusOn("a1") // holdHandled false

	e.m.SelectDown()
	e.tt.Elapse(focus.HoldDuration)
	e.m.SelectUp()

	assert.Equal(t, []bool{true, false}, a1.acts, "tried as a hold, then run as a press (app.js:35)")
}

func TestSelectUpWithoutDownIgnored(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a1 := e.focusOn("a1")

	e.m.SelectUp()

	assert.Empty(t, a1.acts)
	assert.Empty(t, a1.pressing)
}

func TestSecondSelectDownIgnored(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a1 := e.focusOn("a1")
	a1.holdHandled = true

	e.m.SelectDown()
	e.tt.Elapse(400 * time.Millisecond)
	e.m.SelectDown() // does not restart the hold
	e.tt.Elapse(150 * time.Millisecond)

	assert.Equal(t, []bool{true}, a1.acts, "the hold of the first down")
	assert.Equal(t, []bool{true}, a1.pressing[:1])
	e.m.SelectUp()
}

func TestNewPressAfterShortOneStartsFresh(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a1 := e.focusOn("a1")
	a1.holdHandled = true

	e.m.SelectDown()
	e.tt.Elapse(300 * time.Millisecond)
	e.m.SelectUp()
	e.m.SelectDown()
	e.tt.Elapse(300 * time.Millisecond) // 600 ms since the first down, 300 since the second
	assert.Equal(t, []bool{false}, a1.acts)
	e.tt.Elapse(250 * time.Millisecond)

	assert.Equal(t, []bool{false, true}, a1.acts)
}

func TestPressingClearedOnHoldAndRelease(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a1 := e.focusOn("a1")
	a1.holdHandled = true

	e.m.SelectDown()
	assert.Equal(t, []bool{true}, a1.pressing)
	e.tt.Elapse(focus.HoldDuration)
	assert.Equal(t, []bool{true, false}, a1.pressing, "off when the hold fires")
	e.m.SelectUp()
	assert.Equal(t, []bool{true, false}, a1.pressing, "and not again on release")
}

func TestPressingClearedOnItemFocusLeft(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	a1 := e.focusOn("a1")

	e.m.SelectDown()
	e.m.Move(focus.DirRight) // focus leaves while the button is down
	e.m.SelectUp()

	assert.Equal(t, []bool{true, false}, a1.pressing)
	assert.Empty(t, e.it["a2"].pressing)
}

func TestSelectHealsFirst(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.focusOn("a2")
	e.eng.remove(e.it["a2"])
	fresh := e.add(e.home, &item{key: "a2"})

	e.m.SelectDown()
	e.m.SelectUp()

	assert.Equal(t, []bool{false}, fresh.acts)
}

func TestSelectWithNothingFocusedDoesNothing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	empty := &screenBox{id: "empty"}
	e.home.Add(empty)
	e.m.SetScope(empty) // nothing to focus

	assert.NotPanics(t, func() {
		e.m.SelectDown()
		e.m.SelectUp()
		e.m.Select(true)
	})
}

func TestMenuChainNewestFirst(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	var order []string
	e.m.OnMenu(func() bool { order = append(order, "base"); return true })
	removeMid := e.m.OnMenu(func() bool { order = append(order, "mid"); return false })
	e.m.OnMenu(func() bool { order = append(order, "top"); return true })

	assert.True(t, e.m.Press(focus.ButtonMenu))
	assert.Equal(t, []string{"top"}, order, "the first true wins")

	order = nil
	removeMid()
	assert.True(t, e.m.Press(focus.ButtonMenu))
	assert.Equal(t, []string{"top"}, order)
}

func TestMenuChainFallsThrough(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	var order []string
	e.m.OnMenu(func() bool { order = append(order, "base"); return true })
	e.m.OnMenu(func() bool { order = append(order, "top"); return false })

	assert.True(t, e.m.Press(focus.ButtonMenu))

	assert.Equal(t, []string{"top", "base"}, order)
}

func TestMenuUnhandledReturnsFalse(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	assert.False(t, e.m.Press(focus.ButtonMenu), "no handler: the system gets Menu")

	remove := e.m.OnMenu(func() bool { return true })
	assert.True(t, e.m.Press(focus.ButtonMenu))
	remove()
	assert.False(t, e.m.Press(focus.ButtonMenu))
}

func TestMenuHandlerMayRemoveItself(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	calls := 0
	var remove func()
	remove = e.m.OnMenu(func() bool { calls++; remove(); return true })

	assert.True(t, e.m.Press(focus.ButtonMenu))
	assert.False(t, e.m.Press(focus.ButtonMenu))
	assert.Equal(t, 1, calls)
}

func TestPlayPauseWithoutHandlerIsUnused(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	assert.False(t, e.m.Press(focus.ButtonPlayPause), "Q-P7-18: nobody handles it, the system gets it")
	plays := 0
	remove := e.m.OnPlayPause(func() { plays++ })
	assert.True(t, e.m.Press(focus.ButtonPlayPause))
	remove()
	assert.False(t, e.m.Press(focus.ButtonPlayPause), "removed: unused again")
	assert.Equal(t, 1, plays)
	assert.False(t, e.tt.Key(keys.KeyMediaPlayPause, 0), "through the Root too")
}

func TestPlayPauseHandlersRunOldestFirst(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	var order []string
	e.m.OnPlayPause(func() { order = append(order, "a") })
	remove := e.m.OnPlayPause(func() { order = append(order, "b") })

	assert.True(t, e.m.Press(focus.ButtonPlayPause))
	remove()
	assert.True(t, e.m.Press(focus.ButtonPlayPause))

	assert.Equal(t, []string{"a", "b", "a"}, order)
}

func TestScopeSetAndGet(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	assert.Nil(t, e.m.Scope())

	e.m.SetScope(e.tv)
	assert.Same(t, e.tv, e.m.Scope())
	assert.Same(t, e.tv, e.eng.scope)
	assert.Same(t, e.it["b1"], e.m.Initial(e.m.Scope(), ""))

	e.m.SetScope(nil)
	assert.Nil(t, e.m.Scope())
	assert.Nil(t, e.eng.scope)
}

func TestClearForgetsTheKey(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.focusOn("a3")

	e.m.Clear()
	assert.Nil(t, e.m.Current())
	e.m.Heal()

	assert.Equal(t, "a1", e.m.Current().FocusKey(), "Clear is not a removal: the old key is not looked for")
}

func TestCloseStopsObserving(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.m.Close()

	heard := 0
	e.m.OnFocus(func(focus.Item) { heard++ })

	e.eng.Focus(e.it["a2"])

	assert.Zero(t, heard, "the closed Manager hears nothing")
}

func TestManagerFocusChangeDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	e := newEnv(t)
	heard := 0
	e.m.OnFocus(func(focus.Item) { heard++ })
	a1, a2 := e.it["a1"], e.it["a2"]
	e.m.Set(a1, focus.SetOptions{})
	e.m.Heal() // warms the scratch list
	e.m.Initial(e.home, "")

	set := testing.AllocsPerRun(100, func() {
		e.m.Set(a2, focus.SetOptions{})
		e.m.Set(a1, focus.SetOptions{})
	})
	heal := testing.AllocsPerRun(100, e.m.Heal)
	move := testing.AllocsPerRun(100, func() {
		e.m.Move(focus.DirRight)
		e.m.Move(focus.DirLeft)
	})
	initial := testing.AllocsPerRun(100, func() { e.m.Initial(e.home, "") })

	assert.Zero(t, set, "Set")
	assert.Zero(t, heal, "Heal")
	assert.Zero(t, move, "Move")
	assert.Zero(t, initial, "Initial")
	assert.Positive(t, heard)
}
