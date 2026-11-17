//go:build !tvos

package kitdemo

import (
	"fmt"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"

	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// pageM07 is the player's options sheet (SH.options, index.html:505-512) over
// the player; only the two chips of the player's lesson line (:136) are drawn
// behind the dim.
func pageM07() Page {
	return Page{ID: "M07", Title: "Options sheet", Build: func(d *Data) Screen {
		cv := newCanvas()
		cur := d.currentLesson()
		cv.fit(kit.NewChip(kit.ChipY, cur.Tag), 20, 580, 45.94)
		k := kit.NewChip(kit.ChipK, d.T("options.vertical"))
		cv.fit(k, 73.94, 580, 97.52)
		st := newStage(cv)
		st.open = func(s *kit.Sheet) { s.Open(d.optionsSheet(s), kit.SheetOptions{}) }

		// the player under the sheet has 8 `svg.i` (back, more, the four side
		// buttons, the two seek buttons) and a `div.eb` before it.
		return Screen{Root: st, Skip: map[string]int{"svg.i": 8}}
	}}
}

// pageM09 is the download sheet (SH.download, :513-519) over the course.
func pageM09() Page {
	return Page{ID: "M09", Title: "Download sheet", Build: func(d *Data) Screen {
		st := newStage(d.detail())
		st.open = func(s *kit.Sheet) { s.Open(d.downloadSheet(s), kit.SheetOptions{}) }

		return Screen{Root: st}
	}}
}

// pageM14 is the delete sheet (SH.del, :520-522) over the downloads screen,
// which the page leaves out.
func pageM14() Page {
	return Page{ID: "M14", Title: "Delete sheet", Build: func(d *Data) Screen {
		st := newStage(newCanvas())
		st.open = func(s *kit.Sheet) { s.Open(d.deleteSheet(s), kit.SheetOptions{}) }

		// the downloads screen under it: 17 `svg.i`, a `div.d`-free page with one
		// `p.muted` before the sheet's.
		return Screen{Root: st, Skip: map[string]int{"svg.i": 17}}
	}}
}

// pageLogout is the logout sheet (SH.logout, :530) over the account screen.
func pageLogout() Page {
	return Page{ID: "M15-logout", Title: "Logout sheet", Build: func(d *Data) Screen {
		st := newStage(d.account())
		st.open = func(s *kit.Sheet) { s.Open(d.logoutSheet(s), kit.SheetOptions{}) }

		return Screen{Root: st}
	}}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// loopEndSec is the end of the A-B loop the prototype starts with (`p.b`).
const loopEndSec = 100

// downloadLessonNo is the lesson the download sheet is opened for in the
// prototype's M09 (`S.m.sheetK`): the fifth, which has no file yet.
const downloadLessonNo = 5

// lesson returns lesson no of Set 35.
func (d *Data) lesson(no int) model.Lesson {
	for _, l := range d.Set35.Lessons {
		if l.No == no {
			return l
		}
	}

	return d.Set35.Lessons[0]
}

// closer returns the action of a button that closes the sheet.
func closer(s *kit.Sheet) func() { return s.Close }

// optionsSheet is the content of SH.options at the boxes of M07.
func (d *Data) optionsSheet(s *kit.Sheet) ui.Widget {
	speeds := []float32{.5, .75, 1, 1.25, 1.5}
	seg := kit.NewSegmented()
	seg.NamePrefix = "speed:"
	for _, v := range speeds {
		seg.Options = append(seg.Options, fmt.Sprintf("%gx", v))
		seg.Keys = append(seg.Keys, fmt.Sprintf("%g", v))
	}
	seg.Selected = 1

	rowFor := func(name string, ic icon.Name, title, sub string, on bool) *kit.Row {
		r := kit.NewRow(ic, title, sub)
		r.Name, r.Trailing = name, kit.NewSwitch(on)

		return r
	}
	// the loop runs from where the lesson is (`p.a`, 72 s) to `p.b` (100 s).
	loop := format.Clock(float64(d.currentLesson().Progress.PositionSec)) + " → " + format.Clock(loopEndSec) + " · " + d.T("options.loopContinuous")
	mirror := rowFor("mirror#2", icon.Mirror, d.T("options.mirror"), d.T("options.mirrorSub"), true)
	rep := rowFor("loop#2", icon.Rep, d.T("options.loop"), loop, false)
	beat := rowFor("beat", icon.Music, d.T("options.beat"), d.T("options.beatSub"), true)
	quality := kit.NewRow(icon.Hd, d.T("options.quality"), "")
	quality.Name, quality.Value, quality.Last = "quality", d.T("options.qualityAuto")+" ›", true

	eb := kit.Eyebrow(d.T("options.speed"))
	eb.Name = "div.eb#2"
	done := kit.NewButton(kit.ButtonP, d.T("options.done"), icon.NameUnknown)
	done.Name, done.OnPress = "closeSheet", closer(s) // data-act="closeSheet" (:511)

	return newVStack(
		item{w: kit.NewSheetHeader(d.T("options.title"), d.T("a11y.close"), closer(s))},
		item{w: eb, before: kit.SheetHeaderGap},
		item{w: seg, before: 10},
		item{w: mirror, before: 10}, item{w: rep}, item{w: beat}, item{w: quality},
		item{w: done, before: 18},
	)
}

// downloadSheet is the content of SH.download at the boxes of M09.
func (d *Data) downloadSheet(s *kit.Sheet) ui.Widget {
	set := d.Set35
	l := d.lesson(downloadLessonNo)
	one := kit.NewRadio(d.T("download.one"), format.No(l.No)+" · "+l.Title+" · "+format.Clock(float64(l.DurationSec)), format.Size(l.AudioSizeBytes))
	one.Name = "dlChoice:one"
	one.SetOn(true)
	missing, bytes := 0, int64(0)
	for _, x := range set.Lessons {
		if !Downloaded(x.No) && x.No != downloadingNo {
			missing++
			bytes += x.AudioSizeBytes
		}
	}
	all := kit.NewRadio(d.T("download.all")+" "+shortName(set), fmt.Sprintf("%d %s · %s %d", missing, d.T("download.missing"), d.T("download.have"), lessonCount(set)-missing), format.Size(bytes))
	all.Name = "dlChoice:all"
	wifi := kit.NewRow(icon.Globe, d.T("account.wifiOnly"), "")
	wifi.Name, wifi.Trailing, wifi.Last = "pref:wifi", kit.NewSwitch(true), true
	info := newInfoLine(d.T("download.free"))
	confirm := kit.NewButton(kit.ButtonP, d.T("download.confirm")+" · "+format.Size(l.AudioSizeBytes), icon.Dl)
	confirm.Name = "dlConfirm"

	return newVStack(
		item{w: kit.NewSheetHeader(d.T("download.title"), d.T("a11y.close"), closer(s))},
		item{w: one, before: kit.SheetHeaderGap},
		item{w: all, before: kit.RadioGap},
		item{w: wifi, before: kit.RadioGap},
		item{w: info, before: 4},
		item{w: confirm, before: 16},
	)
}

// deleteSheet is the content of SH.del at the boxes of M14.
func (d *Data) deleteSheet(s *kit.Sheet) ui.Widget {
	l := d.currentLesson()
	title := kit.Display(d.T("delete.title"), 24)
	title.Align = ui.TextAlignCenter
	body := kit.Muted(format.No(l.No)+" · "+l.Title+" · "+format.Size(l.AudioSizeBytes)+" "+d.T("delete.willDelete")+"\n"+d.T("delete.stillWatch"), 14)
	body.LineHeight, body.Align, body.Wrap, body.Name = 1.5, ui.TextAlignCenter, true, "p.muted#2"
	del := kit.NewButton(kit.ButtonR, d.T("delete.confirm"), icon.Trash)
	del.Name = "delConfirm"
	cancel := kit.NewButton(kit.ButtonS, d.T("common.cancel"), icon.NameUnknown)
	cancel.Name, cancel.Height, cancel.OnPress = "closeSheet", 48, closer(s)

	return newVStack(
		item{w: newDisc(), before: 10, center: true},
		item{w: title, before: 20},
		item{w: body, before: 12},
		item{w: del, before: 20},
		item{w: cancel, before: 10},
	)
}

// logoutSheet is the content of SH.logout at the boxes of M15-logout.
func (d *Data) logoutSheet(s *kit.Sheet) ui.Widget {
	title := kit.Display(d.T("logout.title"), 24)
	title.Align, title.Name = ui.TextAlignCenter, "div.d#2"
	body := kit.Muted(d.T("logout.body"), 14)
	body.Align, body.Name = ui.TextAlignCenter, "p.muted#2"
	out := kit.NewButton(kit.ButtonR, d.T("logout.confirm"), icon.NameUnknown)
	out.Name = "logout"
	cancel := kit.NewButton(kit.ButtonS, d.T("common.cancel"), icon.NameUnknown)
	cancel.Name, cancel.Height, cancel.OnPress = "closeSheet", 48, closer(s)

	return newVStack(
		item{w: title, before: 10},
		item{w: body, before: 12},
		item{w: out, before: 20},
		item{w: cancel, before: 10},
	)
}
