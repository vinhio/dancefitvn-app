//go:build !tvos

// Adapted from dancefitvn-app/internal/ui/mobile/kitdemo/pages_account.go (the
// widgets and numbers of M15), from jplayer-mobile/internal/ui/views/home.go at
// 49de3c2 (the body made from the data and replaced when it is read again) and
// from jplayer-mobile/internal/ui/views/settings.go at 49de3c2 (a settings row
// whose switch follows the preference it stands for, kept by the Env). The screen
// is MS.account (index.html:463-475) over GET /me and GET /me/devices.

package screens

import (
	"context"
	"errors"
	"strings"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tabAccount is the tab of screen 15 in the tab bar (nav.Tabbed).
const tabAccount = 3

// The margins of MS.account, in DIPs (index.html:463-475).
const (
	// accTitleTop, accTitleBottom are `.h1` at `margin:16px 0 14px`.
	accTitleTop, accTitleBottom float32 = 16, 14
	// accEyebrowTop, accEyebrowBottom are every `.eb` at `margin:22px 0 10px`.
	accEyebrowTop, accEyebrowBottom float32 = 22, 10
	// accRowPad is `.card` at `padding:0 14px`.
	accRowPad float32 = 14
	// accLogoutTop is the red button's `margin-top:14px`.
	accLogoutTop float32 = 14
	// accVersionTop, accVersionSize are `margin-top:6px;font-size:11px`.
	accVersionTop, accVersionSize float32 = 6, 11
)

// The ids of the actions of the rows: the `data-go`, `data-act` + `data-v` of
// MS.account, which layout.json names them by.
const (
	nameGoInfo      = "go.info"        // data-go="info"
	nameGoCoach     = "go.coach"       // data-go="coach"
	namePassword    = "sheet:password" // data-act="sheet" data-v="password"
	nameGoConnect   = "go.connect"     // data-go="connect"
	nameWiFi        = "pref:wifi"      // data-act="pref" data-v="wifi"
	nameAuto        = "pref:auto"      // data-act="pref" data-v="auto"
	nameGoDownloads = "go.downloads"   // data-go="downloads" data-root="1"
	namePrivacy     = "link:privacy"   // K32: no prototype row; opens links.privacy like the forgot link (K15)
	nameTerms       = "link:terms"     // K34: no prototype row; opens links.terms like the privacy row (K32)
	nameLogout      = "sheet:logout"   // data-act="sheet" data-v="logout"
	nameVersion     = "p.muted"        // the version line
	nameBadges      = "badges"         // the row of badges (a div without a class)
	nameProfile     = "profile"        // the card of the avatar (a div without a class)
)

// Versioned is the optional part of the services that tells the app's version
// ("DanceFITVN {v}", K2): the one of gdesk.json. Services that lack it leave the
// version out of the line.
type Versioned interface {
	AppVersion() string
}

// Account is MS.account, screen 15, the root of the tab "Tài khoản": the card of
// the user with the avatar, the badges, the cards of the profile and of the app's
// settings, the red "Đăng xuất" and the version line. It draws what the kept view
// of the account holds at once and reads GET /me and GET /me/devices again while it
// is mounted, so that a screen built after Back is not empty; the pictures (the
// avatar, the medals) load behind it.
type Account struct {
	ui.Base

	e    *env.Env
	dm   env.DownloadManager
	page *nav.Page

	profile  *profileCard
	badgeHdr *kit.Label
	badges   *badgeRow
	tv       *kit.Row
	wifi     *kit.Row
	wifiSw   *kit.Switch
	auto     *kit.Row
	autoSw   *kit.Switch
	dl       *kit.Row
	privacy  *kit.Row // K32: "Chính sách bảo mật", hidden while links.privacy is empty or unknown
	terms    *kit.Row // K34: "Điều khoản sử dụng", hidden while links.terms is empty or unknown
	version  *kit.Label

	body *accountBody

	// what the parts show now, so that a read that changes nothing does nothing
	userKey   string
	badgeKey  string
	avatarURL string
	pics      map[string]*scene.Image // medal pictures by URL
	picGen    int

	pending  []func() // picture loads asked before the screen is mounted
	unsub    func()
	cancelMe func()
	cancelDv func()
}

// accountBody stacks the parts of the page.
type accountBody struct {
	ui.Base

	stack blockStack
}

var (
	_ nav.Tabbed      = (*Account)(nil)
	_ nav.Scroller    = (*Account)(nil)
	_ ui.Mounter      = (*Account)(nil)
	_ ui.Unmounter    = (*Account)(nil)
	_ env.OnlineAware = (*Account)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewAccount returns screen 15 (MS.account, `$P:463-475`, M15), the root of the
// tab "Tài khoản", drawn from what the kept view of the account holds.
func NewAccount(e *env.Env) *Account {
	a := &Account{e: e, dm: e.Downloader(), pics: map[string]*scene.Image{}}
	a.profile = newProfileCard(e.T("avatar.title"), e.Act.AskAvatar)
	a.badgeHdr = heading(kit.Eyebrow(e.T("account.badges")), 2)
	a.badges = newBadgeRow()

	rows := a.row(icon.User, "account.info", "account.infoSub", nameGoInfo, func() { e.Act.Go(nav.Route{Screen: env.RouteInfo}, false) })
	coach := a.row(icon.Award, "account.coach", "account.coachSub", nameGoCoach, func() { e.Act.Go(nav.Route{Screen: env.RouteCoach}, false) })
	pw := a.row(icon.Lock, "account.password", "", namePassword, e.Act.AskPassword)
	pw.Last = true // `.row:last-child`
	profileCol := rowColumn(rows, coach, pw)

	a.tv = a.row(icon.Tv, "account.connect", "", nameGoConnect, func() { e.Act.Go(nav.Route{Screen: env.RouteConnect}, false) })
	a.tv.Chevron = true
	a.wifi = a.row(icon.Globe, "account.wifiOnly", "", nameWiFi, a.toggleWiFi)
	a.wifiSw = kit.NewSwitch(e.WiFiOnly())
	a.wifi.Trailing = a.wifiSw
	a.auto = a.row(icon.Rep, "account.autoNext", "", nameAuto, a.toggleAuto)
	a.autoSw = kit.NewSwitch(e.Prefs().AutoNext())
	a.auto.Trailing = a.autoSw
	a.dl = a.row(icon.Store, "account.downloaded", "", nameGoDownloads, func() { e.Act.Go(nav.Route{Screen: env.RouteDownloads}, true) })
	a.dl.Chevron = true
	a.dl.SetValue(format.Size(a.dm.TotalBytes()))
	// K32: the privacy policy, a row that opens a link (as the forgot link of the login does,
	// K15); K34: the terms of use under it, the same way. The last row shown of the card has
	// no line (syncLinks).
	a.privacy = a.row(icon.Shield, "account.privacy", "", namePrivacy, a.openPrivacy)
	a.terms = a.row(icon.List, "account.terms", "", nameTerms, a.openTerms)
	appCol := rowColumn(a.tv, a.wifi, a.auto, a.dl, a.privacy, a.terms)
	a.privacy.Hidden = a.privacyURL() == ""
	a.terms.Hidden = a.termsURL() == ""
	a.dl.Last, a.privacy.Last, a.terms.Last = a.lastRows()

	logout := kit.NewButton(kit.ButtonT, e.T("logout.confirm"), icon.NameUnknown)
	logout.Name, logout.Ink, logout.OnPress = nameLogout, theme.InkDanger, e.Act.AskLogout // `style="color:var(--dg)"`
	a.version = kit.Muted(versionText(e), accVersionSize)
	a.version.Name, a.version.Align = nameVersion, ui.TextAlignCenter

	eb2 := heading(kit.Eyebrow(e.T("account.profile")), 2)
	eb3 := heading(kit.Eyebrow(e.T("account.app")), 2)
	a.body = &accountBody{}
	a.body.stack.items = []blockItem{
		{w: kit.H1(e.T("account.title")), top: accTitleTop, bottom: accTitleBottom},
		{w: a.profile},
		{w: a.badgeHdr, top: accEyebrowTop, bottom: accEyebrowBottom},
		{w: a.badges},
		{w: eb2, top: accEyebrowTop, bottom: accEyebrowBottom},
		{w: accountCard(profileCol)},
		{w: eb3, top: accEyebrowTop, bottom: accEyebrowBottom},
		{w: accountCard(appCol)},
		{w: logout, top: accLogoutTop},
		{w: a.version, top: accVersionTop},
	}
	for _, it := range a.body.stack.items {
		a.body.stack.Add(it.w)
	}
	a.body.Add(&a.body.stack)

	a.show(e.AccountView())
	a.page = nav.NewPage(a.body, tabAccount)
	a.Add(a.page)

	return a
}

// row makes a `.row` of the card: its icon, the texts of the keys and the action.
func (a *Account) row(ic icon.Name, title, sub, name string, onPress func()) *kit.Row {
	r := kit.NewRow(ic, a.e.T(title), subText(a.e, sub))
	r.Name, r.Chevron, r.OnPress = name, true, onPress

	return r
}

// subText returns the text of a key, "" for no key.
func subText(e *env.Env, key string) string {
	if key == "" {
		return ""
	}

	return e.T(key)
}

// rowColumn stacks rows, each as wide as the room.
func rowColumn(rows ...*kit.Row) *ui.Flex {
	ws := make([]ui.Widget, len(rows))
	for i, r := range rows {
		ws[i] = r
	}
	col := ui.Column(0, ws...)
	col.Align = ui.CrossStretch

	return col
}

// accountCard is `.card` with `style="padding:0 14px"` around a column of rows.
func accountCard(rows ui.Widget) *kit.Card {
	c := kit.NewCard(rows)
	c.Padding = geom.Insets{Left: accRowPad, Right: accRowPad}

	return c
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Tab returns the Account tab.
func (a *Account) Tab() int { return tabAccount }

// Scroll returns the page's scroll.
func (a *Account) Scroll() *ui.Scroll { return a.page.Scroll() }

// UserName returns the name the card shows.
func (a *Account) UserName() string { return a.profile.name.Text }

// Email returns the email the card shows.
func (a *Account) Email() string { return a.profile.mail.Text }

// Since returns the "Thành viên từ ..." line, "" while it is hidden.
func (a *Account) Since() string {
	if a.profile.since.Hidden {
		return ""
	}

	return a.profile.since.Text
}

// Badges returns the badges shown.
func (a *Account) Badges() []*kit.Badge { return a.badges.badges }

// TV returns the value of the row "Kết nối Apple TV": "Đã kết nối" or "Chưa kết
// nối".
func (a *Account) TV() string { return a.tv.Value }

// WiFiOn tells whether the switch "Chỉ tải qua Wi‑Fi" shows on.
func (a *Account) WiFiOn() bool { return a.wifiSw.On }

// AutoOn tells whether the switch "Tự phát bài tiếp theo" shows on.
func (a *Account) AutoOn() bool { return a.autoSw.On }

// Downloaded returns the value of the row "File âm thanh đã tải", the size of
// the files.
func (a *Account) Downloaded() string { return a.dl.Value }

// Version returns the version line.
func (a *Account) Version() string { return a.version.Text }

// Avatar returns the avatar of the card.
func (a *Account) Avatar() *kit.Avatar { return &a.profile.avatar.Avatar }

// Reload reads GET /me and GET /me/devices again, as when the screen is mounted:
// what changed the account from elsewhere (the avatar sheet) calls it, or Nav.Refresh.
func (a *Account) Reload() { a.load() }

// Mounted follows the downloads while the screen is on show, and reads the
// account with the next turn (a view being mounted has no host yet, and ui.Go
// would run the read on the UI thread then).
func (a *Account) Mounted(c *ui.Context) {
	if a.unsub == nil {
		a.unsub = a.dm.Events().Subscribe(a.onDownload)
	}
	a.syncPrefs()
	a.syncDownloaded()
	c.AfterFunc(0, a.start)
}

// start reads the account and starts the picture loads asked for while the screen
// was being built.
func (a *Account) start() {
	pending := a.pending
	a.pending = nil
	for _, f := range pending {
		f()
	}
	a.load()
	a.loadConfig()
}

// Unmounted stops following the downloads and ends the reads under way.
func (a *Account) Unmounted(*ui.Context) {
	if a.unsub != nil {
		a.unsub()
		a.unsub = nil
	}
	if a.cancelMe != nil {
		a.cancelMe()
		a.cancelMe = nil
	}
	if a.cancelDv != nil {
		a.cancelDv()
		a.cancelDv = nil
	}
}

// SetOnline reads the account again when the network comes back.
func (a *Account) SetOnline(online bool) {
	if online {
		a.load()
	}
}

// Layout fills the window with the page.
func (a *Account) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(a.page, ui.Tight(cs.Max))
	c.Place(a.page, geom.Point{})

	return cs.Max
}

// Layout is the stack's.
func (b *accountBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	sz := c.Layout(&b.stack, cs)
	c.Place(&b.stack, geom.Point{})

	return cs.Constrain(sz)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// show draws the kept view: the user of GET /me, else the session's (no badges
// yet), the TV row from the devices, and starts loading the pictures it has not.
func (a *Account) show(v env.AccountState) {
	u, ok := v.Me.User, v.HasMe
	if !ok {
		u, ok = a.e.Account().SessionUser()
	}
	if ok {
		a.showUser(u, v.Me.Badges)
	}
	a.syncTV(v)
}

// showUser puts the user on the card and the badges on the row, each only when it
// is not what it already shows.
func (a *Account) showUser(u model.User, badges []model.Badge) {
	since := ""
	if !u.MemberSince.IsZero() {
		since = a.e.T("account.since") + " " + format.Date(u.MemberSince.Time())
	}
	if key := u.FirstName + "\x00" + u.LastName + "\x00" + u.FullName + "\x00" + u.Email + "\x00" + since; key != a.userKey {
		a.userKey = key
		a.profile.set(u, since)
		a.profile.Relayout()
	}
	if u.AvatarURL != a.avatarURL {
		a.avatarURL = u.AvatarURL
		a.profile.avatar.Avatar.SetImage(nil)
		a.loadPicture(u.AvatarURL, func(img *scene.Image) { a.profile.avatar.Avatar.SetImage(img) }, func() bool { return a.avatarURL == u.AvatarURL })
	}
	a.showBadges(badges)
}

// showBadges makes the row of badges from the API's, when they changed; a medal
// is drawn at once with the prototype's figure, and the picture replaces it when
// it has loaded.
func (a *Account) showBadges(badges []model.Badge) {
	key := ""
	for _, b := range badges {
		key += b.Code + "\x00" + b.Name + "\x00" + b.ImageURL + "\x01"
	}
	if key == a.badgeKey {
		return
	}
	a.badgeKey = key
	a.picGen++
	gen := a.picGen
	row := make([]*kit.Badge, len(badges))
	for i, b := range badges {
		bd := kit.NewBadge(b.Name, accentOf(b.Code), medalSub(b.Code))
		row[i] = bd
		if img, ok := a.pics[b.ImageURL]; ok && b.ImageURL != "" {
			bd.SetImage(img)

			continue
		}
		a.loadPicture(b.ImageURL, func(img *scene.Image) {
			a.pics[b.ImageURL] = img
			bd.SetImage(img)
		}, func() bool { return a.picGen == gen })
	}
	a.badges.set(row)
	kit.SetHidden(a.badgeHdr, len(badges) == 0)
	kit.SetHidden(a.badges, len(badges) == 0)
	a.body.Relayout()
}

// syncTV sets the value of "Kết nối Apple TV" from the devices: a tvOS device in
// the list is a TV that is linked.
func (a *Account) syncTV(v env.AccountState) {
	text := a.e.T("account.notLinked")
	if v.HasDevices {
		for _, d := range v.Devices.Items {
			if d.Platform == model.PlatformTVOS {
				text = a.e.T("account.linked")

				break
			}
		}
	}
	a.tv.SetValue(text)
}

// syncPrefs sets the switches to the preferences, which a screen in between (the
// sheet of the download, the player) may have changed.
func (a *Account) syncPrefs() {
	a.wifiSw.SetOn(a.e.WiFiOnly())
	a.autoSw.SetOn(a.e.Prefs().AutoNext())
}

// syncDownloaded sets the size of the downloaded files; the same size changes
// nothing.
func (a *Account) syncDownloaded() { a.dl.SetValue(format.Size(a.dm.TotalBytes())) }

// onDownload follows the downloads: what is in or gone changes the size; a progress
// changes nothing this screen shows.
func (a *Account) onDownload(ev env.DownloadEvent) {
	switch ev.Kind {
	case env.DownloadEventChanged, env.DownloadEventFinished:
		a.syncDownloaded()
	case env.DownloadEventUnknown, env.DownloadEventProgress, env.DownloadEventPaused,
		env.DownloadEventFailed, env.DownloadEventSpace:
	default:
	}
}

// Privacy returns the row "Chính sách bảo mật" (K32), which is Hidden while the app config
// holds no links.privacy.
func (a *Account) Privacy() *kit.Row { return a.privacy }

// PrivacyShown tells whether the row "Chính sách bảo mật" is on the screen.
func (a *Account) PrivacyShown() bool { return !a.privacy.Hidden }

// TermsRow returns the row "Điều khoản sử dụng" (K34), which is Hidden while the app config
// holds no links.terms.
func (a *Account) TermsRow() *kit.Row { return a.terms }

// TermsShown tells whether the row "Điều khoản sử dụng" is on the screen.
func (a *Account) TermsShown() bool { return !a.terms.Hidden }

// syncLinks shows the rows "Chính sách bảo mật" and "Điều khoản sử dụng" when the app config
// has their link, hides each when its link is empty or the config is not known (no network
// at launch; the screen asks for it once, loadConfig), and leaves the line out under the
// last row shown of the card (`.row:last-child`).
func (a *Account) syncLinks() {
	changed := kit.SetHidden(a.privacy, a.privacyURL() == "")
	if kit.SetHidden(a.terms, a.termsURL() == "") {
		changed = true
	}
	dl, priv, terms := a.lastRows()
	if a.dl.Last != dl || a.privacy.Last != priv || a.terms.Last != terms {
		a.dl.Last, a.privacy.Last, a.terms.Last = dl, priv, terms
		changed = true
	}
	if changed {
		a.dl.Relayout()
		a.privacy.Relayout()
		if a.body != nil {
			a.body.Relayout()
		}
	}
}

// lastRows tells which of the downloaded files', the privacy and the terms rows is the last
// one shown of the card of the app.
func (a *Account) lastRows() (dl, privacy, terms bool) {
	switch {
	case !a.terms.Hidden:
		return false, false, true
	case !a.privacy.Hidden:
		return false, true, false
	default:
		return true, false, false
	}
}

// privacyURL returns links.privacy of the app config the boot fetched, "" for none.
func (a *Account) privacyURL() string {
	cfg, ok := a.e.Cfg()
	if !ok {
		return ""
	}

	return strings.TrimSpace(cfg.Links.Privacy)
}

// openPrivacy opens links.privacy in the system's browser (K32), through the same service
// as the forgot link of the login.
func (a *Account) openPrivacy() {
	if u := a.privacyURL(); u != "" {
		a.e.OpenURL(u)
	}
}

// termsURL returns links.terms of the app config the boot fetched, "" for none.
func (a *Account) termsURL() string {
	cfg, ok := a.e.Cfg()
	if !ok {
		return ""
	}

	return strings.TrimSpace(cfg.Links.Terms)
}

// openTerms opens links.terms in the system's browser (K34), as openPrivacy does.
func (a *Account) openTerms() {
	if u := a.termsURL(); u != "" {
		a.e.OpenURL(u)
	}
}

// loadConfig asks for the app config once when the boot had none (no network at launch), so
// that the rows of the privacy policy and the terms of use can show; what fails leaves them
// hidden.
func (a *Account) loadConfig() {
	if _, ok := a.e.Cfg(); ok {
		return
	}
	ui.Go(a, func(ctx context.Context) (model.AppConfig, error) {
		return a.e.Config(ctx)
	}, func(_ *ui.Context, cfg model.AppConfig, err error) {
		switch {
		case err == nil:
			a.e.SetConfig(cfg)
			a.syncLinks()
		case errors.Is(err, context.Canceled):
		case a.e.Refused(err): // 426: the upgrade screen (03 section 9)
		default: // no network: the row stays hidden
		}
	})
}

// toggleWiFi is `pref:wifi` (`$P:610`): the preference turns over and the switch
// shows it.
func (a *Account) toggleWiFi() {
	on := !a.e.WiFiOnly()
	a.e.SetWiFiOnly(on)
	a.wifiSw.SetOn(on)
}

// toggleAuto is `pref:auto`: "Tự phát bài tiếp theo" turns over.
func (a *Account) toggleAuto() {
	on := !a.e.Prefs().AutoNext()
	a.e.SetAutoNext(on)
	a.autoSw.SetOn(on)
}

// load reads GET /me and GET /me/devices, each one read at a time: a read in
// flight is dropped for the new one. What fails leaves what is on show: the kept
// view or the session's user, which is what a phone with no network shows.
func (a *Account) load() {
	if a.cancelMe != nil {
		a.cancelMe()
	}
	a.cancelMe = ui.Go(a, func(ctx context.Context) (model.Me, error) {
		return a.e.Account().Me(ctx)
	}, a.meLoaded)
	if a.cancelDv != nil {
		a.cancelDv()
	}
	a.cancelDv = ui.Go(a, func(ctx context.Context) (model.DeviceList, error) {
		return a.e.Account().Devices(ctx)
	}, a.devicesLoaded)
}

// meLoaded takes GET /me.
func (a *Account) meLoaded(_ *ui.Context, m model.Me, err error) {
	a.cancelMe = nil
	switch {
	case err == nil:
		a.showUser(m.User, m.Badges)
	case errors.Is(err, context.Canceled):
	case a.e.Refused(err): // 426: the upgrade screen (03 section 9)
	default: // no network or a server in trouble: what is shown stays
	}
}

// devicesLoaded takes GET /me/devices.
func (a *Account) devicesLoaded(_ *ui.Context, d model.DeviceList, err error) {
	a.cancelDv = nil
	switch {
	case err == nil:
		a.syncTV(env.AccountState{Devices: d, HasDevices: true})
	case errors.Is(err, context.Canceled):
	case a.e.Refused(err):
	default:
	}
}

// loadPicture loads the picture at url behind the screen and gives it to set, when
// it is still wanted (current says so). No url, or a load that fails, leaves the
// drawn figure (K4 of the kit: the person and the medal).
func (a *Account) loadPicture(url string, set func(*scene.Image), current func() bool) {
	if url == "" {
		return
	}
	if a.View() == nil { // not mounted yet: ui.Go has no host to deliver to
		a.pending = append(a.pending, func() { a.loadPicture(url, set, current) })

		return
	}
	ui.Go(a, func(ctx context.Context) (*scene.Image, error) {
		return a.e.Avatar(ctx, url)
	}, func(_ *ui.Context, img *scene.Image, err error) {
		if err == nil && img != nil && current() {
			set(img)
		}
	})
}

// versionText is "DanceFITVN {version}" (K2: the app's real version, where the
// prototype has "1.0 (prototype)"); services that do not tell it leave it out.
func versionText(e *env.Env) string {
	v := ""
	if s, ok := e.Services.(Versioned); ok {
		v = s.AppVersion()
	}

	return strings.TrimSpace(e.Fill("account.versionTpl", "v", v))
}
