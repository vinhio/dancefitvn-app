//go:build tvos

// Adapted from dancefitvn-app/internal/app/tvwelcome.go (P0: the column of
// "DANCEFIT", "VIỆT NAM" and the two buttons, which matched the capture's
// positions) and completed with the glow, the three posters, the paragraph and
// the hint of TS.welcome. jplayer-tv has no welcome or pairing screen: the
// button, the focus and the toast it stands on are the kit of P7.1.

package screens

import (
	"time"

	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of TS.welcome (index.html:645-650).
const (
	// welcomeLeft, welcomeTop are `left:130px;top:300px` of the text block.
	welcomeLeft, welcomeTop float32 = 130, 300
	// welcomeBrandSize, welcomeBrandSpacing are `.d.acc` at `font-size:150px;
	// font-weight:800;letter-spacing:3px`.
	welcomeBrandSize, welcomeBrandSpacing float32 = 150, 3
	// welcomeCountrySize, welcomeCountrySpacing, welcomeCountryTop are
	// `font-size:40px;font-weight:700;letter-spacing:22px;margin-top:20px`.
	welcomeCountrySize, welcomeCountrySpacing, welcomeCountryTop float32 = 40, 22, 20
	// welcomeBlurbSize, welcomeBlurbLine, welcomeBlurbTop are `p.muted` at
	// `font-size:30px;margin-top:50px;line-height:1.5`.
	welcomeBlurbSize, welcomeBlurbLine, welcomeBlurbTop float32 = 30, 1.5, 50
	// welcomeRowTop, welcomeRowGap are `display:flex;gap:30px;margin-top:50px`.
	welcomeRowTop, welcomeRowGap float32 = 50, 30
	// welcomeHintLeft, welcomeHintBottom, welcomeHintSize are `left:130px;
	// bottom:70px;font-size:24px` of the hint.
	welcomeHintLeft, welcomeHintBottom, welcomeHintSize float32 = 130, 70, 24
)

// welcomeGuard is how long after it appears the screen takes no Select on its buttons
// (not in the prototype, whose click cannot repeat): the settings' "Đăng xuất khỏi
// Apple TV" lands here at once, on "Đăng nhập", under the same press.
const welcomeGuard = 450 * time.Millisecond

// welcomePoster is one of the three covers on the right: its place and size,
// its radius and title size (`poster(id, w, h, {r, ts})`).
type welcomePoster struct {
	x, y, w, h, radius, ts float32
	id, title              string
}

// welcomePosters are `poster('set35',780,440,{r:28,ts:72})` at 1040,150,
// `poster('set34',375,300,{r:24,ts:46})` at 1040,620 and
// `poster('hotextra',375,300,{r:24,ts:40})` at 1445,620 (:646). They are art of
// the palette, drawn with the course ids of the API and the short names the
// prototype's `c.short` gives (Q-P7-17): there is no API before sign-in.
var welcomePosters = [...]welcomePoster{
	{1040, 150, 780, 440, 28, 72, "set-35", "SET 35"},
	{1040, 620, 375, 300, 24, 46, "set-34", "SET 34"},
	{1445, 620, 375, 300, 24, 40, "hot-extra", "HOT EXTRA"},
}

// The keys layout.json knows the parts by: `div.d.acc`, `p.muted` and
// `div.muted` are the capture's; the country line has no class, so the capture
// does not record it.
const (
	nameWelcomeBrand   = "div.d.acc"
	nameWelcomeCountry = "tv.welcome.country"
	nameWelcomeBlurb   = "p.muted"
	nameWelcomeHint    = "div.muted"
)

// Welcome is TS.welcome (TV01): the red glow, the three covers, "DANCEFIT" over
// "VIỆT NAM", the paragraph, the buttons `w1` (Đăng nhập, to TV02) and `w2`
// (Tìm hiểu thêm, a toast) and the hint. `w1` has the focus first. Menu is not
// used here: it is left to the system (Home).
type Welcome struct {
	ui.Base

	e *env.Env

	bg      tvkit.Backdrop
	posters [len(welcomePosters)]*poster.Poster
	brand   *kit.Label
	country *kit.Label
	blurb   *kit.Label
	hint    *kit.Label
	login   *tvkit.Button
	more    *tvkit.Button

	// armed tells that "Đăng nhập" may be selected: it is not while the screen is
	// new (welcomeGuard), so that the Select that signed the TV out (a repeated or
	// late press of it) does not go on to the code screen.
	armed bool
	guard ui.Timer
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewWelcome returns TV01.
func NewWelcome(e *env.Env) *Welcome {
	w := &Welcome{e: e}
	// radial-gradient(ellipse at 25% 50%, rgba(229,56,59,.4), transparent 55%), var(--bg) (:645)
	w.bg.Layers = []kit.Radial{{Shape: kit.RadialEllipse, CX: .25, CY: .5, Color: tokens.Dark.Red.WithAlpha(.4), End: .55}}
	w.Add(&w.bg)
	for i, p := range welcomePosters {
		w.posters[i] = poster.New(poster.Spec{ID: p.id, Title: p.title, Radius: p.radius, TitleSize: p.ts}, p.w, p.h)
		w.Add(w.posters[i])
	}
	w.brand = &kit.Label{
		Text: e.T("splash.brand"), Role: theme.RoleDisplay, Size: welcomeBrandSize, Weight: 800,
		Spacing: welcomeBrandSpacing, LineHeight: 1, Upper: true, Ink: theme.InkPrimary,
	}
	w.brand.Name = nameWelcomeBrand
	w.brand.Heading = 1 // the title of the screen, `div.d.acc`
	w.country = &kit.Label{
		Text: e.T("splash.country"), Size: welcomeCountrySize, Weight: 700, Spacing: welcomeCountrySpacing, Ink: theme.InkText,
	}
	w.country.Name = nameWelcomeCountry
	w.blurb = &kit.Label{
		Text: e.T("tv.welcome.tagline") + "\n" + e.T("tv.welcome.membersOnly"), // the <br> of :648; the second line is K30
		Size: welcomeBlurbSize, LineHeight: welcomeBlurbLine, Ink: theme.InkText2,
	}
	w.blurb.Name = nameWelcomeBlurb
	w.login = tvkit.NewButton("w1", e.T("tv.welcome.login"), icon.User)
	w.login.OnSelect = w.onLogin
	w.more = tvkit.NewButton("w2", e.T("tv.welcome.more"), icon.NameUnknown)
	w.more.OnSelect = w.onMore
	w.hint = &kit.Label{Text: e.T("tv.welcome.hint"), Size: welcomeHintSize, Ink: theme.InkText2}
	w.hint.Name = nameWelcomeHint
	w.Add(w.brand, w.country, w.blurb, w.login, w.more, w.hint)

	return w
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Mounted starts the guard of the first Select.
func (w *Welcome) Mounted(c *ui.Context) {
	w.armed = false
	w.guard.Stop()
	w.guard = c.AfterFunc(welcomeGuard, func() { w.armed = true })
}

// Unmounted ends the guard.
func (w *Welcome) Unmounted(*ui.Context) { w.guard.Stop() }

// Item returns the control of the key (`w1`, `w2`), nil when there is none.
func (w *Welcome) Item(key string) *tvkit.Button {
	switch key {
	case w.login.Key:
		return w.login
	case w.more.Key:
		return w.more
	default:
		return nil
	}
}

// Layout places the glow and the covers, then the block as CSS flows it: the
// block is as wide as its widest line (it is absolutely positioned, so it
// shrinks to fit), and the brand and the paragraph are blocks of that width.
func (w *Welcome) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	c.Layout(&w.bg, ui.Tight(size))
	c.Place(&w.bg, geom.Point{})
	for i, p := range welcomePosters {
		c.Layout(w.posters[i], ui.Tight(geom.Sz(p.w, p.h)))
		c.Place(w.posters[i], geom.Pt(p.x, p.y))
	}

	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	bs, cn, ps := c.Layout(w.brand, free), c.Layout(w.country, free), c.Layout(w.blurb, free)
	ls, ms := c.Layout(w.login, free), c.Layout(w.more, free)
	width := max(bs.W, cn.W, ps.W, ls.W+welcomeRowGap+ms.W)
	block := ui.Constraints{Min: geom.Sz(width, 0), Max: geom.Sz(ui.Unbounded, ui.Unbounded)}
	bs, ps = c.Layout(w.brand, block), c.Layout(w.blurb, block)

	y := welcomeTop
	c.Place(w.brand, geom.Pt(welcomeLeft, y))
	y += bs.H + welcomeCountryTop
	c.Place(w.country, geom.Pt(welcomeLeft, y))
	y += cn.H + welcomeBlurbTop
	c.Place(w.blurb, geom.Pt(welcomeLeft, y))
	y += ps.H + welcomeRowTop
	c.Place(w.login, geom.Pt(welcomeLeft, y))
	c.Place(w.more, geom.Pt(welcomeLeft+ls.W+welcomeRowGap, y))

	hs := c.Layout(w.hint, free)
	c.Place(w.hint, geom.Pt(welcomeHintLeft, size.H-welcomeHintBottom-hs.H))

	return size
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onLogin is `data-tgo="code"` of `w1`: TV02 over this screen.
func (w *Welcome) onLogin() {
	if !w.armed {
		return
	}
	if w.e.Act != nil {
		w.e.Act.Go(nav.Route{Screen: env.RouteCode}, false)
	}
}

// onMore is `data-tact="toast"` of `w2`.
func (w *Welcome) onMore() {
	if w.e.Act != nil {
		w.e.Act.Toast(w.e.T("tv.welcome.moreToast"))
	}
}
