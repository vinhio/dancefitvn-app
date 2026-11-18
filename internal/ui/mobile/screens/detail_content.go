//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/coll.go at 49de3c2 (the page's
// rows made once from the data, each bound to its item and its actions; the head
// above them) and home_content.go (a column whose children each sit a number of
// DIPs under the one before). The column is MS.detail (index.html:394-399) over
// GET /courses/{id}; the rows are lesRow (index.html:382-393), their view-model
// that of kitdemo/build.go (lessonMeta, current), copied and not imported.

package screens

import (
	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of MS.detail's column (index.html:396-399), in DIPs.
const (
	// detailSide is `#m .scroll{padding:0 20px}`; the hero takes it back.
	detailSide float32 = 20
	// detailBottom is `padding-bottom:110px` of the scroll.
	detailBottom float32 = 110
	// footPad* are `padding:20px 20px 38px` of the button's footer.
	footPadTop, footPadSide, footPadBottom float32 = 20, 20, 38
	// footIcon is the footer button's `<span style="width:18px">PLAY</span>`.
	footIcon float32 = 18
)

// detailContent is what the detail scrolls: the hero, the card of the lesson to go
// on with, the tabs and the lessons, each as MS.detail's blocks are: the card 10
// under the hero, the tabs 22 under the card, the first row 2 under the tabs'
// line. It is made once from the course; nothing is built after.
type detailContent struct {
	ui.Base

	hero *courseHero
	card *kit.Card
	tabs *detailTabs
	list *kit.LessonList
	rows []*kit.LessonRow

	// the counts line of the card ("2/10 đã xem · 5/10 audio đã tải") and what it
	// is made of, so that the downloads change it in place
	e        *env.Env
	counts   *kit.Label
	ids      []string // the lesson of each row
	hasAudio []bool   // whether the course data give it an audio file
	watch    int      // the lessons watched
	total    int      // the lessons of the course
	done     int      // the lessons with their audio on the device
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newDetailFrame returns the screen's frame for the course c the user is enrolled in:
// the content with 110 under it and the footer with the "continue" button.
func newDetailFrame(e *env.Env, c model.Course) (*scrFrame, *detailContent) {
	cl, resumed := continueLesson(c)
	n := lessonTotal(c)
	key := LessonKey(c.ID, cl.No)
	play := func() { e.Act.PlayVideo(c.ID, cl.ID, cl.No) }

	k := &detailContent{e: e, total: n}
	k.hero = newCourseHero(e, c,
		e.Fill("detail.eyebrowTpl", "cat", c.CategoryLabel, "n", itoa(n)),
		// K37: no "…" button: the prototype draws it with no action (index.html:398);
		// the owner removed it
		heroSub(e, c, "detail.subTpl", true), false, false)

	title, btn := e.Fill("detail.startTpl", "no", format.No(1)), e.Fill("detail.watchTpl", "no", format.No(1))
	if resumed {
		title = e.T("detail.watching") + " " + format.No(cl.No)
		btn = e.T("detail.continueLesson") + " " + format.No(cl.No)
	}
	watched, done := 0, 0
	for _, l := range c.Lessons {
		if l.Progress.Watched || e.WatchedLocal(l.ID) {
			watched++
		}
		if st, _ := e.Downloads().State(l.ID); st == kit.DlDone {
			done++
		}
	}
	k.watch, k.done = watched, done
	counts := e.Fill("detail.progressTpl", "w", itoa(watched), "n", itoa(n), "d", itoa(done))
	row := newWatchRow(title, counts, key, e.T("detail.downloadAll"), CourseKey(c.ID), play, func() { e.Act.DlAll(c.ID) })
	k.counts = row.text.sub
	k.card = kit.NewCard(row)
	k.card.Padding = geom.Uniform(watchPad)

	k.tabs = newDetailTabs(e.Fill("detail.tabLessonsTpl", "n", itoa(n)), e.T("detail.tabIntro"))

	k.list = &kit.LessonList{}
	for _, l := range c.Lessons {
		r := newLessonRow(e, c, l, resumed)
		k.rows = append(k.rows, r)
		k.ids = append(k.ids, l.ID)
		k.hasAudio = append(k.hasAudio, l.HasAudio)
		k.list.Add(r)
	}
	k.Add(k.hero, k.card, k.tabs, k.list)

	b := kit.NewButton(kit.ButtonP, btn, icon.Play)
	b.Name, b.IconSize = "playVideo:"+key, footIcon // `data-act="playVideo" data-k`
	b.OnPress = play
	foot := newFoot(b, geom.Insets{Top: footPadTop, Right: footPadSide, Bottom: footPadBottom, Left: footPadSide}, true)

	return newFrame(k, detailBottom, foot), k
}

// newLessonRow returns lesRow for lesson l of course c (index.html:382-393): the
// state from the progress, the meta line, the download from the device; a tap on the
// row plays, one on the download button downloads. resumed says the course has a
// place to go on from.
func newLessonRow(e *env.Env, c model.Course, l model.Lesson, resumed bool) *kit.LessonRow {
	key := LessonKey(c.ID, l.No)
	r := &kit.LessonRow{
		No: l.No, Title: l.Title, Download: kit.DlNone,
		DlName: "dlTap:" + key, PlayName: "playAudio:" + key, // `data-act="dlTap"`, the offline disc's
		Lesson:  e.Fill("a11y.lessonTpl", "no", format.No(l.No), "title", l.Title), // value of the download button and the play disc
		DlLabel: e.T("a11y.download"), PlayLabel: e.T("a11y.play"), CancelLabel: e.T("a11y.cancelDownload"), DoneLabel: e.T("video.side.downloaded"),
	}
	r.Name = "playVideo:" + key
	seen := l.Progress.Watched || e.WatchedLocal(l.ID) // the server's, or this run's
	pos := l.Progress.PositionSec
	current := resumed && c.Resume.LessonID == l.ID && c.Resume.PositionSec > 0 && !seen
	if current {
		pos = c.Resume.PositionSec
	}
	switch {
	case current:
		r.State = kit.LessonCurrent
		r.Meta = e.Fill("lesson.metaWatchingTpl", "tag", l.Tag, "pos", format.Clock(float64(pos)), "dur", format.Clock(float64(l.DurationSec)))
	case seen:
		r.State = kit.LessonWatched
		r.Meta = format.Join(l.Tag, format.Clock(float64(l.DurationSec)), e.T("lesson.watched"))
	default:
		r.State = kit.LessonNormal
		r.Meta = format.Join(l.Tag, format.Clock(float64(l.DurationSec)))
	}
	switch st, p := env.DownloadOf(e.Downloads(), l.ID, l.HasAudio); st {
	case kit.DlIng:
		r.Download, r.Progress = kit.DlIng, p
	case kit.DlDone, kit.DlUnavailable:
		r.Download = st
	case kit.DlNone, kit.LessonDownloadUnknown:
	default:
	}
	r.OnPress = func() { e.Act.PlayVideo(c.ID, l.ID, l.No) }
	r.OnDl = func() { e.Act.DlTap(c.ID, l.ID, l.No) }

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// syncDownloads makes the rows and the count show the files of the device now: a
// row whose file started, finished, was canceled or deleted changes its button, the
// count of "audio đã tải" is made again when it changed. Nothing is rebuilt; with
// nothing changed it does not allocate.
func (k *detailContent) syncDownloads() {
	done := 0
	for i, r := range k.rows {
		st, p := env.DownloadOf(k.e.Downloads(), k.ids[i], k.hasAudio[i])
		if st == kit.DlDone {
			done++
		}
		switch st {
		case kit.DlIng:
			if r.Download != kit.DlIng {
				r.Download = kit.DlIng
				r.Relayout()
			}
			r.SetProgress(p)
		case kit.DlDone:
			if r.Download != kit.DlDone {
				r.Download = kit.DlDone
				r.Relayout()
			}
		case kit.DlUnavailable:
			if r.Download != kit.DlUnavailable {
				r.Download, r.Progress = kit.DlUnavailable, 0
				r.Relayout()
			}
		case kit.DlNone, kit.LessonDownloadUnknown:
			if r.Download != kit.DlNone {
				r.Download, r.Progress = kit.DlNone, 0
				r.Relayout()
			}
		default:
		}
	}
	if done != k.done {
		k.done = done
		k.counts.SetText(k.e.Fill("detail.progressTpl", "w", itoa(k.watch), "n", itoa(k.total), "d", itoa(done)))
	}
}

// progress moves the ring of the row of the lesson to share p (a progress event);
// it allocates nothing.
func (k *detailContent) progress(lessonID string, p float32) {
	for i, id := range k.ids {
		if id == lessonID {
			if r := k.rows[i]; r.Download == kit.DlIng {
				r.SetProgress(p)
			}

			return
		}
	}
}

// Rows returns the lesson rows, for the tests.
func (k *detailContent) Rows() []*kit.LessonRow { return k.rows }

// Layout puts the hero across the window, the card, the tabs and the lessons 20 from
// the sides.
func (k *detailContent) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	inner := max(w-2*detailSide, 0)
	col := ui.Constraints{Min: geom.Sz(inner, 0), Max: geom.Sz(inner, ui.Unbounded)}
	hs := c.Layout(k.hero, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(k.hero, geom.Point{})
	y := hs.H + watchTop
	put := func(wd ui.Widget) float32 {
		s := c.Layout(wd, col)
		c.Place(wd, geom.Pt(detailSide, y))

		return s.H
	}
	y += put(k.card) + tabsTop
	y += put(k.tabs) + tabsBottom
	y += put(k.list)

	return cs.Constrain(geom.Sz(w, y))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// continueLesson is the lesson the card and the footer go on with: the one the
// course's resume names (`inThis`), else the first (`L[0]`); resumed tells which.
func continueLesson(c model.Course) (l model.Lesson, resumed bool) {
	if c.Resume != nil {
		for _, x := range c.Lessons {
			if x.ID == c.Resume.LessonID {
				return x, true
			}
		}
	}
	if len(c.Lessons) > 0 {
		return c.Lessons[0], false
	}

	return model.Lesson{No: 1}, false
}

// lessonTotal is how many lessons the course has (`c.lessons`): the count the API
// gives, or the lessons it listed.
func lessonTotal(c model.Course) int {
	if c.LessonCount > 0 {
		return c.LessonCount
	}

	return len(c.Lessons)
}

// courseMinutes is the length of the course in whole minutes as the prototype writes
// it (`fmt(total).split(':')[0]`): the lessons' sum, or the API's total when the
// lessons are not all listed.
func courseMinutes(c model.Course) string {
	total := 0
	for _, l := range c.Lessons {
		total += l.DurationSec
	}
	if total == 0 {
		total = c.TotalDurationSec
	}

	return format.TotalMinutes(total)
}

// heroSub is the muted line of the hero: "“tagline” · n bài · mm phút" for a detail
// (withMinutes), "“tagline” · n bài học" for a locked course; the quote leaves
// out with a tagline the course has not (`c.quote?'“'+...+'” · ':”`).
func heroSub(e *env.Env, c model.Course, tpl string, withMinutes bool) string {
	n := itoa(lessonTotal(c))
	if c.Tagline != "" {
		return e.Fill(tpl, "tagline", c.Tagline, "n", n, "min", courseMinutes(c))
	}
	if withMinutes {
		return n + " " + e.T("lesson.unitLower") + " · " + courseMinutes(c) + " " + e.T("lesson.minutes")
	}

	return n + " " + e.T("lesson.unitHeading")
}
