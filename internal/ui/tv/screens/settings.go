//go:build tvos

// Adapted from dancefitvn-app/internal/ui/mobile/screens/account.go (the read of
// the user and his badges after Mounted, the badges' pictures loaded one by one
// and dropped when the screen is left, the "member since" line left out with no
// day) and from jplayer-tv/internal/ui/views/settings.go at 781bee5 (the arrow
// that takes the focus from a row of the list to the navigation). The screen is
// TS.settings (index.html:706-712, docs/plan/P7 section 4.2).

package screens

import (
	"context"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/focus"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the settings rows (index.html:711), in DIPs.
const (
	// settingsRowsX, settingsRowsY, settingsRowsW are the rows, `left:780px;
	// right:90px;top:190px`.
	settingsRowsX, settingsRowsY, settingsRowsW float32 = 780, 190, 1050
	// settingsRowsGap is `margin-bottom:20px` of a row.
	settingsRowsGap float32 = 20
	// settingsFootTop, settingsFootSize are `font-size:22px;color:var(--tx3);
	// margin-top:20px` of the version line.
	settingsFootTop, settingsFootSize float32 = 20, 22
)

// The keys of the rows: `data-fid` of TS.settings.
const (
	keySettingsQuality = "s0"
	keySettingsAuto    = "s1"
	keySettingsBeat    = "s2"
	keySettingsLang    = "s3"
	keySettingsHelp    = "s4"
	keySettingsLogout  = "s5"
)

// The rows, in order: `[['hd',...],['rep',...],['music',...],['globe',...],
// ['phone',...],['out',...]]`.
const (
	rowQuality = iota
	rowAuto
	rowBeat
	rowLang
	rowHelp
	rowLogout
	settingsRows
)

// nameSettingsFoot is the key layout.json would give the version line, a `p` with
// no class.
const nameSettingsFoot = "tv.settings.version"

// Settings is TS.settings (TV12): the profile card at the left (the photo, the
// name, since when, the email and the badges, from GET /me) and the six rows at the
// right: the video quality (`s0`, which cycles Tự động, 1080p, 720p), the next
// lesson playing by itself (`s1`), the beat bar (`s2`), the language and the
// support's number (`s3` and `s4`, which do nothing) and the sign-out (`s5`), with
// the version under them. The default focus is the first `.f` of the screen, which
// is the nav bar's first item (renderTV, `settings` excepted from skipping it);
// Left from a row goes to the item that is on, `nav3` (the prototype's pick).
//
// Select on `s5` signs the TV out: the call to the services ends with the welcome
// screen as the root, whatever its answer, since the session is gone locally either
// way. The card and the rows come when the user is read; a read that fails shows
// TV11 (K28).
type Settings struct {
	ui.Base

	e *env.Env

	nav  *tvkit.Nav
	card *settingsCard
	rows [settingsRows]*tvkit.SettingRow
	foot *kit.Label
	hotl string
	have bool
	// signingOut is set once "Đăng xuất khỏi Apple TV" ran: the screen is leaving, and a
	// second press of the same Select signs nothing out again.
	signingOut bool

	cancels    []func() // the reads of the pictures
	rd, avatar reader   // the read of the user, the read of the nav bar's picture
}

var (
	_ ui.Mounter   = (*Settings)(nil)
	_ ui.Unmounter = (*Settings)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewSettings returns TV12: the nav bar until the user is read.
func NewSettings(e *env.Env) *Settings {
	s := &Settings{e: e}
	s.nav = newNavBar(e, tabSettings)
	s.Add(s.nav)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Item returns the row of the key (`s0`..`s5`), nil when there is none yet.
func (s *Settings) Item(key string) *tvkit.SettingRow {
	for _, r := range s.rows {
		if r != nil && r.Key == key {
			return r
		}
	}

	return nil
}

// Nav returns the nav bar.
func (s *Settings) Nav() *tvkit.Nav { return s.nav }

// Card returns the profile card, nil until the user is read.
func (s *Settings) Card() *settingsCard { return s.card }

// Loaded tells whether the content is on screen.
func (s *Settings) Loaded() bool { return s.have }

// Mounted starts reading with the next turn: a view that is being mounted has no
// host yet, and ui.Go would run the read on the UI thread then.
func (s *Settings) Mounted(c *ui.Context) {
	if !s.avatar.Done() && !s.avatar.Busy() {
		c.AfterFunc(0, s.loadAvatar)
	}
	if !s.have && !s.rd.Busy() {
		c.AfterFunc(0, s.load)
	}
}

// Unmounted ends the reads in flight.
func (s *Settings) Unmounted(*ui.Context) {
	for _, f := range s.cancels {
		if f != nil {
			f()
		}
	}
	s.cancels = s.cancels[:0]
	s.rd.Stop()
	s.avatar.Stop()
}

// Layout fills the room with the nav bar, puts the card and the rows where the
// prototype does.
func (s *Settings) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	if s.have {
		c.Layout(s.card, ui.Tight(geom.Sz(cardW, cardH)))
		c.Place(s.card, geom.Pt(cardX, cardY))
		y := settingsRowsY
		for _, r := range s.rows {
			rs := c.Layout(r, ui.Tight(geom.Sz(settingsRowsW, tvkit.SettingRowHeight)))
			c.Place(r, geom.Pt(settingsRowsX, y))
			y += rs.H + settingsRowsGap
		}
		// the last row's margin and the line's collapse into one of 20
		c.Layout(s.foot, ui.Loose(geom.Sz(settingsRowsW, ui.Unbounded)))
		c.Place(s.foot, geom.Pt(settingsRowsX, y-settingsRowsGap+max(settingsRowsGap, settingsFootTop)))
	}
	c.Layout(s.nav, ui.Tight(size))
	c.Place(s.nav, geom.Point{})

	return size
}

// Paint draws the screen; a focused row last, over the next (F-12).
func (s *Settings) Paint(c *ui.Context, p *ui.Painter) { tvkit.PaintRaised(c, p, &s.Base) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// load reads the user and the support's hotline: GET /me decides, the hotline is
// a line of the screen that may be empty.
func (s *Settings) load() {
	if s.have {
		return
	}
	type result struct {
		me   model.Me
		hotl string
	}
	read(&s.rd, s, func(ctx context.Context) (result, error) {
		me, err := s.e.Me(ctx)
		if err != nil {
			return result{}, err
		}
		r := result{me: me}
		if cfg, err := s.e.Config(ctx); err == nil {
			r.hotl = cfg.Support.Hotline
		}

		return r, nil
	}, func(_ *ui.Context, r result, err error) {
		if err != nil {
			failed(s.e, err)

			return
		}
		s.hotl = r.hotl
		s.show(r.me)
	})
}

// loadAvatar reads the picture of the nav bar.
func (s *Settings) loadAvatar() { loadAvatar(s, &s.avatar, s.e, s.nav) }

// show builds the card and the rows for the user.
func (s *Settings) show(me model.Me) {
	e := s.e
	since := ""
	if !me.User.MemberSince.IsZero() {
		since = e.T("account.since") + " " + format.Date(me.User.MemberSince.Time())
	}
	s.card = newSettingsCard(me.User, since, e.T("account.badges"), e.T("tv.settings.editHint"), me.Badges)
	s.Add(s.card)
	if u := me.User.AvatarURL; u != "" {
		s.cancels = append(s.cancels, s.image(u, s.card.photo.SetImage))
	}
	for i, b := range me.Badges {
		if b.ImageURL != "" {
			m := s.card.medals[i]
			s.cancels = append(s.cancels, s.image(b.ImageURL, m.SetImage))
		}
	}

	names := [settingsRows]string{
		e.T("options.quality"), e.T("account.autoNext"), e.T("tv.settings.beat"), e.T("tv.settings.lang"),
		e.T("tv.settings.support"), e.T("tv.settings.logout"),
	}
	icons := [settingsRows]icon.Name{icon.Hd, icon.Rep, icon.Music, icon.Globe, icon.Phone, icon.Out}
	for i := range s.rows {
		r := tvkit.NewSettingRow("s"+itoa(i), icons[i], names[i], "")
		r.OnMove = s.moveLeft // Left goes to the nav bar's active item
		s.rows[i] = r
		s.Add(r)
	}
	s.rows[rowLogout].Danger = true
	s.rows[rowQuality].OnSelect = func() { s.act(rowQuality) }
	s.rows[rowAuto].OnSelect = func() { s.act(rowAuto) }
	s.rows[rowBeat].OnSelect = func() { s.act(rowBeat) }
	s.rows[rowLogout].OnSelect = s.logout
	s.rows[rowLang].SetValue(e.T("tv.settings.langValue"))
	s.rows[rowHelp].SetValue(s.hotl)
	s.foot = &kit.Label{
		Text: e.Fill("tv.settings.versionTpl", "v", e.Version()), Size: settingsFootSize, Ink: theme.InkText3,
	}
	s.foot.Name = nameSettingsFoot
	s.Add(s.foot)
	s.syncRows()
	s.have = true
	s.Relayout()
}

// image loads the picture at url and gives it to set, dropped when the screen is
// left; it returns the cancel of the read.
func (s *Settings) image(url string, set func(*scene.Image)) (cancel func()) {
	return ui.Go(s, func(ctx context.Context) (*scene.Image, error) {
		return s.e.Avatar(ctx, url)
	}, func(_ *ui.Context, img *scene.Image, err error) {
		if err == nil && img != nil {
			set(img)
		}
	})
}

// moveLeft is the arrow Left on a row: the focus goes to the nav bar's item that
// is on, `nav3`, which is where the prototype's pick puts it (K9). Other arrows
// are left to the focus layer.
func (s *Settings) moveLeft(d focus.Dir) bool {
	v := s.View()
	if d != focus.DirLeft || v == nil {
		return false
	}
	v.Context().Focus(s.nav.Item(tabSettings))

	return true
}

// act is `tset` (index.html:743) for the three rows that change something.
func (s *Settings) act(row int) {
	switch row {
	case rowQuality:
		s.e.Prefs().CycleQualityTV()
	case rowAuto:
		p := s.e.Prefs()
		p.SetAutoNext(!p.AutoNext())
	case rowBeat:
		if pb := s.e.Playback(); pb != nil {
			pb.SetBeat(!pb.Beat())
		}
	default:
	}
	s.syncRows()
}

// syncRows writes the values the rows show.
func (s *Settings) syncRows() {
	e := s.e
	s.rows[rowQuality].SetValue(s.qualityText(e.Prefs().Quality()))
	s.rows[rowAuto].SetValue(s.onOff(e.Prefs().AutoNext()))
	beat := false
	if pb := e.Playback(); pb != nil {
		beat = pb.Beat()
	}
	s.rows[rowBeat].SetValue(s.onOff(beat))
}

// qualityText is the name of a video quality: Tự động, 1080p, 720p; a stored 480p
// is the one the prototype does not have.
func (s *Settings) qualityText(q store.Quality) string {
	switch q {
	case store.QualityP1080:
		return s.e.T("options.q1080")
	case store.QualityP720:
		return s.e.T("options.q720")
	case store.QualityP480:
		return s.e.T("options.q480")
	case store.QualityUnknown, store.QualityAuto:
		return s.e.T("options.qualityAuto")
	default:
		return s.e.T("options.qualityAuto")
	}
}

// onOff is "Bật" or "Tắt".
func (s *Settings) onOff(on bool) string {
	if on {
		return s.e.T("tv.on")
	}

	return s.e.T("tv.off")
}

// logout is `tlogout` (index.html:748): the welcome screen is the root at once, as
// the prototype does (`S.tv.auth=false;...;tvGo('welcome',{root:1})`), and the
// services sign the TV out meanwhile: the save of the position, the queue and POST
// /tv/logout take up to 15 s and nobody waits for them. The call is not tied to this
// screen (it is gone the moment the welcome screen is the root) nor to a context of
// the UI: it runs on a goroutine of its own and the services, which own the session,
// finish it. Nothing of the UI is touched from it, and its answer is not needed (the
// session is gone locally either way).
func (s *Settings) logout() {
	if s.signingOut {
		return // a second press of the same Select
	}
	s.signingOut = true
	e := s.e
	go func() { _ = e.LogoutTV(context.Background()) }()
	if e.Act != nil {
		e.Act.Go(nav.Route{Screen: env.RouteWelcome}, true)
	}
}
