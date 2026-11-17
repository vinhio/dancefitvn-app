//go:build !tvos

package screens

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// StubSheet is the placeholder of a sheet a later phase fills in (SH.password,
// SH.avatar, the sheet of devices): a title and "Huỷ" in the column of a sheet,
// so that the buttons that open the sheet work now. ID is the env.Sheet... id of
// the sheet it stands for. A phase that writes the sheet replaces the file that
// makes it (P6.1, P6.3b, P6.4) and its constructor.
type StubSheet struct {
	ui.Base

	// ID is the id of the sheet the stub stands for (env.SheetPassword...).
	ID string

	col *sheetColumn
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewStubSheet returns the placeholder of sheet id, titled with the text of
// titleKey; "Huỷ" closes the sheet through the navigator.
func NewStubSheet(e *env.Env, id, titleKey string) *StubSheet {
	s := &StubSheet{ID: id}
	title := heading(kit.Display(e.T(titleKey), stubSheetTitle), 1)
	title.Align = ui.TextAlignCenter
	cancel := kit.NewButton(kit.ButtonS, e.T("common.cancel"), icon.NameUnknown)
	cancel.Name, cancel.Height = "closeSheet", stubSheetCancel
	cancel.OnPress = func() {
		if e.Nav != nil {
			e.Nav.Sheet().Close()
		}
	}
	s.col = &sheetColumn{items: []sheetItem{{w: title, before: stubSheetTop}, {w: cancel, before: stubSheetGap}}}
	for _, it := range s.col.items {
		s.col.Add(it.w)
	}
	s.Add(s.col)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout puts the column at the sheet's width.
func (s *StubSheet) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = cs.Min.W
	}
	sz := c.Layout(s.col, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(s.col, geom.Point{})

	return cs.Constrain(geom.Sz(w, sz.H))
}

// The measures of the stub sheet: the title's size (a `.d` of 24, as SH.del and
// SH.logout), the room over it and over "Huỷ", whose height is the 48 of the
// secondary button of a sheet (`style="margin-top:10px;height:48px"`).
const (
	stubSheetTitle  float32 = 24
	stubSheetTop    float32 = 10
	stubSheetGap    float32 = 20
	stubSheetCancel float32 = 48
)
