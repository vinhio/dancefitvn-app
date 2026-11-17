//go:build !tvos

package kitdemo

import (
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// pageM02 is the login screen (MS.login, index.html:337-355): the two fields,
// the "forgot" link, the yellow button and the biometric button.
func pageM02() Page {
	return Page{ID: "M02", Title: "Login", Build: func(d *Data) Screen { return screen(d.login(false)) }}
}

// pageM02B is the login screen after a wrong password: the error banner, the
// password field in error with its help line.
func pageM02B() Page {
	return Page{ID: "M02B", Title: "Login, wrong password", Build: func(d *Data) Screen { return screen(d.login(true)) }}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// login lays out the login form at the boxes of M02 (err false) and M02B.
func (d *Data) login(err bool) *canvas {
	cv := newCanvas()
	// y of the elements: M02 `label` 300.59, `label#2` 392.59, the link row
	// 486.59, `button.btn.p` 524.59, `bioLogin` 633.59 (M02.layout.json); M02B
	// adds the 78 px banner above the form and the 15 px help line.
	emailY, passY, rowY, loginY, bioY := float32(300.59), float32(392.59), float32(486.59), float32(524.59), float32(633.59)
	if err {
		emailY, passY, rowY, loginY, bioY = 378.59, 470.59, 586.59, 624.59, 733.59
		d.loginBanner(cv)
	}

	email := kit.NewInput(icon.Mail, "")
	email.SetText(d.Me.User.Email)
	cv.at(&kit.Field{Label: d.T("login.email"), Input: email}, 20, emailY, 353)

	eye := kit.NewIconButton(icon.Eye, d.T("login.password"))
	eye.Name = "pwEye"
	pass := kit.NewInput(icon.Lock, d.T("login.passwordHint"))
	pass.Mask, pass.Trailing, pass.Error = '•', eye, err
	f := &kit.Field{Label: d.T("login.password"), Input: pass}
	if err {
		f.Help = d.T("login.passwordWrong")
	}
	cv.at(f, 20, passY, 353)

	// the "Ghi nhớ đăng nhập" check (:349): a 20 square --pr r6 with the tick.
	cv.box(newPlate("", tokens.Dark.Primary, 6), 20, rowY-2, 20, 20)
	tick := kit.NewIcon(icon.Check, 16, theme.InkOnPrimary)
	tick.Stroke = 2.6
	cv.fit(tick, 22, rowY, 16)
	forgot := &kit.Label{Text: d.T("login.forgot"), Size: 13, Weight: 700, Ink: theme.InkPrimary}
	forgot.Name = "toast"
	cv.fit(forgot, 270.09, rowY, 102.91)

	cv.at(kit.NewButton(kit.ButtonP, d.T("login.submit"), icon.NameUnknown), 20, loginY, 353)
	bio := kit.NewButton(kit.ButtonG, d.T("login.bio"), icon.Shield)
	bio.Name = "bioLogin"
	cv.at(bio, 20, bioY, 353)

	return cv
}

// loginBanner draws the red banner of M02B (:342): a tinted box with the warn
// icon, the title and the hint.
func (d *Data) loginBanner(cv *canvas) {
	cv.box(newPlate("", tokens.Dark.Danger.WithAlpha(.14), 14), 20, 300.59, 353, 62)
	w := kit.NewIcon(icon.Warn, 22, theme.InkDanger)
	cv.fit(w, 38, 312.59, 22)
	title := &kit.Label{Text: d.T("login.errorTitle"), Size: 14, Weight: 700}
	cv.fit(title, 72, 313, 230)
	hint := kit.Muted(d.T("login.errorHint"), 12)
	hint.Name = "div.muted"
	cv.fit(hint, 72, 335.59, 230.41)
}
