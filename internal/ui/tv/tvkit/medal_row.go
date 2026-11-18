//go:build tvos

package tvkit

import (
	"time"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the row of badges of the settings card. The prototype draws
// three (`BADGES`, index.html:311) in `display:flex;justify-content:space-around;
// padding:0 30px`; the rest is the product owner's request of 2026-11-18 (K44):
// every badge must show, however many there are, so that those that do not fit
// at the prototype's size take turns in the room of three.
const (
	// MedalsPerPage is how many medals the row shows at the prototype's size, the
	// three of `BADGES`.
	MedalsPerPage = 3
	// MedalPageTime is how long a page of medals stays before the next one comes.
	MedalPageTime = 4 * time.Second
	// MedalFade is the cross-fade from a page to the next, in CSS `ease`.
	MedalFade = 400 * time.Millisecond
	// medalDot, medalDotGap, medalDotsTop are the page dots under the medals: 10
	// DIPs round, 12 apart, 18 below the captions.
	medalDot, medalDotGap, medalDotsTop float32 = 10, 12, 18
)

// MedalRow is the row of medals of the settings card, in the room the card gives
// it (the card's width less the padding of 30 on each side).
//
// Up to MedalsPerPage medals are one row of the prototype: each gets an equal
// share of the width, centred in it (`space-around`). With more, they are cut into
// pages of at most MedalsPerPage, balanced (ten are 3, 3, 2, 2), each page laid
// out as that row is at the same size; one page shows, and every MedalPageTime
// the next one cross-fades in over MedalFade, round and round, with a dot for each
// page under them. The pages are all as tall as the tallest medal of any, so that
// nothing under the row moves.
//
// All the medals stay in the tree, so a screen reader reads every badge whatever
// page shows; the ones off the page are laid out and not painted. The turns stop
// when the row leaves the screen. With motion reduced (Context.ReduceMotion) the
// pages still turn, but at once, with no fade.
type MedalRow struct {
	ui.Base

	medals []*Medal
	// starts holds the index of each page's first medal, then len(medals).
	starts []int
	page   int // the page showing
	prev   int // the page fading out, while fade runs
	fade   ui.Animated[float32]

	widths []float32 // the medals' widths while laying out, reused

	mounted bool
	timer   ui.Timer
	turn    func()

	// paintPrev, paintPage, c and p paint a page under a layer: made once, so that
	// a frame allocates nothing.
	paintPrev, paintPage func()
	c                    *ui.Context
	p                    *ui.Painter
}

var (
	_ ui.Mounter   = (*MedalRow)(nil)
	_ ui.Unmounter = (*MedalRow)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewMedalRow returns the row of the medals.
func NewMedalRow(medals []*Medal) *MedalRow {
	r := &MedalRow{medals: medals}
	r.fade.Ease, r.fade.Duration = theme.CSSEase, MedalFade
	r.turn = r.next
	r.paintPrev = func() { r.drawPage(r.c, r.p, r.prev) }
	r.paintPage = func() { r.drawPage(r.c, r.p, r.page) }
	for _, m := range medals {
		r.Add(m)
	}
	r.starts = pageStarts(len(medals), r.starts)

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Medal returns the i-th medal, on whatever page.
func (r *MedalRow) Medal(i int) *Medal { return r.medals[i] }

// Pages returns how many pages the medals are in: 1 when they all fit.
func (r *MedalRow) Pages() int { return max(len(r.starts)-1, 1) }

// Page returns the page showing, from 0.
func (r *MedalRow) Page() int { return r.page }

// PageOf returns the page the i-th medal is on.
func (r *MedalRow) PageOf(i int) int {
	for p := 1; p < len(r.starts); p++ {
		if i < r.starts[p] {
			return p - 1
		}
	}

	return 0
}

// Fading tells whether a page is cross-fading in.
func (r *MedalRow) Fading() bool { return r.fade.Running() }

// Mounted starts turning the pages when there are several.
func (r *MedalRow) Mounted(c *ui.Context) {
	r.mounted = true
	r.arm(c)
}

// Unmounted stops turning them.
func (r *MedalRow) Unmounted(*ui.Context) {
	r.mounted = false
	r.timer.Stop()
	r.timer = ui.Timer{}
}

// Layout lays every page out in the room: a page's medals share the width of
// MedalsPerPage, space-around, whatever number the page has. The row is as tall
// as the tallest medal, and a line of dots taller with more than one page.
func (r *MedalRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	n := len(r.medals)
	if n == 0 {
		return cs.Constrain(geom.Sz(w, 0))
	}
	// the room of a medal: an equal share of the row for one row of them, or of
	// MedalsPerPage when paged (a page of two keeps the cell of three)
	share := w / float32(min(n, MedalsPerPage))
	scale := min(share/MedalSize, 1)
	var h float32
	r.widths = r.widths[:0]
	for _, m := range r.medals {
		m.SetScale(scale)
		s := c.Layout(m, ui.Loose(geom.Sz(share, ui.Unbounded)))
		r.widths = append(r.widths, s.W)
		h = max(h, s.H)
	}
	for p := range r.Pages() {
		from, to := r.starts[p], r.starts[p+1]
		var sum float32
		for _, mw := range r.widths[from:to] {
			sum += mw
		}
		space := max(w-sum, 0) / float32(to-from)
		x := space / 2
		for i := from; i < to; i++ {
			c.Place(r.medals[i], geom.Pt(x, 0))
			x += r.widths[i] + space
		}
	}
	if r.Pages() > 1 {
		h += medalDotsTop + medalDot
	}
	r.arm(c)

	return cs.Constrain(geom.Sz(w, h))
}

// Paint draws the page showing, and the page leaving while it fades, with the
// dots under them.
func (r *MedalRow) Paint(c *ui.Context, p *ui.Painter) {
	if len(r.medals) == 0 {
		return
	}
	if r.fade.Running() {
		r.c, r.p = c, p
		p.Layer(1-r.fade.Value(), 0, r.paintPrev)
		p.Layer(r.fade.Value(), 0, r.paintPage)
		r.c, r.p = nil, nil
	} else {
		r.drawPage(c, p, r.page)
	}
	r.drawDots(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// arm starts the timer of the turns when the row is on screen, has several pages
// and none is waiting.
func (r *MedalRow) arm(c *ui.Context) {
	if r.mounted && r.Pages() > 1 && !r.timer.Pending() {
		r.timer = c.AfterFunc(MedalPageTime, r.turn)
	}
}

// next brings the next page, and arms the turn after it.
func (r *MedalRow) next() {
	r.timer = ui.Timer{}
	v := r.View()
	if v == nil || !r.mounted || r.Pages() < 2 {
		return
	}
	r.prev = r.page
	r.page = (r.page + 1) % r.Pages()
	r.fade.Set(0)
	r.fade.To(r, 1)
	r.Repaint()
	r.arm(v.Context())
}

// drawPage paints the medals of a page.
func (r *MedalRow) drawPage(c *ui.Context, p *ui.Painter, page int) {
	if page >= r.Pages() {
		return
	}
	for _, m := range r.medals[r.starts[page]:r.starts[page+1]] {
		p.Child(c, m)
	}
}

// drawDots paints a dot per page, the one showing in --pr, centred under the
// medals; none for a single page.
func (r *MedalRow) drawDots(c *ui.Context, p *ui.Painter) {
	pages := r.Pages()
	if pages < 2 {
		return
	}
	look := theme.From(c)
	b := r.Bounds()
	x := (b.W - (float32(pages)*medalDot + float32(pages-1)*medalDotGap)) / 2
	y := b.H - medalDot
	for i := range pages {
		ink := look.T.Text3
		if i == r.page {
			ink = look.T.Primary
		}
		p.Fill(geom.XYWH(x, y, medalDot, medalDot), scene.Solid(ink), medalDot/2)
		x += medalDot + medalDotGap
	}
}

// pageStarts returns, in buf, where the pages of n medals start: as few pages as
// hold them MedalsPerPage at a time, the medals shared among them as evenly as
// they go (the first pages the longer), then n.
func pageStarts(n int, buf []int) []int {
	buf = buf[:0]
	if n == 0 {
		return append(buf, 0)
	}
	pages := (n + MedalsPerPage - 1) / MedalsPerPage
	at := 0
	for p := range pages {
		buf = append(buf, at)
		at += n / pages
		if p < n%pages {
			at++
		}
	}

	return append(buf, n)
}
