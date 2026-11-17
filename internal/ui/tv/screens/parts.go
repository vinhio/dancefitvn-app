//go:build tvos

// Adapted from dancefitvn-app/internal/ui/mobile/screens/detail.go (a screen that
// reads its data after it is mounted, one read at a time, and drops the answer
// when it is left) for the TV: a failed read goes to TV11 (K28, Q-P7-8) in place
// of the skeleton's retry button.

package screens

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Tab indexes of the nav bar: `tnav(a)` (index.html:643).
const (
	tabHome = iota
	tabCourses
	tabMusic
	tabSettings
)

// waitAnchor is the stand-in for a screen's default control while its data is
// read (K12: the waiting state is the background and the nav bar). The default
// focus rule (renderTV, index.html:723) finds the first control that is not in
// the nav bar; the real one is not built yet, so this one is, with the same key,
// and takes the focus in its place. A screen that gets its data while the anchor
// holds the focus moves the focus to the control it stands for; a focus that went
// to the nav bar stays there. It paints nothing.
type waitAnchor struct {
	tvkit.Item
}

// reader is the read a screen has in flight, one at a time. ui.Go gives no way to
// tell whether a read is running: for a widget outside a view it starts nothing
// and returns a cancel that does nothing, which is not told from a real one, so a
// screen that tested "cancel != nil" to avoid a second read never read again
// after a load that ran detached. reader keeps the truth itself: busy is set only
// when a read really started and is cleared when its answer comes or it is
// stopped.
type reader struct {
	cancel func() // the cancel of the read; nil when none runs
	busy   bool   // a read started and has neither answered nor been stopped
	done   bool   // a read answered without an error
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newWaitAnchor returns the stand-in of the control with key. A screen reader
// that lands on it (the focus is there while the data is read) hears "Đang tải".
func newWaitAnchor(e *env.Env, key string) *waitAnchor {
	a := &waitAnchor{}
	a.Key, a.Name, a.Item.Label = key, key, e.T("a11y.loading")

	return a
}

// newNavBar returns the nav bar of the root screens with item on lit; Select on
// an item switches the screen in place (`data-tgo` on `nav*`, :736).
func newNavBar(e *env.Env, on int) *tvkit.Nav {
	n := tvkit.NewNav(on, e.T("splash.brand"), [tvkit.NavItems]string{
		e.T("tabs.home"), e.T("tv.nav.courses"), e.T("tv.nav.music"), e.T("tv.nav.settings"),
	})
	n.OnSelect = func(i int) {
		if e.Act != nil {
			e.Act.SwitchTab(i)
		}
	}

	return n
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout is a point: the anchor has no size to paint.
func (a *waitAnchor) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Sz(1, 1))
}

// Paint draws nothing.
func (a *waitAnchor) Paint(*ui.Context, *ui.Painter) {}

// Describe describes the anchor as a status that takes no press: it holds the
// focus for the control it stands for, which is not there yet.
func (a *waitAnchor) Describe(c *ui.Context, n *platform.SemanticNode) {
	a.Item.Describe(c, n)
	n.Role, n.Actions = platform.RoleStatus, 0
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// Busy tells whether a read is in flight.
func (r *reader) Busy() bool { return r.busy }

// Done tells whether a read has answered without an error.
func (r *reader) Done() bool { return r.done }

// Stop ends the read in flight, if any: its answer is never delivered.
func (r *reader) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.cancel, r.busy = nil, false
}

// read starts work for w unless one is in flight, and hands its answer to then
// on the UI thread, with r no longer busy. It tells whether it started: it does
// not for a widget outside a view (nothing would run), and a later call, when the
// widget is mounted again, starts it then.
func read[T any](r *reader, w ui.Widget, work func(ctx context.Context) (T, error), then func(c *ui.Context, v T, err error)) bool {
	if r.busy || w.Node().View() == nil {
		return false
	}
	r.busy = true
	cancel := ui.Go(w, work, func(c *ui.Context, v T, err error) {
		r.cancel, r.busy, r.done = nil, false, err == nil
		then(c, v, err)
	})
	if r.busy { // a host that is no Poster has answered by now
		r.cancel = cancel
	}

	return true
}

// loadAvatar reads the signed-in user's picture for the nav bar's `.tav` (the
// default person shows until it arrives, or when there is none); the result is
// dropped when w leaves (r.Stop).
func loadAvatar(w ui.Widget, r *reader, e *env.Env, n *tvkit.Nav) {
	u, ok := e.SessionUser()
	if !ok || u.AvatarURL == "" || r.Done() {
		return
	}
	url := u.AvatarURL
	read(r, w, func(ctx context.Context) (*scene.Image, error) {
		return e.Avatar(ctx, url)
	}, func(_ *ui.Context, img *scene.Image, err error) {
		if err == nil && img != nil {
			n.Avatar().SetImage(img)
		}
	})
}

// failed is the answer of a read that failed, for every screen of this file: a
// canceled read (the screen was left) shows nothing; any other failure shows TV11
// over the screen, whose "Thử lại" reads it again by going back to it (K28).
func failed(e *env.Env, err error) {
	if err == nil || errors.Is(err, context.Canceled) || e.Act == nil {
		return
	}
	e.Act.Go(nav.Route{Screen: env.RouteError}, false)
}

// focusFrom moves the focus to w when the anchor holds it: the data came while
// nobody moved. A focus that went to the nav bar stays there.
func focusFrom(c *ui.Context, a *waitAnchor, w ui.Widget) {
	if a.IsFocused() {
		c.Focus(w)
	}
}

// pad2 writes n as two digits, as `String(n).padStart(2,'0')` does; a larger
// number is as long as it needs.
func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}

	return strconv.Itoa(n)
}

// upper is `toUpperCase()` of the prototype.
func upper(s string) string { return strings.ToUpper(s) }

// categoryInk picks the color of a course's category line: `--info` for the
// special ones, `--ok` for the introductions, #FF6B6E for the rest (index.html:672).
func categoryInk(look *theme.Look, c model.Category) color.Color {
	switch c {
	case model.CategorySpecial:
		return look.C.CatSpecial
	case model.CategoryIntro:
		return look.C.CatIntro
	case model.CategoryRegular, model.CategoryUnknown:
		return look.C.CatRegular
	default:
		return look.C.CatRegular
	}
}

// playLesson is `playLes(k)` of the prototype (index.html:739): offline, TV11 over
// the screen; else the lesson plays as a video and TV07 opens over this screen,
// the up-next screen and the player taking each other's place. The player screen
// has the focus on `p1`, the first control it holds.
func playLesson(e *env.Env, l player.Lesson, list []player.Lesson) {
	if e.Act == nil {
		return
	}
	if !e.Online() {
		e.Act.Go(nav.Route{Screen: env.RouteError}, false)

		return
	}
	if pb := e.Playback(); pb != nil {
		pb.PlayVideo(l, list)
	}
	if s := e.Stack.Screen(); s == env.RoutePlayer || s == env.RouteUpNext {
		e.Act.Replace(nav.Route{Screen: env.RoutePlayer})

		return
	}
	e.Act.Go(nav.Route{Screen: env.RoutePlayer}, false)
}

// spoken is a button's text as a screen reader says it: "‹ Bài trước" and "Bài
// sau ›" of the music screen (`$P:699`) without the chevrons, which it would read
// out as characters.
func spoken(s string) string { return strings.TrimSpace(strings.Trim(s, "‹› ")) }

// itoa writes n in decimal.
func itoa(n int) string { return strconv.Itoa(n) }

// bareScreen is a screen with one control that does nothing: what a route is when
// the services lack what it needs (no player), so that Menu still goes back.
type bareScreen struct {
	ui.Base
}

// newBareScreen returns the screen whose only control is the one of key.
func newBareScreen(e *env.Env, key string) *bareScreen {
	b := &bareScreen{}
	b.Add(newWaitAnchor(e, key))

	return b
}

// Layout fills the room; the control is a point in the middle.
func (b *bareScreen) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	for w := range b.Children() {
		c.Layout(w, ui.Loose(geom.Sz(1, 1)))
		c.Place(w, geom.Pt(size.W/2, size.H/2))
	}

	return size
}

// clockRow is a row of two times, the position at the left and the length at the
// right (`display:flex;justify-content:space-between`): the music screen's `div.mono.
// muted`. It is as wide as it is given and as tall as the taller time.
type clockRow struct {
	ui.Base

	pos, dur *kit.Label
}

// newClockRow returns a row named name over the two labels.
func newClockRow(name string, pos, dur *kit.Label) *clockRow {
	r := &clockRow{pos: pos, dur: dur}
	r.Name = name
	r.Add(pos, dur)

	return r
}

// Layout puts the position at the left and the length at the right.
func (r *clockRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	ps := c.Layout(r.pos, free)
	ds := c.Layout(r.dur, free)
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = ps.W + ds.W
	}
	h := max(ps.H, ds.H)
	c.Place(r.pos, geom.Pt(0, (h-ps.H)/2))
	c.Place(r.dur, geom.Pt(w-ds.W, (h-ds.H)/2))

	return cs.Constrain(geom.Sz(w, h))
}

// LayoutStyle is the row's `div`: the style of its text and the two times.
func (r *clockRow) LayoutStyle(c *ui.Context) (kit.CSS, string) {
	s, _ := r.pos.LayoutStyle(c)

	return s, r.pos.Text + "\n" + r.dur.Text
}
