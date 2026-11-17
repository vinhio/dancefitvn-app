//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/coll.go at 49de3c2 (a page of a
// head and rows made once from the data, kept in place when the data changes) and
// from detail_content.go (the lesson rows' view-model). The screen is MS.offline
// (index.html:461, rows lesRow mode 'off' :382-393) and its M13 capture
// (local/shots/prototype-kit/mobile/M13.layout.json).

package screens

import (
	"context"
	"slices"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Offline is MS.offline, screen 13: the course a user opens while there is no
// network (03 section 2). The offline bar sits on the top line, the scroll starts
// under it (94), a back button, the course's name, the hint and the lessons in
// mode 'off': a lesson with its audio on the device shows the play disc, one
// without is dimmed with a cloud. A tap on a row is `offTap`, one on the disc
// `playAudio`.
//
// The lessons are the course's, when a screen read it in this run or the client
// has it cached (Env.Course, F-7 of P3); otherwise the downloaded lessons of that
// course, from the display data the download kept (OfflineCourse). It follows the
// downloads' events while it is on show: a file that finished or was deleted
// changes its row in place, with nothing rebuilt.
type Offline struct {
	ui.Base

	e  *env.Env
	id string
	dm env.DownloadManager

	page *nav.Page
	body *offBody
	bar  *kit.OfflineBar

	full   bool // the course is the course's own list, not the downloaded ones
	rows   []offRow
	unsub  func()
	cancel func()
}

// offRow is a lesson row with what it needs to follow the downloads: the lesson,
// and the two meta lines it switches between.
type offRow struct {
	row              *kit.LessonRow
	id               string
	metaDone, metaNo string
}

var (
	_ nav.Tabbed   = (*Offline)(nil)
	_ nav.Scroller = (*Offline)(nil)
	_ ui.Mounter   = (*Offline)(nil)
	_ ui.Unmounter = (*Offline)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewOffline returns the offline screen of the course id (MS.offline, screen
// 13), without a tab bar.
func NewOffline(e *env.Env, courseID string) *Offline {
	o := &Offline{e: e, id: courseID, dm: e.Downloader()}
	c, full := e.CachedCourse(courseID)
	if !full {
		c, _ = OfflineCourse(o.dm.Items(), courseID)
	}
	o.full = full
	o.body = newOffBody(e)
	o.page = nav.NewPage(o.body, -1)
	o.bar = kit.NewOfflineBar(e.T("offline.bar"))
	o.page.SetBar(o.bar)
	o.fill(c)
	o.Add(o.page)

	return o
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Tab returns -1: the screen has no tab bar.
func (o *Offline) Tab() int { return -1 }

// Scroll returns the page's scroll.
func (o *Offline) Scroll() *ui.Scroll { return o.page.Scroll() }

// Rows returns the lesson rows, for the tests.
func (o *Offline) Rows() []*kit.LessonRow {
	out := make([]*kit.LessonRow, len(o.rows))
	for i := range o.rows {
		out[i] = o.rows[i].row
	}

	return out
}

// Mounted follows the downloads while the screen is on show, and reads the course
// when the screen has only the downloaded lessons of it (a course no screen has
// read in this run: the client may still have it cached).
func (o *Offline) Mounted(c *ui.Context) {
	if o.unsub == nil {
		o.unsub = o.dm.Events().Subscribe(o.onEvent)
	}
	o.resync()
	if !o.full {
		c.AfterFunc(0, o.load)
	}
}

// Unmounted stops following the downloads and ends the read in flight.
func (o *Offline) Unmounted(*ui.Context) {
	if o.unsub != nil {
		o.unsub()
		o.unsub = nil
	}
	if o.cancel != nil {
		o.cancel()
		o.cancel = nil
	}
}

// Layout fills the window with the page.
func (o *Offline) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(o.page, ui.Tight(cs.Max))
	c.Place(o.page, geom.Point{})

	return cs.Max
}

// OfflineCourse makes the course id from the downloaded lessons of it: the
// lessons in number order with the display data the downloads kept, the course's
// name and size. ok is false when no file is of that course.
func OfflineCourse(items []env.DownloadItem, courseID string) (c model.Course, ok bool) {
	for _, it := range items {
		if it.CourseID != courseID {
			continue
		}
		if !ok {
			ok = true
			c = model.Course{ID: courseID, Name: it.CourseName, LessonCount: it.CourseLessons, CoverURL: it.CoverURL, Enrolled: true}
		}
		c.Lessons = append(c.Lessons, model.Lesson{
			ID: it.LessonID, No: it.No, Title: it.Title, Tag: it.Tag, DurationSec: it.DurationSec,
			HasAudio: true, AudioSizeBytes: it.Bytes,
		})
	}
	slices.SortFunc(c.Lessons, func(a, b model.Lesson) int { return a.No - b.No })

	return c, ok
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// load reads the course, off the UI thread: offline it is the client's cache or
// nothing, and nothing leaves the downloaded lessons on show.
func (o *Offline) load() {
	if o.full || o.cancel != nil {
		return
	}
	o.cancel = ui.Go(o, func(ctx context.Context) (model.Course, error) {
		return o.e.Course(ctx, o.id)
	}, func(_ *ui.Context, c model.Course, err error) {
		o.cancel = nil
		if err != nil {
			return
		}
		o.full = true
		o.fill(c)
	})
}

// fill makes the head and the rows from the course c.
func (o *Offline) fill(c model.Course) {
	o.body.setName(c.Name)
	o.rows = o.rows[:0]
	o.body.list.Clear()
	resumed := c.Resume != nil
	for _, l := range c.Lessons {
		r := o.newRow(c, l, resumed)
		o.rows = append(o.rows, r)
		o.body.list.Add(r.row)
	}
	o.body.setEmpty(len(o.rows) == 0)
	o.page.Relayout()
}

// newRow returns lesRow(l, 'off'): the state of the lesson as the detail has it
// (the current one, the watched), the meta line and the end of the row by whether
// its audio is on the device.
func (o *Offline) newRow(c model.Course, l model.Lesson, resumed bool) offRow {
	e := o.e
	key := LessonKey(c.ID, l.No)
	r := &kit.LessonRow{
		No: l.No, Title: l.Title, Mode: kit.LessonOffline, Download: kit.DlNone,
		DlName: "dlTap:" + key, PlayName: "playAudio:" + key,
		Lesson:  e.Fill("a11y.lessonTpl", "no", format.No(l.No), "title", l.Title), // value of the download button and the play disc
		DlLabel: e.T("a11y.download"), PlayLabel: e.T("a11y.play"), CancelLabel: e.T("a11y.cancelDownload"), DoneLabel: e.T("video.side.downloaded"),
	}
	r.Name = "offTap:" + key // `data-act="offTap" data-k`
	seen := l.Progress.Watched || e.WatchedLocal(l.ID)
	switch {
	case resumed && c.Resume.LessonID == l.ID && c.Resume.PositionSec > 0 && !seen:
		r.State = kit.LessonCurrent
	case seen:
		r.State = kit.LessonWatched
	default:
		r.State = kit.LessonNormal
	}
	row := offRow{
		row: r, id: l.ID,
		metaDone: format.Join(l.Tag, e.T("lesson.audioDone")), metaNo: format.Join(l.Tag, e.T("lesson.needNet")),
	}
	row.apply(o.dm)
	r.OnPress = func() { e.Act.OffTap(c.ID, l.ID, l.No) }
	r.OnPlay = func() { e.Act.PlayAudio(c.ID, l.ID, l.No) }

	return row
}

// apply sets the row for the lesson's file: the play disc and "Audio đã tải" with
// it, the cloud, "Cần có mạng" and the .38 dim without. It reports whether
// anything changed.
func (r *offRow) apply(dm env.DownloadManager) bool {
	st, _ := dm.State(r.id)
	done := st == kit.DlDone
	if done == (r.row.Download == kit.DlDone) && r.row.Meta != "" {
		return false
	}
	if done {
		r.row.Download, r.row.Meta = kit.DlDone, r.metaDone
	} else {
		r.row.Download, r.row.Meta = kit.DlNone, r.metaNo
	}
	r.row.Dim = !done
	r.row.Relayout()

	return true
}

// onEvent is what the screen does on an event of the downloads: a progress
// changes nothing it shows; a file that appeared or went changes its row in
// place, or, with only the downloaded lessons on show, makes the rows again.
func (o *Offline) onEvent(ev env.DownloadEvent) {
	switch ev.Kind {
	case env.DownloadEventChanged, env.DownloadEventFinished:
		o.resync()
	case env.DownloadEventUnknown, env.DownloadEventProgress, env.DownloadEventPaused,
		env.DownloadEventFailed, env.DownloadEventSpace:
	default:
	}
}

// resync makes the rows show the files that are on the device now.
func (o *Offline) resync() {
	if !o.full {
		c, _ := OfflineCourse(o.dm.Items(), o.id)
		if o.sameLessons(c) {
			return
		}
		o.fill(c)

		return
	}
	for i := range o.rows {
		o.rows[i].apply(o.dm)
	}
}

// sameLessons tells whether the rows are those of the lessons of c.
func (o *Offline) sameLessons(c model.Course) bool {
	if len(c.Lessons) != len(o.rows) {
		return false
	}
	for i, l := range c.Lessons {
		if o.rows[i].id != l.ID {
			return false
		}
	}

	return true
}
