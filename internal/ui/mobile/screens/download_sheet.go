//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/sheets/entry.go at 49de3c2 (content
// that holds the widgets it follows the app with, a column of items with the
// room before each, the topic it listens to while it is open) and from the
// kitdemo's M09 page (pages_sheets.go, downloadSheet), which lays this content
// out at the boxes of M09.layout.json. The sheet is SH.download (index.html:
// 511-516).

package screens

import (
	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of SH.download (index.html:511-516), in DIPs.
const (
	// dlInfoTop is `<p class="muted" style="font-size:12px;margin:4px 0 16px">`
	// (:515): the 4 over the info line; dlConfirmTop the 16 under it.
	dlInfoTop, dlConfirmTop float32 = 4, 16
	// dlInfoSize, dlInfoIcon, dlInfoGap are that line's `font-size:12px`, its
	// `ic('info',14)` and the space after the icon (Inter's, at 12 px, 4 DIPs).
	dlInfoSize, dlInfoIcon, dlInfoGap float32 = 12, 14, 4
	// dlInfoLine is the line's height: the 12 px text in the page's 16 px
	// strut is 17 tall (M09: `p.muted` 353 x 17).
	dlInfoLine float32 = 17
)

// DownloadSheet is the content of SH.download (sheet 09, index.html:511-516):
// the header, the two radios "Chỉ bài này" and "Cả khoá {short}" with the size of
// each, the row "Chỉ tải qua Wi‑Fi" with its switch, the line of the free space
// and the button "Tải xuống · {size}". It follows the downloads while it is open:
// a file that finished or a download that was started moves the counts, the free
// space read again moves its line.
//
// The radios and the row act at once (`dlChoice`, `pref:wifi`); the button is
// `dlConfirm`, which the Shell turns into the download (Actions.StartDownloads).
type DownloadSheet struct {
	ui.Base

	// Params is what the sheet was opened on.
	Params env.DownloadSheetParams

	e   *env.Env
	dm  env.DownloadManager
	col *sheetColumn

	one     *kit.Radio
	all     *kit.Radio
	wifi    *kit.Row
	info    *infoLine
	confirm *kit.Button

	choice  env.DownloadChoice
	lesson  model.Lesson // the lesson of "Chỉ bài này"
	missing []model.Lesson
	bytes   int64 // of missing, by the catalog's sizes
	audio   int   // lessons of the course that have an audio
	unsub   func()

	// the texts the widgets were last set to, so that a sync that changes
	// nothing does nothing
	oneSub, oneSize, allSub, allSize, freeText string
}

var (
	_ ui.Mounter   = (*DownloadSheet)(nil)
	_ ui.Unmounter = (*DownloadSheet)(nil)
)

// sheetItem is a part of a sheet's column and the room before it.
type sheetItem struct {
	w      ui.Widget
	before float32
	// center puts a widget narrower than the room in the middle (`margin:0 auto`).
	center bool
}

// sheetColumn stacks the parts of a sheet's content, each as wide as the room, the
// room before each as the prototype's margins give it: the way the block flow puts
// the content of a sheet. A hidden part takes no room, and the room before it goes
// to the part after it.
type sheetColumn struct {
	ui.Base

	items []sheetItem
}

// infoLine is `<p class="muted" style="font-size:12px">${ic('info',14)} text</p>`
// (:515): a 14 icon, the space after it, and the muted text.
type infoLine struct {
	ui.Base

	icon *kit.Icon
	text *kit.Label
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewDownloadSheet returns the content of sheet 09 for p; closeFn closes the
// sheet (the header's cross).
func NewDownloadSheet(e *env.Env, p env.DownloadSheetParams, closeFn func()) *DownloadSheet {
	s := &DownloadSheet{Params: p, e: e, dm: e.Downloader(), choice: p.Choice}
	if s.choice != env.DownloadChoiceAll {
		s.choice = env.DownloadChoiceOne
	}
	s.lesson = lessonOf(p.Course, p.LessonID)
	s.audio = audioLessons(p.Course)

	s.one = kit.NewRadio(e.T("download.one"), "", "")
	s.one.Name, s.one.OnPress = "dlChoice:one", func() { s.setChoice(env.DownloadChoiceOne) }
	s.all = kit.NewRadio(e.Fill("download.allTpl", "short", ShortName(p.Course)), "", "")
	s.all.Name, s.all.OnPress = "dlChoice:all", func() { s.setChoice(env.DownloadChoiceAll) }
	s.wifi = kit.NewRow(icon.Globe, e.T("account.wifiOnly"), "")
	s.wifi.Name, s.wifi.Trailing, s.wifi.Last = "pref:wifi", kit.NewSwitch(e.WiFiOnly()), true // `data-act="pref" data-v="wifi"`
	s.wifi.OnPress = s.toggleWiFi
	s.info = newInfoLine()
	s.confirm = kit.NewButton(kit.ButtonP, "", icon.Dl)
	s.confirm.Name, s.confirm.OnPress = "dlConfirm", s.confirmPress

	s.col = &sheetColumn{items: []sheetItem{
		{w: kit.NewSheetHeader(e.T("download.title"), e.T("a11y.close"), closeFn)},
		{w: s.one, before: kit.SheetHeaderGap},
		{w: s.all, before: kit.RadioGap},
		{w: s.wifi, before: kit.RadioGap},
		{w: s.info, before: dlInfoTop},
		{w: s.confirm, before: dlConfirmTop},
	}}
	for _, it := range s.col.items {
		s.col.Add(it.w)
	}
	s.Add(s.col)
	s.recount()
	s.sync()

	return s
}

// newInfoLine returns the info line with no text.
func newInfoLine() *infoLine {
	l := &infoLine{icon: kit.NewIcon(icon.Info, dlInfoIcon, theme.InkText2), text: kit.Muted("", dlInfoSize)}
	l.Name = "p.muted"
	l.Add(l.icon, l.text)

	return l
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Choice returns the radio that is on.
func (s *DownloadSheet) Choice() env.DownloadChoice { return s.choice }

// Missing returns how many lessons "Cả khoá" would download and their size by the
// catalog.
func (s *DownloadSheet) Missing() (n int, bytes int64) { return len(s.missing), s.bytes }

// Mounted follows the downloads while the sheet is open, and asks for the free
// space to be read again.
func (s *DownloadSheet) Mounted(*ui.Context) {
	if s.unsub == nil {
		s.unsub = s.dm.Events().Subscribe(s.onEvent)
	}
	s.recount()
	s.sync()
	s.dm.RefreshSpace()
}

// Unmounted stops following them.
func (s *DownloadSheet) Unmounted(*ui.Context) {
	if s.unsub != nil {
		s.unsub()
		s.unsub = nil
	}
}

// Layout puts the column at the sheet's width.
func (s *DownloadSheet) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = cs.Min.W
	}
	sz := c.Layout(s.col, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(s.col, geom.Point{})

	return cs.Constrain(geom.Sz(w, sz.H))
}

// Layout stacks the parts across the room.
func (s *sheetColumn) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	y, carry := float32(0), float32(0)
	for _, it := range s.items {
		if it.w.Node().Hidden {
			carry += it.before
			c.Layout(it.w, ui.Tight(geom.Size{})) // no room, and no layout kept: shown again, it lays out afresh

			continue
		}
		y += it.before + carry
		carry = 0
		part := ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}
		if it.center {
			part.Min = geom.Size{}
		}
		sz := c.Layout(it.w, part)
		x := float32(0)
		if it.center {
			x = (w - sz.W) / 2
		}
		c.Place(it.w, geom.Pt(x, y))
		y += sz.H
	}

	return cs.Constrain(geom.Sz(w, y))
}

// SetText changes the line's text; the same text changes nothing.
func (l *infoLine) SetText(s string) { l.text.SetText(s) }

// Layout puts the icon, then the text on the icon's row: 14 and the space of a
// 12 px Inter after it, the line 17 tall.
func (l *infoLine) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	is := c.Layout(l.icon, ui.Loose(geom.Sz(dlInfoIcon, dlInfoIcon)))
	c.Place(l.icon, geom.Point{})
	ts := c.Layout(l.text, ui.Loose(geom.Sz(max(w-is.W-dlInfoGap, 0), ui.Unbounded)))
	c.Place(l.text, geom.Pt(is.W+dlInfoGap, 0))

	return cs.Constrain(geom.Sz(w, max(is.H, ts.H, dlInfoLine)))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// setChoice is `dlChoice` (:600): the radio moves, the button's size follows.
func (s *DownloadSheet) setChoice(c env.DownloadChoice) {
	s.choice = c
	s.sync()
}

// toggleWiFi is `pref:wifi`: the preference turns over and the switch shows it.
func (s *DownloadSheet) toggleWiFi() {
	on := !s.e.WiFiOnly()
	s.e.SetWiFiOnly(on)
	if sw, ok := s.wifi.Trailing.(*kit.Switch); ok {
		sw.SetOn(on)
	}
}

// confirmPress is `dlConfirm` (:601): the lessons of the choice go to the Shell,
// which starts what is not done or under way yet.
func (s *DownloadSheet) confirmPress() {
	if s.choice == env.DownloadChoiceAll {
		s.recount()
		s.e.Act.StartDownloads(s.Params.Course, s.missing)

		return
	}
	s.e.Act.StartDownloads(s.Params.Course, []model.Lesson{s.lesson})
}

// onEvent follows the downloads: what is under way or finished changes the counts,
// the free space read again changes the line; a progress changes nothing here.
func (s *DownloadSheet) onEvent(ev env.DownloadEvent) {
	switch ev.Kind {
	case env.DownloadEventChanged, env.DownloadEventFinished, env.DownloadEventFailed, env.DownloadEventPaused:
		s.recount()
		s.sync()
	case env.DownloadEventSpace:
		s.syncFree()
	case env.DownloadEventUnknown, env.DownloadEventProgress:
	default:
	}
}

// recount reads which lessons "Cả khoá" would download: those with an audio that
// have no file and are not under way (Q-P5-10).
func (s *DownloadSheet) recount() {
	s.missing, s.bytes = env.LessonsToDownload(s.Params.Course, s.dm)
}

// sync makes the widgets show the lesson, the counts and the choice.
func (s *DownloadSheet) sync() {
	l := s.lesson
	sub := format.No(l.No) + " · " + l.Title + " · " + format.Clock(float64(l.DurationSec)) // `${n} · ${song} · ${fmt(dur)}`
	if sub != s.oneSub {
		s.oneSub = sub
		s.one.Sub = sub
		s.one.Relayout()
	}
	if size := s.sizeText(l.AudioSizeBytes); size != s.oneSize {
		s.oneSize = size
		s.one.Trailing = size
		s.one.Relayout()
	}
	have := max(s.audio-len(s.missing), 0)
	sub = s.e.Fill("download.missingTpl", "n", itoa(len(s.missing)), "have", itoa(have))
	if sub != s.allSub {
		s.allSub = sub
		s.all.Sub = sub
		s.all.Relayout()
	}
	if size := s.sizeText(s.bytes); size != s.allSize {
		s.allSize = size
		s.all.Trailing = size
		s.all.Relayout()
	}
	s.one.SetOn(s.choice == env.DownloadChoiceOne)
	s.all.SetOn(s.choice == env.DownloadChoiceAll)
	size, known := s.oneSize, l.AudioSizeBytes > 0
	if s.choice == env.DownloadChoiceAll {
		size, known = s.allSize, s.bytes > 0
	}
	if known {
		s.confirm.SetText(s.e.Fill("download.confirmTpl", "size", size))
	} else {
		s.confirm.SetText(s.e.T("download.confirm")) // no size to say: "Tải xuống" alone
	}
	s.syncFree()
}

// sizeText is a size as the radios and the button say it; a size the catalog does
// not give (audio_size_bytes null, so 0) is "—", not "0,0 MB".
func (s *DownloadSheet) sizeText(bytes int64) string {
	if bytes <= 0 {
		return s.e.T("download.sizeUnknown")
	}

	return format.Size(bytes)
}

// syncFree shows the free space of the device; with none known (Q-P5-9) the
// line is left out, as there is no text for it.
func (s *DownloadSheet) syncFree() {
	free, _, ok := s.dm.FreeSpace()
	if !ok {
		if !s.info.Hidden {
			kit.SetHidden(s.info, true)
			s.freeText = ""
			s.col.Relayout()
		}

		return
	}
	text := s.e.Fill("download.freeTpl", "gb", format.GB(free))
	if text != s.freeText {
		s.freeText = text
		s.info.SetText(text)
	}
	if s.info.Hidden {
		kit.SetHidden(s.info, false)
		s.col.Relayout()
	}
}

// lessonOf returns lesson id of c, the first lesson when c has none of that id
// (the zero lesson when it has no lesson).
func lessonOf(c model.Course, id string) model.Lesson {
	for _, l := range c.Lessons {
		if l.ID == id {
			return l
		}
	}
	if len(c.Lessons) > 0 {
		return c.Lessons[0]
	}

	return model.Lesson{}
}

// audioLessons counts the lessons of c that have an audio file.
func audioLessons(c model.Course) int {
	n := 0
	for _, l := range c.Lessons {
		if l.HasAudio {
			n++
		}
	}

	return n
}
