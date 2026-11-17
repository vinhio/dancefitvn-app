//go:build tvos

package nav

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Route names a screen and what it was opened with: a screen is built from it
// again when the stack goes back to it.
type Route struct {
	// Screen is the screen's id, the prototype's `S.tv.scr` ("welcome", "code",
	// "home"...).
	Screen string
	// Params are the screen's arguments (a course id, `S.tv.set`); the factory
	// reads them, the Stack does not.
	Params any
}

// Stack is `S.tv.stack` with `S.tv.scr` and `S.tv.panel`: the route showing, the
// routes under it and whether the player's panel is open. It is for the UI
// thread. Pushing, popping and flipping the panel allocate nothing once the
// stack has grown.
type Stack struct {
	cur   Route
	has   bool
	stack []Route
	panel bool
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewStack returns an empty Stack with room for a few screens.
func NewStack() *Stack { return &Stack{stack: make([]Route, 0, 8)} }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Go makes r the route showing, as tvGo does (index.html:730): a root empties
// the stack, otherwise the route that was showing is pushed. The panel closes.
func (s *Stack) Go(r Route, root bool) {
	if root {
		clear(s.stack)
		s.stack = s.stack[:0]
	} else if s.has {
		s.stack = append(s.stack, s.cur)
	}
	s.cur, s.has, s.panel = r, true, false
}

// Replace makes r the route showing without pushing the one that was: the
// player and the up-next screen take each other's place (`playLes` from the
// player or up next, index.html:739; the end of a video, :782). The panel
// closes.
func (s *Stack) Replace(r Route) {
	s.cur, s.has, s.panel = r, true, false
}

// Pop makes the route under the current one the current, as tvBack does
// (index.html:731), and returns it; false, with nothing changed, when the
// stack is empty.
func (s *Stack) Pop() (Route, bool) {
	n := len(s.stack)
	if n == 0 {
		return Route{}, false
	}
	s.cur = s.stack[n-1]
	s.stack[n-1] = Route{}
	s.stack = s.stack[:n-1]
	s.panel = false

	return s.cur, true
}

// Current returns the route showing and whether there is one.
func (s *Stack) Current() (Route, bool) { return s.cur, s.has }

// Screen returns the id of the route showing, "" before the first.
func (s *Stack) Screen() string { return s.cur.Screen }

// Depth returns how many routes are under the current one.
func (s *Stack) Depth() int { return len(s.stack) }

// Panel tells whether the player's panel is open: `S.tv.panel`.
func (s *Stack) Panel() bool { return s.panel }

// SetPanel opens or closes the panel.
func (s *Stack) SetPanel(open bool) { s.panel = open }
