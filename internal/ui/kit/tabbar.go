package kit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of .tabs (index.html:79-82, tabbar :324).
const (
	// TabCount is the number of tabs: TABS (:323).
	TabCount = 4
	// tabBarContent is the part of the bar's 84 above the home indicator:
	// 84 - 34 (Q-P2-1: the bar is `50 + max(34, SafeArea().Bottom)` tall).
	tabBarContent float32 = 50
	// tabInsetFloor is the home indicator's 34 the prototype draws (`.hi`,
	// :76-77): the bar never has less under its content.
	tabInsetFloor float32 = 34
	// tabBorder is `.tabs{border-top:1px solid var(--bd)}` (:79).
	tabBorder float32 = 1
	// tabPadTop, tabGap are `.tabs button{padding-top:8px;gap:5px}` (:80).
	tabPadTop, tabGap float32 = 8, 5
	// tabSize is `.tabs button{font-size:11px}`; its weight is 500, or 700 on
	// the current tab (:80, :82).
	tabSize float32 = 11
	// tabPillW, tabPillH, tabPillRadius are `.tabs button .pill{width:56px;
	// height:30px;border-radius:15px}` (:81).
	tabPillW, tabPillH, tabPillRadius float32 = 56, 30, 15
	// tabIcon is `ic(t[0],22,i===a?2.2:1.8)` (:324): 22 with a stroke of 2.2 on
	// the current tab and 1.8 on the others.
	tabIcon                   float32 = 22
	tabStrokeOn, tabStrokeOff float32 = 2.2, 1.8
)

// nameTabBar is the key layout.json gives the bar (`div.tabs`, :768).
const nameTabBar = "div.tabs"

// nameTabPill is the key of a tab's pill.
const nameTabPill = "span.pill"

// TabItem is one of TABS (index.html:323): an icon, a label and the key the
// prototype knows the button by (`go.home`, `go.courses`, `go.downloads`,
// `go.account`).
type TabItem struct {
	// Icon is the symbol in the pill.
	Icon icon.Name
	// Label is the text under it.
	Label string
	// Name is the layout.json key, `go.<route>`.
	Name string
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// TabBar is `.tabs` (index.html:79-82): the bottom bar of the four root screens,
// #1A1B1F with a 1 px --bd line on top and four equal buttons, each an icon in a
// 56 x 30 pill over a 11 px label, 8 from the top and 5 apart. The current tab is
// --pr and 700 with a pill of rgba(255,208,40,.16) and a 2.2 stroke; the others
// are --tx2 and 500 with a 1.8 stroke.
//
// The bar is `50 + max(34, SafeArea().Bottom)` tall, 84 where the prototype's
// is (Q-P2-1); its parent puts it at the bottom. Tapping a tab calls OnTap with
// its index, the current one too: the prototype goes to its root again.
type TabBar struct {
	ui.Base
	// Hush leaves the bar out of a screen reader's reading while Silent says so
	// (a sheet is open over it).
	Hush
	// Items are the four tabs.
	Items [TabCount]TabItem
	// Current is the index of the current tab; -1 for none.
	Current int
	// OnTap is told the index of the tab tapped.
	OnTap func(i int)

	tabs     [TabCount]tabButton
	attached bool
}

// tabButton is one `.tabs button`.
type tabButton struct {
	ui.Pressable

	bar   *TabBar
	index int
	pill  tabPill
	label Label
}

// tabPill is `.pill`: the 56 x 30 pill holding the icon, filled on the current
// tab.
type tabPill struct {
	ui.Base

	icon Icon
	on   bool
}

var (
	_ ui.Describer = (*tabButton)(nil)
	_ ui.Labeler   = (*tabButton)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewTabBar returns a bar of the items with tab current chosen.
func NewTabBar(items [TabCount]TabItem, current int, onTap func(i int)) *TabBar {
	return &TabBar{Items: items, Current: current, OnTap: onTap}
}

// SetCurrent chooses a tab; the same one changes nothing.
func (t *TabBar) SetCurrent(i int) {
	if i != t.Current {
		t.Current = i
		t.Relayout()
	}
}

// TabBarHeight returns how tall the bar is for a bottom safe area of inset DIPs.
func TabBarHeight(inset float32) float32 { return tabBarContent + max(tabInsetFloor, inset) }

// Layout takes the width it gets and the height of the rule, and puts four
// buttons side by side under the border.
func (t *TabBar) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	t.init()
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = 0
	}
	h := TabBarHeight(c.SafeArea().Bottom)
	size := cs.Constrain(geom.Sz(w, h))
	bw := size.W / TabCount
	for i := range t.tabs {
		b := &t.tabs[i]
		b.Name = t.Items[i].Name
		b.sync(i == t.Current, t.Items[i])
		c.Layout(b, ui.Tight(geom.Sz(bw, size.H-tabBorder)))
		c.Place(b, geom.Pt(float32(i)*bw, tabBorder))
	}

	return size
}

// Paint draws the bar's ground and the line on top, then the buttons.
func (t *TabBar) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	b := t.Bounds()
	p.Fill(b, scene.Solid(look.C.TabBar), 0)
	p.Fill(geom.XYWH(0, 0, b.W, tabBorder), scene.Solid(look.T.Border), 0)
	t.Base.Paint(c, p)
}

// AccessibleLabel returns the tab's text.
func (b *tabButton) AccessibleLabel() string { return b.label.Text }

// Describe describes the button as a tab named by its label, selected when
// current. It sets no Toggle: the iOS bridge turns a toggle into a "toggle
// button" that speaks 0 or 1 (P8.3d dump); a tab is a button that is selected.
func (b *tabButton) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label = platform.RoleTab, b.label.Text
	n.Actions |= platform.ActPress
	if b.bar.Current == b.index {
		n.Flags |= platform.FlagSelected
	}
}

// Activate tells the bar the tab was tapped.
func (b *tabButton) Activate() {
	if b.bar.OnTap != nil {
		b.bar.OnTap(b.index)
	}
}

// Layout puts the pill 8 from the top, centered, and the label 5 under it.
func (b *tabButton) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	c.Layout(&b.pill, ui.Tight(geom.Sz(tabPillW, tabPillH)))
	c.Place(&b.pill, geom.Pt((size.W-tabPillW)/2, tabPadTop))
	ls := c.Layout(&b.label, ui.Loose(geom.Sz(size.W, ui.Unbounded)))
	c.Place(&b.label, geom.Pt((size.W-ls.W)/2, tabPadTop+tabPillH+tabGap))

	return size
}

// Layout puts the icon in the middle of the pill.
func (p *tabPill) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(tabPillW, tabPillH))
	is := c.Layout(&p.icon, ui.Tight(geom.Sz(tabIcon, tabIcon)))
	c.Place(&p.icon, geom.Pt(centeredY(size.W, is.W), centeredY(size.H, is.H)))

	return size
}

// Paint draws the pill's fill on the current tab, then the icon.
func (p *tabPill) Paint(c *ui.Context, pt *ui.Painter) {
	if p.on {
		pt.Fill(p.Bounds(), scene.Solid(theme.From(c).C.PillOn), tabPillRadius)
	}
	p.Base.Paint(c, pt)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// init adds the buttons the first time.
func (t *TabBar) init() {
	if t.attached {
		return
	}
	t.attached = true
	t.Name = nameTabBar
	for i := range t.tabs {
		b := &t.tabs[i]
		b.bar, b.index = t, i
		b.Look = noLook
		b.pill.Name = nameTabPill
		b.pill.icon.Name = iconName
		b.pill.Add(&b.pill.icon)
		b.Add(&b.pill, &b.label)
		t.Add(b)
	}
}

// sync makes the button show the item, on or off.
func (b *tabButton) sync(on bool, it TabItem) {
	ink, weight, stroke := theme.InkText2, text.Weight(500), tabStrokeOff
	if on {
		ink, weight, stroke = theme.InkPrimary, text.Weight(700), tabStrokeOn
	}
	b.pill.on = on
	b.pill.icon.Glyph, b.pill.icon.Size, b.pill.icon.Stroke, b.pill.icon.Ink = it.Icon, tabIcon, stroke, ink
	labelStyle{Role: theme.RoleBody, Size: tabSize, Weight: weight, Ink: ink}.apply(&b.label)
	b.label.SetText(it.Label)
}
