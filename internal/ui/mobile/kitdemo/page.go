//go:build !tvos

package kitdemo

import (
	"dancefitvn-app/internal/ui/kit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Page is one prototype state drawn from the kit.
type Page struct {
	// ID is the prototype's state id ("M03", "M07"): the name of the shot's
	// files, which cmd/shotcmp pairs with the prototype's capture.
	ID string
	// Title says what the state is, for the gallery.
	Title string
	// TV marks a page of the 1920 x 1080 stage (Apple TV); the others are
	// phone pages.
	TV bool
	// Build draws the page from the mock data.
	Build func(d *Data) Screen
}

// Screen is a built page: the widget to mount and how to number what it
// leaves out of the prototype's capture.
type Screen struct {
	// Root is the widget, as large as the window.
	Root ui.Widget
	// Skip counts, per widget name, the elements of the prototype's capture that
	// come before the page's first one, because the page leaves them out (the
	// player behind the options sheet): layoutdump numbers from there.
	Skip map[string]int
}

// TVSize is the Apple TV stage (PLAN D8).
var TVSize = geom.Sz(1920, 1080)

// stage is a page under the overlays of the app's frame: the page, the sheet
// host, the toast host, in the order the CSS stacks them (z-index 71, 90).
type stage struct {
	ui.Base

	page  *canvas
	sheet *kit.Sheet
	toast *kit.Toast
	// open, when set, opens the page's sheet once the stage is on screen.
	open func(s *kit.Sheet)
}

var _ ui.Mounter = (*stage)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newStage returns a stage over page with both hosts empty.
func newStage(page *canvas) *stage {
	s := &stage{page: page, sheet: kit.NewSheet(), toast: kit.NewToast()}
	s.Add(page, s.sheet, s.toast)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Mounted opens the page's sheet on the first frame: a sheet opens on a view.
func (s *stage) Mounted(c *ui.Context) {
	if s.open != nil {
		c.AfterFunc(0, func() { s.open(s.sheet) })
	}
}

// Layout gives every layer the whole window.
func (s *stage) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	for ch := range s.Children() {
		c.Layout(ch, ui.Tight(cs.Max))
		c.Place(ch, geom.Point{})
	}

	return cs.Max
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// screen wraps a canvas with no overlays.
func screen(cv *canvas) Screen { return Screen{Root: cv} }
