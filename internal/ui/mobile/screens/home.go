//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/home.go at 49de3c2 (the body made
// from the data and replaced when the data is read again, a Refresher that keeps
// the page's scroll) and from jplayer-mobile/internal/ui/views/form_link.go at
// 49de3c2 (one request at a time: the answer comes back through ui.Go and a stale
// one is dropped). The screen is MS.home (index.html:360-369) over GET /me/home.

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

// homeTab is the tab Home is the root of: `tabbar(0)`.
const homeTab = 0

// Home is MS.home, screen 03, with its loading state (03B, MS.loading) in it: the
// skeleton shows from the start (F-8) until GET /me/home answers; a failure leaves
// the skeleton with "Thử lại" (Q-P3-18). While the network is down the content
// shows under the offline bar, from the client's cache (F-7); when it comes back
// the data is read again and the content replaced in place, the scroll kept.
type Home struct {
	ui.Base

	e       *env.Env
	loading *Loading
	frame   *nav.Page
	content *homeContent
	bar     *kit.OfflineBar

	online  bool
	ready   bool // the content is on show
	cancel  func()
	again   reread // the automatic reads after a first read that failed
	dlUnsub func() // the downloads' events, for the "Audio offline" number
}

var (
	_ nav.Tabbed          = (*Home)(nil)
	_ nav.Scroller        = (*Home)(nil)
	_ env.OnlineAware     = (*Home)(nil)
	_ env.ForegroundAware = (*Home)(nil)
	_ ui.Mounter          = (*Home)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewHome returns the Home screen (MS.home, tab 0): the loading state, which gives
// way to the content when the data is in.
func NewHome(e *env.Env) *Home {
	h := &Home{e: e, loading: NewLoading(e), online: e.Online()}
	h.Add(h.loading)

	return h
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Tab returns the Home tab.
func (h *Home) Tab() int { return homeTab }

// Scroll returns the scroll of what shows: the content's, or the skeleton's.
func (h *Home) Scroll() *ui.Scroll {
	if h.ready {
		return h.frame.Scroll()
	}

	return h.loading.Scroll()
}

// Content returns the column on show, nil while the skeleton is.
func (h *Home) Content() *homeContent { return h.content }

// Mounted starts reading the data with the next turn: a view that is being mounted
// has no host yet, and ui.Go would run the read on the UI thread then.
func (h *Home) Mounted(c *ui.Context) {
	if h.dlUnsub == nil {
		h.dlUnsub = h.e.Downloader().Events().Subscribe(h.onDownload)
	}
	c.AfterFunc(0, h.load)
}

// onDownload follows the downloads: the number of "Audio offline" changes in place
// when a file is in or gone; a progress changes nothing Home shows.
func (h *Home) onDownload(ev env.DownloadEvent) {
	if h.content == nil || ev.Kind == env.DownloadEventProgress {
		return
	}
	h.content.syncOffline(h.e.Downloads().DoneCount())
}

// Unmounted ends the read in flight (ui.Go cancels it with the widget; this makes
// it explicit for the retry's closure).
func (h *Home) Unmounted(*ui.Context) {
	if h.dlUnsub != nil {
		h.dlUnsub()
		h.dlUnsub = nil
	}
	h.again.stop()
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
}

// Foregrounded is the app coming back to the front: while the content is not on
// show the read is made again at once, so that a read the system cut while the app
// was away (a browser in between) neither stays as the skeleton nor says "Thử lại".
func (h *Home) Foregrounded() {
	if h.ready {
		return
	}
	h.again.reset()
	h.load()
}

// SetOnline is the network going or coming back (03 section 2): the offline bar
// shows or goes, the page under it moving 54 to 94 and back; when the network
// returns the data is read again.
func (h *Home) SetOnline(online bool) {
	h.online = online
	h.applyBar()
	if online {
		h.again.reset()
		h.load()
	}
}

// Layout fills the window with what shows.
func (h *Home) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	var w ui.Widget = h.loading
	if h.ready {
		w = h.frame
	}
	c.Layout(w, ui.Tight(cs.Max))
	c.Place(w, geom.Point{})

	return cs.Max
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// load reads GET /me/home, one read at a time: a read in flight is dropped for it.
func (h *Home) load() {
	if h.cancel != nil {
		h.cancel()
	}
	h.cancel = ui.Go(h, func(ctx context.Context) (model.Home, error) {
		return h.e.Home(ctx)
	}, h.loaded)
}

// loaded takes the answer: the content on success; the skeleton's retry when the
// first read fails; what is on show stays when a later one fails (the client has
// already told the app it is offline). A canceled read, the screen left or
// superseded, shows nothing.
func (h *Home) loaded(c *ui.Context, d model.Home, err error) {
	h.cancel = nil
	switch {
	case err == nil:
		h.again.reset()
		h.show(d)
	case errors.Is(err, context.Canceled):
	case h.e.Refused(err): // 426: the upgrade screen (03 section 9)
	case h.ready:
	default:
		h.loading.Content().SetError(h.retry)
		h.again.schedule(c, h.load) // and the screen reads once more by itself
	}
}

// retry is the skeleton's "Thử lại": the sync line comes back and the read again.
func (h *Home) retry() {
	h.loading.Content().SetError(nil)
	h.again.reset()
	h.load()
}

// show puts the content made from d on the screen, the skeleton gone, the scroll
// where it was.
func (h *Home) show(d model.Home) {
	h.content = newHomeContent(h.e, d, h.e.Now())
	if h.frame == nil {
		h.frame = nav.NewPageFlush(h.content, homeTab) // the strip spans the gutters (`margin:0 -20px`)
		h.Add(h.frame)
	} else {
		h.frame.SetContent(h.content)
	}
	h.ready = true
	h.applyBar()
	if h.loading != nil {
		h.Remove(h.loading)
		h.loading = nil
	}
	h.Relayout()
}

// applyBar shows the offline bar while there is no network, over the content.
func (h *Home) applyBar() {
	if h.frame == nil {
		return
	}
	if h.online {
		h.frame.SetBar(nil)

		return
	}
	if h.bar == nil {
		h.bar = kit.NewOfflineBar(h.e.T("offline.bar"))
	}
	h.frame.SetBar(h.bar)
}
