//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/home.go at 49de3c2 (the body made
// from the data and put in place when the data is read, a Refresher that keeps the
// page's scroll) and from jplayer-mobile/internal/ui/views/form_link.go at 49de3c2
// (one request at a time: the answer comes back through ui.Go and a stale one is
// dropped). The screen is MS.detail and, for a course the user has not joined,
// MS.locked (index.html:394-412) over GET /courses/{id}.

package screens

import (
	"context"
	"errors"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Detail is the screen of a course, with no tab bar: the detail of one the user is in
// (screen 05), the locked course of one they are not (05B). It shows a skeleton with
// the back button until GET /courses/{id} answers; a failure leaves the skeleton with
// "Thử lại" (Q-P3-18). While the network is down the shell puts the course's offline
// screen in its place (03 section 2), so this screen has nothing to do for it.
type Detail struct {
	ui.Base

	e  *env.Env
	id string

	frame   *scrFrame
	loading *detailLoading
	content *detailContent
	locked  *lockedContent
	cancel  func()
	dlUnsub func() // the downloads' events, for the rows and the count
}

var (
	_ nav.Scroller = (*Detail)(nil)
	_ ui.Mounter   = (*Detail)(nil)
	_ ui.Unmounter = (*Detail)(nil)
)

// detailLoading is what shows while the course is read: the cover's place as a block
// of --sf2, the back button over it and, 30 under it, the sync line, or the retry
// button once the read failed.
type detailLoading struct {
	ui.Base

	block skeleton
	back  *kit.RoundButton
	sync  *syncLine
	retry *kit.Button
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewDetail returns the screen of the course id.
func NewDetail(e *env.Env, courseID string) *Detail {
	d := &Detail{e: e, id: courseID}
	d.loading = newDetailLoading(e)
	d.setFrame(newFrame(d.loading, detailBottom, nil))

	return d
}

// newDetailLoading returns the skeleton: the --sf2 block of the hero's height, the
// back button, the sync line and the retry button, which is hidden.
func newDetailLoading(e *env.Env) *detailLoading {
	l := &detailLoading{block: skeleton{h: heroH}, back: newBackButton(e)}
	l.sync = &syncLine{
		icon:  kit.NewIcon(icon.Ref, loadSyncIcon, theme.InkText2),
		label: kit.Muted(e.T("loading.sync"), emptySize),
	}
	l.sync.Name = "div.muted"
	l.sync.Add(l.sync.icon, l.sync.label)
	l.retry = kit.NewButton(kit.ButtonT, e.T("common.retry"), icon.NameUnknown)
	l.retry.Name = "retry"
	l.retry.Hidden = true
	l.Add(&l.block, l.back, l.sync, l.retry)

	return l
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Scroll returns the scroll of what shows.
func (d *Detail) Scroll() *ui.Scroll { return d.frame.Scroll() }

// Content returns the detail's column, nil while the skeleton shows or for a locked
// course.
func (d *Detail) Content() *detailContent { return d.content }

// Locked returns the locked course's column, nil unless the course is one.
func (d *Detail) Locked() *lockedContent { return d.locked }

// Retry returns the skeleton's retry button.
func (d *Detail) Retry() *kit.Button {
	if d.loading == nil {
		return nil
	}

	return d.loading.retry
}

// Mounted starts reading the course with the next turn: a view that is being mounted
// has no host yet, and ui.Go would run the read on the UI thread then.
func (d *Detail) Mounted(c *ui.Context) {
	if d.dlUnsub == nil {
		d.dlUnsub = d.e.Downloader().Events().Subscribe(d.onDownload)
	}
	c.AfterFunc(0, d.load)
}

// onDownload follows the downloads while the screen is on show: a progress moves
// the ring of its row; any other change makes the rows and the count of "audio đã
// tải" show the files as they are, in place.
func (d *Detail) onDownload(ev env.DownloadEvent) {
	if d.content == nil {
		return
	}
	if ev.Kind == env.DownloadEventProgress {
		if ev.Total > 0 {
			d.content.progress(ev.LessonID, float32(ev.Done)/float32(ev.Total))
		}

		return
	}
	d.content.syncDownloads()
}

// Unmounted ends the read in flight and stops following the downloads.
func (d *Detail) Unmounted(*ui.Context) {
	if d.dlUnsub != nil {
		d.dlUnsub()
		d.dlUnsub = nil
	}
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
}

// Layout fills the window with the frame.
func (d *Detail) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(d.frame, ui.Tight(cs.Max))
	c.Place(d.frame, geom.Point{})

	return cs.Max
}

// Layout puts the block over the hero's room, the back button over it, the sync line
// or the retry button 30 under.
func (l *detailLoading) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	c.Layout(&l.block, ui.Tight(geom.Sz(w, heroH)))
	c.Place(&l.block, geom.Point{})
	c.Layout(l.back, ui.Loose(cs.Max))
	c.Place(l.back, geom.Pt(heroSide, nav.ContentTop(c)))
	col := ui.Constraints{Min: geom.Sz(max(w-2*detailSide, 0), 0), Max: geom.Sz(max(w-2*detailSide, 0), ui.Unbounded)}
	y := heroH + loadSyncTop
	ss := c.Layout(l.sync, col)
	c.Place(l.sync, geom.Pt(detailSide, y))
	rs := c.Layout(l.retry, col)
	c.Place(l.retry, geom.Pt(detailSide, y))

	return cs.Constrain(geom.Sz(w, y+max(ss.H, rs.H)))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// setFrame shows f in place of the frame there is.
func (d *Detail) setFrame(f *scrFrame) {
	if d.frame != nil {
		d.Remove(d.frame)
	}
	d.frame = f
	d.Add(f)
	d.Relayout()
}

// load reads GET /courses/{id}, one read at a time: a read in flight is dropped for it.
func (d *Detail) load() {
	if d.cancel != nil {
		d.cancel()
	}
	d.cancel = ui.Go(d, func(ctx context.Context) (model.Course, error) {
		return d.e.Course(ctx, d.id)
	}, d.loaded)
}

// loaded takes the answer: the course on success; the retry on the skeleton when the
// read fails. A canceled read, the screen left or superseded, shows nothing.
func (d *Detail) loaded(_ *ui.Context, c model.Course, err error) {
	d.cancel = nil
	switch {
	case err == nil:
		d.show(c)
	case errors.Is(err, context.Canceled):
	case d.e.Refused(err): // 426: the upgrade screen (03 section 9)
	case d.loading != nil:
		d.loading.retry.OnPress = d.retry
		kit.SetHidden(d.loading.retry, false)
		kit.SetHidden(d.loading.sync, true)
		d.loading.Relayout()
	default: // the read failed with no skeleton on screen: nothing to retry
	}
}

// retry is the "Thử lại": the sync line comes back and the read again.
func (d *Detail) retry() {
	kit.SetHidden(d.loading.retry, true)
	kit.SetHidden(d.loading.sync, false)
	d.loading.Relayout()
	d.load()
}

// show puts the screen of the course c in place of the skeleton: the detail of a
// course the user is in, the locked course's screen of one they are not.
func (d *Detail) show(c model.Course) {
	var f *scrFrame
	if c.Enrolled {
		f, d.content = newDetailFrame(d.e, c)
	} else {
		f, d.locked = newLockedFrame(d.e, c)
	}
	d.loading = nil
	d.setFrame(f)
}
