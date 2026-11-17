//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/iptv_list.go at 49de3c2 (the page
// built once and filtered in place: the search field stays where it is and keeps
// the keyboard, the rows that do not pass are left out, the count and the empty text
// follow) and from jplayer-mobile/internal/ui/views/home.go at 49de3c2 (a column
// whose children each sit a number of DIPs under the one before). The column is
// MS.courses (index.html:374-380).

package screens

import (
	"strconv"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of MS.courses's column (index.html:374-380), in DIPs.
const (
	// coursesTop, coursesTitleSize, coursesTitleBottom are the title's
	// `font-size:30px;margin:16px 0 14px`.
	coursesTop, coursesTitleSize, coursesTitleBottom float32 = 16, 30, 14
	// coursesSearchTop is the search field's `margin-top:12px`.
	coursesSearchTop float32 = 12
	// coursesShownSize, coursesShownTop, coursesShownBottom are the count's
	// `font-size:12px;margin:14px 0 10px`.
	coursesShownSize, coursesShownTop, coursesShownBottom float32 = 12, 14, 10
	// coursesSyncTop is the sync line's room above it while the list is read (the
	// skeleton's `margin-top:30px`, loadSyncTop).
	coursesSyncTop = loadSyncTop
	// emptyPad, emptyIcon, emptyGap are `padding:60px 0`, `ic('search',40)` and the
	// `margin-top:12px` of the text under it.
	emptyPad, emptyIcon, emptyGap float32 = 60, 40, 12
	// emptySize is the empty text's size, the page's 16 px.
	emptySize float32 = 16
)

// The four filters, in the order of the buttons (`['Tất cả','Thường','Đặc biệt',
// 'Giới thiệu']`, index.html:376).
const (
	filterAll = iota
	filterRegular
	filterSpecial
	filterIntro
	filterCount
)

// filterKeys are the locale keys of the buttons' texts; their `data-v` is the same
// text, the buttons' names (`filter:Tất cả`).
var filterKeys = [filterCount]string{"filter.all", "filter.regular", "filter.special", "filter.intro"}

// filterCategory is the category each filter lets through; CategoryUnknown is all.
var filterCategory = [filterCount]model.Category{
	model.CategoryUnknown, model.CategoryRegular, model.CategorySpecial, model.CategoryIntro,
}

// coursesContent is what Courses scrolls: the title, the filter, the search field,
// the count and the cards, 14, 12, 14 and 10 DIPs apart as MS.courses's blocks are.
// It is made once and filtered in place: a key of the search toggles which cards
// show, it never builds a widget (R-P3-9). Until the list is read the count and the
// cards are left out and the sync line, or the retry button, is under the field.
type coursesContent struct {
	ui.Base

	e      *env.Env
	title  *kit.Label
	seg    *kit.Segmented
	input  *kit.Input
	shown  *kit.Label
	rows   []*courseRow
	empty  *emptyView
	sync   *syncLine
	retry  *kit.Button
	filter *courseFilter

	state     *env.CoursesState
	counts    [filterCount]int
	shownText []string // "Hiển thị n khoá học" for n = 0 ... len(rows), made when the list is read
	emptyFor  string   // the query the empty text was made for
	emptyMade bool
	ready     bool // the list is read
}

// emptyView is the text of a search that found nothing: `<div style="text-align:
// center;padding:60px 0" class="muted">` with the 40 px search icon and `<p style=
// "margin-top:12px">`.
type emptyView struct {
	ui.Base

	icon  *kit.Icon
	label *kit.Label
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newCoursesContent returns the column with no list yet: the title, the filter and
// the field as the state left them, the sync line under them.
func newCoursesContent(e *env.Env, st *env.CoursesState) *coursesContent {
	k := &coursesContent{e: e, state: st}
	k.title = kit.H1(e.T("courses.title"))
	k.title.Size = coursesTitleSize

	k.seg = kit.NewSegmented(e.T(filterKeys[0]), e.T(filterKeys[1]), e.T(filterKeys[2]), e.T(filterKeys[3]))
	k.seg.Keys = make([]string, filterCount)
	for i, key := range filterKeys {
		k.seg.Keys[i] = e.T(key)
	}
	k.seg.NamePrefix = "filter:" // `data-act="filter" data-v`
	k.seg.Selected = st.Filter
	k.seg.OnSelect = k.selectFilter

	k.input = kit.NewInput(icon.Search, e.T("courses.search"))
	k.input.Search = true
	k.input.EnterAction = platform.EnterSearch
	k.input.SetText(st.Query)
	k.input.OnChange = k.setQuery
	k.input.OnSubmit = func(string) { k.blur() }

	k.shown = kit.Muted("", coursesShownSize)
	k.shown.Name = "div.muted"
	k.shown.Hidden = true

	k.empty = newEmptyView()
	k.empty.Hidden = true

	k.sync = &syncLine{
		icon:  kit.NewIcon(icon.Ref, loadSyncIcon, theme.InkText2),
		label: kit.Muted(e.T("loading.sync"), emptySize),
	}
	k.sync.Name = "div.muted"
	k.sync.Add(k.sync.icon, k.sync.label)
	k.retry = kit.NewButton(kit.ButtonT, e.T("common.retry"), icon.NameUnknown)
	k.retry.Name = "retry"
	k.retry.Hidden = true

	k.Add(k.title, k.seg, k.input, k.shown, k.empty, k.sync, k.retry)

	return k
}

// newEmptyView returns the empty text with its icon.
func newEmptyView() *emptyView {
	v := &emptyView{
		icon:  kit.NewIcon(icon.Search, emptyIcon, theme.InkText2),
		label: kit.Muted("", emptySize),
	}
	v.label.Wrap, v.label.Align = true, ui.TextAlignCenter
	v.Name = "div.muted"
	v.Add(v.icon, v.label)

	return v
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetCourses shows the list: a card for each course (the first of an id when the
// pages overlap), the numbers on the filter's first button and the count from the
// answer's `counts`, not from what the filter lets through.
func (k *coursesContent) SetCourses(page model.CoursePage) {
	for _, r := range k.rows {
		k.Remove(r)
	}
	items := make([]model.Course, 0, len(page.Items))
	seen := make(map[string]struct{}, len(page.Items))
	for _, c := range page.Items {
		if _, dup := seen[c.ID]; dup {
			continue
		}
		seen[c.ID] = struct{}{}
		items = append(items, c)
	}
	k.rows = make([]*courseRow, len(items))
	for i, c := range items {
		id := c.ID
		k.rows[i] = newCourseRow(k.e, c, func() { k.e.Act.Detail(id) })
		k.Add(k.rows[i])
	}
	k.filter = newCourseFilter(items)

	n := page.Counts
	all := n.All
	if all == 0 {
		all = len(items)
	}
	k.counts = [filterCount]int{all, n.Regular, n.Special, n.Intro}
	k.seg.Options[0] = k.e.Fill("courses.allTpl", "n", strconv.Itoa(all)) // `'Tất cả '+MY_COUNT`
	k.relayoutSeg()

	k.shownText = make([]string, len(items)+1)
	for i := range k.shownText {
		k.shownText[i] = k.e.Fill("courses.shownTpl", "n", strconv.Itoa(i))
	}
	k.ready = true
	k.emptyMade = false
	kit.SetHidden(k.sync, true)
	kit.SetHidden(k.retry, true)
	kit.SetHidden(k.shown, false)
	k.refilter()
}

// SetError shows the retry button in place of the sync line while the list has not
// been read; onRetry is its action, nil puts the sync line back.
func (k *coursesContent) SetError(onRetry func()) {
	if k.ready {
		return
	}
	k.retry.OnPress = onRetry
	kit.SetHidden(k.retry, onRetry == nil)
	kit.SetHidden(k.sync, onRetry != nil)
	k.Relayout()
}

// Retry returns the retry button.
func (k *coursesContent) Retry() *kit.Button { return k.retry }

// Filter returns the index of the filter on.
func (k *coursesContent) Filter() int { return k.state.Filter }

// Query returns the text of the search field.
func (k *coursesContent) Query() string { return k.state.Query }

// Input returns the search field.
func (k *coursesContent) Input() *kit.Input { return k.input }

// Segmented returns the filter.
func (k *coursesContent) Segmented() *kit.Segmented { return k.seg }

// Rows returns how many cards the list has, and ShownRows how many of them show.
func (k *coursesContent) Rows() int { return len(k.rows) }

// ShownRows is how many cards pass the filter and the search now.
func (k *coursesContent) ShownRows() int {
	if k.filter == nil {
		return 0
	}

	return len(k.filter.shown)
}

// Row returns the card of the course at index i of the list.
func (k *coursesContent) Row(i int) ui.Widget { return k.rows[i] }

// Count returns the number the filter button i counts: the answer's `counts`.
func (k *coursesContent) Count(i int) int { return k.counts[i] }

// SetFilter chooses filter i as a tap on its button does.
func (k *coursesContent) SetFilter(i int) {
	if i < 0 || i >= filterCount {
		return
	}
	k.seg.Select(i)
	k.relayoutSeg()
	k.selectFilter(i)
}

// SetQuery puts text in the search field as the user's typing would.
func (k *coursesContent) SetQuery(s string) {
	k.input.SetText(s)
	k.setQuery(s)
}

// Layout puts the blocks in a column: each the room's width, the cards 12 apart.
func (k *coursesContent) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	col := ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}
	y := coursesTop
	put := func(wd ui.Widget) float32 {
		s := c.Layout(wd, col)
		c.Place(wd, geom.Pt(0, y))

		return s.H
	}
	y += put(k.title) + coursesTitleBottom
	y += put(k.seg) + coursesSearchTop
	y += put(k.input)
	if !k.ready {
		y += coursesSyncTop
		y += put(k.sync)
		y += put(k.retry)

		return cs.Constrain(geom.Sz(w, y))
	}
	y += coursesShownTop
	y += put(k.shown) + coursesShownBottom
	if k.ShownRows() == 0 {
		y += put(k.empty)

		return cs.Constrain(geom.Sz(w, y))
	}
	for _, i := range k.filter.shown {
		y += put(k.rows[i]) + rowMargin
	}

	return cs.Constrain(geom.Sz(w, y))
}

// Layout centres the icon over the text, 60 from the top and the bottom.
func (v *emptyView) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	is := c.Layout(v.icon, ui.Loose(geom.Sz(emptyIcon, emptyIcon)))
	st := strut(c)
	// the icon is an inline box on the baseline of a line of the 16 px strut: the line
	// is as tall as the icon and the strut's part under its baseline
	line := is.H + max(st.Height-st.Baseline, 0)
	c.Place(v.icon, geom.Pt((w-is.W)/2, emptyPad))
	ls := c.Layout(v.label, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(v.label, geom.Pt(0, emptyPad+line+emptyGap))

	return cs.Constrain(geom.Sz(w, emptyPad+line+emptyGap+ls.H+emptyPad))
}

// LayoutStyle is `div.muted`: the style of the text it holds.
func (v *emptyView) LayoutStyle(c *ui.Context) (kit.CSS, string) { return v.label.LayoutStyle(c) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// selectFilter is a tap on a filter button (`filter`, index.html:593): the
// category changes, the search text stays.
func (k *coursesContent) selectFilter(i int) {
	k.state.Filter = i
	k.refilter()
}

// setQuery is a key of the search (`qInp` `oninput`, index.html:563): the text is
// kept as it is, the cards and the count follow.
func (k *coursesContent) setQuery(s string) {
	k.state.Query = s
	k.refilter()
}

// refilter shows the cards that pass the category and the search, the count of
// them, and the empty text when none does. Nothing is built: the cards that go are
// Hidden, those that come are shown again. Once the list is read.
func (k *coursesContent) refilter() {
	if !k.ready {
		return
	}
	shown := k.filter.apply(filterCategory[k.state.Filter], k.state.Query)
	next := 0
	for i, r := range k.rows {
		hide := true
		if next < len(shown) && shown[next] == i {
			hide = false
			next++
		}
		kit.SetHidden(r, hide)
	}
	k.shown.SetText(k.shownText[len(shown)])
	kit.SetHidden(k.empty, len(shown) > 0)
	if len(shown) == 0 && (!k.emptyMade || k.emptyFor != k.state.Query) {
		// the one place a key allocates: the text of a query nothing matches (`esc(S.m.q)`,
		// not trimmed)
		k.empty.label.SetText(k.e.Fill("courses.emptyTpl", "q", k.state.Query))
		k.emptyFor, k.emptyMade = k.state.Query, true
	}
	k.Relayout()
}

// blur takes the focus away from the field, which hides the keyboard (Enter).
func (k *coursesContent) blur() {
	if v := k.View(); v != nil {
		v.Focus(nil)
	}
}

// relayoutSeg lays the filter buttons out again: Segmented.Relayout marks the control and
// its ancestors only, and a button whose constraints did not change is not laid out, so
// its text (the count of "Tất cả 15") and its look (the weight and ink of the chosen one)
// would stay as they were.
func (k *coursesContent) relayoutSeg() {
	k.seg.Relayout()
	for i := range filterCount {
		if b, ok := k.seg.Item(i).(interface{ Relayout() }); ok {
			b.Relayout()
		}
	}
}
