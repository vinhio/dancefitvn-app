//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/views/form_link.go at 49de3c2 (a form
// of kit.Input and kit.Button: the Enter of a field moves on or submits, one
// request at a time, the answer comes back through ui.Go and a stale one is
// dropped) and from dancefitvn-app/internal/ui/mobile/kitdemo/pages_account.go
// (the widgets and numbers of M15B). The screen is MS.info (index.html:477-484)
// over PATCH /me.

package screens

import (
	"context"
	"errors"
	"strings"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The keys of the server's messages by field (the body of 400 validation_failed,
// me.update) and the inputs they mark.
const (
	fieldFirst = "first_name"
	fieldLast  = "last_name"
	fieldEmail = "email"
	fieldPhone = "phone"
)

// Info is MS.info, screen 15B: the form of the user's name, email and phone, the
// row that opens the password sheet, the note about the website, and the
// "Cập nhật" button over the end of the page. Back leaves the edits behind: nothing
// is kept until the button saves them (`$P:564`).
//
// "Cập nhật" checks the fields as the prototype does ("Tên và email là bắt buộc"),
// then sends them (PATCH /me, one request at a time); what the server refuses is
// shown on the fields it names (Q-P6-6), what it accepts leaves the screen with
// "Đã cập nhật thông tin tài khoản" (or the toast of the email waiting to be
// confirmed).
type Info struct {
	ui.Base

	e      *env.Env
	page   *nav.Page
	body   *infoBody
	submit *kit.Button

	first, last, email, phone *kit.Input

	busy    bool
	touched bool // the user typed: a later read of /me does not overwrite it
	cancel  func()
}

// infoBody stacks the parts of the page.
type infoBody struct {
	ui.Base

	stack blockStack
}

var (
	_ nav.Tabbed   = (*Info)(nil)
	_ nav.Scroller = (*Info)(nil)
	_ ui.Mounter   = (*Info)(nil)
	_ ui.Unmounter = (*Info)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewInfo returns screen 15B (MS.info, `$P:477-484`, M15B), no tab bar, the form
// filled with the user the account view holds.
func NewInfo(e *env.Env) *Info {
	f := &Info{e: e}
	f.first = kit.NewInput(icon.NameUnknown, "")
	f.last = kit.NewInput(icon.NameUnknown, "")
	f.email = kit.NewInput(icon.Mail, "")
	f.email.Keyboard = platform.KeyboardEmail
	f.phone = kit.NewInput(icon.Phone, "")
	f.phone.Keyboard = platform.KeyboardPhone
	chain := []*kit.Input{f.first, f.last, f.email, f.phone}
	for i, in := range chain {
		in.OnChange = func(string) {
			f.touched = true
			in.SetError(false) // typing takes back what the server refused
		}
		if i+1 < len(chain) {
			next := chain[i+1]
			in.EnterAction = platform.EnterNext
			in.OnSubmit = func(string) { f.focus(next) }
		} else {
			in.EnterAction = platform.EnterDone
			in.OnSubmit = func(string) { f.save() }
		}
	}
	if u, ok := e.AccountUser(); ok {
		f.fill(u)
	}
	f.touched = false

	first := &kit.Field{Label: e.T("info.first"), Required: true, Input: f.first}
	last := &kit.Field{Label: e.T("info.last"), Input: f.last}
	pw := newPwLink(e.T("account.password"), e.Act.AskPassword)
	pw.Name = namePassword
	lead := kit.Muted(e.T("info.lead"), infoLeadSize)
	lead.Name = "p.muted"
	title2 := accent(kit.Display(e.T("info.titleB"), infoTitleSize)) // `.d.acc`
	note := kit.NewCard(newNoteBox(e.T("info.note")))
	note.Padding = geom.Insets{Top: infoNotePad, Right: infoNotePad, Bottom: infoNotePad, Left: infoNotePad}

	f.body = &infoBody{}
	f.body.stack.items = []blockItem{
		{w: kit.NewTopBar(e.T("account.info"), e.T("a11y.back"), e.Act.Back)},
		{w: kit.Display(e.T("info.titleA"), infoTitleSize), top: infoTitleTop},
		{w: title2, top: infoTitle2Top},
		{w: lead, top: infoLeadTop, bottom: infoLeadBottom},
		{w: newNameRow(first, last), bottom: kit.FieldGap},
		{w: &kit.Field{Label: e.T("login.email"), Required: true, Input: f.email}, bottom: kit.FieldGap},
		{w: &kit.Field{Label: e.T("info.phone"), Input: f.phone}, bottom: kit.FieldGap},
		{w: pw},
		{w: note, top: infoNoteTop, bottom: infoNoteBottom},
	}
	for _, it := range f.body.stack.items {
		f.body.stack.Add(it.w)
	}
	f.body.Add(&f.body.stack)
	f.page = nav.NewPage(f.body, -1)

	f.submit = kit.NewButton(kit.ButtonP, e.T("info.update"), icon.Check)
	f.submit.IconSize, f.submit.IconStroke = 20, 2.4 // `ic('check',20,2.4)` (:481)
	f.submit.OnPress = f.save
	f.Add(f.page, f.submit)

	return f
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Tab is -1: no tab bar.
func (f *Info) Tab() int { return -1 }

// Scroll returns the page's scroll.
func (f *Info) Scroll() *ui.Scroll { return f.page.Scroll() }

// First returns the field "Tên".
func (f *Info) First() *kit.Input { return f.first }

// Last returns the field "Họ".
func (f *Info) Last() *kit.Input { return f.last }

// Email returns the field "Email".
func (f *Info) Email() *kit.Input { return f.email }

// Phone returns the field "Số điện thoại".
func (f *Info) Phone() *kit.Input { return f.phone }

// Submit returns the button "Cập nhật".
func (f *Info) Submit() *kit.Button { return f.submit }

// Busy tells whether a save is under way: a second tap is ignored.
func (f *Info) Busy() bool { return f.busy }

// Mounted reads GET /me when the account view has none yet (a phone with no
// network at launch), with the next turn: a view being mounted has no host, and
// ui.Go would run the read on the UI thread then.
func (f *Info) Mounted(c *ui.Context) {
	if !f.e.AccountView().HasMe {
		c.AfterFunc(0, f.load)
	}
}

// Unmounted ends the read and the save under way: leaving the screen drops them.
func (f *Info) Unmounted(*ui.Context) {
	if f.cancel != nil {
		f.cancel()
		f.cancel = nil
	}
}

// Layout fills the window with the page and puts the button over its end.
func (f *Info) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(f.page, ui.Tight(cs.Max))
	c.Place(f.page, geom.Point{})
	w := max(cs.Max.W-2*infoButtonSide, 0)
	bs := c.Layout(f.submit, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(f.submit, geom.Pt(infoButtonSide, cs.Max.H-infoButtonBottom-bs.H))

	return cs.Max
}

// Layout is the stack's.
func (b *infoBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	sz := c.Layout(&b.stack, cs)
	c.Place(&b.stack, geom.Point{})

	return cs.Constrain(sz)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// fill puts the user in the fields; the phone as the prototype shows it.
func (f *Info) fill(u model.User) {
	f.first.SetText(u.FirstName)
	f.last.SetText(u.LastName)
	f.email.SetText(u.Email)
	f.phone.SetText(phoneText(u.Phone))
}

// focus moves the focus to the field.
func (f *Info) focus(in *kit.Input) {
	if v := f.View(); v != nil {
		v.Focus(in.Field())
	}
}

// blur takes the focus away, which hides the keyboard.
func (f *Info) blur() {
	if v := f.View(); v != nil {
		v.Focus(nil)
	}
}

// load reads GET /me and fills the fields if the user has typed nothing meanwhile.
func (f *Info) load() {
	f.cancel = ui.Go(f, func(ctx context.Context) (model.Me, error) {
		return f.e.Account().Me(ctx)
	}, func(_ *ui.Context, m model.Me, err error) {
		f.cancel = nil
		switch {
		case err == nil:
			if !f.touched && !f.busy {
				f.fill(m.User)
				f.touched = false
			}
		case errors.Is(err, context.Canceled):
		case f.e.Refused(err):
		default: // the fields stay as they are
		}
	})
}

// save is `submit` of the form (`$P:564-565`): a name or an email that is not
// there is the toast "Tên và email là bắt buộc" and nothing is sent; else the
// profile goes to the server, one request at a time.
func (f *Info) save() {
	if f.busy {
		return
	}
	req := model.MeUpdateRequest{
		FirstName: strings.TrimSpace(f.first.Text()),
		LastName:  strings.TrimSpace(f.last.Text()),
		Email:     strings.TrimSpace(f.email.Text()),
		Phone:     phoneDigits(f.phone.Text()),
	}
	if req.FirstName == "" || !strings.Contains(req.Email, "@") {
		f.e.Act.Toast(f.e.T("toast.infoRequired"))

		return
	}
	f.busy = true
	f.blur()
	if f.cancel != nil {
		f.cancel() // a read of /me that is still out must not overwrite what is saved
	}
	f.cancel = ui.Go(f, func(ctx context.Context) (model.MeUpdate, error) {
		return f.e.Account().UpdateMe(ctx, req)
	}, f.saved)
}

// saved takes the answer of PATCH /me: success goes back (`back()`) with the toast
// of the prototype, or the one of the email that waits for its confirmation; a
// refusal marks the fields it names.
func (f *Info) saved(_ *ui.Context, res model.MeUpdate, err error) {
	f.busy = false
	f.cancel = nil
	if err == nil {
		msg := f.e.T("toast.infoSaved")
		if p := res.EmailChangePending; p != nil && *p != "" {
			msg = f.e.Fill("toast.emailPendingTpl", "email", *p)
		}
		f.e.Act.Back()
		f.e.Act.Toast(msg)

		return
	}
	if errors.Is(err, context.Canceled) || f.e.Refused(err) {
		return
	}
	fail := env.FailureOf(err)
	if fail.Kind == env.FailureValidation {
		for key, in := range map[string]*kit.Input{fieldFirst: f.first, fieldLast: f.last, fieldEmail: f.email, fieldPhone: f.phone} {
			if _, bad := fail.Fields[key]; bad {
				in.SetError(true)
			}
		}
	}
	if msg := f.e.FailureToast(fail); msg != "" {
		f.e.Act.Toast(msg)
	}
}
