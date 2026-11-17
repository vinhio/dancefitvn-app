//go:build !tvos

package kitdemo

import (
	"strings"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// pageM15 is the account screen (MS.account, index.html:463-475): the profile
// card with the avatar, the three badges, the two cards of rows, the red
// button and the tab bar.
func pageM15() Page {
	return Page{ID: "M15", Title: "Account", Build: func(d *Data) Screen { return screen(d.account()) }}
}

// pageM15B is the account information form (MS.info, :477-482): the top bar,
// the fields and the yellow button.
func pageM15B() Page {
	return Page{ID: "M15B", Title: "Account information", Build: func(d *Data) Screen { return screen(d.info()) }}
}

// badgeAccent is the colour of a badge's ribbon and rim, BADGES (:311), by the
// API's badge code.
var badgeAccent = map[string]color.Color{
	"hdv_dancefit": color.RGB(0x7a, 0xc9, 0x43),
	"hdv_stepx":    color.RGB(0xff, 0x7a, 0x1a),
	"hdv_bodyx":    color.RGB(0xe5, 0x38, 0x3b),
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// account lays out M15 at the boxes of M15.layout.json.
func (d *Data) account() *canvas {
	cv := newCanvas()
	u := d.Me.User
	cv.at(kit.H1(d.T("account.title")), 20, 70, 353)

	// the profile card (:464): --pr, radius 20, padding 16.
	cv.box(newPlate("", tokens.Dark.Primary, 20), 20, 118, 353, 112)
	av := kit.NewAvatarButton(76, tokens.Dark.OnPrimary, d.T("avatar.title"))
	av.Name, av.Camera = "sheet:avatar", true
	cv.fit(av, 36, 134, 76)
	name := kit.Display(u.LastName+" "+u.FirstName, 22)
	name.Ink = theme.InkOnPrimary
	cv.at(name, 126, 144, 150.41)
	since := &kit.Label{Text: d.T("account.since") + " " + format.Date(u.MemberSince.Time()), Size: 12, Weight: 600, Ink: theme.InkOnPrimary}
	cv.fit(since, 126, 172, 200)
	mail := &kit.Label{Text: u.Email, Size: 12, Color: tokens.Dark.OnPrimary.WithAlpha(.8), Ellipsis: true}
	cv.fit(mail, 126, 193, 200)

	cv.at(kit.Eyebrow(d.T("account.badges")), 20, 252, 353)
	for i, b := range d.Me.Badges {
		sub := strings.ToUpper(strings.TrimPrefix(b.Code, "hdv_"))
		cv.at(kit.NewBadge(b.Name, badgeAccent[b.Code], sub), 20+float32(i)*121, 276, 111)
	}

	eb := kit.Eyebrow(d.T("account.profile"))
	eb.Name = "div.eb#2"
	cv.at(eb, 20, 412, 353)
	cv.box(newCard("div.card"), 20, 436, 353, 156)
	rows := []struct {
		name, title, sub string
		ic               icon.Name
		last             bool
	}{
		{"go.info", d.T("account.info"), d.T("account.infoSub"), icon.User, false},
		{"go.coach", d.T("account.coach"), d.T("account.coachSub"), icon.Award, false},
		{"sheet:password", d.T("account.password"), "", icon.Lock, true},
	}
	for i, r := range rows {
		row := kit.NewRow(r.ic, r.title, r.sub)
		row.Name, row.Chevron, row.Last = r.name, true, r.last
		cv.at(row, 34, 436+float32(i)*52, 325)
	}

	eb3 := kit.Eyebrow(d.T("account.app"))
	eb3.Name = "div.eb#3"
	cv.at(eb3, 20, 614, 353)
	cv.box(newCard("div.card#2"), 20, 638, 353, 208)
	connect := kit.NewRow(icon.Tv, d.T("account.connect"), "")
	connect.Name, connect.Value, connect.Chevron = "go.connect", d.T("account.notLinked"), true
	wifi := kit.NewRow(icon.Globe, d.T("account.wifiOnly"), "")
	wifi.Name, wifi.Trailing = "pref:wifi", kit.NewSwitch(true)
	auto := kit.NewRow(icon.Rep, d.T("account.autoNext"), "")
	auto.Name, auto.Trailing = "pref:auto", kit.NewSwitch(true)
	dl := kit.NewRow(icon.Store, d.T("account.downloaded"), "")
	dl.Name, dl.Value, dl.Chevron, dl.Last = "go.downloads", format.Size(audioBytes(d)), true, true
	for i, r := range []*kit.Row{connect, wifi, auto, dl} {
		cv.at(r, 34, 638+float32(i)*52, 325)
	}

	out := kit.NewButton(kit.ButtonT, d.T("logout.confirm"), icon.NameUnknown)
	out.Name, out.Ink = "sheet:logout", theme.InkDanger // `style="color:var(--dg)"` (:474)
	cv.at(out, 20, 860, 353)
	ver := kit.Muted(d.T("account.version"), 11)
	ver.Name, ver.Align = "p.muted", ui.TextAlignCenter
	cv.at(ver, 20, 920, 353)

	cv.pin(d.tabs(3, map[int]string{2: "go.downloads#2"}), 0, 768, 393)

	return cv
}

// audioBytes is the size of the downloaded audio of Set 35.
func audioBytes(d *Data) int64 {
	var n int64
	for _, l := range d.Set35.Lessons {
		if Downloaded(l.No) {
			n += l.AudioSizeBytes
		}
	}

	return n
}

// info lays out M15B at the boxes of M15B.layout.json.
func (d *Data) info() *canvas {
	cv := newCanvas()
	u := d.Me.User
	cv.at(kit.NewTopBar(d.T("account.info"), d.T("a11y.back"), nil), 20, 54, 353)

	first := kit.NewInput(icon.NameUnknown, "")
	first.SetText(u.FirstName)
	cv.at(&kit.Field{Label: d.T("info.first"), Required: true, Input: first}, 20, 230, 201)
	last := kit.NewInput(icon.NameUnknown, "")
	last.SetText(u.LastName)
	cv.at(&kit.Field{Label: d.T("info.last"), Input: last}, 233, 230, 201)
	email := kit.NewInput(icon.Mail, "")
	email.SetText(u.Email)
	cv.at(&kit.Field{Label: d.T("login.email"), Required: true, Input: email}, 20, 322, 353)
	phone := kit.NewInput(icon.Phone, "")
	phone.SetText(phoneText(u.Phone))
	cv.at(&kit.Field{Label: d.T("info.phone"), Input: phone}, 20, 414, 353)

	// the "Đổi mật khẩu" row (:482): a bare row, `ic('lock',20,2)`, the title, `ic('cr',20)`.
	cv.box(newPlate("sheet:password", color.Color{}, 0), 20, 506, 353, 52)
	lock := kit.NewIcon(icon.Lock, 20, theme.InkText)
	lock.Stroke = 2
	cv.fit(lock, 20, 520, 20)
	cv.fit(&kit.Label{Text: d.T("account.password"), Size: 15, Weight: 600}, 52, 522.5, 289)
	cv.fit(kit.NewIcon(icon.Cr, 20, theme.InkText2), 353, 522, 20)

	// the note under it (:483): its `ic('info')` is the only part drawn.
	cv.box(newCard("div.card"), 20, 568, 353, 58)
	cv.fit(kit.NewIcon(icon.Info, 22, theme.InkText2), 34, 582, 22)
	update := kit.NewButton(kit.ButtonP, d.T("info.update"), icon.Check)
	update.IconSize, update.IconStroke = 20, 2.4 // `ic('check',20,2.4)` (:481)
	cv.pin(update, 20, 760, 353)

	return cv
}

// phoneText groups a Vietnamese mobile number as the prototype shows it,
// "0905 123 456".
func phoneText(p string) string {
	if len(p) != 10 {
		return p
	}

	return p[:4] + " " + p[4:7] + " " + p[7:]
}
