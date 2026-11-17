//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/root.go at 781bee5.

package focus

import (
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Root is the top of the app's tree: a ui.KeyInterceptor that turns the keys of
// the Siri Remote into Buttons before any widget sees them, as the prototype's
// keydown listener does (tvKey and its listeners, index.html:761-767). It fills the space it
// is given and gives it all to its content.
//
// The arrows are only prepared here (Heal and Pre); a press that Pre does not
// take goes on to the focused widget and then to gDesk's directional focus.
type Root struct {
	ui.Base
	m *Manager
}

var (
	_ ui.KeyInterceptor = (*Root)(nil)
	_ ui.Widget         = (*Root)(nil)
)

// blocking are the modifiers that make a key no remote button: the prototype
// returns on metaKey, ctrlKey and altKey (index.html:767).
const blocking = keys.ModCtrl | keys.ModAlt | keys.ModSuper

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewRoot returns the Root of content, which may be nil.
func NewRoot(m *Manager, content ui.Widget) *Root {
	r := &Root{m: m}
	if content != nil {
		r.Add(content)
	}

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Manager returns the Manager the Root feeds.
func (r *Root) Manager() *Manager { return r.m }

// Layout fills the constraints and gives the content the whole space.
func (r *Root) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	for w := range r.Children() {
		c.Layout(w, ui.Tight(size))
		c.Place(w, geom.Point{})
	}

	return size
}

// InterceptKey maps the key to a button and runs it:
//
//   - an arrow press (repeats too): Heal and Pre; taken by Pre it is used,
//     otherwise it is left to the focused widget and gDesk's focus; the
//     repeats of a press that Pre took are used without asking again, until
//     the button is released;
//   - Enter down (not a repeat) is SelectDown, Enter up is SelectUp;
//   - Escape and Backspace, Space, P and MediaPlayPause presses run Press;
//   - releases of everything but Enter, other keys and keys with Ctrl, Alt
//     or Super are not the Root's.
func (r *Root) InterceptKey(_ *ui.Context, e *ui.KeyEvent) bool {
	if e.Mods&blocking != 0 {
		return false
	}
	b := ButtonOf(e.Key)
	if r.m.trace != nil && b != ButtonUnknown {
		r.m.traceKey(e)
		if e.Down && b.Dir() != DirUnknown {
			r.m.traceMove(b)
		}
	}
	switch {
	case b == ButtonUnknown:
	case !e.Down:
		if b == r.m.taken {
			r.m.taken = ButtonUnknown // the release ends the press Pre took
		}
	case e.Repeat:
		if b == r.m.taken {
			return true // the auto-repeat of a press Pre took : nothing more happens
		}
	default:
		r.m.taken = ButtonUnknown
	}
	switch b {
	case ButtonSelect:
		switch {
		case !e.Down:
			r.m.SelectUp()
		case !e.Repeat:
			r.m.SelectDown()
		default: // an Enter repeat is a held press: the Manager times the hold itself
		}

		return true
	case ButtonLeft, ButtonRight, ButtonUp, ButtonDown:
		return e.Down && r.m.Intercept(b)
	case ButtonMenu, ButtonPlayPause:
		return e.Down && r.m.Press(b)
	default:
		return false
	}
}
