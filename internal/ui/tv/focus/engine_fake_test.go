//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/engine_fake_test.go at 781bee5.

package focus_test

import (
	"slices"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/ui/tv/focus"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// fakeEngine is a focus.Engine over a list of widgets. Candidates are the
// widgets under the scope, or, in the default scope, those of the active
// screen and of the chrome; Move walks the candidates in order.
type fakeEngine struct {
	items     []ui.Widget
	focused   ui.Widget
	scope     ui.Widget
	screen    ui.Widget // the active screen of the default scope
	observers []func(old, now ui.Widget)
	refuse    map[ui.Widget]bool
	moves     int
	reveals   []ui.Widget
}

var _ focus.Engine = (*fakeEngine)(nil)

// item is an Item with a log of what was done to it.
type item struct {
	ui.Base
	key         string
	def         bool
	holdHandled bool
	acts        []bool
	pressing    []bool
	pulses      int
}

var (
	_ focus.Item      = (*item)(nil)
	_ focus.Activator = (*item)(nil)
	_ focus.Presser   = (*item)(nil)
)

func (i *item) FocusKey() string     { return i.key }
func (i *item) FocusDefault() bool   { return i.def }
func (i *item) SetPressing(on bool)  { i.pressing = append(i.pressing, on) }
func (i *item) Pulse()               { i.pulses++ }
func (i *item) Activate(l bool) bool { i.acts = append(i.acts, l); return !l || i.holdHandled }

// screenBox is a Screen, chromeBox a Chrome.
type screenBox struct {
	ui.Base
	id string
}

func (s *screenBox) ScreenID() string { return s.id }

type chromeBox struct {
	ui.Base
	kind focus.ChromeKind
}

func (c *chromeBox) FocusChrome() focus.ChromeKind { return c.kind }

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// lateClock is a focus.Clock bound to the tester's view once it exists.
type lateClock struct{ c *ui.Context }

func (l *lateClock) AfterFunc(d time.Duration, fn func()) ui.Timer { return l.c.AfterFunc(d, fn) }

// env is the fixture: a sidebar (n1, n2), a playbar (p1) and two screens, home
// (a1 a2 a3) and tv (b1 b2), under a Root in a Tester, over the fake engine.
type env struct {
	t    *testing.T
	eng  *fakeEngine
	m    *focus.Manager
	tt   *ui.Tester
	root *focus.Root
	side *chromeBox
	play *chromeBox
	home *screenBox
	tv   *screenBox
	it   map[string]*item
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{t: t, eng: &fakeEngine{refuse: map[ui.Widget]bool{}}, it: map[string]*item{}}
	clk := &lateClock{}
	e.m = focus.NewManager(e.eng, clk)
	e.side = &chromeBox{kind: focus.ChromeSidebar}
	e.play = &chromeBox{kind: focus.ChromePlaybar}
	e.home = &screenBox{id: "home"}
	e.tv = &screenBox{id: "tv"}
	shell := &ui.Base{}
	shell.Add(e.side, e.play, e.home, e.tv)
	for _, p := range []struct {
		parent ui.Widget
		key    string
	}{{e.side, "n1"}, {e.side, "n2"}, {e.play, "p1"}, {e.home, "a1"}, {e.home, "a2"}, {e.home, "a3"}, {e.tv, "b1"}, {e.tv, "b2"}} {
		e.add(p.parent, &item{key: p.key})
	}
	e.eng.screen = e.home
	e.root = focus.NewRoot(e.m, shell)
	tt, err := ui.NewTester(e.root, 640, 360)
	require.NoError(t, err)
	e.tt = tt
	clk.c = tt.View().Context()
	t.Cleanup(e.m.Close)

	return e
}

// add puts it under parent and into the engine's list.
func (e *env) add(parent ui.Widget, it *item) *item {
	parent.Node().Add(it)
	e.eng.items = append(e.eng.items, it)
	e.it[it.key] = it

	return it
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func (f *fakeEngine) Focused() ui.Widget { return f.focused }

func (f *fakeEngine) Focus(w ui.Widget) {
	if w != nil && (f.refuse[w] || !slices.Contains(f.items, w)) {
		return
	}
	f.change(w)
}

func (f *fakeEngine) change(w ui.Widget) {
	old := f.focused
	f.focused = w
	for _, fn := range f.observers {
		fn(old, w)
	}
}

func (f *fakeEngine) Move(d focus.Dir) bool {
	f.moves++
	var buf [16]ui.Widget
	list := f.Candidates(buf[:0], nil)
	i := slices.Index(list, f.focused)
	if i < 0 {
		return false
	}
	switch d {
	case focus.DirRight, focus.DirDown:
		i++
	case focus.DirLeft, focus.DirUp:
		i--
	default:
		return false
	}
	if i < 0 || i >= len(list) {
		return false
	}
	f.change(list[i])

	return true
}

func (f *fakeEngine) Reveal(w ui.Widget) { f.reveals = append(f.reveals, w) }

func (f *fakeEngine) SetScope(w ui.Widget) { f.scope = w }

func (f *fakeEngine) Candidates(dst []ui.Widget, within ui.Widget) []ui.Widget {
	for _, w := range f.items {
		switch {
		case within != nil:
			if under(w, within) {
				dst = append(dst, w)
			}
		case f.scope != nil:
			if under(w, f.scope) {
				dst = append(dst, w)
			}
		default:
			if under(w, f.screen) || inChrome(w) {
				dst = append(dst, w)
			}
		}
	}

	return dst
}

func (f *fakeEngine) OnFocusChange(fn func(old, now ui.Widget)) func() {
	f.observers = append(f.observers, fn)

	return func() { f.observers = nil }
}

// remove takes w out of the tree and the list, clearing the focus when it held
// it, as gDesk does on removal.
func (f *fakeEngine) remove(w ui.Widget) {
	f.items = slices.DeleteFunc(f.items, func(x ui.Widget) bool { return x == w })
	if p := w.Node().Parent(); p != nil {
		p.Node().Remove(w)
	}
	if f.focused == w {
		f.change(nil)
	}
}

// under tells whether w is root or inside it.
func under(w, root ui.Widget) bool {
	for p := w; p != nil; p = p.Node().Parent() {
		if p == root {
			return true
		}
	}

	return false
}

func inChrome(w ui.Widget) bool {
	for p := w.Node().Parent(); p != nil; p = p.Node().Parent() {
		if _, ok := p.(*chromeBox); ok {
			return true
		}
	}

	return false
}

// down, up and repeat send a key's events alone, as the remote does: Tester.Key
// sends a press and its release together.
func (e *env) down(k keys.Key) bool {
	return e.tt.View().HandleEvent(platform.Event{Kind: platform.EventKeyDown, Key: k})
}

func (e *env) repeat(k keys.Key) bool {
	return e.tt.View().HandleEvent(platform.Event{Kind: platform.EventKeyDown, Key: k, Repeat: true})
}

func (e *env) up(k keys.Key) bool {
	return e.tt.View().HandleEvent(platform.Event{Kind: platform.EventKeyUp, Key: k})
}

// focusOn focuses the item with key k, silently.
func (e *env) focusOn(k string) *item {
	e.t.Helper()
	it := e.it[k]
	e.m.Set(it, focus.SetOptions{Silent: true, NoScroll: true})
	require.Same(e.t, it, e.eng.focused)

	return it
}
