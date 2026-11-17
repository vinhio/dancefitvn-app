//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/views/form_link.go at 49de3c2 (a sheet
// with a form: one request at a time, the answer comes back through ui.Go and the
// sheet stays open on an error, the work dropped when the sheet closes) and from
// dancefitvn-app/internal/ui/mobile/screens/password_sheet.go (the sheet column
// and its toasts). The sheet is SH.avatar (index.html:520-526, M15D); the picture
// it crops is avatar.Decode's, the upload avatar.Export's (docs/plan/03 section 6,
// local/plans/P6.md sections 1.6 and 5).

package screens

import (
	"context"
	"errors"
	"fmt"
	"image"

	"dancefitvn-app/internal/avatar"
	"dancefitvn-app/internal/model"
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

// The measures of SH.avatar (index.html:520-526), in DIPs.
const (
	// avHintSize, avSideGap are `<p class="muted" style="font-size:12px;margin:12px 0">`
	// (the two lines of text over and under the slider): the text's size and the
	// 12 above and under it.
	avHintSize, avSideGap float32 = 12, 12
	// avZoomGap, avZoomSign are the slider row's `gap:12px` and the `<b class="muted">`
	// of its ends at the page's 16 px, 700.
	avZoomGap, avZoomSign float32 = 12, 16
	// avZoomRow is the row's height, the line of a 16 px `<b>`: 20.
	avZoomRow float32 = 20
	// avTrackMid is where the middle of the slider's track is under the slider's top:
	// 9 and half of its 6.
	avTrackMid float32 = 12
	// avPickSize, avPickIcon are `font-size:14px;font-weight:700` of the two links
	// and `ic('cam',18)`; avPickHeight their height, the line of the 18 icon on its
	// baseline: 21.
	avPickSize, avPickIcon, avPickHeight float32 = 14, 18, 21
	// avPickTop, avPickBottom are `margin:6px 0 16px` of the row of links; the 12
	// over it is the larger margin of the p over it (collapsed), avSideGap.
	avPickBottom float32 = 16
	// avButtonGap is `display:flex;gap:12px` of the two buttons; avButtonBorder the
	// 1 px (1.2 rounded down by Chrome) border of `.btn.g`, which the flex-shrink
	// counts in.
	avButtonGap, avButtonBorder float32 = 12, 1
)

// The names layout.json gives the parts of the sheet.
const (
	namePickPhoto = "pickPhoto" // `data-act="pickPhoto"` of both links
	nameAvatarOk  = "avatarOk"  // `data-act="avatarOk"`
	nameCloseSh   = "closeSheet"
	nameHint      = "p.muted"
	nameSign      = "b.muted"
)

// AvatarSheet is the content of SH.avatar (sheet 15D, M15D): the crop view with the
// avatar, the slider that zooms it, the two links that open the library and the
// buttons "Huỷ" and "Xác nhận". It opens on the prototype's view (zoom 1.2, no
// offset) over the user's current picture, or the person.
//
// A picture picked (PickPhoto, in ui.Go; decoded off the UI thread, shown once it is)
// is at zoom 1, offset 0. A finger on the crop view moves it with no limit, the
// slider zooms from 1 to 3 in steps of .01. "Xác nhận" cuts the 512 square
// (avatar.Export, off the UI thread), uploads it (UploadAvatar), closes the sheet with
// "Đã cập nhật ảnh đại diện" and has screen 15 read the account again; with no new
// picture it only closes (K25: the server keeps the crop, so cropping it again would
// zoom in each time). The sheet stays open on an error.
type AvatarSheet struct {
	ui.Base

	e   *env.Env
	col *sheetColumn

	crop           *kit.CropView
	zoom           *kit.Slider
	camera, lib    *inlineLink
	cancel, accept *kit.Button

	work    *image.RGBA // the picture picked, decoded and reduced; nil before one is
	version uint32      // counts the pictures shown, for scene.Image.Version

	picking, busy bool
	cancelPick    func()
	cancelSave    func()
	cancelLoad    func()
}

// zoomRow is the row of the slider: "–", the slider, "+" (`display:flex;align-items:
// center;gap:12px`).
type zoomRow struct {
	ui.Base

	minus, plus *kit.Label
	slider      *kit.Slider
}

// linksRow is `display:flex;justify-content:space-between` of the two links.
type linksRow struct {
	ui.Base

	left, right *inlineLink
}

// buttonsRow is `display:flex;gap:12px` of "Huỷ" and "Xác nhận", each `width:100%`
// shrunk to fit.
type buttonsRow struct {
	ui.Base

	left, right *kit.Button
}

var (
	_ ui.Mounter   = (*AvatarSheet)(nil)
	_ ui.Unmounter = (*AvatarSheet)(nil)
)

// pickResult is what the work of a pick brings back: the decoded picture, or why it
// could not be read (the pick's own failure is the work's error).
type pickResult struct {
	img *image.RGBA
	err error
}

// saveResult is what the work of a save brings back: the answer of the upload, or the
// failure of the export (the upload's own failure is the work's error).
type saveResult struct {
	avatar    model.Avatar
	exportErr error
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewAvatarSheet returns the content of SH.avatar (sheet 15D, `$P:520-526`, M15D),
// opened by Actions.AskAvatar at zoom 1.2 and no new picture; its cross and "Huỷ"
// close the sheet through the navigator (Env.Nav).
func NewAvatarSheet(e *env.Env) *AvatarSheet {
	s := &AvatarSheet{e: e}
	closeFn := func() {
		if e.Nav != nil {
			e.Nav.Sheet().Close()
		}
	}
	initial := avatar.Initial
	s.crop = kit.NewCropView(initial)
	s.crop.Label = e.T("avatar.title")
	s.zoom = kit.NewSlider(avatar.MinZoom, avatar.MaxZoom, avatar.ZoomStep, initial.Zoom) // `<input type="range" min="1" max="3" step="0.01" value="1.2">`
	s.zoom.Label = e.T("a11y.zoom")
	s.zoom.OnChange = s.zoomed
	s.camera = newPickLink(e.T("avatar.camera"), icon.Cam, s.pick)
	s.lib = newPickLink(e.T("avatar.library"), icon.NameUnknown, s.pick) // K7: both open the library
	s.cancel = kit.NewButton(kit.ButtonG, e.T("common.cancel"), icon.NameUnknown)
	s.cancel.Name, s.cancel.OnPress = nameCloseSh, closeFn
	s.accept = kit.NewButton(kit.ButtonR, e.T("avatar.confirm"), icon.NameUnknown)
	s.accept.Name, s.accept.OnPress = nameAvatarOk, s.confirm

	hint := kit.Muted(e.T("avatar.hint"), avHintSize)
	hint.Name, hint.Align = nameHint, ui.TextAlignCenter
	note := kit.Muted(e.T("avatar.note"), avHintSize)
	note.Name, note.Align = nameHint, ui.TextAlignCenter

	s.col = &sheetColumn{items: []sheetItem{
		{w: kit.NewSheetHeader(e.T("avatar.title"), e.T("a11y.close"), closeFn)},
		{w: s.crop, before: kit.SheetHeaderGap},
		{w: hint, before: avSideGap},
		{w: newZoomRow(e.T("avatar.zoomOut"), e.T("avatar.zoomIn"), s.zoom), before: avSideGap},
		{w: note, before: avSideGap},
		{w: newLinksRow(s.camera, s.lib), before: avSideGap},
		{w: newButtonsRow(s.cancel, s.accept), before: avPickBottom},
	}}
	for _, it := range s.col.items {
		s.col.Add(it.w)
	}
	s.Add(s.col)

	return s
}

// newZoomRow returns the row of the slider with the signs at its ends.
func newZoomRow(minus, plus string, slider *kit.Slider) *zoomRow {
	r := &zoomRow{slider: slider}
	for _, t := range []struct {
		l    **kit.Label
		text string
	}{{&r.minus, minus}, {&r.plus, plus}} {
		*t.l = &kit.Label{Text: t.text, Size: avZoomSign, Weight: 700, Ink: theme.InkText2}
		(*t.l).Name = nameSign
	}
	r.Add(r.minus, slider, r.plus)

	return r
}

// newLinksRow returns the row of the two links.
func newLinksRow(left, right *inlineLink) *linksRow {
	r := &linksRow{left: left, right: right}
	r.Add(left, right)

	return r
}

// newButtonsRow returns the row of the two buttons.
func newButtonsRow(left, right *kit.Button) *buttonsRow {
	r := &buttonsRow{left: left, right: right}
	r.Add(left, right)

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Crop returns the crop view.
func (s *AvatarSheet) Crop() *kit.CropView { return s.crop }

// Zoom returns the slider.
func (s *AvatarSheet) Zoom() *kit.Slider { return s.zoom }

// Camera returns the link "Chụp ảnh mới" (`pickPhoto`); Library the one "Chọn từ
// thư viện" (`pickPhoto#2`).
func (s *AvatarSheet) Camera() ui.Widget { return s.camera }

// Library returns the link "Chọn từ thư viện".
func (s *AvatarSheet) Library() ui.Widget { return s.lib }

// Cancel returns the button "Huỷ"; Accept the button "Xác nhận" (`avatarOk`).
func (s *AvatarSheet) Cancel() *kit.Button { return s.cancel }

// Accept returns the button "Xác nhận".
func (s *AvatarSheet) Accept() *kit.Button { return s.accept }

// Picked tells whether a new picture is in the crop view: what "Xác nhận" would
// upload.
func (s *AvatarSheet) Picked() bool { return s.work != nil }

// Picking tells whether the picker is open or the picture it gave is being decoded.
func (s *AvatarSheet) Picking() bool { return s.picking }

// Busy tells whether the picture is being cut and uploaded: a second tap is
// ignored.
func (s *AvatarSheet) Busy() bool { return s.busy }

// Mounted reads the user's current picture behind the sheet, with the next turn (a
// view being mounted has no host, and ui.Go would run the read on the UI thread
// then).
func (s *AvatarSheet) Mounted(c *ui.Context) {
	c.AfterFunc(0, s.loadCurrent)
}

// Unmounted drops what is under way: the sheet was closed.
func (s *AvatarSheet) Unmounted(*ui.Context) {
	for _, f := range []*func(){&s.cancelPick, &s.cancelSave, &s.cancelLoad} {
		if *f != nil {
			(*f)()
			*f = nil
		}
	}
}

// Layout puts the column at the sheet's width.
func (s *AvatarSheet) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = cs.Min.W
	}
	sz := c.Layout(s.col, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(s.col, geom.Point{})

	return cs.Constrain(geom.Sz(w, sz.H))
}

// Layout puts "–" and "+" at the ends and the slider between, the row as tall as the
// signs, the slider's track on the row's middle.
func (r *zoomRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	loose := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	ms := c.Layout(r.minus, loose)
	ps := c.Layout(r.plus, loose)
	h := max(ms.H, ps.H, avZoomRow)
	c.Place(r.minus, geom.Pt(0, centered(h, ms.H)))
	c.Place(r.plus, geom.Pt(w-ps.W, centered(h, ps.H)))
	x := ms.W + avZoomGap
	sw := max(w-ps.W-avZoomGap-x, 0)
	c.Layout(r.slider, ui.Constraints{Min: geom.Sz(sw, 0), Max: geom.Sz(sw, ui.Unbounded)})
	// the track (6 tall, 9 under the slider's top) is centred on the row
	c.Place(r.slider, geom.Pt(x, h/2-avTrackMid))

	return cs.Constrain(geom.Sz(w, h))
}

// Layout puts the links at the two ends, as tall as the row (`align-items:stretch`).
func (r *linksRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	loose := ui.Loose(geom.Sz(w, ui.Unbounded))
	ls := c.Layout(r.left, loose)
	rs := c.Layout(r.right, loose)
	c.Place(r.left, geom.Point{})
	c.Place(r.right, geom.Pt(w-rs.W, 0))

	return cs.Constrain(geom.Sz(w, max(ls.H, rs.H)))
}

// Layout shares the width between the buttons as flex-shrink does: both are as wide
// as the row, so each gives up a share of the excess as its inner width (the border
// is not counted) is of the sum of both.
func (r *buttonsRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	over := w + avButtonGap // 2w of the two bases, w - gap to hold them
	inner := 2*w - 2*avButtonBorder
	wl := w - over*(w-2*avButtonBorder)/inner
	wr := w - over*w/inner
	ls := c.Layout(r.left, ui.Constraints{Min: geom.Sz(wl, 0), Max: geom.Sz(wl, ui.Unbounded)})
	rs := c.Layout(r.right, ui.Constraints{Min: geom.Sz(wr, 0), Max: geom.Sz(wr, ui.Unbounded)})
	c.Place(r.left, geom.Point{})
	c.Place(r.right, geom.Pt(wl+avButtonGap, 0))

	return cs.Constrain(geom.Sz(w, max(ls.H, rs.H)))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// centered returns the y that puts a part of height h in the middle of outer.
func centered(outer, h float32) float32 { return (outer - h) / 2 }

// zoomed is the slider's `oninput` (`S.avatar.zoom=+z.value`, `applyCrop`): the
// picture scales about its centre, where it is.
func (s *AvatarSheet) zoomed(v float64) {
	view := s.crop.View
	view.Zoom = v
	s.crop.SetView(view)
}

// show puts a picture in the crop view at view and the slider at its zoom.
func (s *AvatarSheet) show(img *scene.Image, view avatar.View) {
	s.crop.SetImage(img)
	s.crop.SetView(view)
	s.zoom.SetValue(view.Zoom)
}

// loadCurrent reads the user's picture behind the sheet and shows it as long as no
// new one was picked; with no picture, or one that does not load, the person stays.
func (s *AvatarSheet) loadCurrent() {
	u, ok := s.e.AccountUser()
	if !ok || u.AvatarURL == "" || s.View() == nil {
		return
	}
	url := u.AvatarURL
	s.cancelLoad = ui.Go(s, func(ctx context.Context) (*scene.Image, error) {
		return s.e.Avatar(ctx, url)
	}, func(_ *ui.Context, img *scene.Image, err error) {
		s.cancelLoad = nil
		if err == nil && img != nil && s.work == nil {
			s.crop.SetImage(img)
		}
	})
}

// pick is `pickPhoto` (`#filePick`, index.html:628): the system's picker opens (K7:
// "Chụp ảnh mới" opens the library as well); the picture it gives is decoded off the
// UI thread. One at a time.
func (s *AvatarSheet) pick() {
	if s.picking || s.busy {
		return
	}
	s.picking = true
	s.cancelPick = ui.Go(s, func(ctx context.Context) (pickResult, error) {
		data, err := s.e.Account().PickPhoto(ctx)
		if err != nil {
			return pickResult{}, err
		}
		img, err := avatar.Decode(data)

		return pickResult{img: img, err: err}, nil
	}, s.picked)
}

// picked takes the answer of the picker: a picture is shown at zoom 1, offset 0
// (`S.avatar.zoom=1;x=0;y=0`); leaving the picker, a platform with none and a picker
// already open show nothing; a picture that cannot be read is a toast.
func (s *AvatarSheet) picked(_ *ui.Context, res pickResult, err error) {
	s.picking = false
	s.cancelPick = nil
	switch {
	case err != nil:
		if msg := s.e.PhotoToast(err); msg != "" {
			s.e.Act.Toast(msg)
		}
	case res.err != nil:
		s.e.Act.Toast(s.e.T("toast.photoUnreadable"))
	default:
		s.work = res.img
		s.version++
		s.show(&scene.Image{
			Pix: res.img.Pix, Stride: res.img.Stride, Width: res.img.Rect.Dx(), Height: res.img.Rect.Dy(),
			Version: uint64(s.version),
		}, avatar.Picked)
	}
}

// confirm is `avatarOk`: with no new picture the sheet only closes (K25); else the
// square under the ring is cut and uploaded off the UI thread, one at a time.
func (s *AvatarSheet) confirm() {
	if s.busy {
		return
	}
	if s.work == nil {
		if s.e.Nav != nil {
			s.e.Nav.Sheet().Close()
		}

		return
	}
	work, view := s.work, s.crop.View
	s.busy = true
	s.accept.SetDisabled(true)
	s.cancelSave = ui.Go(s, func(ctx context.Context) (saveResult, error) {
		jpg, err := avatar.Export(work, view)
		if err != nil {
			return saveResult{exportErr: fmt.Errorf("avatar sheet: %w", err)}, nil
		}
		av, err := s.e.Account().UploadAvatar(ctx, jpg)

		return saveResult{avatar: av}, err
	}, s.saved)
}

// saved takes the answer of the upload: success closes the sheet with its toast and
// has screen 15 read the account again (the new picture); a refusal leaves the sheet
// open with a toast that says what is wrong.
func (s *AvatarSheet) saved(_ *ui.Context, res saveResult, err error) {
	s.busy = false
	s.cancelSave = nil
	s.accept.SetDisabled(false)
	switch {
	case res.exportErr != nil:
		s.e.Act.Toast(s.e.T("common.errorTitle"))
	case err == nil:
		if s.e.Nav != nil {
			s.e.Nav.Sheet().Close()
			if r, ok := s.e.Nav.Screen().(interface{ Reload() }); ok {
				r.Reload()
			}
		}
		s.e.Act.Toast(s.e.T("toast.avatarSaved"))
	case errors.Is(err, context.Canceled) || s.e.Refused(err):
	default:
		if msg := s.e.FailureToast(env.FailureOf(err)); msg != "" {
			s.e.Act.Toast(msg)
		}
	}
}
