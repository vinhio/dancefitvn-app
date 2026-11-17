//go:build !tvos

package screens

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// stubBack is the key of the stub's back button: `data-act="back"`.
const stubBack = "back"

// Stub is a screen that shows nothing but a back button: the placeholder of a
// route a later phase fills in (P3.3 to P6), so that the tabs and the actions
// that lead to it work now. Route says which one.
type Stub struct {
	ui.Base

	// Route is the id of the route the stub stands for.
	Route string
	// OnLeft, when set, runs when the screen is left (nav.Leaver): the video
	// stub's way of telling the player.
	OnLeft func()

	page *nav.Page
	tab  int
}

var (
	_ nav.Tabbed   = (*Stub)(nil)
	_ nav.Scroller = (*Stub)(nil)
	_ nav.Leaver   = (*Stub)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewStub returns the placeholder of route, showing the tab bar with tab current
// (-1: none).
func NewStub(e *env.Env, route string, tab int) *Stub {
	back := kit.NewRoundButton(icon.Cl, false, e.T("a11y.back"))
	back.Name = stubBack
	back.OnPress = e.Act.Back
	s := &Stub{Route: route, tab: tab}
	row := ui.Row(0, back)
	s.page = nav.NewPage(row, tab)
	s.Add(s.page)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Left tells OnLeft that the screen was left.
func (s *Stub) Left() {
	if s.OnLeft != nil {
		s.OnLeft()
	}
}

// Tab returns the tab the stub is the root of.
func (s *Stub) Tab() int { return s.tab }

// Scroll returns the page's scroll.
func (s *Stub) Scroll() *ui.Scroll { return s.page.Scroll() }

// Layout fills the window with the page.
func (s *Stub) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(s.page, ui.Tight(cs.Max))
	c.Place(s.page, geom.Point{})

	return cs.Max
}
