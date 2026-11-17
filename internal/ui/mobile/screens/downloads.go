//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/home.go at 49de3c2 (the body made
// from the data and replaced when the data changes, in a page that keeps its
// scroll) and from options_sheet.go (a view that follows a topic while it is
// mounted). The screen is MS.downloads (index.html:450-460), with screen 12 in it
// while there is nothing to show.

package screens

import (
	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tabDownloads is the tab "Tải xuống" of the tab bar (TABS, index.html:323).
const tabDownloads = 2

// Downloads is MS.downloads, screen 11 (and screen 12 while there are no files and
// no downloads under way): the title, the summary, the card of the device's space,
// the cards of the downloads under way and the files grouped by course. It follows
// the downloads while it is on show: a start, a finish, a cancel, a deletion makes
// the lists again; a progress moves the bar and the text of one card and builds
// nothing; the free space read again moves the card of the space.
type Downloads struct {
	ui.Base

	e     *env.Env
	dm    env.DownloadManager
	page  *nav.Page
	body  *dlBody
	unsub func()
}

// dlBody is what Downloads scrolls: the title, then the summary, the space card,
// the running cards and the groups, or the empty block.
type dlBody struct {
	ui.Base

	stack   blockStack
	storage *storageCard // nil on the empty screen
	running []*runCard
	files   int // rows of files
	empty   bool
}

var (
	_ nav.Tabbed   = (*Downloads)(nil)
	_ nav.Scroller = (*Downloads)(nil)
	_ ui.Mounter   = (*Downloads)(nil)
	_ ui.Unmounter = (*Downloads)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewDownloads returns MS.downloads (screen 11, tab 2) over the downloads of the
// services, as they are now.
func NewDownloads(e *env.Env) *Downloads {
	d := &Downloads{e: e, dm: e.Downloader()}
	d.body = newDlBody(e, d.dm)
	d.page = nav.NewPage(d.body, tabDownloads)
	d.Add(d.page)

	return d
}

// newDlBody makes the column from the files and the downloads under way.
func newDlBody(e *env.Env, dm env.DownloadManager) *dlBody {
	b := &dlBody{}
	items, jobs := dm.Items(), dm.Jobs()
	title := kit.H1(e.T("downloads.title"))
	if len(items) == 0 && len(jobs) == 0 {
		b.empty = true
		b.stack.items = []blockItem{
			{w: title, top: dlTitleTop, bottom: dlTitleBottomEmpty},
			{w: newEmptyBlock(e), top: emptyTop},
		}
		b.finish()

		return b
	}

	summary := kit.Muted(e.Fill("downloads.summaryTpl", "n", itoa(len(items)), "mb", format.MB(dm.TotalBytes())), dlSummarySize)
	summary.Name = "p.muted"
	card, storage := newStorageCard(e)
	b.storage = storage
	free, total, ok := dm.FreeSpace()
	storage.set(free, total, ok)
	b.stack.items = []blockItem{
		{w: title, top: dlTitleTop, bottom: dlTitleBottom},
		{w: summary},
		{w: card, top: dlCardMargin, bottom: dlCardMargin},
	}
	if len(jobs) > 0 {
		b.stack.items = append(b.stack.items, blockItem{
			w: heading(kit.Eyebrow(e.Fill("downloads.runningTpl", "n", itoa(len(jobs)))), 2), top: dlRunTop, bottom: dlRunBottom,
		})
		for _, j := range jobs {
			rc := newRunCard(e, j)
			b.running = append(b.running, rc)
			b.stack.items = append(b.stack.items, blockItem{w: rc, bottom: dlRunCardBottom})
		}
	}
	groups := env.Group(items)
	for gi, g := range groups {
		total := max(g.CourseLessons, len(g.Items))
		b.stack.items = append(b.stack.items, blockItem{
			w:   newGroupHead(e.Fill("downloads.groupTpl", "course", g.CourseName, "n", itoa(len(g.Items)), "total", itoa(total)), g.CourseName),
			top: dlGroupTop, bottom: dlGroupBottom,
		})
		for i, it := range g.Items {
			last := gi == len(groups)-1 && i == len(g.Items)-1 // `.row:last-child`
			b.stack.items = append(b.stack.items, blockItem{w: newFileRow(e, it, last)})
			b.files++
		}
	}
	b.finish()

	return b
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Tab returns the Downloads tab.
func (d *Downloads) Tab() int { return tabDownloads }

// Scroll returns the page's scroll.
func (d *Downloads) Scroll() *ui.Scroll { return d.page.Scroll() }

// Empty tells whether the screen shows screen 12.
func (d *Downloads) Empty() bool { return d.body.empty }

// Running returns how many cards of downloads under way the screen shows.
func (d *Downloads) Running() int { return len(d.body.running) }

// Files returns how many rows of files the screen shows.
func (d *Downloads) Files() int { return d.body.files }

// Mounted follows the downloads while the screen is on show, makes the lists again
// (they may have changed since the screen was built) and asks for the free space to
// be read again.
func (d *Downloads) Mounted(*ui.Context) {
	if d.unsub == nil {
		d.unsub = d.dm.Events().Subscribe(d.onEvent)
	}
	d.rebuild()
	d.dm.RefreshSpace()
}

// Unmounted stops following them.
func (d *Downloads) Unmounted(*ui.Context) {
	if d.unsub != nil {
		d.unsub()
		d.unsub = nil
	}
}

// Layout fills the window with the page.
func (d *Downloads) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(d.page, ui.Tight(cs.Max))
	c.Place(d.page, geom.Point{})

	return cs.Max
}

// Layout is the stack's.
func (b *dlBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	sz := c.Layout(&b.stack, cs)
	c.Place(&b.stack, geom.Point{})

	return cs.Constrain(sz)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// finish adds the parts to the column.
func (b *dlBody) finish() {
	for _, it := range b.stack.items {
		b.stack.Add(it.w)
	}
	b.Add(&b.stack)
}

// onEvent is what the screen does on an event: a progress moves one card, the space
// moves its card, anything else that changes the lists makes them again.
func (d *Downloads) onEvent(ev env.DownloadEvent) {
	switch ev.Kind {
	case env.DownloadEventProgress:
		for _, rc := range d.body.running {
			if rc.LessonID == ev.LessonID {
				rc.set(ev.Done, ev.Total, 0)

				break
			}
		}
	case env.DownloadEventSpace:
		if s := d.body.storage; s != nil {
			free, total, ok := d.dm.FreeSpace()
			s.set(free, total, ok)
		}
	case env.DownloadEventChanged, env.DownloadEventFinished, env.DownloadEventPaused, env.DownloadEventFailed:
		d.rebuild()
	case env.DownloadEventUnknown:
	default:
	}
}

// rebuild makes the lists again from the downloads, the scroll kept.
func (d *Downloads) rebuild() {
	d.body = newDlBody(d.e, d.dm)
	d.page.SetContent(d.body)
}
