//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/np/{nowplaying.go,controls.go,meta.go}
// at 49de3c2 (a screen over the player that follows its Changes while it is
// mounted and changes a widget only when its value changed, the transport row and
// the row of buttons under it, the title and times lines). The screen is MS.audio
// (index.html:432-440, docs/plan/03 section 3 and 5).

package screens

import (
	"strconv"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Audio is screen 08, "Nghe": the sound file of the lesson. The top row (back,
// "Đang phát từ" over the course, "more"), the cover, the title, the line "Bài NN
// · File âm thanh của bài học", the chips (the tag and whether the file is on the
// device), the waveform with the times under it, the transport row (loop, ±10 s,
// play, speed) and the row of the timer, the 8 beats and the list.
//
// It is the view of the player (env.Playback): a tap asks it, and the screen
// follows its Changes while it is mounted. Unlike 06 it never tells the player
// Leave: a sound goes on playing when the screen is left (03 section 1), in the
// background and with the screen locked (04 section 4).
type Audio struct {
	ui.Base

	e  *env.Env
	pb env.Playback

	bg      radialBG
	top     *audioTop
	cover   *poster.Poster
	title   *kit.Label
	sub     *kit.Label
	chips   *chipsRow
	wave    *kit.Wave
	times   *timesRow
	ctl     *transport
	acts    *actions
	unsub   func()
	dlUnsub func() // the downloads' events, for the chip of the file
	lesson  string // the id the labels were made for
	sec     int    // the whole second the position label shows
	speeds  map[float64]string
}

var (
	_ ui.Mounter     = (*Audio)(nil)
	_ ui.Unmounter   = (*Audio)(nil)
	_ nav.Tabbed     = (*Audio)(nil)
	_ ui.MagicTapper = (*Audio)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewAudio returns screen 08 over the player of the services. Without one (a
// Services that has none) it is the stub with a back button.
func NewAudio(e *env.Env) nav.Screen {
	pb := e.Playback()
	if pb == nil {
		return NewStub(e, env.RouteAudio, -1)
	}
	a := &Audio{e: e, pb: pb}
	a.speeds = make(map[float64]string, len(player.SheetRates)+len(player.CycleRates))
	for _, r := range player.SheetRates {
		a.speeds[r] = speedText(r)
	}
	for _, r := range player.CycleRates {
		a.speeds[r] = speedText(r)
	}
	a.bg.layers = []kit.Radial{{
		// radial-gradient(circle at 50% 20%, rgba(255,208,40,.28), transparent 55%) (:432)
		Shape: kit.RadialCircle, CX: .5, CY: aBgAt, Color: color.RGBA(255, 208, 40, aBgAlpha), End: aBgEnd,
	}}

	back := newLightBack(e)
	more := kit.NewRoundButton(icon.More, false, e.T("a11y.more")) // `<button class="rb" data-act="toast">` (:433)
	more.Name = nameAudioMore
	more.OnPress = func() { e.Act.Toast(e.T("toast.audioMore")) }
	a.top = newAudioTop(back, more)
	a.cover = audioPoster()

	a.title = heading(kit.Display("", aTitleSize), 1)
	a.title.Wrap = true
	a.sub = kit.Muted("", aSubSize)
	a.sub.Name = nameMuted
	a.chips = newChipsRow(e.T("audio.downloadedTpl"), e.T("audio.streaming"))

	a.wave = kit.NewWave(e.T("a11y.position"))
	a.wave.OnSeek = func(share float32) { pb.Seek(float64(share) * pb.Dur()) } // `data-scrub="m"`
	a.wave.Reading = func() (float64, float64) { return pb.Pos(), pb.Dur() }
	a.wave.OnStep = pb.SeekBy // a screen reader's increment and decrement: +-10 s
	a.times = newTimesRow()
	a.ctl = a.newTransport()
	a.acts = a.newActions()
	a.Add(&a.bg, a.cover, a.top, a.title, a.sub, a.chips, a.wave, a.times, a.ctl, a.acts)
	a.syncAll()

	return a
}

// newTransport makes the row of the controls and wires its buttons.
func (a *Audio) newTransport() *transport {
	e, pb := a.e, a.pb
	t := &transport{
		loop:   kit.NewIconButton(icon.Rep, e.T("video.side.loop")),
		back10: seekBtn(nameSeekBack, icon.B10, e.T("a11y.seekBack")),
		f10:    seekBtn(nameSeekFwd, icon.F10, e.T("a11y.seekFwd")),
		toggle: kit.NewDiscButton(icon.Play, e.T("a11y.play")),
		speed:  newLesButton(nameCycleSpeed),
	}
	t.loop.Name, t.loop.Size = nameLoop, aLoopIcon
	t.loop.State = platform.ToggledOff
	t.loop.OnPress = func() { pb.SetLoop(!pb.Loop()) } // jumps to A, and does not loop (03 section 3)
	t.back10.OnPress = func() { pb.SeekBy(-seekStep) }
	t.f10.OnPress = func() { pb.SeekBy(seekStep) }
	t.toggle.Name, t.toggle.Size, t.toggle.Padding = nameTogglePl, aPlayD, aPlayPad
	t.toggle.OnPress = pb.Toggle
	t.speed.label.Size, t.speed.label.Weight = aSpeedSize, 800
	t.speed.speak = e.T("options.speed") // `$P:507`: the text is the value, "0.75x"
	t.speed.OnPress = pb.CycleRate
	t.Add(t.loop, t.back10, t.toggle, t.f10, t.speed)

	return t
}

// newActions makes the last row: the timer and the 8 beats, which say what they
// would do in a toast, and the list, which is Back.
func (a *Audio) newActions() *actions {
	e, pb := a.e, a.pb
	r := &actions{}
	r.btns[0] = newActButton(nameAudioMore, icon.Timer, e.T("audio.timer")) // `toast` with the message of :440
	r.btns[0].OnPress = func() { e.Act.Toast(e.T("toast.timer")) }
	r.btns[1] = newActButton(nameBeat, icon.Music, e.T("audio.beat"))
	r.btns[1].toggle = true
	r.btns[1].OnPress = func() { pb.SetBeat(!pb.Beat()) }
	r.btns[2] = newActButton(nameBackRow, icon.List, "")
	r.btns[2].OnPress = e.Act.Back
	r.Add(r.btns[0], r.btns[1], r.btns[2])

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// MagicTap is the two-finger double tap of a screen reader: play or pause.
func (a *Audio) MagicTap(*ui.Context) bool {
	a.pb.Toggle()

	return true
}

// Tab says the screen has no tab bar.
func (a *Audio) Tab() int { return -1 }

// WaveLit returns how many bars of the waveform are lit (the heard ones).
func (a *Audio) WaveLit() int { return a.wave.Lit() }

// Mounted follows the player and reads what it holds.
func (a *Audio) Mounted(*ui.Context) {
	if a.unsub == nil {
		a.unsub = a.pb.Changes().Subscribe(a.onChange)
	}
	if a.dlUnsub == nil {
		a.dlUnsub = a.e.Downloader().Events().Subscribe(a.onDownload)
	}
	a.syncAll()
}

// Unmounted stops following the player. The sound plays on.
func (a *Audio) Unmounted(*ui.Context) {
	if a.unsub != nil {
		a.unsub()
		a.unsub = nil
	}
	if a.dlUnsub != nil {
		a.dlUnsub()
		a.dlUnsub = nil
	}
}

// onDownload follows the downloads: the chip shows "Đã tải · size" when the file of
// the lesson is on the device and "Streaming" when it is not, as it changes (its
// size is the file's, which a file that just finished may have made differ from
// the catalog's); a progress changes nothing it shows.
func (a *Audio) onDownload(ev env.DownloadEvent) {
	if ev.Kind != env.DownloadEventProgress {
		a.chips.okc.SetText(a.downloadedText(a.pb.Lesson()))
		a.syncDownload()
	}
}

// Layout fills the window; the parts stand at the y of the prototype from the top
// (the content top, 54, and the 4 of the row), the cover in the middle, the block
// of the lesson 20 from the sides.
func (a *Audio) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Max
	c.Layout(&a.bg, ui.Tight(size))
	c.Place(&a.bg, geom.Point{})
	dy := nav.ContentTop(c) - nav.StatusBar

	c.Layout(a.top, ui.Loose(size))
	c.Place(a.top, geom.Pt(0, aRowTop+dy))
	cs2 := c.Layout(a.cover, ui.Loose(size))
	c.Place(a.cover, geom.Pt((size.W-cs2.W)/2, aPosterTop+dy))

	inner := max(size.W-2*aSide, 0)
	tight := ui.Constraints{Min: geom.Sz(inner, 0), Max: geom.Sz(inner, ui.Unbounded)}
	loose := ui.Loose(geom.Sz(inner, ui.Unbounded))
	y := aBlockTop + dy
	place := func(w ui.Widget, cons ui.Constraints, gapAfter float32) {
		s := c.Layout(w, cons)
		c.Place(w, geom.Pt(aSide, y))
		y += s.H + gapAfter
	}
	place(a.title, tight, aSubTop)
	place(a.sub, tight, aSubBottom)
	place(a.chips, loose, aWaveTop)
	place(a.wave, tight, aTimesTop)
	place(a.times, tight, aCtlTop)
	place(a.ctl, tight, aActsTop)
	place(a.acts, tight, 0)

	return size
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onChange follows the player.
func (a *Audio) onChange(ch player.Change) {
	switch ch.Kind {
	case player.ChangeLesson:
		a.syncLesson()
		a.syncState()
	case player.ChangeState:
		a.syncState()
	case player.ChangeTime, player.ChangeSeek:
		a.syncLive()
	case player.ChangeDuration:
		a.syncDuration()
	case player.ChangeRate:
		a.syncRate()
	case player.ChangeOptions:
		a.syncOptions()
	case player.ChangeUnknown, player.ChangeEnded, player.ChangeError:
	default:
	}
}

// syncAll makes every widget show what the player holds.
func (a *Audio) syncAll() {
	a.syncLesson()
	a.syncState()
	a.syncRate()
	a.syncOptions()
}

// syncLesson sets the texts of the lesson: the course, the cover, the title, the
// line under it, the tag, the file's chip and the count of the list button.
func (a *Audio) syncLesson() {
	l := a.pb.Lesson()
	if l.ID != a.lesson {
		a.lesson = l.ID
		a.top.eb.SetText(a.e.T("audio.from"))
		a.top.name.SetText(l.CourseName)
		a.cover.SetSpec(poster.Spec{ID: l.CourseID, Title: l.CourseShort, Radius: aPosterRadius, TitleSize: aPosterTitle})
		a.cover.SetCover(a.e.Cover, l.CoverURL) // a lesson has no picture: its course's cover (K14)
		a.title.SetText(l.Title)
		a.sub.SetText(a.e.Fill("audio.subTpl", "no", format.No(l.No)))
		a.chips.tag.SetText(l.Tag)
		kit.SetHidden(a.chips.tag, l.Tag == "") // no empty pill for a lesson with no tag
		a.acts.btns[2].SetText(a.e.Fill("audio.lessonsTpl", "n", strconv.Itoa(l.Count)))
		a.chips.okc.SetText(a.downloadedText(l))
		a.sec = -1
	}
	a.syncDuration()
	a.syncLive()
	a.syncDownload()
}

// downloadedText is "Đã tải · 8,8 MB" with the size of the file as it is on the
// device (Q-P5-5: the entry of Items), else the size the API gives the lesson's
// audio file, from the course a screen read; "Đã tải" alone when neither is
// known. It reads the list: call it when the lesson or the downloads change, not
// at each tick.
func (a *Audio) downloadedText(l player.Lesson) string {
	for _, it := range a.e.Downloads().Items() {
		if it.LessonID == l.ID && it.Bytes > 0 {
			return a.e.Fill("audio.downloadedTpl", "size", format.Size(it.Bytes))
		}
	}
	if c, ok := a.e.CachedCourse(l.CourseID); ok {
		for i := range c.Lessons {
			if c.Lessons[i].ID == l.ID && c.Lessons[i].AudioSizeBytes > 0 {
				return a.e.Fill("audio.downloadedTpl", "size", format.Size(c.Lessons[i].AudioSizeBytes))
			}
		}
	}

	return a.e.T("video.side.downloaded")
}

// syncState sets the symbol of the play button.
func (a *Audio) syncState() {
	t := a.ctl.toggle
	if a.pb.Playing() {
		t.SetIcon(icon.Pause)
		t.Label = a.e.T("a11y.pause")
	} else {
		t.SetIcon(icon.Play)
		t.Label = a.e.T("a11y.play")
	}
	a.syncDownload()
}

// syncDuration sets the length.
func (a *Audio) syncDuration() {
	a.times.dur.SetText(format.Clock(a.pb.Dur()))
	a.syncLive()
}

// syncLive is the tick (liveM, index.html:630-636): the lit bars and the
// position's text, each changed only when its value did.
func (a *Audio) syncLive() {
	pos, dur := a.pb.Pos(), a.pb.Dur()
	share := float32(0)
	if dur > 0 {
		share = float32(pos / dur)
	}
	a.wave.SetPos(share)
	if s := int(pos); s != a.sec {
		a.sec = s
		a.times.pos.SetText(format.Clock(pos))
		a.syncDownload()
	}
}

// syncRate sets the speed button: `{s}x`.
func (a *Audio) syncRate() {
	r := a.pb.Rate()
	txt, ok := a.speeds[r]
	if !ok {
		txt = speedText(r)
	}
	a.ctl.speed.SetText(txt)
}

// syncOptions sets the loop's color and the beat button (`--pr` when on).
func (a *Audio) syncOptions() {
	ink := theme.InkText2
	if a.pb.Loop() {
		ink = theme.InkPrimary
	}
	if l := a.ctl.loop; l.Ink != ink {
		l.Ink = ink
		l.Relayout()
	}
	a.ctl.loop.State, a.ctl.loop.Value = platform.ToggledOff, loopValue(a.pb.Loop(), a.pb.A(), a.pb.B())
	if a.pb.Loop() {
		a.ctl.loop.State = platform.ToggledOn
	}
	a.acts.btns[1].SetOn(a.pb.Beat())
	a.syncLive() // turning the loop on seeks to A: the player may say so after
}

// syncDownload shows the chip of the file: on the device or streamed.
func (a *Audio) syncDownload() {
	state, _ := a.e.Downloads().State(a.pb.Lesson().ID)
	on := state == kit.DlDone
	if on != a.chips.showOkc {
		a.chips.show(on)
	}
}

// newLightBack is `backBtn()` without the dark class (index.html:325): a --sf
// disc, as 08 and 10 have it.
func newLightBack(e *env.Env) *kit.RoundButton {
	b := kit.NewRoundButton(icon.Cl, false, e.T("a11y.back"))
	b.Name = nameBack
	b.OnPress = e.Act.Back

	return b
}

// seekBtn returns the 32 DIP icon button of a seek.
func seekBtn(name string, glyph icon.Name, label string) *kit.IconButton {
	b := kit.NewIconButton(glyph, label)
	b.Name, b.Size, b.Ink = name, aSeekIcon, theme.InkText

	return b
}
