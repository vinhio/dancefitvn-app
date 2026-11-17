//go:build !tvos

package kitdemo

import (
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Gallery is the root of the kitdemo build: a list of the phone pages, a tap
// opens one, Back (the system's, or the Android key) comes back to the list. It
// is for looking at the kit on a simulator or an emulator; the picture the
// tests write does not depend on it.
type Gallery struct {
	ui.Base

	data *Data
	list *ui.Scroll
	page ui.Widget
	used bool // the press of Back that was taken
}

var _ ui.KeyInterceptor = (*Gallery)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewGallery returns the gallery over the data; open, when it names a page
// (the DFV_KIT of the app), opens that page at once.
func NewGallery(d *Data, open string) *Gallery {
	g := &Gallery{data: d}
	var items []item
	for _, p := range Pages() {
		if p.TV {
			continue
		}
		r := kit.NewRow(0, p.ID, p.Title)
		r.Name = "page:" + p.ID
		r.Chevron = true
		r.OnPress = func() { g.Open(p.ID) }
		items = append(items, item{w: r})
	}
	g.list = ui.NewScroll(newVStack(items...))
	g.list.HideBars = true
	g.Add(g.list)
	if open != "" {
		g.Open(open)
	}

	return g
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Open shows the page with the id and tells whether there is one.
func (g *Gallery) Open(id string) bool {
	p, ok := Find(id)
	if !ok || p.TV {
		return false
	}
	g.closePage()
	g.page = p.Build(g.data).Root
	kit.SetHidden(g.list, true)
	g.Add(g.page)
	g.Relayout()

	return true
}

// Back goes from a page to the list and tells whether there was a page.
func (g *Gallery) Back() bool {
	if g.page == nil {
		return false
	}
	g.closePage()
	kit.SetHidden(g.list, false)
	g.Relayout()

	return true
}

// InterceptKey takes Back while a page is open; its release too.
func (g *Gallery) InterceptKey(_ *ui.Context, e *ui.KeyEvent) bool {
	if e.Key != keys.KeyBack {
		return false
	}
	if !e.Down {
		used := g.used
		g.used = false

		return used
	}
	if !e.Repeat {
		g.used = g.Back()
	}

	return g.used
}

// Layout gives the open page the whole window, or the list the window under
// the safe area.
func (g *Gallery) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	room := cs.Max
	if g.page != nil {
		c.Layout(g.page, ui.Tight(room))
		c.Place(g.page, geom.Point{})

		return room
	}
	top := c.SafeArea().Top
	c.Layout(g.list, ui.Tight(geom.Sz(room.W-40, room.H-top)))
	c.Place(g.list, geom.Pt(20, top))

	return room
}

// Paint fills the window with --bg.
func (g *Gallery) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(g.Bounds(), scene.Solid(theme.From(c).T.Bg), 0)
	g.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// closePage takes the open page out of the tree.
func (g *Gallery) closePage() {
	if g.page != nil {
		g.Remove(g.page)
		g.page = nil
	}
}

// Pages returns every page, phone pages first, in the order of the plan.
func Pages() []Page {
	return []Page{
		pageM02(), pageM02B(), pageM03(), pageM04(), pageM05(), pageToast(), pageM07(), pageM09(), pageM13(),
		pageM14(), pageM15(), pageLogout(), pageM15B(), pageM16(), pageM16Code(), pagePosters(), pageIcons(), pageTV05(),
	}
}

// Find returns the page with the id.
func Find(id string) (Page, bool) {
	for _, p := range Pages() {
		if p.ID == id {
			return p, true
		}
	}

	return Page{}, false
}
