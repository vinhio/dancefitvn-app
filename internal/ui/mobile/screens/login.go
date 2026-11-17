//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/form_link.go at 49de3c2 (a form
// of kit.Input and kit.Button: the Enter of a field moves on or submits, one
// request at a time, the answer comes back through ui.Go and a stale one is
// dropped); the code is new: this form is MS.login (index.html:337-351).

package screens

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of MS.login (index.html:338-351), in DIPs; the y of each element
// follows from these and the page's 54 (M02.layout.json: brand 88, title 146,
// second line 188, intro 238, email label 300.59 ...).
const (
	// loginBrandTop, loginBrandSize: `margin-top:34px` and the span's
	// `font-size:28px;font-weight:800` of the brand; the div itself is 16/700
	// (what layout.json records), so brandLine reports that.
	loginBrandTop  float32 = 34
	loginBrandSize float32 = 28
	// loginTitleTop, loginTitleSize: `.d` at `font-size:38px;margin-top:30px`.
	loginTitleTop  float32 = 30
	loginTitleSize float32 = 38
	// loginLine2Top, loginLine2Size: `.d.acc` at `font-size:36px;margin-top:4px`.
	loginLine2Top  float32 = 4
	loginLine2Size float32 = 36
	// loginIntroTop, loginIntroSize, loginIntroLine, loginIntroBottom:
	// `font-size:14px;margin:14px 0 22px;line-height:1.45`.
	loginIntroTop    float32 = 14
	loginIntroSize   float32 = 14
	loginIntroLine   float32 = 1.45
	loginIntroBottom float32 = 22
	// The banner `display:flex;gap:12px;border-left:4px solid var(--dg);
	// border-radius:14px;padding:12px 14px;margin-bottom:16px`; its title is
	// `<b style="font-size:14px">`, its sub line `font-size:12px;margin-top:3px`.
	bannerGap, bannerLeft, bannerRadius float32 = 12, 4, 14
	bannerPadY, bannerPadX              float32 = 12, 14
	bannerBottom                        float32 = 16
	bannerTitleSize, bannerSubSize      float32 = 14, 12
	bannerSubTop                        float32 = 3
	bannerIcon                          float32 = 22
	// loginRowSize, loginCheck, loginCheckIcon: the remember row
	// `font-size:13px;margin:6px 0 20px`, its 20 square `border-radius:6px`
	// with `ic('check',16,2.6)`, the gap 8 after it.
	loginRowSize, loginRowGap        float32 = 13, 8
	loginCheck, loginCheckRadius     float32 = 20, 6
	loginCheckIcon, loginCheckStroke float32 = 16, 2.6
	loginRowBottom                   float32 = 20
	// loginForgotSize: `a.acc` 13/700.
	loginForgotSize float32 = 13
	// loginDividerGap, loginDividerSize: `display:flex;align-items:center;
	// gap:12px;margin:20px 0;font-size:12px`.
	loginDividerGap, loginDividerSize, loginDividerMargin float32 = 12, 12, 20
	// loginFooterTop, loginFooterSize: `font-size:13px;margin-top:36px`. The
	// footer is the muted notice login.membersOnly (K30), in the place and the
	// size of the prototype's "Chưa có tài khoản?" run.
	loginFooterTop, loginFooterSize float32 = 36, 13
)

// The names layout.json gives the login's parts that are not kit widgets.
const (
	nameBrand    = "div.d.acc"
	nameIntro    = "p.muted"
	nameForgot   = "toast" // `data-act="toast"` (K15: it opens the site)
	namePwEye    = "pwEye"
	nameBioLogin = "bioLogin"
)

// Login is MS.login (screens 02 and 02B): the brand and the title, the form of
// email and password, the sign-in button and, when the device offers biometry
// and holds a slot (K13), the "Đăng nhập bằng Face ID / Vân tay" button. It has
// no tab bar; Back at it leaves the app.
//
// The errors of auth.login show in the banner over the form, with the texts of
// the prototype where it has them and of locales/vi.json (Q-P3-3) where not.
type Login struct {
	ui.Base

	e    *env.Env
	page *nav.Page
	bg   radialBG
	col  *ui.Flex

	banner      *banner
	bannerBlock *ui.Flex
	email, pass *kit.Input
	passField   *kit.Field
	eye         *kit.IconButton
	submit, bio *kit.Button
	bioBlock    *ui.Flex
	forgot      *textLink

	busy bool
}

// brandLine is `<div class="d acc"><span style="font-size:28px;font-weight:800">
// DANCEFIT</span></div>`: the div is 16/700 (layout.json's `div.d.acc`), the span
// in it 28/800. The line is 28 high, the span's.
type brandLine struct {
	ui.Base

	label *kit.Label
}

// banner is the red box over the form: the warning icon, the title and the sub
// line.
type banner struct {
	ui.Base

	icon  *kit.Icon
	title *kit.Label
	line  *inlineLine
	sub   *kit.Label
	// onPress is the action a tap on the box does (K31: the device limit starts the
	// sign-in again, which opens the sheet of devices with a fresh list); nil for the
	// banners that only tell.
	onPress func()
}

var _ ui.PointerHandler = (*banner)(nil)

// checkBox is the 20 square of "Ghi nhớ đăng nhập": --pr, 6 rounded, with the
// tick.
type checkBox struct {
	ui.Base

	tick *kit.Icon
}

// textLink is a text that is a button: `a.acc` of the forgot link. It looks like
// the text it holds; nothing changes while it is pressed (the prototype's links
// do not).
type textLink struct {
	ui.Pressable

	label *kit.Label
}

var (
	_ nav.Tabbed   = (*Login)(nil)
	_ nav.Scroller = (*Login)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewLogin returns the login screen of the route's params.
func NewLogin(e *env.Env, p env.LoginParams) *Login {
	l := &Login{e: e}
	l.bg.layers = []kit.Radial{{
		// radial-gradient(circle at 85% 5%, rgba(255,208,40,.3), transparent 45%) (:339)
		Shape: kit.RadialCircle, CX: .85, CY: .05, Color: color.RGBA(255, 208, 40, .3), End: .45,
	}}

	l.banner = newBanner()
	l.bannerBlock = ui.Column(0, l.banner, &ui.Box{Height: bannerBottom})
	l.bannerBlock.Align = ui.CrossStretch
	l.bannerBlock.Hidden = true

	l.email = kit.NewInput(icon.Mail, "")
	l.email.Keyboard, l.email.EnterAction = platform.KeyboardEmail, platform.EnterNext
	l.email.SetText(e.LastEmail())
	l.email.OnSubmit = func(string) { l.focus(l.pass) }
	emailField := &kit.Field{Label: e.T("login.email"), Input: l.email}

	l.eye = kit.NewIconButton(icon.Eye, e.T("a11y.showPassword")) // `pwEye` has no text: the password is hidden, so the button shows it
	l.eye.Name = namePwEye
	l.eye.OnPress = l.toggleEye
	l.pass = kit.NewInput(icon.Lock, e.T("login.passwordPlaceholder"))
	l.pass.Mask, l.pass.Trailing = '•', l.eye
	l.pass.EnterAction = platform.EnterGo
	l.pass.OnSubmit = func(string) { l.submitForm() }
	l.passField = &kit.Field{Label: e.T("login.password"), Input: l.pass}

	l.forgot = newTextLink(e.T("login.forgot"), loginForgotSize, theme.InkPrimary)
	l.forgot.Name = nameForgot
	l.forgot.OnPress = func() { l.openLink(func(k model.Links) string { return k.ForgotPassword }) }
	remember := ui.Row(loginRowGap,
		newCheckBox(),
		&kit.Label{Text: e.T("login.remember"), Size: loginRowSize},
	)
	row := ui.Row(0, remember, l.forgot)
	row.Justify = ui.JustifyBetween

	l.submit = kit.NewButton(kit.ButtonP, e.T("login.submit"), icon.NameUnknown)
	l.submit.OnPress = l.submitForm
	l.bio = kit.NewButton(kit.ButtonG, e.T("login.bio"), icon.Shield)
	l.bio.Name = nameBioLogin
	l.bio.OnPress = l.bioLogin
	or := &kit.Label{Text: e.T("login.or"), Size: loginDividerSize, Ink: theme.InkText3}
	divider := ui.Row(loginDividerGap, hline(), or, hline())
	l.bioBlock = ui.Column(0,
		&ui.Box{Height: loginDividerMargin}, divider, &ui.Box{Height: loginDividerMargin}, l.bio)
	l.bioBlock.Align = ui.CrossStretch
	ready := e.BioReady()
	l.bioBlock.Hidden = ready == sys.BiometryNone || ready == sys.BiometryUnknown || !faceIDOn(e)

	intro := kit.Muted(e.T("login.intro"), loginIntroSize)
	intro.LineHeight, intro.Wrap, intro.Name = loginIntroLine, true, nameIntro
	// K30: no sign-up in the app. The footer is plain muted text, centred, not
	// a link and not focusable; its accessibility label is the text itself.
	footer := kit.Muted(e.T("login.membersOnly"), loginFooterSize)
	footer.Align, footer.Wrap, footer.Name = ui.TextAlignCenter, true, nameIntro

	l.col = ui.Column(0,
		&ui.Box{Height: loginBrandTop}, newBrandLine(e.T("splash.brand")),
		&ui.Box{Height: loginTitleTop}, heading(kit.Display(e.T("login.title"), loginTitleSize), 1),
		&ui.Box{Height: loginLine2Top}, accent(kit.Display(e.T("login.titleLine2"), loginLine2Size)),
		&ui.Box{Height: loginIntroTop}, intro,
		&ui.Box{Height: loginIntroBottom},
		l.bannerBlock,
		emailField, &ui.Box{Height: kit.FieldGap},
		l.passField, &ui.Box{Height: kit.FieldGap},
		row, &ui.Box{Height: loginRowBottom},
		l.submit,
		l.bioBlock,
		&ui.Box{Height: loginFooterTop}, footer,
	)
	l.col.Align = ui.CrossStretch
	l.page = nav.NewPage(l.col, -1)
	l.Add(&l.bg, l.page)
	if p.Banner == env.LoginBannerExpired {
		l.showBanner(e.T("session.expired"), e.T("session.expiredSub"))
	}

	return l
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Email returns the email field.
func (l *Login) Email() *kit.Input { return l.email }

// Password returns the password field.
func (l *Login) Password() *kit.Input { return l.pass }

// SubmitButton returns the sign-in button.
func (l *Login) SubmitButton() *kit.Button { return l.submit }

// BioButton returns the biometric button, which is Hidden when the device
// offers none.
func (l *Login) BioButton() *kit.Button { return l.bio }

// Eye returns the button that shows or hides the password (`pwEye`).
func (l *Login) Eye() *kit.IconButton { return l.eye }

// BioShown tells whether the biometric button (and the "hoặc" above it) is on
// the screen: K13.
func (l *Login) BioShown() bool { return !l.bioBlock.Hidden }

// Busy tells whether a sign-in is under way: a second tap is ignored.
func (l *Login) Busy() bool { return l.busy }

// Banner returns the title and the sub line of the banner, and whether it is
// shown.
func (l *Login) Banner() (title, sub string, shown bool) {
	return l.banner.title.Text, l.banner.sub.Text, !l.bannerBlock.Hidden
}

// Tab is -1: no tab bar.
func (l *Login) Tab() int { return -1 }

// Scroll returns the page's scroll.
func (l *Login) Scroll() *ui.Scroll { return l.page.Scroll() }

// Submit puts password in the password field and signs in with it: what a tap on
// "Đăng nhập" does, for a state of the shot script and for the tests.
func (l *Login) Submit(password string) {
	l.pass.SetText(password)
	l.submitForm()
}

// Layout fills the window with the glow and the page.
func (l *Login) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	full := ui.Tight(cs.Max)
	c.Layout(&l.bg, full)
	c.Place(&l.bg, geom.Point{})
	c.Layout(l.page, full)
	c.Place(l.page, geom.Point{})

	return cs.Max
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// submitForm is the form's submit (K6): a field left empty takes the focus and
// nothing is asked; else the keyboard goes away and auth.login runs once, a tap
// meanwhile doing nothing.
func (l *Login) submitForm() {
	if l.busy {
		return
	}
	email, pw := strings.TrimSpace(l.email.Text()), l.pass.Text()
	switch {
	case email == "":
		l.focus(l.email)

		return
	case pw == "":
		l.focus(l.pass)

		return
	default: // both fields filled: go on to the request
	}
	l.setBusy(true)
	l.blur()
	ui.Go(l, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, l.e.Login(ctx, email, pw)
	}, func(_ *ui.Context, _ struct{}, err error) {
		l.setBusy(false)
		l.loginDone(err)
	})
}

// loginDone shows what auth.login answered.
func (l *Login) loginDone(err error) {
	switch {
	case err == nil:
		l.e.Act.SignedIn()
	case errors.Is(err, context.Canceled):
		// the screen was left: nothing to show (P1.11)
	default:
		l.showLoginError(err)
	}
}

// showLoginError turns an error of auth.login into the banner (Q-P3-3): the
// prototype's title and the attempts left for a wrong password, the minutes for
// a locked account, the limit of devices (deviceLimit: the sheet that lets one
// go, K31), no network, and the server's trouble for the rest. The password is
// cleared (the limit of devices keeps it for the retry); only a wrong one marks
// the field.
func (l *Login) showLoginError(err error) {
	var ae *api.Error
	code := ""
	if errors.As(err, &ae) {
		code = ae.Code
	}
	e := l.e
	wrong := false
	var title, sub string
	switch {
	case e.Refused(err): // 426: the upgrade screen with the store pages (03 section 9)
		return
	case code == "invalid_credentials":
		wrong = true
		title = e.T("login.errorTitle")
		sub = e.T("login.err.check")
		if ae.Body.AttemptsLeft > 0 {
			sub = e.Fill("login.err.attemptsTpl", "n", strconv.Itoa(ae.Body.AttemptsLeft))
		}
	case code == "account_locked":
		title = e.T("login.err.lockedTitle")
		sub = e.Fill("login.err.lockedTpl", "n", strconv.Itoa(max((ae.Body.RetryAfter+59)/60, 1)))
	case code == "device_limit":
		l.deviceLimit(ae.Body.Devices)

		return
	case errors.Is(err, api.ErrOffline):
		title, sub = e.T("login.err.offlineTitle"), e.T("login.err.offlineSub")
	default:
		title, sub = e.T("common.errorTitle"), e.T("common.errorSub")
	}
	l.pass.SetText("")
	l.pass.SetError(wrong)
	if wrong {
		l.passField.Help = e.T("login.passwordWrong")
	} else {
		l.passField.Help = ""
	}
	l.passField.Relayout()
	l.showBanner(title, sub)
}

// deviceLimit handles the 409 device_limit of a sign-in (K31): the banner says it, and
// the sheet of devices opens with the phones the server lists, most recent first, for
// the user to choose the one to replace. The password stays in its field: the sign-in
// is sent again with it. The banner is a button that sends the sign-in again (a fresh
// list), for the user who closed the sheet. With no phone in the list (the server
// listed only TVs, or none) the banner alone is shown.
func (l *Login) deviceLimit(devices []model.Device) {
	e := l.e
	l.pass.SetError(false)
	l.passField.Help = ""
	l.passField.Relayout()
	l.showBanner(e.T("login.err.deviceTitle"), e.T("login.err.deviceSub"))
	list := env.ReplaceableDevices(devices)
	if len(list) == 0 {
		return
	}
	l.banner.onPress = l.submitForm
	email, pw := strings.TrimSpace(l.email.Text()), l.pass.Text() // held by the sheet's retry, nowhere else
	e.Act.AskDevices(env.DevicesSheetParams{Devices: list, Login: &env.LoginReplace{
		Retry: func(ctx context.Context, id string) error { return e.LoginReplacing(ctx, email, pw, id) },
		Done:  l.replaceDone,
	}})
}

// replaceDone is the end of the sheet of devices of a sign-in: the sign-in went through,
// or the error is the form's to show (a wrong password, a locked account ...).
func (l *Login) replaceDone(err error) {
	if err == nil {
		l.e.Act.SignedIn()

		return
	}
	l.showLoginError(err)
}

// faceIDOn tells whether the app config lets the biometric sign-in show
// (`features.face_id_login`, K13). A config that is not known (the network was down
// at launch) leaves it to the device alone: the flag can only turn the button off.
func faceIDOn(e *env.Env) bool {
	cfg, ok := e.Cfg()

	return !ok || cfg.Features.FaceIDLogin
}

// bioLogin is `bioLogin` (K13): the system's sheet, then the refresh with the
// slot's token. A cancel stays here; a refusal of the slot is an expired session
// and the button goes; a failed or locked biometry sends the user to the
// password with a toast.
func (l *Login) bioLogin() {
	if l.busy {
		return
	}
	l.setBusy(true)
	p := sys.BioPrompt{Reason: l.e.T("bio.reason"), Cancel: l.e.T("bio.cancel")}
	ui.Go(l, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, l.e.BioLogin(ctx, p)
	}, func(_ *ui.Context, _ struct{}, err error) {
		l.setBusy(false)
		l.bioDone(err)
	})
}

// bioDone handles the end of a biometric sign-in.
func (l *Login) bioDone(err error) {
	e := l.e
	switch {
	case err == nil:
		e.Act.SignedIn()
	case errors.Is(err, context.Canceled):
		// the user said no, or the system took the sheet away: stay
	case e.Refused(err): // 426: the upgrade screen (03 section 9)
	case errors.Is(err, api.ErrUnauthorized):
		kit.SetHidden(l.bioBlock, true)
		l.showBanner(e.T("session.expired"), e.T("session.expiredSub"))
	case errors.Is(err, api.ErrOffline):
		l.showBanner(e.T("login.err.offlineTitle"), e.T("login.err.offlineSub"))
	case env.FailureOf(err).Kind == env.FailureDeviceLimit:
		// the refresh holds no password to send again with a device to replace (K31): the
		// banner says it, and the password's own sign-in opens the sheet of devices
		l.showBanner(e.T("login.err.deviceTitle"), e.T("login.err.deviceSub"))
		l.focus(l.pass)
	default:
		l.focus(l.pass)
		e.Act.Toast(e.T("bio.failed"))
	}
}

// openLink opens the link of the app config that pick returns (K15): the
// forgot-password page, the website of the sign-up. The config the boot fetched
// is used; with none (no network at launch) it is asked for once.
func (l *Login) openLink(pick func(model.Links) string) {
	if cfg, ok := l.e.Cfg(); ok {
		l.e.OpenURL(pick(cfg.Links))

		return
	}
	ui.Go(l, func(ctx context.Context) (model.AppConfig, error) {
		return l.e.Config(ctx)
	}, func(_ *ui.Context, cfg model.AppConfig, err error) {
		if err != nil {
			if !errors.Is(err, context.Canceled) && !l.e.Refused(err) {
				l.e.Act.Toast(l.e.T("login.err.offlineSub"))
			}

			return
		}
		l.e.SetConfig(cfg)
		l.e.OpenURL(pick(cfg.Links))
	})
}

// toggleEye is `pwEye`: the password shows, or is hidden again.
func (l *Login) toggleEye() {
	if l.pass.Mask == 0 {
		l.pass.SetMask('•')
		l.eye.Label = l.e.T("a11y.showPassword")
	} else {
		l.pass.SetMask(0)
		l.eye.Label = l.e.T("a11y.hidePassword")
	}
}

// showBanner shows the banner with a title and a sub line.
func (l *Login) showBanner(title, sub string) {
	l.banner.onPress = nil
	l.banner.title.SetText(title)
	l.banner.sub.SetText(sub)
	kit.SetHidden(l.bannerBlock, false)
	l.bannerBlock.Relayout()
}

// setBusy marks a request under way: the sign-in button is dimmed and takes no
// press.
func (l *Login) setBusy(b bool) {
	l.busy = b
	l.submit.SetDisabled(b)
	l.bio.SetDisabled(b)
}

// focus gives the focus to the field.
func (l *Login) focus(in *kit.Input) {
	if v := l.View(); v != nil {
		v.Focus(in.Field())
	}
}

// blur takes the focus away, which hides the keyboard.
func (l *Login) blur() {
	if v := l.View(); v != nil {
		v.Focus(nil)
	}
}

// newBrandLine returns the brand: 28/800 in a div of 16/700.
func newBrandLine(s string) *brandLine {
	b := &brandLine{label: &kit.Label{
		Text: s, Role: theme.RoleDisplay, Size: loginBrandSize, Weight: 800, Spacing: .6, LineHeight: 1,
		Upper: true, Ink: theme.InkPrimary,
	}}
	b.Name = nameBrand
	b.Add(b.label)

	return b
}

// Layout is as wide as the room and as tall as the span.
func (b *brandLine) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	ls := c.Layout(b.label, ui.Loose(geom.Sz(cs.Max.W, ui.Unbounded)))
	c.Place(b.label, geom.Point{})

	return cs.Constrain(geom.Sz(cs.Max.W, ls.H))
}

// LayoutStyle is `div.d.acc`: the div's own 16/700, spacing .6, line height 16.
func (b *brandLine) LayoutStyle(*ui.Context) (kit.CSS, string) {
	return kit.CSS{
		FontFamily: theme.Family(theme.RoleDisplay), FontSize: "16px", FontWeight: "700", LineHeight: "16px",
		LetterSpacing: "0.6px", Color: kit.CSSColor(tokens.Dark.Primary), Background: "rgba(0, 0, 0, 0)",
		Radius: "0px", Border: "0px " + kit.CSSColor(tokens.Dark.Primary),
	}, strings.ToUpper(b.label.Text)
}

// accent colours a label --pr.
func accent(l *kit.Label) *kit.Label {
	l.Ink = theme.InkPrimary
	l.Name = nameBrand

	return l
}

// newBanner returns the banner, hidden by its block until an error.
func newBanner() *banner {
	b := &banner{
		icon:  kit.NewIcon(icon.Warn, bannerIcon, theme.InkDanger),
		title: &kit.Label{Size: bannerTitleSize, Weight: 700, Wrap: true},
		sub:   kit.Muted("", bannerSubSize),
	}
	b.sub.Name, b.sub.Wrap = "div.muted", true
	b.line = newInline(b.title)
	b.Add(b.icon, b.line, b.sub)

	return b
}

// Layout puts the icon, then the title over the sub line, inside the border and
// the padding; the box is as wide as the room.
func (b *banner) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	left := bannerLeft + bannerPadX
	c.Layout(b.icon, ui.Loose(geom.Sz(bannerIcon, bannerIcon)))
	c.Place(b.icon, geom.Pt(left, bannerPadY))
	x := left + bannerIcon + bannerGap
	room := max(w-x-bannerPadX, 0)
	ts := c.Layout(b.line, ui.Loose(geom.Sz(room, ui.Unbounded)))
	c.Place(b.line, geom.Pt(x, bannerPadY))
	ss := c.Layout(b.sub, ui.Loose(geom.Sz(room, ui.Unbounded)))
	c.Place(b.sub, geom.Pt(x, bannerPadY+ts.H+bannerSubTop))

	return cs.Constrain(geom.Sz(w, bannerPadY+ts.H+bannerSubTop+ss.H+bannerPadY))
}

// HandlePointer does the banner's action on a tap that ends over it; a banner with none
// leaves the touch to the page (the scroll).
func (b *banner) HandlePointer(_ *ui.Context, e *ui.PointerEvent) {
	if b.onPress == nil {
		return
	}
	switch e.Kind {
	case ui.PointerDown:
		e.Handle()
	case ui.PointerUp:
		e.Handle()
		if e.Inside {
			b.onPress()
		}
	default:
	}
}

// Paint draws the tinted box and its left border, under the children.
func (b *banner) Paint(c *ui.Context, p *ui.Painter) {
	r := kit.SnapBox(p, b.Bounds()) // `.banner`: whole-CSS-px box (Chrome)
	danger := theme.From(c).T.Danger
	p.Fill(r, scene.Solid(danger.WithAlpha(.14)), bannerRadius)
	p.Border(r, geom.Insets{Left: bannerLeft}, danger, bannerRadius)
	b.Base.Paint(c, p)
}

// newCheckBox returns the tick box of "Ghi nhớ đăng nhập".
func newCheckBox() *checkBox {
	b := &checkBox{tick: kit.NewIcon(icon.Check, loginCheckIcon, theme.InkOnPrimary)}
	b.tick.Stroke = loginCheckStroke
	b.Add(b.tick)

	return b
}

// Layout makes the square and centres the tick.
func (b *checkBox) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	ts := c.Layout(b.tick, ui.Loose(geom.Sz(loginCheckIcon, loginCheckIcon)))
	c.Place(b.tick, geom.Pt((loginCheck-ts.W)/2, (loginCheck-ts.H)/2))

	return cs.Constrain(geom.Sz(loginCheck, loginCheck))
}

// Paint draws the --pr square under the tick.
func (b *checkBox) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(b.Bounds(), scene.Solid(theme.From(c).T.Primary), loginCheckRadius)
	b.Base.Paint(c, p)
}

// newTextLink returns a link of text at size, 700, in ink.
func newTextLink(s string, size float32, ink theme.Ink) *textLink {
	t := &textLink{label: &kit.Label{Text: s, Size: size, Weight: 700, Ink: ink}}
	t.Look = func(*ui.Context, ui.Interaction) ui.Decoration { return ui.Decoration{} }
	t.Add(t.label)

	return t
}

// LayoutStyle is the style of the text it holds (`a.acc`).
func (t *textLink) LayoutStyle(c *ui.Context) (kit.CSS, string) { return t.label.LayoutStyle(c) }

// AccessibleLabel returns the link's text.
func (t *textLink) AccessibleLabel() string { return t.label.Text }

// hline returns the 1 px rule of the divider: `<i style="flex:1;height:1px;
// background:var(--bd)">`.
func hline() *ui.Box {
	b := &ui.Box{Height: 1, Background: scene.Solid(tokens.Dark.Border)}
	b.Grow = 1

	return b
}
