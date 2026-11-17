//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/np/nowplaying.go at 49de3c2 (a screen
// that follows the player while it is mounted) and from this package's video.go
// (the timer made once, stopped when the screen goes). jPlayer has no countdown;
// the rules are the prototype's: MS.done and the 1 s interval at index.html:441-448
// and :788-790, docs/plan/03 section 5 and 5a.

package screens

import (
	"strconv"
	"strings"
	"time"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The end-of-lesson countdown: `cdM=5`, then `cdM--` every `setInterval(...,
// 1000)` (index.html:775, :788-790).
const (
	// doneCount is the seconds the countdown starts at.
	doneCount = 5
	// doneTick is the interval of the countdown.
	doneTick = time.Second
)

// Done is screen 10, "Hết bài": a video that played to its end gives way to it. The
// finished lesson, the card of the course's progress, the card of the next
// lesson with the seconds left before it plays, "Xem bài NN" and "Về khoá học".
//
// The seconds count 5 to 0 once a second while the screen is shown; at 0 it asks
// the Shell to play the next lesson (Actions.PlayNext), which the player does
// when the preference "Tự phát bài tiếp theo" is on, 06 taking the place of 10.
// A course on its last lesson has no next card and no countdown. Leaving the
// screen stops it. The screen does not tell the player Leave: 06, which it
// replaced, did.
type Done struct {
	ui.Base

	e    *env.Env
	pb   env.Playback
	bg   radialBG
	page *nav.Page
	body *doneContent

	ctx    *ui.Context // the view's, from Mounted: the timer needs it
	timer  ui.Timer
	tick   func()
	left   int
	held   bool
	counts [doneCount + 1]string // "TIẾP THEO · TỰ PHÁT SAU N GIÂY", made once
}

var (
	_ nav.Tabbed   = (*Done)(nil)
	_ nav.Scroller = (*Done)(nil)
	_ ui.Mounter   = (*Done)(nil)
	_ ui.Unmounter = (*Done)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewDone returns screen 10 over the player of the services, built for the lesson
// it holds. Without a player it is the stub with a back button.
func NewDone(e *env.Env) nav.Screen {
	pb := e.Playback()
	if pb == nil {
		return NewStub(e, env.RouteDone, -1)
	}
	d := &Done{e: e, pb: pb, left: doneCount}
	d.tick = d.onTick
	d.bg.layers = []kit.Radial{{
		// radial-gradient(circle at 50% 22%, rgba(255,208,40,.3), transparent 50%) (:441)
		Shape: kit.RadialCircle, CX: .5, CY: dBgAt, Color: color.RGBA(255, 208, 40, dBgAlpha), End: dBgEnd,
	}}
	for i := range d.counts {
		// `.eb` is upper case (text-transform): made here, once, so that a tick sets
		// a string that exists and the label upper-cases nothing
		d.counts[i] = strings.ToUpper(e.Fill("done.nextTpl", "s", strconv.Itoa(i)))
	}
	d.body = d.newBody()
	d.page = nav.NewPage(d.body, -1)
	d.Add(&d.bg, d.page)

	return d
}

// newBody makes the content for the lesson the player holds.
func (d *Done) newBody() *doneContent {
	e, pb := d.e, d.pb
	l := pb.Lesson()
	list := d.lessons(l)
	n := max(l.Count, len(list))
	w := d.watchedCount(l, list)

	b := &doneContent{}
	b.back = newLightBack(e)
	b.badge = newDoneBadge()
	b.title = heading(kit.Display(e.Fill("done.titleTpl", "no", format.No(l.No)), dTitleSize), 1)
	b.title.Align = ui.TextAlignCenter
	b.tag = &kit.Label{Text: e.Fill("done.songTagTpl", "song", l.Title, "tag", l.Tag), Weight: 700, Ink: theme.InkPrimary, Align: ui.TextAlignCenter, Wrap: true}
	b.tag.Name = nameDoneTag
	b.course = kit.Muted(l.CourseName, dCourseSize)
	b.course.Name, b.course.Align = nameMuted, ui.TextAlignCenter

	b.prog = newProgressRow()
	share := float32(0)
	if n > 0 {
		share = float32(w) / float32(n)
	}
	b.prog.ring.SetShare(share)
	b.prog.ring.SetText(e.Fill("done.ringTpl", "w", strconv.Itoa(w), "n", strconv.Itoa(n)))
	b.prog.wl.SetText(e.Fill("done.watchedTpl", "w", strconv.Itoa(w), "n", strconv.Itoa(n)))
	b.prog.left.SetText(e.Fill("done.leftTpl", "left", strconv.Itoa(max(n-w, 0)), "short", setCase(l.CourseShort)))
	b.progCrd = kit.NewCard(b.prog)
	b.progCrd.Padding = geom.Uniform(dCardPad)

	if nx, ok := nextOf(l, list); ok && pb.HasNext() {
		b.next = newNextRow()
		b.next.no.label.SetText(format.No(nx.No))
		b.next.sl.SetText(nx.Title)
		b.next.meta.SetText(e.Fill("done.songTagTpl", "song", nx.Tag, "tag", format.Clock(float64(nx.DurationSec))))
		b.next.SetCount(doneCount, d.counts[doneCount])
		b.nextCrd = kit.NewCard(b.next)
		b.nextCrd.Padding = geom.Uniform(dNextPad)

		b.watch = kit.NewButton(kit.ButtonP, e.Fill("done.watchTpl", "no", format.No(nx.No)), 0)
		b.watch.Name = "playVideo:" + LessonKey(l.CourseID, nx.No) // `data-act="playVideo" data-k` (:447)
		id, no, cid := nx.ID, nx.No, l.CourseID
		b.watch.OnPress = func() { e.Act.PlayVideo(cid, id, no) }
	} else {
		b.para = kit.Muted(e.T("done.allWatched"), 0)
		b.para.Name, b.para.Align, b.para.Wrap = namePara, ui.TextAlignCenter, true
	}
	b.toCrs = kit.NewButton(kit.ButtonT, e.T("done.toCourse"), 0)
	b.toCrs.Name = "toDetail:" + CourseKey(l.CourseID) // `data-act="toDetail" data-set` (:448)
	cid := l.CourseID
	b.toCrs.OnPress = func() { e.Act.ToDetail(cid) }

	b.Add(b.back, b.badge, b.title, b.tag, b.course, b.progCrd)
	if b.nextCrd != nil {
		b.Add(b.nextCrd, b.watch)
	} else {
		b.Add(b.para)
	}
	b.Add(b.toCrs)

	return b
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Tab says the screen has no tab bar.
func (d *Done) Tab() int { return -1 }

// Scroll returns the page's scroll.
func (d *Done) Scroll() *ui.Scroll { return d.page.Scroll() }

// Seconds returns the seconds the countdown shows; Counting whether it is still
// running.
func (d *Done) Seconds() int { return d.left }

// Counting tells whether the countdown is running: a next lesson, the screen
// mounted and not held.
func (d *Done) Counting() bool { return d.timer.Pending() }

// Hold stops the countdown at 5 and keeps it there: the shot state of M10 is the
// screen with the countdown pinned (docs/plan/P4.md).
func (d *Done) Hold() {
	d.held = true
	d.timer.Stop()
	d.show(doneCount)
}

// Restart runs the countdown again from 5, which a countdown that reached 0 or was
// held does not do by itself.
func (d *Done) Restart() {
	d.held = false
	d.timer.Stop()
	d.show(doneCount)
	if d.body.next != nil {
		d.arm()
	}
}

// Mounted starts the countdown, `cdM=5` on entering 10 (index.html:775, :782).
func (d *Done) Mounted(c *ui.Context) {
	d.ctx = c
	if d.body.next == nil || d.held || d.timer.Pending() {
		return
	}
	d.show(doneCount)
	d.arm()
}

// Unmounted stops the countdown: leaving 10 ends it.
func (d *Done) Unmounted(*ui.Context) {
	d.timer.Stop()
	d.ctx = nil
}

// Layout fills the window with the page over the gradient.
func (d *Done) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	full := ui.Tight(cs.Max)
	c.Layout(&d.bg, full)
	c.Place(&d.bg, geom.Point{})
	c.Layout(d.page, full)
	c.Place(d.page, geom.Point{})

	return cs.Max
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// arm waits a second for the next tick.
func (d *Done) arm() {
	if d.ctx != nil {
		d.timer = d.ctx.AfterFunc(doneTick, d.tick)
	}
}

// onTick is `cdM--`: the seconds show one less; at 0 the next lesson is asked for.
func (d *Done) onTick() {
	if d.left > 0 {
		d.show(d.left - 1)
	}
	if d.left > 0 {
		d.arm()

		return
	}
	d.e.Act.PlayNext()
}

// show sets the seconds the eyebrow shows.
func (d *Done) show(n int) {
	d.left = n
	if d.body.next != nil {
		d.body.next.SetCount(n, d.counts[n])
	}
}

// lessons returns the lessons of the course of l as the last read of it gave
// them, nil when none was read.
func (d *Done) lessons(l player.Lesson) []model.Lesson {
	if c, ok := d.e.CachedCourse(l.CourseID); ok {
		return c.Lessons
	}

	return nil
}

// watchedCount is `L.filter(x=>S.watched[key(x)]).length` (:441): the lessons the
// server says are watched, those this run watched, and the lesson just finished
// when it was played to the share that makes it watched.
func (d *Done) watchedCount(l player.Lesson, list []model.Lesson) int {
	current := l.Watched
	if dur := d.pb.Dur(); dur > 0 && d.pb.Pos() >= player.WatchedRatio*dur {
		current = true
	}
	n, seen := 0, false
	for i := range list {
		isCur := list[i].ID == l.ID
		seen = seen || isCur
		if list[i].Progress.Watched || d.e.WatchedLocal(list[i].ID) || (isCur && current) {
			n++
		}
	}
	if !seen && current {
		n++
	}

	return n
}

// nextOf returns the lesson after l in the course (`L[l.n]`), false on the last.
func nextOf(l player.Lesson, list []model.Lesson) (model.Lesson, bool) {
	for i := range list {
		if list[i].No == l.No+1 {
			return list[i], true
		}
	}

	return model.Lesson{}, false
}

// setCase is `c.short.replace('SET','Set')` (:445): the first "SET" of the short
// name, which the API's Short() has in capitals, written as "Set".
func setCase(short string) string { return strings.Replace(short, "SET", "Set", 1) }
