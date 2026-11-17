//go:build !tvos

// Adapted from download_sheet.go in this package (a sheet of radio rows over a
// column: the choice moves with a press, the button at the end acts on it) and
// from jplayer-mobile/internal/ui/views/form_link.go at 49de3c2 (one request at a
// time, the answer through ui.Go, the sheet staying open on an error). The sheet
// has no prototype design (docs/plan/03 section 9, Q-P6-15): it is made of the kit's
// sheet header, radio rows (the `.radio` of SH.download with a device icon),
// `.btn.r` and `.btn.s`.

package screens

import (
	"context"
	"errors"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of the sheet of devices, in DIPs. There is no prototype design: the
// numbers are the ones of SH.download (index.html:511-516) and SH.logout (:530)
// that the parts come from.
const (
	// devSubSize is the line under the header, a `p.muted` at `font-size:14px`;
	// devSubBottom the room under it before the first row.
	devSubSize, devSubBottom float32 = 14, 14
	// devRemoveTop is the room before the red button (`margin-top` of a `.btn` after
	// the rows' `margin-bottom:10px`); devCancelHeight, devCancelTop the `style=
	// "margin-top:10px;height:48px"` of "Huỷ" (:530).
	devRemoveTop, devCancelHeight, devCancelTop float32 = 16, 48, 10
)

// The names layout.json gives the parts of the sheet.
const (
	nameDevChoice = "devChoice:" // + the id of the device, as `dlChoice:one`
	nameDevRemove = "devRemove"
	// nameDevCancel is `closeSheet` once more: the backdrop and the cross are the first two
	nameDevCancel = "closeSheet#3"
)

// DevicesSheet is the content of the sheet of devices (docs/plan/03 section 9): a TV
// code that the server refused for the limit of devices, and the devices it counts,
// one to be removed. "Gỡ thiết bị" removes the chosen one (DELETE /me/devices/{id},
// a device already gone is gone all the same) and confirms the same code again; a
// TV linked closes the sheet and the screen under it shows the linked state with
// "Đã kết nối Apple TV". The device the phone is is listed but cannot be chosen
// ("Máy này"): removing it would sign the phone out. The first TV that is not
// this phone is chosen at first, else the first device that is not.
//
// The same sheet serves a sign-in that the server refused for the limit of devices
// (K31, params.Login): the list is the phones of the error, most recent first; no
// device is chosen at first, since a choice revokes a device; the button reads
// "Gỡ và đăng nhập" and sends the login again with the chosen device to be replaced
// (replace_device_id), the server revoking it once the password is verified. The
// outcome goes to the login screen (LoginReplace.Done), a close without one tells it
// too (LoginReplace.Cancel).
type DevicesSheet struct {
	ui.Base

	e      *env.Env
	params env.DevicesSheetParams

	col    *sheetColumn
	rows   []*kit.Radio
	ids    []string // the device of each row
	remove *kit.Button
	chosen int // index of the chosen row, -1 for none

	busy    bool
	cancel  func()
	settled bool // the login outcome or its cancel was handed over (login sheet)
}

var _ ui.Unmounter = (*DevicesSheet)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewDevicesSheet returns the content of the sheet of devices (03 section 9; no
// prototype design, Q-P6-15) for the code and the devices of p. Its cross and "Huỷ"
// close the sheet through the navigator (Env.Nav).
func NewDevicesSheet(e *env.Env, p env.DevicesSheetParams) *DevicesSheet {
	s := &DevicesSheet{e: e, params: p, chosen: -1}
	label := e.T("devices.remove")
	if p.Login != nil {
		label = e.T("devices.removeAndLogin")
	}
	s.remove = kit.NewButton(kit.ButtonR, label, icon.NameUnknown)
	s.remove.Name, s.remove.OnPress = nameDevRemove, s.submit
	s.build(p.Devices)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Code returns the code the sheet confirms again.
func (s *DevicesSheet) Code() string { return s.params.Code }

// Rows returns the radio rows, one for each device, in the order of the list.
func (s *DevicesSheet) Rows() []*kit.Radio { return s.rows }

// Chosen returns the id of the device chosen, "" for none.
func (s *DevicesSheet) Chosen() string {
	if s.chosen < 0 {
		return ""
	}

	return s.ids[s.chosen]
}

// RemoveButton returns the button "Gỡ thiết bị".
func (s *DevicesSheet) RemoveButton() *kit.Button { return s.remove }

// Busy tells whether a removal or a confirmation is under way: a second tap is
// ignored.
func (s *DevicesSheet) Busy() bool { return s.busy }

// Login tells whether the sheet is a sign-in's (K31).
func (s *DevicesSheet) Login() bool { return s.params.Login != nil }

// Unmounted drops the request under way: the sheet was closed. A sheet of a
// sign-in closed with no outcome tells the form.
func (s *DevicesSheet) Unmounted(*ui.Context) {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if l := s.params.Login; l != nil && !s.settled {
		s.settled = true
		if l.Cancel != nil {
			l.Cancel()
		}
	}
}

// Layout puts the column at the sheet's width.
func (s *DevicesSheet) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = cs.Min.W
	}
	sz := c.Layout(s.col, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(s.col, geom.Point{})

	return cs.Constrain(geom.Sz(w, sz.H))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// build makes the column for the devices: the header, the line under it, a row
// for each device, the red button and "Huỷ". The rows are made again when the
// server answers a limit once more with another list.
func (s *DevicesSheet) build(devices []model.Device) {
	e := s.e
	if s.col != nil {
		s.col.Remove(s.remove) // the button is the new column's
		s.Remove(s.col)
	}
	closeFn := s.closeSheet
	sub := kit.Muted(e.T("login.err.deviceSub"), devSubSize)
	sub.Name = nameConnLead
	cancel := kit.NewButton(kit.ButtonS, e.T("common.cancel"), icon.NameUnknown)
	cancel.Name, cancel.Height, cancel.OnPress = nameDevCancel, devCancelHeight, closeFn

	s.rows, s.ids, s.chosen = s.rows[:0], s.ids[:0], -1
	items := []sheetItem{
		{w: kit.NewSheetHeader(e.T("login.err.deviceTitle"), e.T("a11y.close"), closeFn)},
		{w: sub, before: kit.SheetHeaderGap},
	}
	here := s.currentIDs()
	if s.params.Login != nil {
		here = nil // signed out: the account view is a past session's, no device here is this phone's
	}
	for i, d := range devices {
		r := kit.NewRadio(d.Name, s.lastSeen(d), "")
		r.Name = nameDevChoice + d.ID
		r.Icon = icon.Phone
		if d.Platform == model.PlatformTVOS {
			r.Icon = icon.Tv
		}
		if d.Current || here[d.ID] {
			r.Trailing, r.Disabled = e.T("devices.thisDevice"), true
		} else {
			r.OnPress = func() { s.choose(i) }
		}
		before := kit.RadioGap
		if i == 0 {
			before = devSubBottom
		}
		items = append(items, sheetItem{w: r, before: before})
		s.rows = append(s.rows, r)
		s.ids = append(s.ids, d.ID)
	}
	items = append(items,
		sheetItem{w: s.remove, before: devRemoveTop},
		sheetItem{w: cancel, before: devCancelTop},
	)
	s.col = &sheetColumn{items: items}
	for _, it := range items {
		s.col.Add(it.w)
	}
	s.Add(s.col)
	if s.params.Login == nil { // a sign-in chooses nothing for the user: a choice revokes a device
		s.choose(s.first(devices, here))
	}
	s.remove.SetDisabled(s.chosen < 0)
}

// currentIDs returns the ids of the devices the account view knows to be this
// phone: the 409's list does not say.
func (s *DevicesSheet) currentIDs() map[string]bool {
	here := map[string]bool{}
	for _, d := range s.e.AccountView().Devices.Items {
		if d.Current {
			here[d.ID] = true
		}
	}

	return here
}

// first returns the index of the device chosen at first: the first TV that is not
// this phone, else the first device that is not; -1 when every one is.
func (s *DevicesSheet) first(devices []model.Device, here map[string]bool) int {
	pick := -1
	for i, d := range devices {
		if d.Current || here[d.ID] {
			continue
		}
		if d.Platform == model.PlatformTVOS {
			return i
		}
		if pick < 0 {
			pick = i
		}
	}

	return pick
}

// lastSeen is the sub line of a device, "Hoạt động 13/11/2026"; empty when the
// server did not say.
func (s *DevicesSheet) lastSeen(d model.Device) string {
	if d.LastSeenAt.IsZero() {
		return ""
	}

	return s.e.Fill("devices.lastSeenTpl", "date", format.Date(d.LastSeenAt))
}

// choose moves the choice to the row i; a row that cannot be chosen (this phone)
// and the same row change nothing.
func (s *DevicesSheet) choose(i int) {
	if s.busy || i < 0 || i >= len(s.rows) || s.rows[i].Disabled {
		return
	}
	s.chosen = i
	for j, r := range s.rows {
		r.SetOn(j == i)
	}
	s.remove.SetDisabled(false)
}

// closeSheet closes the sheet through the navigator.
func (s *DevicesSheet) closeSheet() {
	if s.e.Nav != nil {
		s.e.Nav.Sheet().Close()
	}
}

// submit is "Gỡ thiết bị": the chosen device is removed, then the code is
// confirmed again, one request at a time. For a sign-in it is "Gỡ và đăng nhập":
// the login is sent again with the device to be replaced.
func (s *DevicesSheet) submit() {
	id := s.Chosen()
	if s.busy || id == "" {
		return
	}
	s.busy = true
	if l := s.params.Login; l != nil {
		s.cancel = ui.Go(s, func(ctx context.Context) (struct{}, error) {
			return struct{}{}, l.Retry(ctx, id)
		}, func(_ *ui.Context, _ struct{}, err error) { s.loginDone(l, err) })

		return
	}
	code := s.params.Code
	s.cancel = ui.Go(s, func(ctx context.Context) (removal, error) {
		if err := s.e.Account().DeleteDevice(ctx, id); err != nil {
			return removal{}, err
		}
		_, err := s.e.Account().PairConfirm(ctx, code)

		return removal{gone: true}, err
	}, func(_ *ui.Context, r removal, err error) { s.done(id, r.gone, err) })
}

// removal is what the work of submit hands back with its error: whether the device
// is gone, which tells a failed confirmation from a failed removal.
type removal struct{ gone bool }

// done takes the answer of the removal and of the confirmation after it: a TV linked
// closes the sheet; a failure of the removal leaves it open with the reason; a code
// that is refused now (expired, wrong) closes it and tells the screen under it, which
// marks the boxes; a limit again shows the devices the server lists now.
func (s *DevicesSheet) done(id string, gone bool, err error) {
	s.busy = false
	s.cancel = nil
	if err == nil {
		s.closeSheet()
		s.tell(nil)

		return
	}
	if errors.Is(err, context.Canceled) || s.e.Refused(err) {
		return
	}
	fail := env.FailureOf(err)
	switch {
	case fail.Kind == env.FailureDeviceLimit:
		s.build(withoutDevice(fail.Devices, id))
	case gone:
		s.closeSheet()
		s.tell(err)
	default: // the removal failed: the sheet stays for another try
		if msg := s.e.FailureToast(fail); msg != "" {
			s.e.Act.Toast(msg)
		}
	}
}

// loginDone takes the answer of the login sent again (K31). A sign-in closes the sheet and
// goes on; a limit again shows the phones the server lists now; the id the server does not
// accept (400 on replace_device_id: the list is out of date, say the device went from
// another place) closes the sheet with the toast, the banner of the form staying to start
// again with a fresh list; no network leaves the sheet for another try; a wrong password, a
// locked account or the server in trouble are the form's.
func (s *DevicesSheet) loginDone(l *env.LoginReplace, err error) {
	s.busy = false
	s.cancel = nil
	if err == nil {
		s.finish(l, nil)

		return
	}
	if errors.Is(err, context.Canceled) || s.e.Refused(err) {
		return
	}
	fail := env.FailureOf(err)
	switch fail.Kind {
	case env.FailureDeviceLimit:
		if list := env.ReplaceableDevices(fail.Devices); len(list) > 0 {
			s.build(list)

			return
		}
		s.finish(l, err)
	case env.FailureOffline:
		s.e.Act.Toast(s.e.FailureToast(fail))
	case env.FailureValidation:
		s.e.Act.Toast(s.e.FailureToast(fail))
		s.dismiss(l) // the form shows nothing of its own: the toast said it
	default:
		s.finish(l, err)
	}
}

// finish closes the sheet and hands the outcome to the form: nil is the sign-in, an error is
// the form's to show.
func (s *DevicesSheet) finish(l *env.LoginReplace, err error) {
	s.settled = true
	s.closeSheet()
	l.Done(err)
}

// dismiss closes the sheet and tells the form it ended with nothing for it to show.
func (s *DevicesSheet) dismiss(l *env.LoginReplace) {
	s.settled = true
	s.closeSheet()
	if l.Cancel != nil {
		l.Cancel()
	}
}

// tell hands the outcome of the confirmation to the screen under the sheet, which
// shows it as it shows its own (the linked state and its toast, the boxes in red and
// the toast of the refusal); with none under the sheet it is a toast.
func (s *DevicesSheet) tell(err error) {
	if s.e.Nav != nil {
		if scr, ok := s.e.Nav.Screen().(codeOutcome); ok {
			scr.paired(err)

			return
		}
	}
	if err == nil {
		s.e.Act.Toast(s.e.T("toast.tvLinked"))

		return
	}
	if msg := s.e.FailureToast(env.FailureOf(err)); msg != "" {
		s.e.Act.Toast(msg)
	}
}

// withoutDevice returns the list without the device (a server that lists the one
// just removed still counts it).
func withoutDevice(list []model.Device, id string) []model.Device {
	out := make([]model.Device, 0, len(list))
	for _, d := range list {
		if d.ID != id {
			out = append(out, d)
		}
	}

	return out
}
