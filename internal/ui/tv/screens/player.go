//go:build tvos

// Adapted from dancefitvn-app/internal/ui/mobile/screens/video.go (a screen that
// follows the player's Changes while it is mounted, changes a widget only when its
// value changed, and tells the player when it is left) and from
// jplayer-tv/internal/ui/watch/screen.go at 781bee5 (the video plane under the
// controls, the Play/Pause handler kept while the screen is up). The screen is
// TS.player (index.html:680-693, docs/plan/P7 section 4.2).

package screens

import (
	"context"
	"strconv"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of TS.player (index.html:680-688), in DIPs.
const (
	// playerPlaneX, playerPlaneW, playerPlaneH are the stage of the video,
	// `left:656px;top:0;width:608px;height:1080px`.
	playerPlaneX, playerPlaneW, playerPlaneH float32 = 656, 608, 1080
	// playerBlockX, playerInfoY, playerInfoW are the lesson block, `left:90px;
	// top:100px;width:520px`.
	playerBlockX, playerInfoY, playerInfoW float32 = 90, 100, 520
	// playerEyebrowSize, playerEyebrowSpacing are `font-size:24px;font-weight:800;
	// letter-spacing:2px` of the course line and of "TIẾP THEO".
	playerEyebrowSize, playerEyebrowSpacing float32 = 24, 2
	// playerTitleTop, playerTitleSize, playerTitleLine are `.d` at `font-size:90px;
	// font-weight:800;margin-top:24px;line-height:.95`.
	playerTitleTop, playerTitleSize, playerTitleLine float32 = 24, 90, .95
	// playerChipsTop, playerChipsGap, playerChipH, playerChipR are `display:flex;
	// gap:14px;margin-top:30px` of the chips and their `height:48px;
	// border-radius:24px`.
	playerChipsTop, playerChipsGap, playerChipH, playerChipR float32 = 30, 14, 48, 24
	// playerTagPad, playerTagSize are the tag chip's `padding:0 22px;font-size:24px`;
	// playerKindPad, playerKindSize the kind chip's `padding:0 20px;font-size:22px`.
	playerTagPad, playerTagSize, playerKindPad, playerKindSize float32 = 22, 24, 20, 22
	// playerStatusTop, playerStatusGap, playerStatusH are `margin-top:34px;
	// display:flex;flex-direction:column;gap:14px` of the status lines, 34 tall each
	// (the icon's 28 on the baseline and the 26 px text's descent).
	playerStatusTop, playerStatusGap, playerStatusH float32 = 34, 14, 34
	// playerStatusIcon, playerStatusSize are `ic(n,28)` and the lines'
	// `font-size:26px;font-weight:700`; the space between them is inlineIconSpace.
	playerStatusIcon, playerStatusSize float32 = 28, 26
	playerStatusSpace                          = inlineIconSpace
	// playerBottomY is the transport block, `left:90px;top:760px;width:520px`.
	playerBottomY float32 = 760
	// playerBeatsBottom is the bar's `margin-bottom:30px`.
	playerBeatsBottom float32 = 30
	// playerTimeSize is `font-size:30px;font-weight:700` of the times (`.mono`);
	// playerDurOpacity the length's `opacity:.6`.
	playerTimeSize, playerDurOpacity float32 = 30, .6
	// playerScrubTop is `margin-top:10px` of the position bar; playerRowTop and
	// playerRowGap are `display:flex;gap:20px;margin-top:30px` of the buttons.
	playerScrubTop, playerRowTop, playerRowGap float32 = 10, 30, 20
	// playerBtnPlay, playerBtnOpts are the buttons' icons: `width:28px` PLAY and
	// `ic('speed',30)`.
	playerBtnPlay, playerBtnOpts float32 = 28, 30
	// playerNextX, playerNextY, playerNextW are the column of the next lessons,
	// `left:1340px;top:100px;width:490px`.
	playerNextX, playerNextY, playerNextW float32 = 1340, 100, 490
	// playerNextHeadBottom, playerRowsGap are the heading's `margin-bottom:20px` and
	// the rows' `margin-bottom:12px`.
	playerNextHeadBottom, playerRowsGap float32 = 20, 12
	// playerNextRows is how many lessons the column lists, `L.slice(l.n,l.n+4)`.
	playerNextRows = 4
)

// The keys of the controls: `data-fid` of TS.player.
const (
	keyPlayerToggle = "p1"
	keyPlayerOpts   = "p2"
	keyPlayerNext   = "pn"
)

// Names layout.json knows the parts by: `div.d` of the title, the chips and the
// status icons are the capture's; the lines without a class have none.
const (
	namePlayerTitle   = "div.d"
	namePlayerEyebrow = "tv.player.eyebrow"
	namePlayerPos     = "tv.player.pos"
	namePlayerDur     = "tv.player.dur"
	namePlayerScrub   = "div.scrub"
	namePlayerNext    = "tv.player.next"
	namePlayerSpeed   = "tv.player.speed"
	namePlayerMirror  = "tv.player.mirror"
	namePlayerLoop    = "tv.player.loop"
)

// playerBg is `<div class="scr" style="background:#0B0B0C">`: the page behind the
// controls, with the hole of the video left out.
var playerBg = color.RGB(0x0b, 0x0b, 0x0c)

// playerKindBg is `background:#2A2D33` of the kind chip, and white its text.
var playerKindBg = color.RGB(0x2a, 0x2d, 0x33)

// Player is TS.player (TV07): the lesson's video in the hole at 656,0 (the K5 stage
// until the first frame), the lesson's name, tag and kind at the left with the
// options that are on, the beat bar, the clock, the position bar and the buttons
// `p1` (Phát / Tạm dừng) and `p2` (Tuỳ chọn) under them, and at the right the
// next four lessons of the course, `pn0`..`pn3`. `p1` has the focus first.
//
// It is the view of the player (env.Playback): the screen follows its Changes while
// it is mounted, changing a widget only when its value changed (4 Hz while it
// plays, 0 allocations), and the Play/Pause key toggles while it is up. A video
// that ends gives way to the up-next screen (`$P:782`); the network going away
// pauses and shows TV11 over it (`$P:812`). `p2` opens the panel (TV08), which
// takes the arrows until it closes.
type Player struct {
	ui.Base

	e  *env.Env
	pb env.Playback

	plane *tvkit.VideoPlane

	eyebrow *kit.Label
	title   *kit.Label
	tag     *kit.Chip
	kind    *kit.Chip
	speed   *statusRow
	mirror  *statusRow
	loop    *statusRow

	beats *tvkit.Beats
	pos   *kit.Label
	dur   *kit.Label
	scrub *tvkit.Scrub
	play  *tvkit.Button
	opts  *tvkit.Button

	nextHead *kit.Label
	rows     []*tvkit.NextRow

	body  *playerBody // everything but the panel, left out of a reading while the panel is up
	panel *playerPanel

	list   []player.Lesson // the lessons of the course, as the player walks them
	lesson string          // the id the labels were made for
	rate   float64         // the speed the status line was made for
	sec    int             // the whole second the clock shows

	unsub, stopOnline, removePlay func()
	rd                            reader // the read of the course
	ctx                           *ui.Context
}

// playerBody is the content of the screen under the panel: the plane, the lesson
// block, the transport and the next lessons. While the panel is open it is
// `inert` (the panel is a modal over it, TV08): a screen reader reads the panel
// and nothing under it. Adapted from jplayer-tv/internal/ui/kit/hush.go at
// 781bee5 (Hush on the shell while a dialog is open).
type playerBody struct {
	ui.Base
	kit.Hush

	p *Player
}

// statusRow is a line of the status column: an icon and its text, 34 tall, the
// text on the icon's baseline.
type statusRow struct {
	ui.Base

	icon  *kit.Icon
	label *kit.Label
}

var (
	_ ui.Mounter    = (*Player)(nil)
	_ ui.Unmounter  = (*Player)(nil)
	_ env.PanelHost = (*Player)(nil)
	_ env.Holder    = (*Player)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewPlayer returns TV07 over the player of the services. Without one (a Services
// that has none) it is a bare screen whose only control is `p1`, so that Menu
// still goes back.
func NewPlayer(e *env.Env) ui.Widget {
	pb := e.Playback()
	if pb == nil {
		return newBareScreen(e, keyPlayerToggle)
	}
	s := &Player{e: e, pb: pb}
	s.plane = tvkit.NewVideoPlane(pb)

	s.eyebrow = &kit.Label{
		Size: playerEyebrowSize, Weight: 800, Spacing: playerEyebrowSpacing, Upper: true, Wrap: true, Ink: theme.InkPrimary,
	}
	s.eyebrow.Name = namePlayerEyebrow
	s.title = &kit.Label{
		Role: theme.RoleDisplay, Size: playerTitleSize, Weight: 800, LineHeight: playerTitleLine, Spacing: homeDisplaySpacing,
		Upper: true, Wrap: true, Ink: theme.InkText,
	}
	s.title.Name = namePlayerTitle
	s.title.Heading = 1 // the lesson, the title of the screen
	s.tag = kit.NewChip(kit.ChipY, "")
	s.tag.Height, s.tag.Radius, s.tag.PadX, s.tag.Size = playerChipH, playerChipR, playerTagPad, playerTagSize
	s.kind = kit.NewChip(kit.ChipKindUnknown, e.T("options.vertical"))
	s.kind.Height, s.kind.Radius, s.kind.PadX, s.kind.Size = playerChipH, playerChipR, playerKindPad, playerKindSize
	s.kind.Bg, s.kind.Fg = playerKindBg, color.RGB(255, 255, 255)
	s.speed = newStatusRow(icon.Speed, namePlayerSpeed)
	s.mirror = newStatusRow(icon.Mirror, namePlayerMirror)
	s.mirror.label.Text = e.T("options.mirror")
	s.loop = newStatusRow(icon.Rep, namePlayerLoop)

	s.beats = tvkit.NewBeats()
	s.beats.Label = e.T("tv.panel.beat") // `#tBeats`: the bar is one element
	s.pos = &kit.Label{Role: theme.RoleMono, Size: playerTimeSize, Weight: 700, Ink: theme.InkText}
	s.pos.Name = namePlayerPos
	s.dur = &kit.Label{Role: theme.RoleMono, Size: playerTimeSize, Weight: 700, Ink: theme.InkText}
	s.dur.Name = namePlayerDur
	s.scrub = tvkit.NewScrub()
	s.scrub.Name, s.scrub.Label = namePlayerScrub, e.T("a11y.position")

	s.play = tvkit.NewButton(keyPlayerToggle, e.T("tv.player.play"), icon.Play)
	s.play.Size, s.play.IconSize, s.play.PlainIcon = tvkit.ButtonSmall, playerBtnPlay, true
	s.play.OnSelect = func() { pb.Toggle() } // `tplay`
	s.opts = tvkit.NewButton(keyPlayerOpts, e.T("tv.player.options"), icon.Speed)
	s.opts.Size, s.opts.IconSize = tvkit.ButtonSmall, playerBtnOpts
	s.opts.OnSelect = s.openPanel // `topts`

	s.nextHead = &kit.Label{
		Text: e.T("tv.player.next"), Size: playerEyebrowSize, Weight: 800, Spacing: playerEyebrowSpacing, Upper: true, Ink: theme.InkText2,
	}
	s.nextHead.Name = namePlayerNext
	s.nextHead.Heading = 2 // the title of the list that follows

	s.body = &playerBody{p: s}
	s.body.Silent = func() bool { return s.panel != nil } // `inert` under the panel
	s.Add(s.body)
	s.body.Add(s.plane, s.eyebrow, s.title, s.tag, s.kind, s.speed, s.mirror, s.loop,
		s.beats, s.pos, s.dur, s.scrub, s.play, s.opts, s.nextHead)
	s.syncAll()

	return s
}

// newStatusRow returns a status line with the symbol icn; name is the key of its
// text (the capture records the icons only).
func newStatusRow(icn icon.Name, name string) *statusRow {
	r := &statusRow{icon: kit.NewIcon(icn, playerStatusIcon, theme.InkPrimary)}
	r.label = &kit.Label{Size: playerStatusSize, Weight: 700, Ink: theme.InkPrimary}
	r.label.Name = name
	r.Add(r.icon, r.label)
	r.Hidden = true

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Item returns the control of the key (`p1`, `p2`, `pn0`..), nil when there is none.
func (s *Player) Item(key string) *tvkit.Item {
	switch key {
	case keyPlayerToggle:
		return &s.play.Item
	case keyPlayerOpts:
		return &s.opts.Item
	default:
		for _, r := range s.rows {
			if r.Key == key {
				return &r.Item
			}
		}
		if s.panel != nil {
			return s.panel.item(key)
		}
	}

	return nil
}

// Plane returns the video's plane widget.
func (s *Player) Plane() *tvkit.VideoPlane { return s.plane }

// PanelOpen tells whether the panel is open.
func (s *Player) PanelOpen() bool { return s.panel != nil }

// Rows returns the next-lesson rows shown.
func (s *Player) Rows() []*tvkit.NextRow { return s.rows }

// Beats returns the beat bar.
func (s *Player) Beats() *tvkit.Beats { return s.beats }

// Hole is the rectangle the video's plane shows through, in window coordinates.
func (s *Player) Hole() geom.Rect { return s.plane.WindowFrame() }

// Mounted follows the player, keeps the Play/Pause key and the network watch, and
// reads the course for the lessons that come next.
func (s *Player) Mounted(c *ui.Context) {
	s.ctx = c
	if s.unsub == nil {
		s.unsub = s.pb.Changes().Subscribe(s.onChange)
	}
	if s.removePlay == nil && s.e.Act != nil {
		s.removePlay = s.e.Act.OnPlayPause(s.pb.Toggle)
	}
	if s.stopOnline == nil {
		s.stopOnline = s.e.WatchOnline(s.onOnline)
	}
	s.syncAll()
	if !s.rd.Busy() && len(s.list) == 0 {
		c.AfterFunc(0, s.load)
	}
}

// Unmounted stops following, gives the key back and tells the player the screen
// was left, but for a lesson taking this screen's place.
func (s *Player) Unmounted(*ui.Context) {
	for _, f := range [...]*func(){&s.unsub, &s.removePlay, &s.stopOnline} {
		if *f != nil {
			(*f)()
			*f = nil
		}
	}
	s.rd.Stop()
	if s.e.Stack == nil || s.e.Stack.Screen() != env.RoutePlayer {
		s.pb.Leave()
	}
}

// PanelClosed takes the panel away: Menu closed it (`tvBack`).
func (s *Player) PanelClosed() { s.dropPanel() }

// Layout puts the content (the body) over the whole screen and the panel over it.
func (s *Player) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	c.Layout(s.body, ui.Tight(size))
	c.Place(s.body, geom.Point{})
	if s.panel != nil {
		ps := c.Layout(s.panel.Panel, ui.Tight(geom.Sz(tvkit.PanelW, size.H-2*tvkit.PanelInset)))
		c.Place(s.panel.Panel, geom.Pt(size.W-tvkit.PanelInset-ps.W, tvkit.PanelInset))
	}

	return size
}

// Paint fills the page with #0B0B0C on both sides of the hole, which stays
// transparent for the plane, and paints the controls.
func (s *Player) Paint(c *ui.Context, p *ui.Painter) {
	r := s.Bounds()
	paint := scene.Solid(playerBg)
	p.Fill(geom.XYWH(0, 0, playerPlaneX, r.H), paint, 0)
	p.Fill(geom.XYWH(playerPlaneX+playerPlaneW, 0, r.W-playerPlaneX-playerPlaneW, r.H), paint, 0)
	s.Base.Paint(c, p) // the body, then the panel over it
}

// Layout puts the plane in its hole, the lesson block, the transport block and the
// column of the next lessons where the prototype does.
func (b *playerBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	s := b.p
	c.Layout(s.plane, ui.Tight(geom.Sz(playerPlaneW, playerPlaneH)))
	c.Place(s.plane, geom.Pt(playerPlaneX, 0))
	s.layoutInfo(c)
	s.layoutBottom(c)
	s.layoutNext(c)

	return cs.Constrain(cs.Max)
}

// Paint draws the content, the control that holds the focus last.
func (b *playerBody) Paint(c *ui.Context, p *ui.Painter) { tvkit.PaintRaised(c, p, &b.Base) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// skip lays a hidden part out at no size and tells whether it is hidden: shown
// again it lays out afresh.
func skip(c *ui.Context, w ui.Widget) bool {
	if !w.Node().Hidden {
		return false
	}
	c.Layout(w, ui.Tight(geom.Size{}))

	return true
}

// layoutInfo flows the lesson block: the course line, the title, the chips and the
// status lines, as CSS stacks them.
func (s *Player) layoutInfo(c *ui.Context) {
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	block := ui.Constraints{Min: geom.Sz(playerInfoW, 0), Max: geom.Sz(playerInfoW, ui.Unbounded)}
	y := playerInfoY
	es := c.Layout(s.eyebrow, block)
	c.Place(s.eyebrow, geom.Pt(playerBlockX, y))
	y += es.H + playerTitleTop
	ts := c.Layout(s.title, block)
	c.Place(s.title, geom.Pt(playerBlockX, y))
	y += ts.H + playerChipsTop
	tg := c.Layout(s.tag, free)
	c.Place(s.tag, geom.Pt(playerBlockX, y))
	c.Layout(s.kind, free)
	chipsGap := playerChipsGap
	if s.tag.Hidden { // no tag, no chip and no gap after it
		chipsGap = 0
	}
	c.Place(s.kind, geom.Pt(playerBlockX+tg.W+chipsGap, y))
	y += playerChipH + playerStatusTop
	for _, r := range [...]*statusRow{s.speed, s.mirror, s.loop} {
		if skip(c, r) {
			continue
		}
		rs := c.Layout(r, free)
		c.Place(r, geom.Pt(playerBlockX, y))
		y += rs.H + playerStatusGap
	}
}

// layoutBottom flows the transport block: the beat bar when it is on, the clock
// with the length at its right, the position bar and the two buttons.
func (s *Player) layoutBottom(c *ui.Context) {
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	y := playerBottomY
	if !skip(c, s.beats) {
		bs := c.Layout(s.beats, ui.Tight(geom.Sz(playerInfoW, tvkit.BeatsHeight)))
		c.Place(s.beats, geom.Pt(playerBlockX, y))
		y += bs.H + playerBeatsBottom
	}
	s.dur.Color = theme.From(c).T.Text.WithAlpha(playerDurOpacity) // `opacity:.6`
	ps := c.Layout(s.pos, free)
	ds := c.Layout(s.dur, free)
	c.Place(s.pos, geom.Pt(playerBlockX, y))
	c.Place(s.dur, geom.Pt(playerBlockX+playerInfoW-ds.W, y))
	y += max(ps.H, ds.H) + playerScrubTop
	c.Layout(s.scrub, ui.Tight(geom.Sz(playerInfoW, tvkit.ScrubHeight)))
	c.Place(s.scrub, geom.Pt(playerBlockX, y))
	y += tvkit.ScrubHeight + playerRowTop
	p1 := c.Layout(s.play, free)
	c.Place(s.play, geom.Pt(playerBlockX, y))
	c.Layout(s.opts, free)
	c.Place(s.opts, geom.Pt(playerBlockX+p1.W+playerRowGap, y))
}

// layoutNext places the heading of the next lessons and their rows.
func (s *Player) layoutNext(c *ui.Context) {
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	hs := c.Layout(s.nextHead, free)
	c.Place(s.nextHead, geom.Pt(playerNextX, playerNextY))
	y := playerNextY + hs.H + playerNextHeadBottom
	for _, r := range s.rows {
		rs := c.Layout(r, ui.Tight(geom.Sz(playerNextW, tvkit.NextRowHeight)))
		c.Place(r, geom.Pt(playerNextX, y))
		y += rs.H + playerRowsGap
	}
}

// Layout puts the icon at the top of the line and its text on the icon's
// baseline, a space after it.
func (r *statusRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	is := c.Layout(r.icon, ui.Tight(geom.Sz(playerStatusIcon, playerStatusIcon)))
	ls := c.Layout(r.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(r.icon, geom.Point{})
	c.Place(r.label, geom.Pt(is.W+playerStatusSpace, is.H-r.label.Baseline()))

	return cs.Constrain(geom.Sz(is.W+playerStatusSpace+ls.W, playerStatusH))
}

// onChange follows the player.
func (s *Player) onChange(ch player.Change) {
	switch ch.Kind {
	case player.ChangeLesson:
		s.syncLesson()
		s.syncState()
		s.syncLive()
	case player.ChangeState:
		s.syncState()
	case player.ChangeTime, player.ChangeSeek:
		s.syncLive()
	case player.ChangeDuration:
		s.syncLesson()
		s.syncLive()
	case player.ChangeRate:
		s.syncRate()
	case player.ChangeOptions:
		s.syncOptions()
	case player.ChangeEnded:
		s.ended()
	case player.ChangeError:
		s.failed()
	case player.ChangeUnknown:
	default:
	}
}

// failed is a lesson that did not load (its stream or its playlist answered an
// error): the player is in StateError and wants nothing, so `p1` shows "Phát" (the
// ChangeState before this change synced it) and a press asks the player again,
// which fetches the stream again. The toast is the phone's `player.err.load`: the
// prototype has no failed state on TV.
func (s *Player) failed() {
	if s.e.Act != nil {
		s.e.Act.Toast(s.e.T("player.err.load"))
	}
}

// ended is a video played to its end: the up-next screen takes this one's place
// (`$P:782`).
func (s *Player) ended() {
	if s.pb.Kind() != player.KindVideo || s.e.Act == nil {
		return
	}
	s.e.ResetUpNext() // `cdT=5`
	s.e.Act.Replace(nav.Route{Screen: env.RouteUpNext})
}

// onOnline is the network changing: going away pauses and shows TV11 over the
// player (`$P:812`).
func (s *Player) onOnline(online bool) {
	if online || s.e.Act == nil {
		return
	}
	s.pb.Pause()
	s.e.Act.Go(nav.Route{Screen: env.RouteError}, false)
}

// syncAll makes every widget show what the player holds.
func (s *Player) syncAll() {
	s.syncLesson()
	s.syncState()
	s.syncOptions()
	s.syncRate()
	s.syncLive()
}

// syncLesson sets the texts that depend on the lesson, and the next lessons.
func (s *Player) syncLesson() {
	l := s.pb.Lesson()
	n := max(l.Count, len(s.list))
	s.eyebrow.SetText(s.e.Fill("tv.player.eyebrowTpl", "course", l.CourseName, "nn", pad2(l.No), "n", strconv.Itoa(n)))
	s.title.SetText(l.Title)
	s.tag.SetText(l.Tag)
	kit.SetHidden(s.tag, l.Tag == "") // no empty pill for a lesson with no tag
	dur := s.pb.Dur()
	if dur <= 0 {
		dur = float64(l.DurationSec)
	}
	if txt := format.Clock(dur); txt != s.dur.Text {
		s.dur.SetText(txt)
		s.sec = -1 // the reading of the bar holds the length: made again
	}
	if l.ID != s.lesson {
		s.lesson = l.ID
		s.sec = -1
		s.loadList()
		s.fillNext()
	}
}

// syncState sets the first button, `Phát` or `Tạm dừng`, and the stage.
func (s *Player) syncState() {
	if s.pb.Playing() {
		s.play.Icon = icon.Pause
		s.play.SetText(s.e.T("tv.player.pause"))
	} else {
		s.play.Icon = icon.Play
		s.play.SetText(s.e.T("tv.player.play"))
	}
	s.plane.Picture()
}

// syncLive sets the clock, the bar and the lit beat; it changes a widget only
// when its value changed and allocates nothing.
func (s *Player) syncLive() {
	pos, dur := s.pb.Pos(), s.pb.Dur()
	if dur <= 0 {
		dur = float64(s.pb.Lesson().DurationSec)
	}
	share := float32(0)
	if dur > 0 {
		share = float32(pos / dur)
	}
	s.scrub.SetValue(share)
	if sec := int(pos); sec != s.sec {
		s.sec = sec
		clock := format.Clock(pos)
		s.pos.SetText(clock)
		s.scrub.SetSpoken(clock + " / " + s.dur.Text) // "01:12 / 03:40", once a second
	}
	if !s.beats.Hidden {
		s.beats.SetLit(s.pb.BeatIndex())
	}
}

// syncRate sets the speed line, shown when the speed is not 1, and the panel's
// check.
func (s *Player) syncRate() {
	r := s.pb.Rate()
	if r != s.rate || s.speed.label.Text == "" {
		s.rate = r
		s.speed.label.SetText(s.e.Fill("tv.player.speedTpl", "r", jsNumber(r)))
	}
	kit.SetHidden(s.speed, r == 1)
	if s.panel != nil {
		s.panel.syncRate(r)
	}
}

// syncOptions sets the mirror and loop lines, the beat bar and the panel's values.
func (s *Player) syncOptions() {
	pb := s.pb
	kit.SetHidden(s.mirror, !pb.Mirror())
	loopText := s.e.Fill("tv.player.loopTpl", "a", format.Clock(pb.A()), "b", format.Clock(pb.B()))
	s.loop.label.SetText(loopText)
	kit.SetHidden(s.loop, !pb.Loop())
	if kit.SetHidden(s.beats, !pb.Beat()) {
		s.body.Relayout()
	}
	if s.panel != nil {
		s.panel.syncOptions(pb)
	}
}

// jsNumber writes r as JavaScript writes a number: 0.75, 1.25, 2.
func jsNumber(r float64) string { return strconv.FormatFloat(r, 'f', -1, 64) }

// loadList takes the lessons of the course from what a screen read before; none
// yet when it was not read.
func (s *Player) loadList() {
	if c, ok := s.e.CachedCourse(s.pb.Lesson().CourseID); ok {
		s.list = env.PlayerLessons(c)
	}
}

// load reads the course of the lesson, for the lessons that come next.
func (s *Player) load() {
	cid := s.pb.Lesson().CourseID
	if cid == "" || len(s.list) > 0 {
		return
	}
	read(&s.rd, s, func(ctx context.Context) (model.Course, error) {
		return s.e.Course(ctx, cid)
	}, func(_ *ui.Context, c model.Course, err error) {
		if err != nil {
			return // the next lessons are an extra; the video plays without them
		}
		s.e.RememberCourse(c)
		s.list = env.PlayerLessons(c)
		s.syncLesson()
		s.fillNext()
	})
}

// fillNext makes the rows of the next four lessons (`L.slice(l.n,l.n+4)`); a
// focus that was on one of the old rows goes to `p1`.
func (s *Player) fillNext() {
	if len(s.list) == 0 && len(s.rows) == 0 {
		return
	}
	focused := false
	for _, r := range s.rows {
		focused = focused || r.IsFocused()
		s.body.Remove(r)
	}
	s.rows = s.rows[:0]
	l := s.pb.Lesson()
	from := env.LessonIndex(s.list, l.ID) + 1
	if from > 0 {
		for i := from; i < len(s.list) && i < from+playerNextRows; i++ {
			x := s.list[i]
			sub := s.e.Fill("tv.detail.rowTpl", "tag", x.Tag, "dur", format.Clock(float64(x.DurationSec)))
			row := tvkit.NewNextRow(keyPlayerNext+strconv.Itoa(i-from), pad2(x.No), x.Title, sub)
			row.OnSelect = func() { playLesson(s.e, x, s.list) }
			s.rows = append(s.rows, row)
			s.body.Add(row)
		}
	}
	if focused && s.ctx != nil {
		s.ctx.Focus(s.play)
	}
	s.body.Relayout()
}
