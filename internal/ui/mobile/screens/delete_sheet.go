//go:build !tvos

// Pattern from the kitdemo's M14 page (pages_sheets.go, deleteSheet), which lays
// this content out at the boxes of M14.layout.json. The sheet is SH.del (index.html:
// 517-519).

package screens

import (
	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of SH.del (index.html:517-519), in DIPs.
const (
	// delDiscTop is `padding-top:10px` of the block (:517); delDisc the 64 disc,
	// delDiscIcon its `ic('trash',28)`; delDiscAlpha the `rgba(255,90,95,.16)`.
	delDiscTop, delDisc, delDiscIcon float32 = 10, 64, 28
	delDiscAlpha                     float32 = .16
	// delTitleSize, delTitleTop are the `.d` at `font-size:24px;margin-top:20px`.
	delTitleSize, delTitleTop float32 = 24, 20
	// delBodySize, delBodyLine, delBodyTop are the `p.muted` at `font-size:14px;
	// line-height:1.5;margin:12px 0 20px`; delBodyBottom is its 20 under it.
	delBodySize, delBodyLine, delBodyTop, delBodyBottom float32 = 14, 1.5, 12, 20
	// delCancelHeight, delCancelTop are `style="margin-top:10px;height:48px"`
	// of "Huỷ".
	delCancelHeight, delCancelTop float32 = 48, 10
)

// DeleteSheet is the content of SH.del (sheet 14, index.html:517-519): the red
// disc with the trash, the question, the lesson whose file would go, "Xoá khỏi
// máy" and "Huỷ". The button "Xoá khỏi máy" is `delConfirm`, which the Shell
// turns into the deletion (Actions.DeleteDownload); "Huỷ" closes the sheet.
type DeleteSheet struct {
	ui.Base

	// Params is the lesson whose file the sheet asks about.
	Params env.DeleteSheetParams

	col    *sheetColumn
	title  *kit.Label
	body   *kit.Label
	del    *kit.Button
	cancel *kit.Button
}

// deleteDisc is the 64 disc of the sheet that asks to delete: red at .16 with the
// trash icon in --dg (:517).
type deleteDisc struct {
	ui.Base

	icon *kit.Icon
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewDeleteSheet returns the content of sheet 14 for p. "Huỷ" closes the sheet
// through the navigator (Env.Nav).
func NewDeleteSheet(e *env.Env, p env.DeleteSheetParams) *DeleteSheet {
	s := &DeleteSheet{Params: p}
	s.title = heading(kit.Display(e.T("delete.title"), delTitleSize), 1)
	s.title.Align = ui.TextAlignCenter

	title, bytes := deletedFile(e, p)
	s.body = kit.Muted(e.Fill("delete.bodyTpl", "no", format.No(p.No), "title", title, "size", format.Size(bytes)), delBodySize)
	s.body.LineHeight, s.body.Align, s.body.Wrap, s.body.Name = delBodyLine, ui.TextAlignCenter, true, "p.muted"

	s.del = kit.NewButton(kit.ButtonR, e.T("delete.confirm"), icon.Trash)
	s.del.Name = "delConfirm"
	id := p.LessonID
	s.del.OnPress = func() { e.Act.DeleteDownload(id) }
	s.cancel = kit.NewButton(kit.ButtonS, e.T("common.cancel"), icon.NameUnknown)
	s.cancel.Name, s.cancel.Height = "closeSheet", delCancelHeight
	s.cancel.OnPress = func() {
		if e.Nav != nil {
			e.Nav.Sheet().Close()
		}
	}

	disc := &deleteDisc{icon: kit.NewIcon(icon.Trash, delDiscIcon, theme.InkDanger)}
	disc.Add(disc.icon)
	s.col = &sheetColumn{items: []sheetItem{
		{w: disc, before: delDiscTop, center: true},
		{w: s.title, before: delTitleTop},
		{w: s.body, before: delBodyTop},
		{w: s.del, before: delBodyBottom},
		{w: s.cancel, before: delCancelTop},
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
func (s *DeleteSheet) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = cs.Min.W
	}
	sz := c.Layout(s.col, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(s.col, geom.Point{})

	return cs.Constrain(geom.Sz(w, sz.H))
}

// Layout makes the disc a square and centers the icon.
func (d *deleteDisc) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(d.icon, ui.Loose(geom.Sz(delDiscIcon, delDiscIcon)))
	is := d.icon.Frame().Size()
	c.Place(d.icon, geom.Pt((delDisc-is.W)/2, (delDisc-is.H)/2))

	return cs.Constrain(geom.Sz(delDisc, delDisc))
}

// Paint draws the disc under the icon.
func (d *deleteDisc) Paint(c *ui.Context, p *ui.Painter) {
	r := d.Bounds()
	p.Fill(r, scene.Solid(theme.From(c).T.Danger.WithAlpha(delDiscAlpha)), delDisc/2)
	d.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// deletedFile returns the title and the size of the file the sheet asks about:
// the file's own (what the download kept), else the lesson's of the course the
// screens read; empty and 0 when neither is known.
func deletedFile(e *env.Env, p env.DeleteSheetParams) (title string, bytes int64) {
	for _, it := range e.Downloader().Items() {
		if it.LessonID == p.LessonID {
			return it.Title, it.Bytes
		}
	}
	if c, ok := e.CachedCourse(p.CourseID); ok {
		for _, l := range c.Lessons {
			if l.ID == p.LessonID {
				return l.Title, l.AudioSizeBytes
			}
		}
	}

	return "", 0
}
