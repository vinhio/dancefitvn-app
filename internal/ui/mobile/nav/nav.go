// Adapted from jplayer-mobile/internal/ui/shell/root.go (AddBack, Back,
// InterceptKey), jplayer-mobile/internal/ui/nav/{route,stack}.go (the routes and
// the stack of a tab) and jplayer-mobile/internal/ui/sheets/host.go (the overlay
// hosts in the tree) at 49de3c2. The prototype has one stack, no tab keeps its
// own, and no transition (index.html:540-552), so the stack holds routes and a
// screen is rebuilt by its factory; jPlayer's animations, its tabs' stacks and
// its priorities of Back are not taken.

package nav

import (
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/gesture"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Screen is what a Factory builds: the widget of one screen, which fills the
// window (Tight constraints) and stands in a Page or lays itself out. It may
// implement Scroller, Tabbed and Leaver.
type Screen = ui.Widget

// Route names a screen and what it was opened with: a screen is built from it
// again, when the stack goes back to it and on Refresh.
type Route struct {
	// Screen is the screen's id, the prototype's `S.m.scr` ("home", "courses",
	// "detail", "video"...).
	Screen string
	// Params are the screen's arguments (a course id, a lesson key); the
	// factory reads them, the Nav does not.
	Params any
}

// Scroller is a Screen with a scroll whose offset Refresh keeps (`renderM` keeps
// the scroll of the same screen, index.html:538-539).
type Scroller interface {
	Scroll() *ui.Scroll
}

// Tabbed is a Screen that shows the tab bar when Tab is 0 to TabCount-1, with
// that tab current, and is a root of the app: Back from it goes Home. A screen
// that is not Tabbed, or answers -1, has no tab bar.
type Tabbed interface {
	Tab() int
}

// Leaver is a Screen told when it is left by Go, Replace or Back (not by
// Refresh): "leaving the video screen stops the playback" (03 section 1,
// `go()` and `back()`, index.html:545, 551).
type Leaver interface {
	Left()
}

// TabRoute is one tab: how the tab bar shows it, and the screen it goes to.
type TabRoute struct {
	// Item is the tab bar's icon, label and layout.json name.
	Item kit.TabItem
	// Route is the screen the tab is the root of (`data-go`, `data-root="1"`).
	Route Route
}

// Options are what New needs.
type Options struct {
	// Home is the screen Back goes to when nothing is left on the stack (`p||'home'`,
	// index.html:551).
	Home Route
	// Tabs are the four tabs, in the order of the tab bar.
	Tabs [kit.TabCount]TabRoute
	// Factory builds the screen of a route. It is called on the UI thread, at
	// each Go, Replace, Back and Refresh.
	Factory func(Route) Screen
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Nav is the root of the phone shell (the prototype's `#m`): the current screen,
// the tab bar when that screen has one, the sheet host and the toast host, in
// that order (so the touches and the paint go to the toast first, the sheet next,
// then the tab bar, the screen last). Mount it as the root of the view; an app
// that wraps it in another root passes the Back key to InterceptKey itself, as
// the view gives keys to the root's interceptor when nothing has the focus.
type Nav struct {
	ui.Base

	opts  Options
	stack []Route // the screens under the current one, the last the top
	cur   Route
	has   bool // cur is a route; false before the first Go

	host  screenHost
	bar   *kit.TabBar
	sheet *kit.Sheet
	toast *kit.Toast

	steps    []backStep
	seq      int
	backUsed bool

	edge     gesture.Edge
	keep     float32 // the scroll offset Refresh puts back after the next layout
	keepNow  bool
	tapTabFn func(int)
}

// screenHost holds the current screen, as large as the Nav.
type screenHost struct {
	ui.Base
	// Hush leaves the screen out of a screen reader's reading while a sheet is open
	// over it: its scrim takes every touch, so nothing under it can be reached
	// (gDesk's modal layer does the same for a ui.Layer, which the sheet is not).
	kit.Hush

	screen Screen
}

// backStep is a layer that gives way to Back before the stack pops, AddBack's.
type backStep struct {
	id int
	fn func() bool
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// New returns a Nav with no screen: call Go with the first route.
func New(o Options) *Nav {
	n := &Nav{opts: o, stack: make([]Route, 0, 8), edge: gesture.EdgeBack()}
	n.Name = nameNav
	items := [kit.TabCount]kit.TabItem{}
	for i, t := range o.Tabs {
		items[i] = t.Item
	}
	n.tapTabFn = n.tapTab
	n.bar = kit.NewTabBar(items, -1, n.tapTabFn)
	n.bar.Hidden = true
	n.sheet = kit.NewSheet()
	n.toast = kit.NewToast()
	// Under an open sheet the screen and the tab bar are out of a screen reader's
	// reach; the toast never covers anything (it is a status over the screen).
	n.host.Silent = n.sheet.IsOpen
	n.bar.Silent = n.sheet.IsOpen
	n.Add(&n.host, n.bar, n.sheet, n.toast)

	return n
}

// Go opens a screen (`go()`, index.html:543-550): the current one goes on the stack
// and r is built at the top; with root the stack is cleared first (a tab, `data-root="1"`).
// An open sheet closes. The toast stays: it times out by itself.
func (n *Nav) Go(r Route, root bool) {
	if root {
		n.stack = n.stack[:0]
	} else if n.has {
		n.stack = append(n.stack, n.cur)
	}
	n.sheet.Close()
	n.show(r)
}

// Replace builds r in place of the current screen, with the stack as it is: the
// player showing another lesson (03 section 3, `playVideo` from 06 or 10). An
// open sheet closes.
func (n *Nav) Replace(r Route) {
	n.sheet.Close()
	n.show(r)
}

// Back is what the system's Back key and the edge swipe do (`back()`, index.html:551;
// docs/plan/03 section 1): the first of the steps AddBack registered (last
// registered first) that has something to close closes it; else the open sheet
// closes; else the top of the stack is popped and rebuilt; else a root screen
// other than Home goes Home. It returns false when none of that applied: the app
// is at Home (or on a screen with no tab bar and nothing under it), and the system
// takes the key (Android sends the app to the background).
func (n *Nav) Back() bool {
	for i := len(n.steps) - 1; i >= 0; i-- {
		if n.steps[i].fn() {
			return true
		}
	}
	if n.sheet.Back() {
		return true
	}
	if k := len(n.stack); k > 0 {
		r := n.stack[k-1]
		n.stack = n.stack[:k-1]
		n.show(r)

		return true
	}
	if n.isRoot() && n.cur.Screen != n.opts.Home.Screen {
		n.show(n.opts.Home)

		return true
	}

	return false
}

// CanBack tells whether Back would do something now: a step is registered (it
// may still have nothing to close), the sheet is open, the stack is not empty, or
// a root screen is not Home.
func (n *Nav) CanBack() bool {
	return len(n.steps) > 0 || n.sheet.IsOpen() || len(n.stack) > 0 ||
		(n.isRoot() && n.cur.Screen != n.opts.Home.Screen)
}

// Current returns the route of the screen showing; the zero Route before the first
// Go.
func (n *Nav) Current() Route { return n.cur }

// Depth returns how many screens are under the current one.
func (n *Nav) Depth() int { return len(n.stack) }

// Stack returns the routes under the current one, bottom first. The slice is the
// Nav's: do not keep it.
func (n *Nav) Stack() []Route { return n.stack }

// Screen returns the widget of the screen showing, nil before the first Go.
func (n *Nav) Screen() Screen { return n.host.screen }

// Refresh builds the current screen again from its route, keeping its scroll
// (`renderM` on the same screen, index.html:536-539); the sheet and the stack stay.
func (n *Nav) Refresh() {
	if !n.has {
		return
	}
	if s, ok := n.host.screen.(Scroller); ok {
		n.keep, n.keepNow = s.Scroll().Offset().Y, true
	}
	n.set(n.opts.Factory(n.cur))
}

// Sheet returns the sheet host: open a sheet with Sheet().Open, close it with the
// handle. It is over the screen and the tab bar, under the toast.
func (n *Nav) Sheet() *kit.Sheet { return n.sheet }

// Toast shows msg for 2200 ms over everything, replacing the one showing and
// starting the time again (`toastM`, index.html:552).
func (n *Nav) Toast(msg string) { n.toast.Show(msg) }

// Toasts returns the toast host.
func (n *Nav) Toasts() *kit.Toast { return n.toast }

// TabBar returns the tab bar, hidden while the screen has none.
func (n *Nav) TabBar() *kit.TabBar { return n.bar }

// AddBack registers a layer that Back asks before it closes the sheet and pops
// the stack: step closes what it holds and returns true, or returns false when
// there is nothing to close. Steps are asked last registered first. The returned
// function removes the step; call it when the layer goes.
func (n *Nav) AddBack(step func() bool) (remove func()) {
	n.seq++
	id := n.seq
	n.steps = append(n.steps, backStep{id: id, fn: step})

	return func() {
		for i, s := range n.steps {
			if s.id == id {
				n.steps = append(n.steps[:i:i], n.steps[i+1:]...)

				return
			}
		}
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// show makes r the current route and builds its screen, telling the one it
// leaves.
func (n *Nav) show(r Route) {
	if l, ok := n.host.screen.(Leaver); ok {
		l.Left()
	}
	n.cur, n.has = r, true
	n.keepNow = false
	n.set(n.opts.Factory(r))
}

// set puts w in the screen's place and sets the tab bar for it.
func (n *Nav) set(w Screen) {
	n.host.set(w)
	tab := -1
	if t, ok := w.(Tabbed); ok {
		tab = t.Tab()
	}
	kit.SetHidden(n.bar, tab < 0 || tab >= kit.TabCount)
	n.bar.SetCurrent(tab)
	n.Relayout()
	if v := n.View(); v != nil {
		v.Context().SemanticsChanged(platform.ChangeScreen)
	}
}

// isRoot tells whether the screen showing is one of the tabs' roots: it has the tab bar.
func (n *Nav) isRoot() bool {
	t, ok := n.host.screen.(Tabbed)

	return ok && t.Tab() >= 0 && t.Tab() < kit.TabCount
}

// tapTab is the tab bar's OnTap: the tab's root, the stack cleared, also when it is
// the current tab (`data-root="1"`, index.html:324).
func (n *Nav) tapTab(i int) {
	if i >= 0 && i < kit.TabCount {
		n.Go(n.opts.Tabs[i].Route, true)
	}
}

// set replaces the screen.
func (h *screenHost) set(w Screen) {
	if h.screen != nil {
		h.Remove(h.screen)
	}
	h.screen = w
	if w != nil {
		h.Add(w)
	}
	h.Relayout()
}
