//go:build tvos

package tvkit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/tv/tvtest"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// stage is a fixture root: the children at the points they were added at,
// each as large as it asks (loose), or tight when a size was given.
type stage struct {
	ui.Base
	at   []geom.Point
	size []geom.Size
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// put adds w at (x, y), as large as it asks.
func (s *stage) put(w ui.Widget, x, y float32) ui.Widget {
	s.at, s.size = append(s.at, geom.Pt(x, y)), append(s.size, geom.Size{})
	s.Add(w)

	return w
}

// putSized adds w at (x, y) with exactly the size.
func (s *stage) putSized(w ui.Widget, x, y, width, height float32) ui.Widget {
	s.at, s.size = append(s.at, geom.Pt(x, y)), append(s.size, geom.Sz(width, height))
	s.Add(w)

	return w
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout lays the children out at their points.
func (s *stage) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	i := 0
	for w := range s.Children() {
		if sz := s.size[i]; sz != (geom.Size{}) {
			c.Layout(w, ui.Tight(sz))
		} else {
			c.Layout(w, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
		}
		c.Place(w, s.at[i])
		i++
	}

	return cs.Constrain(cs.Max)
}

// texts returns the app's strings.
func texts() map[string]string { return locales.MustTexts() }

// mount mounts root at 1920x1080 with real fonts and the motion pinned.
func mount(t testing.TB, root ui.Widget) *ui.Tester {
	t.Helper()
	tt := tvtest.MountReal(t, root)
	tvtest.PinMotion(tt)

	return tt
}
