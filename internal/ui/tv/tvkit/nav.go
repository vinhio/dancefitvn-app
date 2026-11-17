//go:build tvos

package tvkit

import (
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/focus"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of `tnav(a)` (index.html:643) and its rules (:183-188).
const (
	// NavItems is the number of items of `.tnav`: Trang chủ, Khoá học, Nhạc
	// tập, Cài đặt.
	NavItems = 4

	// logoX, logoY are `.tlogo{left:90px;top:56px}` (:186).
	logoX, logoY float32 = 90, 56
	// logoSize, logoSpacing, logoWeight are `.tlogo{font-size:40px;
	// letter-spacing:1px;font-weight:800}` in --disp.
	logoSize, logoSpacing float32     = 40, 1
	logoWeight            text.Weight = 800

	// barTop, barGap, barPad, barRadius are `.tnav{top:34px;gap:12px;
	// padding:8px;border-radius:40px}` (:183).
	barTop, barGap, barPad, barRadius float32 = 34, 12, 8, 40
	// itemPadV, itemPadH, itemRadius, itemSize are `.tnav .f{padding:12px 32px;
	// border-radius:30px;font-size:28px}` (:184).
	itemPadV, itemPadH, itemRadius, itemSize float32 = 12, 32, 30, 28
	// itemWeight, itemOnWeight are `font-weight:500` and `.f.on{font-weight:700}`.
	itemWeight, itemOnWeight text.Weight = 500, 700

	// avatarRight, avatarTop, avatarSize are `.tav{right:90px;top:42px;
	// width:60px;height:60px}` (:187).
	avatarRight, avatarTop, avatarSize float32 = 90, 42, 60
)

// Names layout.json gives the parts of the bar.
const (
	nameLogo = "div.tlogo"
	nameBar  = "div.tnav"
	nameAv   = "div.tav"
)

// barBg is `.tnav{background:rgba(255,255,255,.06)}` (:183).
var barBg = color.RGBA(255, 255, 255, .06)

// Nav is the navigation of the four root screens: `.tlogo`, `.tnav` and `.tav`
// of tnav(a) (index.html:643). It takes all the room its parent gives and
// places the three parts as the prototype's absolute positions do, the bar in
// the middle of the width, the logo at the left and the avatar at the right.
//
// The items are `nav0`..`nav3`, focusable, and the bar is a focus.Chrome: the
// default focus skips them (renderTV, :723) unless a key names one. Item i is
// `.on` when it is On; the shell sets OnSelect to switch the screen in place.
type Nav struct {
	ui.Base
	// On is the index of the item that is on, the screen showing; -1 for none.
	On int
	// OnSelect runs when item i is selected.
	OnSelect func(i int)

	logo kit.Label
	bar  navBar
	av   *kit.Avatar
}

// navBar is `div.tnav`: the pill that holds the four items.
type navBar struct {
	ui.Base
	items [NavItems]NavItem
}

// NavItem is one `.f` of `.tnav`.
type NavItem struct {
	Item
	label kit.Label
	nav   *Nav
	index int
	body  func(c *ui.Context, p *ui.Painter) // paintBody as a method value, made once
}

var (
	_ focus.Chrome = (*navBar)(nil)
	_ ui.Labeler   = (*NavItem)(nil)
	_ ui.Describer = (*NavItem)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewNav returns the navigation with the brand and the four labels, item on
// being the one that is on.
func NewNav(on int, brand string, labels [NavItems]string) *Nav {
	n := &Nav{On: on}
	n.logo.Name = nameLogo
	n.logo.Text = brand
	n.bar.Name = nameBar
	for i := range n.bar.items {
		it := &n.bar.items[i]
		it.nav, it.index = n, i
		it.Key = NavKey(i)
		it.Name, it.Item.Label = it.Key, labels[i]
		it.label.Text = labels[i]
		it.FocusScale = NavFocusScale
		it.OnSelect = it.selected
		it.Add(&it.label)
		n.bar.Add(it)
	}
	n.av = kit.NewAvatar(avatarSize, color.Color{})
	n.av.Name = nameAv
	n.Add(&n.logo, &n.bar, n.av)

	return n
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NavKey is the `data-fid` of nav item i: "nav0".."nav3".
func NavKey(i int) string { return "nav" + string(rune('0'+i)) }

// SetOn changes the item that is on, laying the bar out again (the item that is
// on is bold, hence wider).
func (n *Nav) SetOn(i int) {
	if n.On == i {
		return
	}
	old := n.On
	n.On = i
	// an item's layout is cached: the two whose weight changes lay out again
	for _, k := range [...]int{old, i} {
		if it := n.Item(k); it != nil {
			it.Relayout()
		}
	}
}

// Item returns item i, nil out of range.
func (n *Nav) Item(i int) *NavItem {
	if i < 0 || i >= NavItems {
		return nil
	}

	return &n.bar.items[i]
}

// Avatar returns the picture at the right.
func (n *Nav) Avatar() *kit.Avatar { return n.av }

// Layout takes the room it is given and places the logo, the bar and the
// avatar.
func (n *Nav) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	look := theme.From(c)
	n.logo.Role, n.logo.Weight, n.logo.Spacing, n.logo.Ink = theme.RoleDisplay, logoWeight, logoSpacing, theme.InkPrimary
	if n.logo.Size != logoSize {
		n.logo.Size = logoSize
		n.logo.Relayout()
	}
	c.Layout(&n.logo, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&n.logo, geom.Pt(logoX, logoY))
	bs := c.Layout(&n.bar, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&n.bar, geom.Pt((size.W-bs.W)/2, barTop)) // left:50%;translateX(-50%)
	n.av.Ring = look.T.Primary
	as := c.Layout(n.av, ui.Tight(geom.Sz(avatarSize, avatarSize)))
	c.Place(n.av, geom.Pt(size.W-avatarRight-as.W, avatarTop))

	return size
}

// FocusChrome tells that the bar is the navigation: its items are left out of
// the default focus.
func (b *navBar) FocusChrome() focus.ChromeKind { return focus.ChromeSidebar }

// Layout puts the items in a row, 12 apart, with 8 around.
func (b *navBar) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	x := barPad
	h := float32(0)
	for i := range b.items {
		s := c.Layout(&b.items[i], ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
		c.Place(&b.items[i], geom.Pt(x, barPad))
		x += s.W + barGap
		h = max(h, s.H)
	}

	return cs.Constrain(geom.Sz(x-barGap+barPad, h+2*barPad))
}

// Paint draws the pill, then the items; the focused one is painted last.
func (b *navBar) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(b.Bounds(), scene.Solid(barBg), barRadius)
	PaintRaised(c, p, &b.Base)
}

// AccessibleLabel returns the item's text.
func (it *NavItem) AccessibleLabel() string { return it.label.Text }

// Describe describes the item as a tab of the bar, named by its text and
// selected while its screen shows (`.tnav .f.on`). Adapted from
// jplayer-tv/internal/ui/kit/navitem.go at 781bee5 (NavItem.Describe).
func (it *NavItem) Describe(c *ui.Context, n *platform.SemanticNode) {
	it.Item.Describe(c, n)
	n.Role = platform.RoleTab
	if it.IsOn() {
		n.Flags |= platform.FlagSelected
	}
}

// IsOn tells whether the item is the one that is on.
func (it *NavItem) IsOn() bool { return it.nav != nil && it.nav.On == it.index }

// Layout sizes the item around its text with 12 and 32 of padding.
func (it *NavItem) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := itemWeight
	if it.IsOn() {
		w = itemOnWeight
	}
	it.label.Role = theme.RoleBody
	if it.label.Size != itemSize || it.label.Weight != w {
		it.label.Size, it.label.Weight = itemSize, w
		it.label.Relayout()
	}
	ts := c.Layout(&it.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&it.label, geom.Pt(itemPadH, itemPadV))

	return cs.Constrain(geom.Sz(ts.W+2*itemPadH, ts.H+2*itemPadV))
}

// Paint draws the item at its focus scale.
func (it *NavItem) Paint(c *ui.Context, p *ui.Painter) {
	if it.body == nil {
		it.body = it.paintBody
	}
	it.Item.Scaled(c, p, it.body)
}

// LayoutStyle is `div.f` of the bar: the text style, the fill and the radius.
func (it *NavItem) LayoutStyle(c *ui.Context) (kit.CSS, string) {
	look := theme.From(c)
	s, txt := it.label.LayoutStyle(c)
	s.Color = kit.CSSColor(it.ink().Of(look))
	s.Background = kit.CSSColor(it.fill(look))
	if it.fill(look).A == 0 {
		s.Background = "rgba(0, 0, 0, 0)"
	}
	s.Radius = "30px"

	return s, txt
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// selected is the item's OnSelect: it tells the Nav.
func (it *NavItem) selected() {
	if it.nav != nil && it.nav.OnSelect != nil {
		it.nav.OnSelect(it.index)
	}
}

// paintBody draws the fill and the text; `#tv .tnav .f.focus` has no shadow.
func (it *NavItem) paintBody(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	if f := it.fill(look); f.A > 0 {
		p.Fill(it.Bounds(), scene.Solid(f), itemRadius)
	}
	it.label.Ink = it.ink()
	it.Base.Paint(c, p)
}

// fill is the background: --pr focused (`#tv .tnav .f.focus`, :185), --sf2 for
// the item that is on (`.tnav .f.on`, :184), none otherwise.
func (it *NavItem) fill(look *theme.Look) color.Color {
	switch {
	case it.IsFocused():
		return look.T.Primary
	case it.IsOn():
		return look.T.Surface2
	default:
		return color.Color{}
	}
}

// ink is the text color: --on focused, --tx for the item that is on, --tx2
// otherwise.
func (it *NavItem) ink() theme.Ink {
	switch {
	case it.IsFocused():
		return theme.InkOnPrimary
	case it.IsOn():
		return theme.InkText
	default:
		return theme.InkText2
	}
}
