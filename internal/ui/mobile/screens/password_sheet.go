//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/views/form_link.go at 49de3c2 (a sheet
// with a form: the Enter of a field moves on or submits, one request at a time,
// the answer comes back through ui.Go and the sheet stays open on an error) and
// from the kitdemo's M15B-password page (pages_sheets.go) for the boxes. The sheet
// is SH.password (index.html:527-529), whose button `pwSave` the prototype answers
// with a toast and no change (K2 drops that; P6 section 1.3 and 5 give the checks).

package screens

import (
	"context"
	"errors"
	"unicode/utf8"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// pwMinLength is the shortest new password the sheet sends (Q-P6-5, the website's
// rule): the prototype has no check.
const pwMinLength = 8

// pwMask is the character a typed password shows as (`type=password`).
const pwMask = '•'

// namePwSave is `data-act="pwSave"` of the button.
const namePwSave = "pwSave"

// PasswordSheet is the content of SH.password (M15B-password): the three fields of
// the current, the new and the repeated password, and "Lưu mật khẩu". The checks
// are the ones of Q-P6-5 (all three filled, the new one at least 8 long, the
// repeat equal to it), each a toast; the server's "wrong_password" marks the
// current field and says so in a toast. The sheet stays open on any error and
// closes, with "Đã đổi mật khẩu", on success.
type PasswordSheet struct {
	ui.Base

	e   *env.Env
	col *sheetColumn

	current, next, repeat *kit.Input
	save                  *kit.Button

	busy   bool
	cancel func()
}

var _ ui.Unmounter = (*PasswordSheet)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewPasswordSheet returns the content of SH.password (`$P:527-529`,
// M15B-password), opened by Actions.AskPassword; its cross closes the sheet through
// the navigator (Env.Nav).
func NewPasswordSheet(e *env.Env) *PasswordSheet {
	s := &PasswordSheet{e: e}
	closeFn := func() {
		if e.Nav != nil {
			e.Nav.Sheet().Close()
		}
	}
	s.current, s.next, s.repeat = s.newInput(), s.newInput(), s.newInput()
	s.current.EnterAction, s.next.EnterAction, s.repeat.EnterAction = platform.EnterNext, platform.EnterNext, platform.EnterDone
	s.current.OnSubmit = func(string) { s.focus(s.next) }
	s.next.OnSubmit = func(string) { s.focus(s.repeat) }
	s.repeat.OnSubmit = func(string) { s.submit() }
	s.save = kit.NewButton(kit.ButtonP, e.T("password.save"), icon.NameUnknown)
	s.save.Name, s.save.OnPress = namePwSave, s.submit

	s.col = &sheetColumn{items: []sheetItem{
		{w: kit.NewSheetHeader(e.T("account.password"), e.T("a11y.close"), closeFn)},
		{w: &kit.Field{Label: e.T("password.current"), Input: s.current}, before: kit.SheetHeaderGap},
		{w: &kit.Field{Label: e.T("password.new"), Input: s.next}, before: kit.FieldGap},
		{w: &kit.Field{Label: e.T("password.repeat"), Input: s.repeat}, before: kit.FieldGap},
		{w: s.save, before: kit.FieldGap},
	}}
	for _, it := range s.col.items {
		s.col.Add(it.w)
	}
	s.Add(s.col)

	return s
}

// newInput returns one of the fields: a lock, `type=password` and the dots as the
// placeholder; typing takes back the red a refusal left.
func (s *PasswordSheet) newInput() *kit.Input {
	in := kit.NewInput(icon.Lock, s.e.T("password.placeholder"))
	in.Mask = pwMask
	in.OnChange = func(string) { in.SetError(false) }

	return in
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Current returns the field "Mật khẩu hiện tại".
func (s *PasswordSheet) Current() *kit.Input { return s.current }

// New returns the field "Mật khẩu mới".
func (s *PasswordSheet) New() *kit.Input { return s.next }

// Repeat returns the field "Nhập lại mật khẩu mới".
func (s *PasswordSheet) Repeat() *kit.Input { return s.repeat }

// Save returns the button "Lưu mật khẩu" (`pwSave`).
func (s *PasswordSheet) Save() *kit.Button { return s.save }

// Busy tells whether the password is being sent: a second tap is ignored.
func (s *PasswordSheet) Busy() bool { return s.busy }

// Unmounted drops the request under way: the sheet was closed.
func (s *PasswordSheet) Unmounted(*ui.Context) {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// Layout puts the column at the sheet's width.
func (s *PasswordSheet) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
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

// focus moves the focus to the field.
func (s *PasswordSheet) focus(in *kit.Input) {
	if v := s.View(); v != nil {
		v.Focus(in.Field())
	}
}

// blur takes the focus away, which hides the keyboard.
func (s *PasswordSheet) blur() {
	if v := s.View(); v != nil {
		v.Focus(nil)
	}
}

// submit is `pwSave`: the three checks (a toast each, nothing sent), then the
// password goes to the server, one request at a time.
func (s *PasswordSheet) submit() {
	if s.busy {
		return
	}
	cur, next, again := s.current.Text(), s.next.Text(), s.repeat.Text()
	switch {
	case cur == "" || next == "" || again == "":
		s.e.Act.Toast(s.e.T("toast.pwRequired"))

		return
	case utf8.RuneCountInString(next) < pwMinLength:
		s.e.Act.Toast(s.e.T("toast.pwShort"))

		return
	case again != next:
		s.e.Act.Toast(s.e.T("toast.pwMismatch"))

		return
	default: // the three checks hold
	}
	s.busy = true
	s.blur()
	s.cancel = ui.Go(s, func(ctx context.Context) (model.PasswordResult, error) {
		return s.e.Account().ChangePassword(ctx, cur, next)
	}, s.changed)
}

// changed takes the answer of PUT /me/password: success closes the sheet with its
// toast; a wrong current password marks that field; any refusal leaves the sheet
// open.
func (s *PasswordSheet) changed(_ *ui.Context, _ model.PasswordResult, err error) {
	s.busy = false
	s.cancel = nil
	if err == nil {
		s.current.SetText("")
		s.next.SetText("")
		s.repeat.SetText("")
		if s.e.Nav != nil {
			s.e.Nav.Sheet().Close()
		}
		s.e.Act.Toast(s.e.T("toast.pwSaved"))

		return
	}
	if errors.Is(err, context.Canceled) || s.e.Refused(err) {
		return
	}
	fail := env.FailureOf(err)
	if fail.Kind == env.FailureWrongPassword {
		s.current.SetError(true)
		s.focus(s.current)
	}
	if msg := s.e.FailureToast(fail); msg != "" {
		s.e.Act.Toast(msg)
	}
}
