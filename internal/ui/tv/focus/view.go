//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/view.go at 781bee5.

package focus

import (
	"time"

	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// ViewEngine is the Engine of a *ui.View mounted with ui.WithDirectionalFocus:
// it maps the Manager's calls to gDesk's directional focus (View.MoveFocus,
// AppendFocusCandidates, SetFocusScope, OnFocusChange).
//
// The Manager and the widgets are built before the view that shows them
// exists, so a ViewEngine may be made empty and bound later: observers added
// before Bind are kept and registered with the view when it arrives, and the
// calls that need a view do nothing until then. A ViewEngine is also a Clock
// over the view's, for NewManager(e, e).
type ViewEngine struct {
	// V is the view; set it with Bind, or directly before the first call.
	V *ui.View

	observers []*viewObserver
	scope     ui.Widget // the last scope asked for, replayed by Bind
}

// viewObserver is an OnFocusChange function and the way to take it off the
// view once the view has it.
type viewObserver struct {
	fn      func(old, now ui.Widget)
	removeV func()
	removed bool
}

var (
	_ Engine = (*ViewEngine)(nil)
	_ Clock  = (*ViewEngine)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewViewEngine returns an engine of v, which may be nil: Bind it later.
func NewViewEngine(v *ui.View) *ViewEngine { return &ViewEngine{V: v} }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Bind makes v the engine's view, registers the observers added before and
// applies the scope set before, which the view could not take then.
func (e *ViewEngine) Bind(v *ui.View) {
	e.V = v
	e.sync()
	if e.scope != nil && v != nil {
		v.SetFocusScope(e.scope)
	}
}

// Focused returns the focused widget, nil when none or without a view.
func (e *ViewEngine) Focused() ui.Widget {
	if e.V == nil {
		return nil
	}

	return e.V.Focused()
}

// Focus focuses w, nil clears the focus. A widget that refuses the focus
// leaves it where it was (View.Focus).
func (e *ViewEngine) Focus(w ui.Widget) {
	if e.V == nil {
		return
	}
	e.sync()
	e.V.Focus(w)
}

// Move moves the focus to the nearest candidate in d: View.MoveFocus, which
// reveals the new widget as well.
func (e *ViewEngine) Move(d Dir) bool {
	if e.V == nil {
		return false
	}

	return e.V.MoveFocus(focusDirection(d))
}

// Reveal scrolls the containers of w so that w shows, by the Scrolls'
// RevealMargin, SmoothReveal and OnReveal (Base.ScrollIntoView).
func (e *ViewEngine) Reveal(w ui.Widget) {
	if w != nil {
		w.Node().ScrollIntoView()
	}
}

// SetScope confines the focus to w's subtree: View.SetFocusScope. Before the
// engine has a view the scope waits for it (Bind); nil lifts the scope.
func (e *ViewEngine) SetScope(w ui.Widget) {
	e.scope = w
	if e.V != nil {
		e.V.SetFocusScope(w)
	}
}

// Candidates appends to dst the widgets the focus may go to, in tree order:
// those of within, or of the scope when within is nil
// (View.AppendFocusCandidates). It allocates nothing when dst has room.
func (e *ViewEngine) Candidates(dst []ui.Widget, within ui.Widget) []ui.Widget {
	if e.V == nil {
		return dst
	}

	return e.V.AppendFocusCandidates(dst, within)
}

// OnFocusChange calls fn after every change of the focused widget
// (View.OnFocusChange). Before the engine has a view the function waits for
// it.
func (e *ViewEngine) OnFocusChange(fn func(old, now ui.Widget)) (remove func()) {
	o := &viewObserver{fn: fn}
	e.observers = append(e.observers, o)
	e.sync()

	return func() {
		o.removed = true
		if o.removeV != nil {
			o.removeV()
			o.removeV = nil
		}
	}
}

// AfterFunc calls fn on the UI thread after d, on the view's clock. Without a
// view it arranges nothing and returns the zero Timer.
func (e *ViewEngine) AfterFunc(d time.Duration, fn func()) ui.Timer {
	if e.V == nil {
		return ui.Timer{}
	}

	return e.V.Context().AfterFunc(d, fn)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// sync registers with the view the observers it does not have yet.
func (e *ViewEngine) sync() {
	if e.V == nil {
		return
	}
	for _, o := range e.observers {
		if o.removeV == nil && !o.removed {
			o.removeV = e.V.OnFocusChange(o.fn)
		}
	}
}

// focusDirection maps a Dir to gDesk's FocusDirection.
func focusDirection(d Dir) ui.FocusDirection {
	switch d {
	case DirLeft:
		return ui.FocusLeft
	case DirRight:
		return ui.FocusRight
	case DirUp:
		return ui.FocusUp
	case DirDown:
		return ui.FocusDown
	default:
		return ui.FocusDirectionUnknown
	}
}
