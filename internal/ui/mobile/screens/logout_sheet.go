//go:build !tvos

// Pattern from the kitdemo's M15-logout page (pages_sheets.go, logoutSheet),
// which lays this content out at the boxes of M15-logout.layout.json. The sheet
// is SH.logout (index.html:530).

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

// The measures of SH.logout (index.html:530), in DIPs.
const (
	// outTitleSize, outTitleTop are the `.d` at `font-size:24px` under the
	// sheet's `padding-top:10px`.
	outTitleSize, outTitleTop float32 = 24, 10
	// outBodySize, outBodyTop are the `p.muted` at `font-size:14px;margin:12px
	// 0 20px`; outBodyBottom is the 20 under it.
	outBodySize, outBodyTop, outBodyBottom float32 = 14, 12, 20
	// outCancelHeight, outCancelTop are `style="margin-top:10px;height:48px"` of
	// "Huỷ".
	outCancelHeight, outCancelTop float32 = 48, 10
)

// LogoutSheet is the content of SH.logout (index.html:530): "Đăng xuất?", the
// line that the downloaded files stay, the red "Đăng xuất" and "Huỷ". The red
// button is `logout`, which the Shell turns into the sign-out (Actions.Logout);
// "Huỷ" closes the sheet.
type LogoutSheet struct {
	ui.Base

	col    *sheetColumn
	title  *kit.Label
	body   *kit.Label
	out    *kit.Button
	cancel *kit.Button
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewLogoutSheet returns the content of SH.logout. "Huỷ" closes the sheet
// through the navigator (Env.Nav).
func NewLogoutSheet(e *env.Env) *LogoutSheet {
	s := &LogoutSheet{}
	s.title = heading(kit.Display(e.T("logout.title"), outTitleSize), 1)
	s.title.Align, s.title.Name = ui.TextAlignCenter, "div.d#2"
	s.body = kit.Muted(e.T("logout.body"), outBodySize)
	s.body.Align, s.body.Name = ui.TextAlignCenter, "p.muted#2"
	s.out = kit.NewButton(kit.ButtonR, e.T("logout.confirm"), icon.NameUnknown)
	s.out.Name, s.out.OnPress = "logout", e.Act.Logout // `data-act="logout"`
	s.cancel = kit.NewButton(kit.ButtonS, e.T("common.cancel"), icon.NameUnknown)
	s.cancel.Name, s.cancel.Height = "closeSheet#2", outCancelHeight
	s.cancel.OnPress = func() {
		if e.Nav != nil {
			e.Nav.Sheet().Close()
		}
	}
	s.col = &sheetColumn{items: []sheetItem{
		{w: s.title, before: outTitleTop},
		{w: s.body, before: outBodyTop},
		{w: s.out, before: outBodyBottom},
		{w: s.cancel, before: outCancelTop},
	}}
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
func (s *LogoutSheet) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = cs.Min.W
	}
	sz := c.Layout(s.col, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(s.col, geom.Point{})

	return cs.Constrain(geom.Sz(w, sz.H))
}
