//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/home.go at 49de3c2 (the body made
// from the data and replaced when the data is read again, a Refresher that keeps
// the page's scroll), from jplayer-mobile/internal/ui/views/form_link.go at 49de3c2
// (one request at a time: the answer comes back through ui.Go and a stale one is
// dropped) and from jplayer-mobile/internal/ui/views/iptv_list.go at 49de3c2 (the
// page built once, filtered in place). The screen is MS.courses (index.html:374-380)
// over GET /me/courses, every page merged.

package screens

import (
	"context"
	"errors"

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

// coursesTab is the tab Courses is the root of: `tabbar(1)`.
const coursesTab = 1

// Courses is MS.courses, screen 04 (with 04-special and 04-empty, which are its
// filter and its search): the title, the filter, the search field, the count and a
// card per course. The list is read when the screen mounts; while the network is down
// it shows from the client's cache under the offline bar (F-7); when the network is
// back it is read again and replaced in place, the filter, the text and the scroll
// kept. A key of the search, or a tap on the filter, hides and shows the cards that
// are there: nothing is built and the field keeps the focus.
type Courses struct {
	ui.Base

	e       *env.Env
	page    *nav.Page
	content *coursesContent
	bar     *kit.OfflineBar

	online bool
	ready  bool // the list was read once
	cancel func()
	again  reread // the automatic reads after a first read that failed
}

var (
	_ nav.Tabbed          = (*Courses)(nil)
	_ nav.Scroller        = (*Courses)(nil)
	_ env.OnlineAware     = (*Courses)(nil)
	_ env.ForegroundAware = (*Courses)(nil)
	_ ui.Mounter          = (*Courses)(nil)
	_ ui.Unmounter        = (*Courses)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewCourses returns the Courses screen (MS.courses, tab 1) with the filter and the
// text the app last left it with.
func NewCourses(e *env.Env) *Courses {
	k := &Courses{e: e, online: e.Online()}
	k.content = newCoursesContent(e, &e.CoursesView)
	k.page = nav.NewPage(k.content, coursesTab)
	k.Add(k.page)
	k.applyBar()

	return k
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Tab returns the Courses tab.
func (k *Courses) Tab() int { return coursesTab }

// Scroll returns the page's scroll.
func (k *Courses) Scroll() *ui.Scroll { return k.page.Scroll() }

// Content returns the column, for the tests.
func (k *Courses) Content() *coursesContent { return k.content }

// Mounted starts reading the list with the next turn: a view that is being mounted
// has no host yet, and ui.Go would run the read on the UI thread then.
func (k *Courses) Mounted(c *ui.Context) { c.AfterFunc(0, k.load) }

// Unmounted ends the read in flight.
func (k *Courses) Unmounted(*ui.Context) {
	k.again.stop()
	if k.cancel != nil {
		k.cancel()
		k.cancel = nil
	}
}

// Foregrounded is the app coming back to the front: while the list was never read it
// is read again at once (a read the system cut while the app was away).
func (k *Courses) Foregrounded() {
	if k.ready {
		return
	}
	k.again.reset()
	k.load()
}

// SetOnline is the network going or coming back (03 section 2): the offline bar
// shows or goes, the page under it moving 54 to 94 and back; when the network
// returns the list is read again.
func (k *Courses) SetOnline(online bool) {
	k.online = online
	k.applyBar()
	if online {
		k.again.reset()
		k.load()
	}
}

// Layout fills the window with the page.
func (k *Courses) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(k.page, ui.Tight(cs.Max))
	c.Place(k.page, geom.Point{})

	return cs.Max
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// load reads GET /me/courses, one read at a time: a read in flight is dropped for it.
func (k *Courses) load() {
	if k.cancel != nil {
		k.cancel()
	}
	k.cancel = ui.Go(k, func(ctx context.Context) (model.CoursePage, error) {
		return k.e.Courses(ctx)
	}, k.loaded)
}

// loaded takes the answer: the cards on success; the retry when the first read
// fails; what is on show stays when a later one fails (the client has already told
// the app it is offline). A canceled read, the screen left or superseded, shows
// nothing.
func (k *Courses) loaded(c *ui.Context, p model.CoursePage, err error) {
	k.cancel = nil
	switch {
	case err == nil:
		k.ready = true
		k.again.reset()
		k.content.SetCourses(p)
	case errors.Is(err, context.Canceled):
	case k.e.Refused(err): // 426: the upgrade screen (03 section 9)
	default:
		k.content.SetError(k.retry)
		if !k.ready {
			k.again.schedule(c, k.load)
		}
	}
}

// retry is the "Thử lại": the sync line comes back and the read again.
func (k *Courses) retry() {
	k.content.SetError(nil)
	k.again.reset()
	k.load()
}

// applyBar shows the offline bar while there is no network, over the page.
func (k *Courses) applyBar() {
	if k.online {
		k.page.SetBar(nil)

		return
	}
	if k.bar == nil {
		k.bar = kit.NewOfflineBar(k.e.T("offline.bar"))
	}
	k.page.SetBar(k.bar)
}
