//go:build !tvos

package screens

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of MS.loading (index.html:372), in DIPs.
const (
	// The three skeleton blocks `background:var(--sf2);border-radius:8px;
	// height:H`: `14px;width:190px;margin-top:20px`, `28px;width:240px;
	// margin-top:10px`, `232px;margin-top:22px;border-radius:20px`.
	skelRadius                    float32 = 8
	skelTop1, skelW1, skelH1      float32 = 20, 190, 14
	skelTop2, skelW2, skelH2      float32 = 10, 240, 28
	skelTop3, skelH3, skelRadius3 float32 = 22, 232, 20
	// The three cards `.card` with `flex:1;height:72px` in a row `gap:8px;
	// margin-top:16px`.
	loadCardH, loadCardGap, loadCardTop float32 = 72, 8, 16
	// loadSyncTop is the `margin-top:30px` of the muted line, loadSyncIcon its
	// `ic('ref',16)`, and loadSyncGap the space after the icon (Inter's space at
	// 16 px).
	loadSyncTop, loadSyncIcon, loadSyncGap float32 = 30, 16, 4.5
	// The placeholders of Home's course grid (K39), under the sync line: two rows
	// of two `.card` blocks, `homeSecTop` below the line, `homeGridGap` apart, each
	// as tall as a tile of a 393 wide phone (poster 109 + 10 + 17 + 6 + 20 + 12).
	loadGridRows, loadGridCols         = 2, 2
	loadTileH                  float32 = 174
)

// loadingTab is the tab of the loading screen: `tabbar(0)`, Home.
const loadingTab = 0

// LoadingContent is what MS.loading puts in the page: the skeleton of the home
// and, under it, the line "Đang đồng bộ khoá học…". Home shows it while the
// answer of GET /me/home is on its way (F-8); with SetError the line gives
// place to the retry button (Q-P3-18).
type LoadingContent struct {
	ui.Base

	col   *ui.Flex
	sync  *syncLine
	retry *kit.Button
	grid  *ui.Flex // the placeholders of the course grid; hidden by an error
}

// Loading is MS.loading as a screen: LoadingContent in a page with the tab bar
// of Home. Home's own screen replaces it when the data is there.
type Loading struct {
	ui.Base

	page    *nav.Page
	content *LoadingContent
}

// skeleton is a block of `--sf2` of a fixed height, as wide as its width or the
// room (a block-level `<div>`).
type skeleton struct {
	ui.Base

	w, h, radius float32
}

// syncLine is `<div class="muted" style="text-align:center">ic('ref',16) text
// </div>`: the icon, a space and the text, centred, on the baseline of the body
// strut.
type syncLine struct {
	ui.Base

	icon  *kit.Icon
	label *kit.Label
}

var (
	_ nav.Tabbed   = (*Loading)(nil)
	_ nav.Scroller = (*Loading)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewLoadingContent returns the skeleton with its sync line.
func NewLoadingContent(e *env.Env) *LoadingContent {
	l := &LoadingContent{}
	cards := ui.Row(loadCardGap)
	for range 3 {
		c := kit.NewCard(nil)
		c.Height, c.Grow = loadCardH, 1
		cards.Add(c)
	}
	l.sync = &syncLine{
		icon:  kit.NewIcon(icon.Ref, loadSyncIcon, theme.InkText2),
		label: kit.Muted(e.T("loading.sync"), 16),
	}
	l.sync.Name = "div.muted"
	l.sync.Add(l.sync.icon, l.sync.label)
	l.retry = kit.NewButton(kit.ButtonT, e.T("common.retry"), icon.NameUnknown)
	l.retry.Name = "retry"
	l.retry.Hidden = true
	l.grid = ui.Column(homeGridGap)
	for range loadGridRows {
		row := ui.Row(homeGridGap)
		for range loadGridCols {
			c := kit.NewCard(nil)
			c.Height, c.Grow = loadTileH, 1
			row.Add(c)
		}
		l.grid.Add(row)
	}
	l.col = ui.Column(0,
		&ui.Box{Height: skelTop1},
		&skeleton{w: skelW1, h: skelH1, radius: skelRadius},
		&ui.Box{Height: skelTop2},
		&skeleton{w: skelW2, h: skelH2, radius: skelRadius},
		&ui.Box{Height: skelTop3},
		&skeleton{h: skelH3, radius: skelRadius3},
		&ui.Box{Height: loadCardTop},
		cards,
		&ui.Box{Height: loadSyncTop},
		l.sync,
		l.retry,
		&ui.Box{Height: homeSecTop},
		l.grid,
	)
	l.Add(l.col)

	return l
}

// NewLoading returns the loading screen: the content in the page of the Home
// tab.
func NewLoading(e *env.Env) *Loading {
	l := &Loading{content: NewLoadingContent(e)}
	l.page = nav.NewPage(l.content, loadingTab)
	l.Add(l.page)

	return l
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetError shows the retry button in place of the sync line; onRetry is its
// action. A nil onRetry puts the sync line back.
func (l *LoadingContent) SetError(onRetry func()) {
	l.retry.OnPress = onRetry
	kit.SetHidden(l.retry, onRetry == nil)
	kit.SetHidden(l.sync, onRetry != nil)
	kit.SetHidden(l.grid, onRetry != nil)
	l.col.Relayout() // the column's cached layout does not know the two flags
}

// Retry returns the retry button, shown after SetError.
func (l *LoadingContent) Retry() *kit.Button { return l.retry }

// Layout puts the column in the room.
func (l *LoadingContent) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	sz := c.Layout(l.col, ui.Constraints{Max: geom.Sz(w, ui.Unbounded)})
	c.Place(l.col, geom.Point{})

	return cs.Constrain(geom.Sz(w, sz.H))
}

// Content returns the loading content, for a screen that shows it in a page of
// its own.
func (l *Loading) Content() *LoadingContent { return l.content }

// Tab returns the Home tab.
func (l *Loading) Tab() int { return loadingTab }

// Scroll returns the page's scroll.
func (l *Loading) Scroll() *ui.Scroll { return l.page.Scroll() }

// Layout fills the window with the page.
func (l *Loading) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(l.page, ui.Tight(cs.Max))
	c.Place(l.page, geom.Point{})

	return cs.Max
}

// Layout takes the width asked for, or the room's, and the height.
func (s *skeleton) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	w := s.w
	if w <= 0 {
		w = cs.Max.W
	}

	return cs.Constrain(geom.Sz(w, s.h))
}

// Paint draws the block in --sf2.
func (s *skeleton) Paint(c *ui.Context, p *ui.Painter) {
	r := s.Bounds()
	p.Fill(r, scene.Solid(theme.From(c).T.Surface2), min(s.radius, r.H/2))
}

// LayoutStyle is `div.muted`: the style of the text it holds.
func (s *syncLine) LayoutStyle(c *ui.Context) (kit.CSS, string) { return s.label.LayoutStyle(c) }

// Layout centres the icon and the text, 16 and a space apart, in the room's
// width; the icon stands on the strut's baseline, the text on the same.
func (s *syncLine) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	is := c.Layout(s.icon, ui.Loose(geom.Sz(loadSyncIcon, loadSyncIcon)))
	ls := c.Layout(s.label, ui.Loose(geom.Sz(max(w-is.W-loadSyncGap, 0), ui.Unbounded)))
	st := strut(c)
	lb := s.label.Baseline()
	asc := max(st.Baseline, lb, is.H)
	desc := max(st.Height-st.Baseline, ls.H-lb)
	x := (w - is.W - loadSyncGap - ls.W) / 2
	c.Place(s.icon, geom.Pt(x, asc-is.H))
	c.Place(s.label, geom.Pt(x+is.W+loadSyncGap, asc-lb))

	return cs.Constrain(geom.Sz(w, asc+desc))
}
