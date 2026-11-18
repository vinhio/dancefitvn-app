//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/views/form_link.go at 49de3c2 (a form
// with one request at a time: the answer comes back through ui.Go, a stale one is
// dropped, the screen stays on an error) and from dancefitvn-app/internal/ui/
// mobile/kitdemo/pages_connect.go (the widgets and numbers of M16). The screen is
// MS.connect (index.html:496-502) over POST /tv/pair/confirm, GET /me/devices and
// DELETE /me/devices/{id}.

package screens

import (
	"context"
	"errors"
	"slices"
	"time"

	"dancefitvn-app/internal/model"
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

// Connect is MS.connect, screen 16, "Kết nối Apple TV": with no TV linked the picture
// of a TV showing a code, "Nhập mã trên TV", the six boxes of the code and the fixed
// "Kết nối TV" (M16, M16-code); with one linked the green disc, "Đã kết nối TV", the
// account the TV uses and "Ngắt kết nối" (M16-linked). The state follows
// Env.TVDevice, read again while the screen is mounted.
//
// "Kết nối TV" does nothing while fewer than six characters are typed. It sends
// the code (POST /tv/pair/confirm, one request at a time): a TV linked shows the
// linked state with "Đã kết nối Apple TV"; a code the server refuses marks the
// boxes red with the toast "Mã không đúng. Kiểm tra lại mã trên TV" and keeps the
// code; one that ran out (410) says so in a toast; a TV over the limit of devices
// opens the sheet of devices (DevicesSheet), which confirms the code again once a
// device is gone. "Ngắt kết nối" removes the TV (DELETE /me/devices/{id}) and
// shows the empty code again.
type Connect struct {
	ui.Base

	e    *env.Env
	page *nav.Page
	body *connectBody

	// the parts of the unlinked state
	tv    *tvPicture
	title *kit.Label
	lead  *kit.Label
	boxes *kit.CodeBoxes
	link  *kit.Button // fixed over the page's end
	// the parts of the linked state
	ok     *okDisc
	done   *kit.Label
	user   *kit.Label
	unlink *kit.Button

	linked bool
	busy   bool
	cancel func()
	// tvs are the ids of the TVs the last answer of GET /me/devices listed: the
	// server's word, kept on the screen so that "Ngắt kết nối" does not depend on
	// the kept account view, which a sign-out forgets and a late answer may miss.
	tvs []string
	// retried tells whether a failed read of the devices was read again once.
	retried bool
	retry   ui.Timer
	// dismiss is the turn that takes the keyboard down after the sixth character,
	// dismissFn what it runs (made once: a keystroke allocates nothing).
	dismiss   ui.Timer
	dismissFn func()
}

// connectBody stacks the parts of the page.
type connectBody struct {
	ui.Base

	stack blockStack
}

// codeOutcome is what the sheet of devices tells the screen after it confirmed the
// code itself: nil for a TV linked, else why it was not.
type codeOutcome interface {
	paired(err error)
}

var (
	_ nav.Tabbed   = (*Connect)(nil)
	_ nav.Scroller = (*Connect)(nil)
	_ ui.Mounter   = (*Connect)(nil)
	_ ui.Unmounter = (*Connect)(nil)
	_ codeOutcome  = (*Connect)(nil)
)

// connRetryAfter is how long after a failed read of the devices the screen reads
// them once more.
const connRetryAfter = 2 * time.Second

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewConnect returns screen 16 (MS.connect, `$P:496-502`, M16, M16-code and
// M16-linked), no tab bar, in the state the account view holds.
func NewConnect(e *env.Env) *Connect {
	s := &Connect{e: e}
	s.dismissFn = s.dismissKeyboard
	s.tv = newTVPicture(e.T("connect.codeLabel"), e.T("connect.sampleCode"))
	s.title = heading(kit.Display(e.T("connect.title"), connTitleSize), 1)
	s.title.Align = ui.TextAlignCenter
	s.lead = kit.Muted(e.T("connect.lead"), connLeadSize)
	s.lead.LineHeight, s.lead.Align, s.lead.Wrap, s.lead.Name = connLeadLine, ui.TextAlignCenter, true, nameConnLead
	s.boxes = kit.NewCodeBoxes(e.T("connect.codeLabel"))
	s.boxes.OnChange = s.typed
	s.link = kit.NewButton(kit.ButtonP, e.T("connect.button"), icon.Tv)
	s.link.Name, s.link.OnPress = nameLinkTV, s.confirm
	s.link.SetDisabled(true) // `opacity:.45` while there are fewer than 6 characters

	s.ok = newOkDisc()
	s.done = heading(kit.Display(e.T("connect.linkedTitle"), connDoneSize), 1)
	s.done.Align = ui.TextAlignCenter
	s.user = kit.Muted("", connUserSize)
	s.user.Align, s.user.Wrap, s.user.Name = ui.TextAlignCenter, true, nameConnLead
	s.unlink = kit.NewButton(kit.ButtonG, e.T("connect.unlink"), icon.NameUnknown)
	s.unlink.Name, s.unlink.OnPress = nameUnlinkTV, s.disconnect

	s.body = &connectBody{}
	s.body.stack.items = []blockItem{
		{w: kit.NewTopBar(e.T("account.connect"), e.T("a11y.back"), e.Act.Back)},
		{w: s.tv, top: connTVTop},
		{w: s.title, top: connTitleTop},
		{w: s.lead, top: connLeadTop, bottom: connLeadBottom},
		{w: s.boxes},
		{w: s.ok, top: connOkTop},
		{w: s.done, top: connDoneTop},
		{w: s.user, top: connUserTop, bottom: connUserBottom},
		{w: s.unlink},
	}
	for _, it := range s.body.stack.items {
		s.body.stack.Add(it.w)
	}
	s.body.Add(&s.body.stack)
	s.page = nav.NewPage(s.body, -1)
	s.Add(s.page, s.link)
	_, s.linked = e.TVDevice()
	s.show()

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Tab is -1: no tab bar.
func (s *Connect) Tab() int { return -1 }

// Scroll returns the page's scroll.
func (s *Connect) Scroll() *ui.Scroll { return s.page.Scroll() }

// Boxes returns the six boxes of the code.
func (s *Connect) Boxes() *kit.CodeBoxes { return s.boxes }

// Link returns the button "Kết nối TV" (`linkTV`).
func (s *Connect) Link() *kit.Button { return s.link }

// Unlink returns the button "Ngắt kết nối" (`unlinkTV`).
func (s *Connect) Unlink() *kit.Button { return s.unlink }

// Linked tells whether the linked state is showing.
func (s *Connect) Linked() bool { return s.linked }

// Busy tells whether a code or a removal is being sent: a second tap is ignored.
func (s *Connect) Busy() bool { return s.busy }

// Mounted reads GET /me/devices, with the next turn: a view being mounted has no
// host, and ui.Go would run the read on the UI thread then.
func (s *Connect) Mounted(c *ui.Context) {
	c.AfterFunc(0, s.load)
}

// Unmounted ends the read and the request under way: leaving the screen drops them.
func (s *Connect) Unmounted(*ui.Context) {
	s.dismiss.Stop()
	s.retry.Stop()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// Layout fills the window with the page and puts the button over its end.
func (s *Connect) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(s.page, ui.Tight(cs.Max))
	c.Place(s.page, geom.Point{})
	if !s.link.Hidden {
		w := max(cs.Max.W-2*connButtonSide, 0)
		bs := c.Layout(s.link, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
		c.Place(s.link, geom.Pt(connButtonSide, cs.Max.H-connButtonBottom-bs.H))
	}

	return cs.Max
}

// Layout is the stack's.
func (b *connectBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	sz := c.Layout(&b.stack, cs)
	c.Place(&b.stack, geom.Point{})

	return cs.Constrain(sz)
}

// paired takes what the sheet of devices found out when it confirmed the code once a
// device was gone (codeOutcome).
func (s *Connect) paired(err error) { s.confirmed(nil, model.PairConfirm{}, err) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// show puts on the state s.linked says: the parts of the other one are hidden and
// take no room.
func (s *Connect) show() {
	for _, w := range []ui.Widget{s.tv, s.title, s.lead, s.boxes, s.link} {
		kit.SetHidden(w, s.linked)
	}
	for _, w := range []ui.Widget{s.ok, s.done, s.user, s.unlink} {
		kit.SetHidden(w, !s.linked)
	}
	if s.linked {
		email := ""
		if u, ok := s.e.AccountUser(); ok {
			email = u.Email
		}
		s.user.SetText(s.e.Fill("connect.linkedTpl", "email", email))
	}
	s.page.Relayout()
	s.Relayout()
}

// setLinked changes the state, which the answers of the server do.
func (s *Connect) setLinked(on bool) {
	if on != s.linked {
		s.linked = on
		s.show()
	}
}

// typed is `codeInp`'s oninput (`$P:570`): the button takes its colour back at six
// characters. The prototype is a web page that scrolls above its keyboard; here the
// keyboard covers "Kết nối TV", fixed over the page's end (3.7), so it goes down
// with the sixth character, on the next turn (the field is still handling the
// edit). A tap on a box brings it back to correct the code.
func (s *Connect) typed(code string) {
	full := len(code) >= kit.CodeLength
	s.link.SetDisabled(!full)
	s.dismiss.Stop()
	if !full {
		return
	}
	if v := s.View(); v != nil {
		s.dismiss = v.Context().AfterFunc(0, s.dismissFn)
	}
}

// dismissKeyboard takes the keyboard down once the sixth character is in.
func (s *Connect) dismissKeyboard() {
	if len(s.boxes.Value) >= kit.CodeLength && s.boxes.Field().Node().Focused() {
		s.blur()
	}
}

// blur takes the focus away, which hides the keyboard.
func (s *Connect) blur() {
	if v := s.View(); v != nil {
		v.Focus(nil)
	}
}

// load reads GET /me/devices and shows the state it tells; an answer that fails
// leaves the screen as it is.
func (s *Connect) load() {
	s.cancel = ui.Go(s, func(ctx context.Context) (model.DeviceList, error) {
		return s.e.Account().Devices(ctx)
	}, func(c *ui.Context, d model.DeviceList, err error) {
		s.cancel = nil
		switch {
		case err == nil:
			// the state is what this answer says, not what the kept view holds: a
			// sign-out empties the view, and an answer that lands over a write of
			// the list (or after a reset) is not kept
			s.tvs = tvIDs(d.Items)
			if !s.busy {
				s.setLinked(len(s.tvs) > 0)
			}
		case errors.Is(err, context.Canceled):
		case s.e.Refused(err):
		default:
			// the screen stays as it is, and asks once more shortly: the first read
			// after a sign-in may meet a network that is still coming back
			if !s.retried && c != nil {
				s.retried = true
				s.retry = c.AfterFunc(connRetryAfter, s.load)
			}
		}
	})
}

// confirm is `linkTV` (`$P:622`): fewer than six characters do nothing; else the
// code goes to the server, one request at a time.
func (s *Connect) confirm() {
	code := s.boxes.Value
	if s.busy || len(code) < kit.CodeLength {
		return
	}
	s.busy = true
	s.dismiss.Stop()
	s.blur()
	if s.cancel != nil {
		s.cancel() // a read of the devices that is still out must not undo the link
	}
	s.cancel = ui.Go(s, func(ctx context.Context) (model.PairConfirm, error) {
		return s.e.Account().PairConfirm(ctx, code)
	}, s.confirmed)
}

// confirmed takes the answer of POST /tv/pair/confirm: success is the linked state
// with its toast; a code the server refuses stays in the boxes and the toast says so; a TV over
// the limit opens the sheet of devices, with the code to confirm again.
func (s *Connect) confirmed(_ *ui.Context, res model.PairConfirm, err error) {
	s.busy = false
	s.cancel = nil
	if err == nil {
		if id := res.Device.ID; id != "" && !slices.Contains(s.tvs, id) {
			s.tvs = append(s.tvs, id)
		}
		s.setLinked(true)
		s.e.Act.Toast(s.e.T("toast.tvLinked"))

		return
	}
	if errors.Is(err, context.Canceled) || s.e.Refused(err) {
		return
	}
	fail := env.FailureOf(err)
	switch fail.Kind {
	case env.FailureDeviceLimit:
		s.e.Act.AskDevices(env.DevicesSheetParams{Code: s.boxes.Value, Devices: fail.Devices})

		return
	case env.FailureCodeInvalid, env.FailureCodeExpired:
		// the prototype shows the toast alone (Q-P6.4-2): no red boxes, the code stays
		if v := s.View(); v != nil {
			v.Focus(s.boxes.Field())
		}
	default:
	}
	if msg := s.e.FailureToast(fail); msg != "" {
		s.e.Act.Toast(msg)
	}
}

// disconnect is `unlinkTV` (`$P:623`): the TV is removed (each, if the list holds
// several), then the screen shows the empty code again.
func (s *Connect) disconnect() {
	if s.busy {
		return
	}
	ids := s.tvs
	if kept := tvIDs(s.e.AccountView().Devices.Items); len(kept) > 0 {
		ids = kept
	}
	s.busy = true
	if s.cancel != nil {
		s.cancel()
	}
	s.cancel = ui.Go(s, func(ctx context.Context) (struct{}, error) {
		if len(ids) == 0 {
			// the screen shows a TV the lists it holds do not name: ask the server
			d, err := s.e.Account().Devices(ctx)
			if err != nil {
				return struct{}{}, err
			}
			ids = tvIDs(d.Items)
		}
		for _, id := range ids {
			if err := s.e.Account().DeleteDevice(ctx, id); err != nil {
				return struct{}{}, err
			}
		}

		return struct{}{}, nil
	}, s.disconnected)
}

// disconnected takes the answer of the removal: the code entry comes back with an
// empty code; a failure leaves the linked state and says why.
func (s *Connect) disconnected(_ *ui.Context, _ struct{}, err error) {
	s.busy = false
	s.cancel = nil
	if err == nil {
		s.tvs = nil
		s.boxes.SetValue("")
		s.link.SetDisabled(true)
		s.setLinked(false)

		return
	}
	if errors.Is(err, context.Canceled) || s.e.Refused(err) {
		return
	}
	if msg := s.e.FailureToast(env.FailureOf(err)); msg != "" {
		s.e.Act.Toast(msg)
	}
}

// tvIDs returns the ids of the tvOS devices of the list.
func tvIDs(items []model.Device) []string {
	var ids []string
	for _, d := range items {
		if d.Platform == model.PlatformTVOS {
			ids = append(ids, d.ID)
		}
	}

	return ids
}
