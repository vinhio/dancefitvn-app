//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/np/{nowplaying.go,vui.go} at 49de3c2
// (a screen over the player that follows its Changes while mounted, the picture
// under a layer of controls that fades, the Leaver that tells the player). The
// screen is MS.video (index.html:415-430, docs/plan/03 section 5 and 5a).

package screens

import (
	"strconv"
	"strings"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// seekStep is the 10 of `data-v="-10"` and `data-v="10"` (index.html:430).
const seekStep = 10

// Video is screen 06: the lesson's video over the K5 stage, with the controls of
// MS.video over it: the top (back, course, lesson, options), the column of side
// buttons (speed, mirror, A-B loop, audio file), and the panel (the lesson, the
// eight beats, the position bar, the transport row). The controls hide 3 s after
// the last touch while the video plays and show again on a tap (03 section 5a).
//
// It is the view of the player (env.Playback): a tap or a button asks it, and the
// screen follows its Changes while it is mounted, changing a widget only when its
// value changed. Leaving the screen tells the player (Leave: the video pauses and
// its position is saved, 03 section 1).
type Video struct {
	ui.Base

	e    *env.Env
	pb   env.Playback
	hide *autoHide
	tap  tapArea

	stage *kit.Stage
	speak *stageButton
	view  *kit.VideoView
	ctl   *vControls
	top   *vTop
	side  *vSideCol
	panel *vPanel

	sheet   *kit.SheetHandle
	quality bool // the options sheet has the quality row (K21)
	unsub   func()
	dlUnsub func() // the downloads' events, for the audio file's button
	lesson  string // the id the labels were made for
	sec     int    // the whole second the clock label shows
	speeds  map[float64]string
}

// vControls is the layer of controls over the picture, `.vui`: it holds the top,
// the side column and the panel and fades them as one.
type vControls struct {
	ui.Base
	// Hush takes the layer out of a screen reader's reading from the moment the
	// auto-hide hides it (not after the fade): its buttons take no touch then
	// (`.scr.vid.hid .vui{pointer-events:none}`), and a hidden control must not be
	// pressed through VoiceOver either (Q-P4.4-3).
	kit.Hush

	v       *Video
	c       *ui.Context
	p       *ui.Painter
	layered func()
}

var (
	_ nav.Leaver            = (*Video)(nil)
	_ ui.Mounter            = (*Video)(nil)
	_ ui.Unmounter          = (*Video)(nil)
	_ ui.PointerInterceptor = (*Video)(nil)
	_ ui.PointerHandler     = (*Video)(nil)
	_ ui.MagicTapper        = (*Video)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewVideo returns screen 06 over the player of the services. Without one (a
// Services that has none) it is the stub with a back button.
func NewVideo(e *env.Env) nav.Screen {
	pb := e.Playback()
	if pb == nil {
		return NewStub(e, env.RouteVideo, -1)
	}
	v := &Video{e: e, pb: pb, quality: qualityRowShown(e)}
	v.speeds = make(map[float64]string, len(player.SheetRates))
	for _, r := range player.SheetRates {
		v.speeds[r] = speedText(r)
	}

	v.stage = kit.NewStage()
	v.stage.Name = nameStageTap // `data-act="stageTap"` (:416)
	v.view = kit.NewVideoView(nil, kit.FitContain)
	v.view.Under = true // the stage shows until the first frame (K5)
	if s := pb.Video(); s != nil {
		v.view.SetSource(s)
	}

	v.speak = &stageButton{v: v}
	v.ctl = &vControls{v: v}
	v.ctl.Silent = func() bool { return !v.hide.Shown() }
	v.ctl.layered = v.ctl.paintChildren
	v.hide = newAutoHide(v.ctl, pb.State, func() bool { return pb.Kind() == player.KindVideo })
	// F-14: a sheet the shell opens (the download sheet of the side button) keeps
	// the controls shown too
	v.hide.ext = func() bool { return e.Nav != nil && e.Nav.Sheet().IsOpen() }

	back := newBackButton(e)
	more := kit.NewRoundButton(icon.More, true, e.T("a11y.more")) // `<button class="rb dk" data-act="sheet" data-v="options">`
	more.Name = nameSheetOpen
	more.OnPress = v.act(v.openOptions)
	v.top = newVTop(back, more)
	// `back` is a data-act of 06, so it resets the count too; the screen is
	// about to go
	back.OnPress = v.act(e.Act.Back)

	v.side = &vSideCol{
		speed:  kit.NewSideButton(icon.Speed, ""),
		mirror: kit.NewSideButton(icon.Mirror, e.T("video.side.mirror")),
		loop:   kit.NewSideButton(icon.Rep, e.T("video.side.loop")),
		dl:     kit.NewSideButton(icon.Dl, e.T("video.side.audio")),
	}
	v.side.speed.Label = e.T("options.speed") // `$P:507`: the text on the button is its value, "0.75x"
	v.side.mirror.Toggle, v.side.loop.Toggle = true, true
	v.side.speed.Name, v.side.speed.OnPress = nameSheetOpen, v.act(v.openOptions)
	v.side.mirror.Name, v.side.mirror.OnPress = nameMirror, v.act(func() { pb.SetMirror(!pb.Mirror()) })
	v.side.loop.Name, v.side.loop.OnPress = nameLoop, v.act(func() { pb.SetLoop(!pb.Loop()) })
	v.side.dl.OnPress = v.act(func() {
		l := pb.Lesson()
		e.Act.DlTap(l.CourseID, l.ID, l.No)
	})
	v.side.Add(v.side.speed, v.side.mirror, v.side.loop, v.side.dl)

	v.panel = v.newPanel()
	v.ctl.Add(v.top, v.side, v.panel)
	v.Add(v.stage, v.view, v.speak, v.ctl)

	v.syncAll()

	return v
}

// newPanel makes the bottom panel and wires its buttons.
func (v *Video) newPanel() *vPanel {
	e, pb := v.e, v.pb
	p := &vPanel{
		tag:    kit.NewChip(kit.ChipY, ""),
		kind:   kit.NewChip(kit.ChipK, e.T("options.vertical")),
		title:  heading(kit.Display("", vTitleSize), 1),
		course: &kit.Label{Size: vCourseSize, Wrap: true},
		beats:  kit.NewBeats(),
		scrub:  kit.NewScrub(e.T("a11y.position")),
		mpos:   &kit.Label{Size: vTimesSize, Weight: 700},
		dur:    &kit.Label{Size: vTimesSize, Weight: 700},
		prev:   newLesButton(namePrevLes),
		next:   newLesButton(nameNextLes),
		back10: seekIcon(nameSeekBack, icon.B10, e.T("a11y.seekBack")),
		f10:    seekIcon(nameSeekFwd, icon.F10, e.T("a11y.seekFwd")),
		toggle: kit.NewDiscButton(icon.Play, e.T("a11y.pause")),
	}
	p.title.Wrap = true
	// `$P:699`: "‹ Bài trước" and "Bài sau ›" of the same buttons on TV, without the
	// chevrons, which a screen reader would read out
	p.prev.speak = strings.Trim(e.T("tv.music.prev"), "‹› ")
	p.next.speak = strings.Trim(e.T("tv.music.next"), "‹› ")
	p.toggle.Name = nameTogglePl
	// `scrub` is the one control that does not reset the count (index.html:587,
	// :627): it is not a data-act.
	p.scrub.OnSeek = func(share float32) { pb.Seek(float64(share) * pb.Dur()) }
	p.scrub.Reading = func() (float64, float64) { return pb.Pos(), pb.Dur() }
	p.scrub.OnStep = pb.SeekBy          // a screen reader's increment and decrement: +-10 s
	p.beats.Label = e.T("options.beat") // `#mBeats`: the bar is one element
	p.prev.OnPress = v.act(func() {
		if pb.HasPrev() {
			pb.Prev()
		}
	})
	p.next.OnPress = v.act(func() {
		if pb.HasNext() {
			pb.Next()
		}
	})
	p.back10.OnPress = v.act(func() { pb.SeekBy(-seekStep) })
	p.f10.OnPress = v.act(func() { pb.SeekBy(seekStep) })
	p.toggle.OnPress = v.act(pb.Toggle)
	p.Add(p.tag, p.kind, p.title, p.course, p.beats, p.scrub, p.mpos, p.dur, p.prev, p.back10, p.toggle, p.f10, p.next)

	return p
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Left tells the player that the screen was left (nav.Leaver).
func (v *Video) Left() { v.pb.Leave() }

// Shown tells whether the controls are shown (not hidden by the auto-hide).
func (v *Video) Shown() bool { return v.hide.Shown() }

// SetQualityRow says whether the options sheet has the row of the video quality
// (K21: iOS only); NewVideo asks the build.
func (v *Video) SetQualityRow(on bool) { v.quality = on }

// PendingHide tells whether the 3 s of the auto-hide are running.
func (v *Video) PendingHide() bool { return v.hide.Pending() }

// Opacity is the opacity of the controls layer now.
func (v *Video) Opacity() float32 { return v.hide.fade.Value() }

// Mounted follows the player and starts the 3 s of the controls (`go()` sets
// `ctlHid=false` and `ctlT=Date.now()` on entering 06, index.html:545).
func (v *Video) Mounted(c *ui.Context) {
	v.hide.mount(c)
	if v.unsub == nil {
		v.unsub = v.pb.Changes().Subscribe(v.onChange)
	}
	if v.dlUnsub == nil {
		v.dlUnsub = v.e.Downloader().Events().Subscribe(v.onDownload)
	}
	v.syncAll()
	v.hide.Reset()
}

// Unmounted stops following the player and the timer.
func (v *Video) Unmounted(*ui.Context) {
	if v.unsub != nil {
		v.unsub()
		v.unsub = nil
	}
	if v.dlUnsub != nil {
		v.dlUnsub()
		v.dlUnsub = nil
	}
	v.hide.unmount()
}

// InterceptPointer takes a press while the controls are hidden: their buttons
// take no touch then (`.scr.vid.hid .vui{pointer-events:none}`), so the press is a
// tap on the picture even where a button lies under the finger.
func (v *Video) InterceptPointer(_ *ui.Context, e *ui.PointerEvent) bool {
	if e.Kind != ui.PointerDown || !isPrimary(e) || v.hide.Shown() {
		return false
	}
	v.tap.begin(e.Pos)

	return true
}

// HandlePointer is the click on the picture, `stageTap` (index.html:785).
func (v *Video) HandlePointer(_ *ui.Context, e *ui.PointerEvent) { v.tap.handle(e, v.hide.StageTap) }

// MagicTap is the two-finger double tap of a screen reader: play or pause, as the
// big button does (the controls show and the 3 s start again).
func (v *Video) MagicTap(*ui.Context) bool {
	v.hide.Reset()
	v.pb.Toggle()

	return true
}

// Layout fills the window: the stage and the picture over all of it, the
// controls over them.
func (v *Video) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	full := ui.Tight(cs.Max)
	for _, w := range [...]ui.Widget{v.stage, v.view, v.speak, v.ctl} {
		c.Layout(w, full)
		c.Place(w, geom.Point{})
	}

	return cs.Max
}

// Paint fills the page black (`.scr.vid` `background:#000`, index.html:415), then
// paints the stage, the picture and the controls.
func (v *Video) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(v.Bounds(), scene.Solid(color.RGB(0, 0, 0)), 0)
	v.Base.Paint(c, p)
}

// Layout puts the top at the top, the side column at 14 from the right and 300
// from the top, and the panel on the bottom.
func (l *vControls) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Max
	v := l.v
	loose := ui.Loose(size)
	c.Layout(v.top, loose)
	c.Place(v.top, geom.Point{})
	ps := c.Layout(v.panel, loose)
	c.Place(v.panel, geom.Pt(0, size.H-ps.H))
	// The column is 300 from the top (`.vside{top:300px}`), where the prototype's one-line
	// title leaves room: it ends above the chips, over the title's top. A long title or
	// course line makes the panel taller, and the column would lie over the title ("Âm
	// thanh" over it): it moves up to end vSideGap above the title, never above the top's
	// scrim.
	ss := c.Layout(v.side, loose)
	titleTop := size.H - ps.H + v.panel.title.Frame().Y
	y := min(vSideTop, titleTop-vSideGap-ss.H)
	c.Place(v.side, geom.Pt(size.W-vSide-ss.W, max(y, vTopH)))

	return size
}

// Paint draws the controls at the opacity of their fade: nothing once they are
// gone (they keep their boxes, which layout.json records, M06-hidden).
func (l *vControls) Paint(c *ui.Context, p *ui.Painter) {
	op := l.v.hide.Opacity()
	switch {
	case op <= 0:
	case op >= 1:
		l.Base.Paint(c, p)
	default:
		l.c, l.p = c, p
		p.Layer(op, 0, l.layered)
		l.c, l.p = nil, nil
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// paintChildren is what the layer runs.
func (l *vControls) paintChildren() { l.Base.Paint(l.c, l.p) }

// act wraps the action of a button of screen 06 other than the stage and the
// scrub: they all reset the 3 s first (`if(S.m.scr==='video'&&act!=='stageTap')
// ctlShow()`, index.html:587).
func (v *Video) act(fn func()) func() {
	return func() {
		v.hide.Reset()
		fn()
	}
}

// openOptions is `sheet` `data-v="options"`: the options sheet opens over the
// screen, and the controls stay shown while it is open.
func (v *Video) openOptions() {
	if v.e.Nav == nil {
		return
	}
	sh := v.e.Nav.Sheet()
	v.hide.Sheet(true)
	content := newOptionsSheet(v.e, v.pb, sh.Close, v.quality)
	v.sheet = sh.Open(content, kit.SheetOptions{OnClose: v.sheetClosed})
}

// sheetClosed is the sheet closing, however it closes: the 3 s start again.
func (v *Video) sheetClosed() {
	v.sheet = nil
	v.hide.Sheet(false)
}

// onChange follows the player.
func (v *Video) onChange(ch player.Change) {
	switch ch.Kind {
	case player.ChangeLesson:
		v.syncLesson()
		v.syncState()
		v.hide.Reset() // the next lesson starts with the controls shown (`$P:790`)
	case player.ChangeState:
		v.syncState()
		v.hide.State(v.pb.State())
	case player.ChangeTime, player.ChangeSeek:
		v.syncLive()
	case player.ChangeDuration:
		v.syncDuration()
	case player.ChangeRate:
		v.syncRate()
	case player.ChangeOptions:
		v.syncOptions()
	case player.ChangeUnknown, player.ChangeEnded, player.ChangeError:
	default:
	}
}

// onDownload follows the downloads: the audio file's button shows the state of the
// lesson's file as it changes ("Âm thanh", "Đang tải", "Đã tải"); a progress
// changes nothing it shows.
func (v *Video) onDownload(ev env.DownloadEvent) {
	if ev.Kind != env.DownloadEventProgress {
		v.syncDownload()
	}
}

// syncAll makes every widget show what the player holds.
func (v *Video) syncAll() {
	v.syncLesson()
	v.syncState()
	v.syncRate()
	v.syncOptions()
}

// syncLesson sets the texts of the lesson: the lines of the top, the chip, the
// title, the course line and the numbers of the prev and next buttons.
func (v *Video) syncLesson() {
	l := v.pb.Lesson()
	if l.ID != v.lesson {
		v.lesson = l.ID
		v.top.eb.SetText(l.CourseName)
		v.top.line.SetText(v.e.Fill("video.lessonOfTpl", "no", format.No(l.No), "n", strconv.Itoa(l.Count)))
		p := v.panel
		p.tag.SetText(l.Tag)
		kit.SetHidden(p.tag, l.Tag == "") // no empty pill for a lesson with no tag
		p.title.SetText(l.Title)
		course := l.CourseName
		if l.Tagline != "" {
			course = v.e.Fill("video.courseLineTpl", "course", l.CourseName, "quote", l.Tagline)
		}
		p.course.SetText(course)
		p.prev.SetText(v.e.Fill("video.prevTpl", "no", format.No(max(1, l.No-1))))
		p.next.SetText(v.e.Fill("video.nextTpl", "no", format.No(min(max(l.Count, l.No), l.No+1))))
		p.prev.value = format.No(max(1, l.No-1))
		p.next.value = format.No(min(max(l.Count, l.No), l.No+1))
		v.side.dl.Name = "dlTap:" + LessonKey(l.CourseID, l.No)
		v.sec = -1
	}
	v.syncDuration()
	v.syncLive()
	v.syncDownload()
}

// syncState sets the symbol of the play button and the audio file's button.
func (v *Video) syncState() {
	t := v.panel.toggle
	if v.pb.Playing() {
		t.SetIcon(icon.Pause)
		t.Label = v.e.T("a11y.pause")
	} else {
		t.SetIcon(icon.Play)
		t.Label = v.e.T("a11y.play")
	}
	v.syncDownload()
}

// syncDuration sets the length and the loop's band, which are shares of it.
func (v *Video) syncDuration() {
	v.panel.dur.SetText(format.Clock(v.pb.Dur()))
	v.syncBand()
}

// syncLive is the tick (liveM, index.html:630-636): the position's text, the
// fill and the knob and the lit beat, each changed only when its value did.
func (v *Video) syncLive() {
	pos, dur := v.pb.Pos(), v.pb.Dur()
	share := float32(0)
	if dur > 0 {
		share = float32(pos / dur)
	}
	p := v.panel
	p.scrub.SetPos(share)
	if s := int(pos); s != v.sec {
		v.sec = s
		p.mpos.SetText(format.Clock(pos))
		v.syncDownload()
	}
	if !p.beats.Hidden {
		p.beats.SetLit(v.pb.BeatIndex())
	}
}

// syncRate sets the speed button: `{s}x`, on when the speed is not 1.
func (v *Video) syncRate() {
	r := v.pb.Rate()
	txt, ok := v.speeds[r]
	if !ok {
		txt = speedText(r)
	}
	v.side.speed.SetText(txt)
	v.side.speed.Value = txt
	v.side.speed.SetOn(r != 1)
}

// syncOptions sets the mirror, the loop and the beats.
func (v *Video) syncOptions() {
	mirror := v.pb.Mirror()
	v.view.SetMirror(mirror)
	v.side.mirror.SetOn(mirror)
	v.side.loop.SetOn(v.pb.Loop())
	v.side.loop.Value = loopValue(v.pb.Loop(), v.pb.A(), v.pb.B())
	beat := v.pb.Beat()
	if v.panel.beats.Hidden == beat {
		kit.SetHidden(v.panel.beats, !beat)
		v.panel.Relayout()
		v.ctl.Relayout()
	}
	v.syncBand()
	if beat {
		v.panel.beats.SetLit(v.pb.BeatIndex())
	}
}

// syncBand shows the A-B band, from A/dur to B/dur, while the loop is on.
func (v *Video) syncBand() {
	dur := v.pb.Dur()
	if dur <= 0 {
		v.panel.scrub.SetBand(false, 0, 0)

		return
	}
	v.panel.scrub.SetBand(v.pb.Loop(), float32(v.pb.A()/dur), float32(v.pb.B()/dur))
}

// syncDownload sets the audio file's button: "Âm thanh" with no file, "Đang tải"
// while it comes, "Đã tải" (and on) when it is there (index.html:423).
func (v *Video) syncDownload() {
	d := v.side.dl
	l := v.pb.Lesson()
	state, _ := env.DownloadOf(v.e.Downloads(), l.ID, !l.NoAudio)
	// a lesson with no audio file to download leaves no button (it is the last of
	// the column, so nothing moves)
	if kit.SetHidden(d, state == kit.DlUnavailable) {
		v.side.Relayout()
		v.ctl.Relayout()
	}
	switch state {
	case kit.DlDone:
		d.SetText(v.e.T("video.side.downloaded"))
		d.SetOn(true)
	case kit.DlIng:
		d.SetText(v.e.T("video.side.downloading"))
		d.SetOn(false)
	case kit.DlNone, kit.LessonDownloadUnknown:
		d.SetText(v.e.T("video.side.audio"))
		d.SetOn(false)
	default:
		d.SetText(v.e.T("video.side.audio"))
		d.SetOn(false)
	}
}

// speedText is `${s}x` of the prototype: 0.75 is "0.75x", 1 is "1x".
func speedText(r float64) string { return strconv.FormatFloat(r, 'f', -1, 64) + "x" }
