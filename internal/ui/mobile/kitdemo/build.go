//go:build !tvos

package kitdemo

import (
	"fmt"
	"strings"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// tabs returns the bottom bar (TABS, index.html:323) with tab current chosen;
// names pins the layout.json key of a tab whose name an earlier element of the
// prototype already took ("go.account#2").
func (d *Data) tabs(current int, names map[int]string) *kit.TabBar {
	items := [kit.TabCount]kit.TabItem{
		{Icon: icon.Home, Label: d.T("tabs.home"), Name: "go.home"},
		{Icon: icon.Grid, Label: d.T("tabs.courses"), Name: "go.courses"},
		{Icon: icon.Dl, Label: d.T("tabs.downloads"), Name: "go.downloads"},
		{Icon: icon.User, Label: d.T("tabs.account"), Name: "go.account"},
	}
	for i, n := range names {
		items[i].Name = n
	}

	return kit.NewTabBar(items, current, nil)
}

// back returns the back button (backBtn, index.html:325) named name, `.dk`
// when dark.
func (d *Data) back(name string, dark bool) *kit.RoundButton {
	b := kit.NewRoundButton(icon.Cl, dark, d.T("a11y.back"))
	b.Name = name

	return b
}

// coursePoster returns the 156 x 100 cover of a course in a strip
// (`poster(id,156,100,{ts:20})`).
func coursePoster(c model.Course) *poster.Poster {
	s := poster.ForCourse(c)
	s.TitleSize = 20

	return poster.New(s, 156, 100)
}

// category maps a course's category to the card's.
func category(c model.Category) kit.CardCategory {
	switch c {
	case model.CategorySpecial:
		return kit.CardSpecial
	case model.CategoryIntro:
		return kit.CardIntro
	case model.CategoryRegular, model.CategoryUnknown:
		return kit.CardRegular
	default:
		return kit.CardRegular
	}
}

// chipKind maps a course's category to the chip's.
func chipKind(c model.Category) kit.ChipKind {
	switch c {
	case model.CategorySpecial:
		return kit.ChipDb
	case model.CategoryIntro:
		return kit.ChipGt
	case model.CategoryRegular, model.CategoryUnknown:
		return kit.ChipTh
	default:
		return kit.ChipTh
	}
}

// lessonKey is the key the prototype names a lesson's controls by:
// "set35-3".
func lessonKey(c model.Course, l model.Lesson) string {
	return fmt.Sprintf("%s-%d", ProtoID(c.ID), l.No)
}

// lessonMeta is the second line of a lesson row (lesRow, index.html:382-393).
func (d *Data) lessonMeta(l model.Lesson, offline bool) string {
	switch {
	case offline && Downloaded(l.No):
		return l.Tag + " · " + d.T("lesson.audioDone")
	case offline:
		return l.Tag + " · " + d.T("lesson.needNet")
	case current(l):
		return l.Tag + " · " + d.T("lesson.watching") + " " + format.Clock(float64(l.Progress.PositionSec)) + " / " + format.Clock(float64(l.DurationSec))
	case l.Progress.Watched:
		return l.Tag + " · " + format.Clock(float64(l.DurationSec)) + " · " + d.T("lesson.watched")
	default:
		return l.Tag + " · " + format.Clock(float64(l.DurationSec))
	}
}

// current tells whether l is the lesson under way: some of it seen, not all.
func current(l model.Lesson) bool { return l.Progress.PositionSec > 0 && !l.Progress.Watched }

// lessonRows returns the rows of Set 35 in a LessonList (lesRow, :382-393):
// mode online (M05) or offline (M13).
func (d *Data) lessonRows(offline bool) *kit.LessonList {
	list := &kit.LessonList{}
	set := d.Set35
	for _, l := range set.Lessons {
		k := lessonKey(set, l)
		r := &kit.LessonRow{
			No: l.No, Title: l.Title, Meta: d.lessonMeta(l, offline), Download: kit.DlNone,
			DlName: "dlTap:" + k, PlayName: "playAudio:" + k,
			DlLabel: d.T("a11y.download"), PlayLabel: d.T("a11y.play"), CancelLabel: d.T("a11y.cancelDownload"), DoneLabel: d.T("video.side.downloaded"),
		}
		r.Name = "playVideo:" + k
		if offline {
			r.Name, r.Mode = "offTap:"+k, kit.LessonOffline
		} else if current(l) {
			// the "now watching" card of the detail screen holds an element of the
			// same data-act and data-k before the rows: the row is the second.
			r.Name += "#2"
		}
		switch {
		case current(l):
			r.State = kit.LessonCurrent
		case l.Progress.Watched:
			r.State = kit.LessonWatched
		default:
			r.State = kit.LessonNormal
		}
		switch {
		case l.No == downloadingNo:
			r.Download, r.Progress = kit.DlIng, downloadingProgress
		case Downloaded(l.No):
			r.Download = kit.DlDone
		default: // not downloaded: the row keeps no download mark
		}
		r.Dim = offline && r.Download != kit.DlDone
		list.Add(r)
	}

	return list
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// shortName is a course's name without the brand, "Set 35" (the prototype's
// `short.replace('SET','Set')`).
func shortName(c model.Course) string { return strings.TrimPrefix(c.Name, "DanceFIT ") }

// minutes is the length of a course in whole minutes, as the prototype writes
// it (`fmt(total).split(':')[0]`).
func minutes(c model.Course) int {
	total := 0
	for _, l := range c.Lessons {
		total += l.DurationSec
	}

	return total / 60
}

// lessonCount is how many lessons Set 35 has.
func lessonCount(c model.Course) int { return len(c.Lessons) }

// downloadedCount is how many lessons of the course have their audio.
func downloadedCount(c model.Course) int {
	n := 0
	for _, l := range c.Lessons {
		if Downloaded(l.No) {
			n++
		}
	}

	return n
}

// watchedCount is how many lessons of the course are watched.
func watchedCount(c model.Course) int {
	n := 0
	for _, l := range c.Lessons {
		if l.Progress.Watched {
			n++
		}
	}

	return n
}
