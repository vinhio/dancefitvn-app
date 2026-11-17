//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/manager.go at 781bee5.

package focus

import (
	"slices"
	"time"

	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Engine is what the Manager needs from gDesk's directional focus. *ui.View
// satisfies it through ViewEngine; tests use a fake.
type Engine interface {
	// Focused returns the widget holding the focus, nil when none does.
	Focused() ui.Widget
	// Focus gives the focus to w; nil clears it. A widget that refuses it
	// (disabled, hidden, outside the scope) leaves the focus where it was.
	Focus(w ui.Widget)
	// Move moves the focus to the nearest candidate in d (moveFocus,
	// index.html:751-760; gDesk's NearestInDirection, K9) and tells whether it
	// moved.
	Move(d Dir) bool
	// Reveal scrolls the containers of w so that w is in view
	// (scrollTV, index.html:729).
	Reveal(w ui.Widget)
	// SetScope confines the focus to w and its subtree; nil goes back to the
	// default scope.
	SetScope(w ui.Widget)
	// Candidates appends to dst the widgets the focus may go to, in order, and
	// returns it. Within nil means the scope's list, which for the default
	// scope includes the navigation bar; otherwise only the focusable widgets
	// inside within.
	Candidates(dst []ui.Widget, within ui.Widget) []ui.Widget
	// OnFocusChange calls fn on every change of the focused widget, however it
	// came about (Focus, Move, arrows, a removal), with the old and the new
	// widget, either of which may be nil.
	OnFocusChange(fn func(old, now ui.Widget)) (remove func())
}

// Clock runs a call later on the UI thread. *ui.Context fits.
type Clock interface {
	AfterFunc(d time.Duration, fn func()) ui.Timer
}

// SetOptions are the options of Set.
type SetOptions struct {
	// Silent leaves the OnFocus listeners out.
	Silent bool
	// NoScroll skips Reveal.
	NoScroll bool
}

// Manager is the prototype's focus handling (renderTV's default pick and heal,
// tvKey's chains), minus the geometry. It has no per-screen memory, as the
// prototype has none (Q-P7-2). It is for the UI thread only.
type Manager struct {
	e     Engine
	clock Clock
	// removeChange takes the Manager off the engine.
	removeChange func()

	scope   ui.Widget // nil is the default scope
	lastKey string    // the key of the last focused Item, kept through a removal (S.tv.focus)
	inSet   bool      // Set is moving the focus and notifies after Reveal

	pre func(Button) bool // the hook that sees every button first
	// taken is the button the Pre hook took on its last press: the
	// Root swallows the auto-repeats of that press. ButtonUnknown when none.
	taken Button

	onFocus  registry[func(Item)]
	onMenu   registry[func() bool]
	onPlay   registry[func()]
	scratch  []ui.Widget // reused candidate list
	pressing Presser     // the item showing .pressing, if any

	// Select state: down, and held once the 550 ms timer fired.
	down   bool
	held   bool
	timer  ui.Timer
	holdFn func() // prebuilt: the timer's call

	trace *tracer // the focus trace (trace.go), nil when off
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewManager returns a Manager over e. The clock times the Select hold; with a
// nil clock a hold never fires. Close takes the Manager off the engine.
func NewManager(e Engine, c Clock) *Manager {
	m := &Manager{e: e, clock: c}
	m.holdFn = m.holdFired
	m.removeChange = e.OnFocusChange(m.focusChanged)
	m.SetTrace(envTrace())

	return m
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Close takes the Manager off the engine and cancels the hold timer.
func (m *Manager) Close() {
	m.timer.Stop()
	m.down, m.held = false, false
	if m.removeChange != nil {
		m.removeChange()
		m.removeChange = nil
	}
}

// Current returns the focused Item. It is nil when nothing is focused or
// the focused widget is no Item.
func (m *Manager) Current() Item {
	it, _ := m.e.Focused().(Item)

	return it
}

// Set focuses it (S.tv.focus = fid, index.html:722). Setting the focused item
// only reveals it. A nil item does nothing. The OnFocus listeners run after the
// reveal.
func (m *Manager) Set(it Item, o SetOptions) {
	if it == nil {
		return
	}
	if m.e.Focused() == ui.Widget(it) {
		if !o.NoScroll {
			m.e.Reveal(it)
		}

		return
	}
	m.inSet = true
	m.e.Focus(it)
	m.inSet = false
	if m.e.Focused() != ui.Widget(it) {
		return // the engine refused it
	}
	if !o.NoScroll {
		m.e.Reveal(it)
	}
	if !o.Silent {
		m.notifyFocus(it)
	}
}

// Initial focuses the default item of root and returns it (renderTV's rule,
// index.html:722-724): the item with preferKey (the key that stayed focused, or
// the one the caller asks for, as the settings screen asks for nav0), then the
// item with FocusDefault, then the first that is not in the navigation bar. A
// nil root means the engine's default scope; the navigation bar's items are
// left out of the default pick unless root is the bar. There is no memory of
// earlier screens. With no item it clears the focus and returns nil.
func (m *Manager) Initial(root ui.Widget, preferKey string) Item {
	items := m.initialItems(root)

	var pick Item
	if preferKey != "" {
		for _, w := range items {
			if it := w.(Item); it.FocusKey() == preferKey {
				pick = it

				break
			}
		}
	}
	if pick == nil {
		for _, w := range items {
			if it := w.(Item); it.FocusDefault() && !chromeItem(root, it) {
				pick = it

				break
			}
		}
	}
	if pick == nil {
		for _, w := range items {
			if it := w.(Item); !chromeItem(root, it) {
				pick = it

				break
			}
		}
	}
	if pick == nil && len(items) > 0 {
		pick = items[0].(Item) // fs[0] when only the navigation bar is there
	}
	if pick == nil {
		m.Clear()

		return nil
	}
	m.Set(pick, SetOptions{})

	return pick
}

// HasInitial tells whether Initial(root, ...) would find a default item: root
// has a focusable Item outside the navigation bar (inside it when root is the
// bar). It changes nothing, so a caller can keep the focus where it is when the
// answer is no. It does not allocate.
func (m *Manager) HasInitial(root ui.Widget) bool {
	for _, w := range m.initialItems(root) {
		if !chromeItem(root, w) {
			return true
		}
	}

	return false
}

// initialItems fills m.scratch with the Items among root's candidates and
// returns them.
func (m *Manager) initialItems(root ui.Widget) []ui.Widget {
	m.scratch = m.e.Candidates(m.scratch[:0], root)
	items := m.scratch[:0]
	for _, w := range m.scratch {
		if _, ok := w.(Item); ok {
			items = append(items, w)
		}
	}
	m.scratch = items

	return items
}

// Heal repairs the focus after a screen or a list was rebuilt (renderTV, index.html:722-723):
// when the focused item is still among the candidates nothing happens; else
// the item with the same key is focused silently, or, when there is none, the
// initial item of the scope.
func (m *Manager) Heal() {
	cur := m.e.Focused()
	m.scratch = m.e.Candidates(m.scratch[:0], nil)
	if cur != nil && slices.Contains(m.scratch, cur) {
		return
	}
	key := m.lastKey
	if cur != nil {
		m.e.Focus(nil)
	}
	if key != "" {
		for _, w := range m.scratch {
			if it, ok := w.(Item); ok && it.FocusKey() == key {
				m.Set(it, SetOptions{Silent: true})

				return
			}
		}
	}
	m.Initial(m.scope, "")
}

// Clear removes the focus. Heal then starts from the initial item.
func (m *Manager) Clear() {
	m.e.Focus(nil)
	m.lastKey = ""
}

// Move moves the focus one step in d through the engine, for code paths; the
// remote's arrows reach the engine through gDesk's key routing instead, after
// the focused item's own handler had its chance (tvKey, index.html:766).
func (m *Manager) Move(d Dir) bool { return m.e.Move(d) }

// SetScope confines the focus to w and its subtree, a dialog or a drawer; nil
// restores the default scope.
func (m *Manager) SetScope(w ui.Widget) {
	m.scope = w
	m.e.SetScope(w)
}

// Scope returns the scope set by SetScope, nil for the default.
func (m *Manager) Scope() ui.Widget { return m.scope }

// OnFocus calls fn with the item that took the focus by a non-silent Set or by
// the engine (arrows). The prototype's bus 'focus'.
func (m *Manager) OnFocus(fn func(Item)) (remove func()) { return m.onFocus.add(fn) }

// SetPre sets the hook that sees every button after Heal and before the common
// handling; returning true takes the button (the screen's key hook). nil removes it. Unlike
// the prototype's, the hook sees Select too: it is asked when Select
// goes down, and a Select it takes shows no .pressing, does not hold and does
// not select.
func (m *Manager) SetPre(fn func(Button) bool) { m.pre = fn }

// OnMenu adds a Menu handler. Handlers run newest first and the first to
// return true takes the press.
func (m *Manager) OnMenu(fn func() bool) (remove func()) { return m.onMenu.add(fn) }

// OnPlayPause adds a Play/Pause handler. Every handler runs, oldest first.
// Without a handler the press is unused and left to the system (Q-P7-18: the
// prototype toggles only on the player and music screens, index.html:764).
func (m *Manager) OnPlayPause(fn func()) (remove func()) { return m.onPlay.add(fn) }

// Intercept runs the start of a press: Heal, then the Pre hook. It tells
// whether Pre took the button.
func (m *Manager) Intercept(b Button) bool {
	m.Heal()
	if m.pre != nil && m.pre(b) {
		m.taken = b

		return true
	}

	return false
}

// Press handles the buttons but Select (tvKey, index.html:761-766): Heal, Pre,
// then the arrows move the focus, Menu runs its chain and Play/Pause its
// handlers. It tells whether the press was used; a Menu nobody takes and a
// Play/Pause without a handler are not, so the system gets them (Menu at a
// root screen leaves for the Apple TV Home).
func (m *Manager) Press(b Button) bool {
	if m.Intercept(b) {
		return true
	}
	switch b {
	case ButtonLeft, ButtonRight, ButtonUp, ButtonDown:
		return m.Move(b.Dir())
	case ButtonMenu:
		return m.menu()
	case ButtonPlayPause:
		return m.playPause()
	default:
		return false
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// focusChanged is the engine's observer: lastKey follows every change, the
// listeners hear of it unless Set is the cause (it notifies after the reveal).
func (m *Manager) focusChanged(old, now ui.Widget) {
	if m.trace != nil {
		m.trace.line("focus " + widgetKey(old) + " -> " + widgetKey(now))
	}
	if now == nil {
		return // lastKey stays: Heal looks for it after a removal
	}
	it, ok := now.(Item)
	if !ok {
		m.lastKey = ""

		return
	}
	m.lastKey = it.FocusKey()
	if !m.inSet {
		m.notifyFocus(it)
	}
}

// notifyFocus tells the OnFocus listeners.
func (m *Manager) notifyFocus(it Item) {
	m.onFocus.each(func(fn func(Item)) bool { fn(it); return false })
}

// menu runs the Menu chain, newest first.
func (m *Manager) menu() bool {
	return m.onMenu.eachNewest(func(fn func() bool) bool { return fn() })
}

// playPause runs the Play/Pause handlers, oldest first, and tells whether one
// ran.
func (m *Manager) playPause() bool {
	ran := false
	m.onPlay.each(func(fn func()) bool { fn(); ran = true; return false })

	return ran
}

// chromeItem tells whether w is in the navigation bar and root is not the bar:
// such an item is left out of the default pick.
func chromeItem(root, w ui.Widget) bool {
	if c, ok := root.(Chrome); ok && c.FocusChrome() == ChromeSidebar {
		return false
	}

	return hasChrome(w)
}

// hasChrome tells whether w is inside the navigation bar.
func hasChrome(w ui.Widget) bool {
	for p := w.Node().Parent(); p != nil; p = p.Node().Parent() {
		if c, ok := p.(Chrome); ok && c.FocusChrome() != ChromeUnknown {
			return true
		}
	}

	return false
}

// registry is a list of handlers that may be added and removed while it is
// being run.
type registry[F any] struct {
	next    int
	entries []regEntry[F]
	running int
	dirty   bool
}

type regEntry[F any] struct {
	id   int
	fn   F
	dead bool
}

// add appends fn and returns the function that removes it.
func (r *registry[F]) add(fn F) (remove func()) {
	r.next++
	id := r.next
	r.entries = append(r.entries, regEntry[F]{id: id, fn: fn})

	return func() {
		for i := range r.entries {
			if r.entries[i].id == id && !r.entries[i].dead {
				r.entries[i].dead = true
				r.dirty = true
			}
		}
		r.compact()
	}
}

// compact drops removed entries once nothing is running.
func (r *registry[F]) compact() {
	if r.running > 0 || !r.dirty {
		return
	}
	r.entries = slices.DeleteFunc(r.entries, func(e regEntry[F]) bool { return e.dead })
	r.dirty = false
}

// each calls f with the live handlers, oldest first, until f returns true; it
// tells whether one did. Handlers added meanwhile are not called.
func (r *registry[F]) each(f func(F) bool) bool {
	r.running++
	defer func() { r.running--; r.compact() }()
	for i, n := 0, len(r.entries); i < n; i++ {
		if e := r.entries[i]; !e.dead && f(e.fn) {
			return true
		}
	}

	return false
}

// eachNewest is each, newest first.
func (r *registry[F]) eachNewest(f func(F) bool) bool {
	r.running++
	defer func() { r.running--; r.compact() }()
	for i := len(r.entries) - 1; i >= 0; i-- {
		if e := r.entries[i]; !e.dead && f(e.fn) {
			return true
		}
	}

	return false
}
