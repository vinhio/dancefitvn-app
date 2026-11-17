// Adapted from jplayer-mobile/internal/ui/shell/root.go at 49de3c2 (InterceptKey
// and the way a Back press and its release are taken together).

package nav

import (
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// nameNav is the key layout.json gives the root (the prototype's `#m`).
const nameNav = "div#m"

var (
	_ ui.KeyInterceptor     = (*Nav)(nil)
	_ ui.PointerInterceptor = (*Nav)(nil)
	_ ui.PointerHandler     = (*Nav)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// InterceptKey takes the Back key (keys.KeyBack, which the Android driver sends for
// the system's back button and its back gesture) when Back does something. The
// release is taken when its press was, so that it does not reach the system and
// leave the app; a key held down is not taken again. At Home with nothing to close
// it returns false for both and the system acts.
//
// While a text field has the focus and the on-screen keyboard is up, the first Back
// only puts the keyboard away (the field's focus goes, as Enter's Done does) and the
// next one acts, as the system's own Back does with an input method: a sheet or a
// screen with a form is not closed under the finger that wants the keyboard gone.
func (n *Nav) InterceptKey(c *ui.Context, e *ui.KeyEvent) bool {
	if e.Key != keys.KeyBack {
		return false
	}
	if !e.Down {
		used := n.backUsed
		n.backUsed = false

		return used
	}
	if !e.Repeat {
		n.backUsed = n.hideKeyboard(c) || n.Back()
	}

	return n.backUsed
}

// hideKeyboard puts the on-screen keyboard away when it is up over a focused text
// field, and reports whether it did.
func (n *Nav) hideKeyboard(c *ui.Context) bool {
	if c == nil || c.Keyboard().Bottom <= 0 {
		return false
	}
	v := c.View()
	if _, text := v.Focused().(ui.TextHandler); !text {
		return false
	}
	v.Focus(nil)

	return true
}

// Layout gives the screen the whole window, the tab bar its strip at the bottom
// when the screen has one, and the sheet and the toast hosts the whole window to
// place their own parts in.
func (n *Nav) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	full := ui.Tight(size)
	c.Layout(&n.host, full)
	c.Place(&n.host, geom.Point{})
	if n.keepNow {
		n.keepNow = false
		if s, ok := n.host.screen.(Scroller); ok {
			s.Scroll().ScrollTo(geom.Pt(0, n.keep), false)
		}
	}
	h := TabBarHeight(c)
	c.Layout(n.bar, ui.Tight(geom.Sz(size.W, h)))
	c.Place(n.bar, geom.Pt(0, size.H-h))
	c.Layout(n.sheet, full)
	c.Place(n.sheet, geom.Point{})
	c.Layout(n.toast, full)
	c.Place(n.toast, geom.Point{})

	return size
}

// Paint fills the window with --bg (`#m` background), then the screen, the tab
// bar and the overlays.
func (n *Nav) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(n.Bounds(), scene.Solid(theme.From(c).T.Bg), 0)
	n.Base.Paint(c, p)
}

// Layout gives the screen the whole room.
func (h *screenHost) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if h.screen != nil {
		c.Layout(h.screen, ui.Tight(cs.Max))
		c.Place(h.screen, geom.Point{})
	}

	return cs.Max
}
