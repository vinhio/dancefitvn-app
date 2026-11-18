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
	"time"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/focus"
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
	// playerNextX, playerNextY, playerNextW are the column of the lessons,
	// `left:1340px;top:100px;width:490px`.
	playerNextX, playerNextY, playerNextW float32 = 1340, 100, 490
	// playerNextHeadBottom, playerRowsGap are the heading's `margin-bottom:20px` and
	// the rows' `margin-bottom:12px`.
	playerNextHeadBottom, playerRowsGap float32 = 20, 12
	// playerListPad is the room the sliding list leaves around its rows, so that a
	// focused row (scaled 1.02, its shadow) is not cut by the clip: the 8 of
	// `#tvList{padding:8px}` (index.html:678, detailListPad). K46: the prototype
	// lists four rows that fit, so it has no clip here.
	playerListPad float32 = 8
)

// The keys of the controls: `data-fid` of TS.player. The lessons after the one
// playing are `pn0`.. as in the prototype (`L.slice(l.n,...)`); K46 adds the one
// playing, `pc`, and the ones before it, `pb0`.. in course order.
const (
	keyPlayerToggle  = "p1"
	keyPlayerOpts    = "p2"
	keyPlayerNext    = "pn"
	keyPlayerCurrent = "pc"
	keyPlayerBefore  = "pb"
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

// Player is TS.player (TV07): the lesson's video in the hole at 656,0 (the course's cover under
// a loading ring until the first frame, K47), the lesson's name, tag and kind at the left with the
// options that are on, the beat bar, the clock, the position bar and the buttons
// `p1` (Phát / Tạm dừng) and `p2` (Tuỳ chọn) under them, and at the right all the
// lessons of the course in order (K46), the one playing marked and in view, the
// ones after it `pn0`.. . `p1` has the focus first.
//
// Everything but the video is the chrome: 3 s after the last button while the
// video plays it fades away, and a button brings it back (K46, player_chrome.go).
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
	slide    *tvkit.Slide // the clip the lessons slide in (K46)
	list     *playerList  // the content that slides: the rows
	rows     []*tvkit.NextRow

	body   *playerBody   // everything but the panel, left out of a reading while the panel is up
	chrome *playerChrome // everything but the video: it fades
	panel  *playerPanel

	lessons []player.Lesson // the lessons of the course, as the player walks them
	lesson  string          // the id the labels were made for
	rate    float64         // the speed the status line was made for
	sec     int             // the whole second the clock shows

	scrubbing bool          // the arrows are moving the position (K43): the bar and the clock show target
	target    float64       // where the scrub is, in seconds
	last      time.Duration // when the last arrow came, on the view's clock
	timer     ui.Timer      // the wait for the seek that follows the last arrow
	commit    func()        // scrubDue as a function value, made once

	unsub, stopOnline, removePlay func()
	rd                            reader // the read of the course
	ctx                           *ui.Context

	hid       bool // the chrome is hidden, or fading out (K46)
	fade      ui.Animated[float32]
	touched   time.Duration // when the last button came, on the view's clock
	idle      ui.Timer      // the wait for the chrome to hide
	idleFn    func()        // chromeDue as a function value, made once
	removeKey func()
}

// playerBody is the content of the screen under the panel: the plane and the
// chrome. While the panel is open it is `inert` (the panel is a modal over it,
// TV08): a screen reader reads the panel and nothing under it. Adapted from
// jplayer-tv/internal/ui/kit/hush.go at 781bee5 (Hush on the shell while a dialog
// is open).
type playerBody struct {
	ui.Base
	kit.Hush

	p *Player
}

// playerChrome is everything of the screen but the video: the lesson block, the
// transport and the lessons. It paints under the fade of the auto-hide (K46) and,
// while hidden, is left out of a reading, as `.scr.vid.hid .vui` is inert on the
// phone (`pointer-events:none`, 03 section 5a).
type playerChrome struct {
	ui.Base
	kit.Hush

	p *Player

	// dimmed, c and pt paint the children under a layer while the chrome fades:
	// made once, so that a frame allocates nothing.
	dimmed func()
	c      *ui.Context
	pt     *ui.Painter
}

// playerList is the content of the sliding clip: the rows of the lessons, one
// over the other, in the padding.
type playerList struct {
	ui.Base
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
	s.scrub.Name, s.scrub.Key, s.scrub.Label = namePlayerScrub, keyPlayerScrub, e.T("a11y.position")
	s.scrub.OnMove, s.scrub.OnSelect = s.scrubMove, s.scrubSelect // K43: the remote scrubs
	s.scrub.OnStep, s.scrub.OnBlur = s.scrubStep, s.commitScrub

	s.play = tvkit.NewButton(keyPlayerToggle, e.T("tv.player.play"), icon.Play)
	s.play.Size, s.play.IconSize, s.play.PlainIcon = tvkit.ButtonSmall, playerBtnPlay, true
	s.play.IsDefault = true                  // the bar above it takes the focus too (K43), but `p1` has it first
	s.play.OnSelect = func() { pb.Toggle() } // `tplay`
	s.opts = tvkit.NewButton(keyPlayerOpts, e.T("tv.player.options"), icon.Speed)
	s.opts.Size, s.opts.IconSize = tvkit.ButtonSmall, playerBtnOpts
	s.opts.OnSelect = s.openPanel // `topts`

	s.nextHead = &kit.Label{
		// K46: "BÀI HỌC" (tv.detail.lessons, the heading of the same list on TV06)
		// where the prototype's list of what follows says "TIẾP THEO".
		Text: e.T("tv.detail.lessons"), Size: playerEyebrowSize, Weight: 800, Spacing: playerEyebrowSpacing, Upper: true, Ink: theme.InkText2,
	}
	s.nextHead.Name = namePlayerNext
	s.nextHead.Heading = 2 // the title of the list that follows
	s.list = &playerList{}
	s.slide = tvkit.NewSlide(nil, s.list)

	s.fade.Duration, s.fade.Ease = playerChromeFade, theme.CSSEase
	s.fade.Set(1)
	s.idleFn = s.chromeDue
	s.body = &playerBody{p: s}
	s.body.Silent = func() bool { return s.panel != nil } // `inert` under the panel
	s.chrome = &playerChrome{p: s}
	s.chrome.Silent = func() bool { return s.hid } // `.scr.vid.hid .vui{pointer-events:none}`
	s.chrome.dimmed = func() { tvkit.PaintRaised(s.chrome.c, s.chrome.pt, &s.chrome.Base) }
	s.Add(s.body)
	s.body.Add(s.plane, s.chrome)
	s.chrome.Add(s.eyebrow, s.title, s.tag, s.kind, s.speed, s.mirror, s.loop,
		s.beats, s.pos, s.dur, s.scrub, s.play, s.opts, s.nextHead, s.slide)
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
	case keyPlayerScrub:
		return &s.scrub.Item
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

// Rows returns the rows of the lessons, in course order.
func (s *Player) Rows() []*tvkit.NextRow { return s.rows }

// Slide returns the clip the lessons slide in.
func (s *Player) Slide() *tvkit.Slide { return s.slide }

// Scrub returns the position bar.
func (s *Player) Scrub() *tvkit.Scrub { return s.scrub }

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
	if s.removeKey == nil && s.e.Act != nil {
		s.removeKey = s.e.Act.OnKey(s.chromeKey)
	}
	s.syncAll()
	s.touch() // the 3 s begin when the screen does
	if !s.rd.Busy() && len(s.lessons) == 0 {
		c.AfterFunc(0, s.load)
	}
}

// Unmounted stops following, gives the key back and tells the player the screen
// was left, but for a lesson taking this screen's place.
func (s *Player) Unmounted(*ui.Context) {
	for _, f := range [...]*func(){&s.unsub, &s.removePlay, &s.stopOnline, &s.removeKey} {
		if *f != nil {
			(*f)()
			*f = nil
		}
	}
	s.rd.Stop()
	s.dropScrub()
	s.idle.Stop()
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
	size := cs.Constrain(cs.Max)
	c.Layout(s.plane, ui.Tight(geom.Sz(playerPlaneW, playerPlaneH)))
	c.Place(s.plane, geom.Pt(playerPlaneX, 0))
	c.Layout(s.chrome, ui.Tight(size))
	c.Place(s.chrome, geom.Point{})

	return size
}

// Layout puts the lesson block, the transport block and the column of the lessons
// where the prototype does, in the chrome's own coordinates (it covers the screen).
func (h *playerChrome) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	s := h.p
	s.layoutInfo(c)
	s.layoutBottom(c)
	size := cs.Constrain(cs.Max)
	s.layoutNext(c, size)

	return size
}

// Paint draws the content, the control that holds the focus last, at the opacity
// of the auto-hide: nothing at 0, and under a layer while it fades.
func (h *playerChrome) Paint(c *ui.Context, p *ui.Painter) {
	switch op := h.p.fade.Value(); {
	case op >= 1:
		tvkit.PaintRaised(c, p, &h.Base)
	case op > 0:
		h.c, h.pt = c, p
		p.Layer(op, 0, h.dimmed)
		h.c, h.pt = nil, nil
	default:
	}
}

// Layout puts the rows in a column, 122 apart (110 and the 12 between them), in
// the padding of the clip.
func (l *playerList) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := max(cs.Max.W-2*playerListPad, 0)
	y := playerListPad
	for r := range l.Children() {
		c.Layout(r, ui.Tight(geom.Sz(w, tvkit.NextRowHeight)))
		c.Place(r, geom.Pt(playerListPad, y))
		y += tvkit.NextRowHeight + playerRowsGap
	}

	return cs.Constrain(geom.Sz(cs.Max.W, y-playerRowsGap+playerListPad))
}

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

// layoutNext places the heading of the lessons and the clip their rows slide in.
func (s *Player) layoutNext(c *ui.Context, size geom.Size) {
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	hs := c.Layout(s.nextHead, free)
	c.Place(s.nextHead, geom.Pt(playerNextX, playerNextY))
	// K46: the rows slide in a clip from the first row's place down to the bottom
	// edge; the padding keeps the first row where the prototype puts it.
	y := playerNextY + hs.H + playerNextHeadBottom - playerListPad
	c.Layout(s.slide, ui.Tight(geom.Sz(playerNextW+2*playerListPad, max(size.H-y, 0))))
	c.Place(s.slide, geom.Pt(playerNextX-playerListPad, y))
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
		s.stateChrome()
	case player.ChangeState:
		s.syncState()
		s.stateChrome()
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
	s.dropScrub()
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
	n := max(l.Count, len(s.lessons))
	s.eyebrow.SetText(s.e.Fill("tv.player.eyebrowTpl", "course", l.CourseName, "nn", pad2(l.No), "n", strconv.Itoa(n)))
	s.title.SetText(l.Title)
	s.plane.SetCover(s.e.Cover, l.CoverURL) // K47: the course's cover under the loading ring
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
		s.dropScrub() // a scrub of the lesson before is not this one's
		s.sec = -1
		s.loadList()
		s.fillNext()
	}
}

// syncState sets the first button, `Phát` or `Tạm dừng`, and the plane's cover.
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
	pos, dur := s.pb.Pos(), s.length()
	if s.scrubbing {
		pos = s.target // the clock and the bar show where the arrows have taken it
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
		s.chrome.Relayout()
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
		s.lessons = env.PlayerLessons(c)
	}
}

// load reads the course of the lesson, for the lessons that come next.
func (s *Player) load() {
	cid := s.pb.Lesson().CourseID
	if cid == "" || len(s.lessons) > 0 {
		return
	}
	read(&s.rd, s, func(ctx context.Context) (model.Course, error) {
		return s.e.Course(ctx, cid)
	}, func(_ *ui.Context, c model.Course, err error) {
		if err != nil {
			return // the next lessons are an extra; the video plays without them
		}
		s.e.RememberCourse(c)
		s.lessons = env.PlayerLessons(c)
		s.syncLesson()
		s.fillNext()
	})
}

// rowMove is Left on a next-lesson row: it goes to `p2`, the button the prototype's
// geometry reaches, not to the position bar (K43), whose Left would seek.
func (s *Player) rowMove(d focus.Dir) bool {
	if d != focus.DirLeft || s.ctx == nil {
		return false
	}
	s.ctx.Focus(s.opts)

	return true
}

// fillNext makes the rows of all the lessons of the course, in course order (K46;
// the prototype lists `L.slice(l.n,l.n+4)`, the four that follow): the lessons
// after the one playing keep the prototype's keys `pn0`.., the one playing is `pc`
// and marked, the ones before it `pb0`..; the clip is put so that the one playing
// is in view. A focus that was on one of the old rows goes to `p1`.
func (s *Player) fillNext() {
	if len(s.lessons) == 0 && len(s.rows) == 0 {
		return
	}
	focused := false
	for _, r := range s.rows {
		focused = focused || r.IsFocused()
		s.list.Remove(r)
	}
	s.rows = s.rows[:0]
	l := s.pb.Lesson()
	cur := env.LessonIndex(s.lessons, l.ID)
	for i := range s.lessons {
		if cur < 0 {
			break // a lesson the course lacks: no list
		}
		x := s.lessons[i]
		key := keyPlayerCurrent
		switch {
		case i < cur:
			key = keyPlayerBefore + strconv.Itoa(i)
		case i > cur:
			key = keyPlayerNext + strconv.Itoa(i-cur-1)
		default:
		}
		sub := s.e.Fill("tv.detail.rowTpl", "tag", x.Tag, "dur", format.Clock(float64(x.DurationSec)))
		row := tvkit.NewNextRow(key, pad2(x.No), x.Title, sub)
		if i == cur {
			row.SetCurrent(true)
			row.Detail = sub + ", " + s.e.T("lesson.watching") // read after the title: "đang xem"
		}
		row.OnSelect = func() { playLesson(s.e, x, s.lessons) }
		row.OnMove = s.rowMove
		s.rows = append(s.rows, row)
		s.list.Add(row)
	}
	if cur >= 0 {
		s.slide.Reveal(playerListPad+float32(cur)*(tvkit.NextRowHeight+playerRowsGap), tvkit.NextRowHeight)
	}
	if focused && s.ctx != nil {
		s.ctx.Focus(s.play)
	}
	s.chrome.Relayout()
}
