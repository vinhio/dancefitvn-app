//go:build tvos

package tvkit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

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

// The numbers of `.trow` and its parts (index.html:192-196).
const (
	// rowHeight, rowPadX, rowGap, rowRadius are `.trow{gap:26px;height:88px;
	// padding:0 30px;border-radius:22px}`.
	rowHeight, rowPadX, rowGap, rowRadius float32 = 88, 30, 26, 22
	// rowNoSize, rowNoBorder, rowNoFont are `.trow .no{width:56px;height:56px;
	// border:3px solid var(--bd);font-weight:800;font-size:24px}`.
	rowNoSize, rowNoBorder, rowNoFont float32 = 56, 3, 24
	// rowTitleSize, rowSubSize are `.tt b{font-size:30px;font-weight:700}` and
	// `.tt span{font-size:22px;opacity:.7}`.
	rowTitleSize, rowSubSize float32 = 30, 22
	// rowSubOpacity is `.tt span{opacity:.7}`.
	rowSubOpacity float32 = .7
	// rowIconSize, rowIconStroke are the lesson's `<svg style="width:32px">` with
	// `stroke-width="2.4"` (:678).
	rowIconSize, rowIconStroke float32 = 32, 2.4
	// rowDropY, rowDropBlur, rowDropAlpha are `#tv .trow.focus{box-shadow:
	// 0 20px 40px rgba(0,0,0,.5)}` (:193).
	rowDropY, rowDropBlur, rowDropAlpha float32 = 20, 40, .5
	// rowNoWeight, rowTitleWeight are the 800 and 700 of the rules above.
	rowNoWeight, rowTitleWeight text.Weight = 800, 700
)

// The keys layout.json knows the parts of a row by.
const (
	nameRowNo    = "span.no"
	nameRowTexts = "span.tt"
	nameRowTitle = "b"
)

// Row is the lesson row `dl*` of TS.detail (`.f.trow`, index.html:678): the round
// number, the title over its line of tag, length and "Đã xem", and the triangle.
// At rest it is --sf; focused it is --pr with --on text, the shadow of 0 20 40
// rgba(0,0,0,.5), scaled to 1.02, and the number turns --red with white text.
//
// Set Key to the `data-fid` (it is also the Name layout.json knows the row by)
// and OnSelect to the `data-tact`. A Raise container paints the focused row over
// the next one (F-12).
type Row struct {
	Item
	// No is the number in the circle, "01".
	No string
	// Title is the bold line; Sub the line under it.
	Title, Sub string

	num   rowNo
	texts rowTexts
	glyph kit.Icon
	body  func(c *ui.Context, p *ui.Painter) // paintBody as a method value, made once
}

// rowNo is `span.no`: the circle and its number.
type rowNo struct {
	ui.Base
	row   *Row
	label kit.Label
}

// rowTexts is `span.tt`: the title and the line under it, one over the other.
type rowTexts struct {
	ui.Base
	title, sub kit.Label
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewRow returns a lesson row of key with the number, the title and the line.
func NewRow(key, no, title, sub string) *Row {
	r := &Row{No: no, Title: title, Sub: sub}
	r.Key, r.Name, r.Item.Label, r.Detail = key, key, title, sub
	r.FocusScale = RowFocusScale
	r.num.row = r
	r.num.Name = nameRowNo
	r.num.label.Role, r.num.label.Size, r.num.label.Weight = theme.RoleBody, rowNoFont, rowNoWeight
	r.num.label.Text = no
	r.num.Add(&r.num.label)
	r.texts.Name = nameRowTexts
	r.texts.title.Name = nameRowTitle
	r.texts.title.Role, r.texts.title.Size, r.texts.title.Weight = theme.RoleBody, rowTitleSize, rowTitleWeight
	r.texts.title.Text, r.texts.title.Ellipsis = title, true
	r.texts.sub.Role, r.texts.sub.Size = theme.RoleBody, rowSubSize
	r.texts.sub.Text, r.texts.sub.Ellipsis = sub, true
	r.texts.Add(&r.texts.title, &r.texts.sub)
	r.glyph.Glyph, r.glyph.Size, r.glyph.Stroke = icon.Tri, rowIconSize, rowIconStroke
	r.Add(&r.num, &r.texts, &r.glyph)

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetSub changes the line under the title; the same text changes nothing.
func (r *Row) SetSub(s string) {
	if s != r.Sub {
		r.Sub, r.Detail = s, s
		r.texts.sub.SetText(s)
		r.Relayout()
	}
}

// Describe describes the row as a button named by its number and title ("03 ·
// Stink and Dutty"), valued by the line under the title (tag, length, "Đã xem").
func (r *Row) Describe(c *ui.Context, n *platform.SemanticNode) {
	r.Item.Describe(c, n)
	n.Label = rowName(r.No, r.Title)
}

// Layout is the width it gets and 88: the number, the texts that take what is
// left, and the triangle at the right, all centered on the height.
func (r *Row) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(cs.Max.W, rowHeight))
	ns := c.Layout(&r.num, ui.Tight(geom.Sz(rowNoSize, rowNoSize)))
	is := c.Layout(&r.glyph, ui.Tight(geom.Sz(rowIconSize, rowIconSize)))
	room := max(size.W-2*rowPadX-ns.W-is.W-2*rowGap, 0) // `.tt{flex:1}`
	ts := c.Layout(&r.texts, ui.Constraints{Min: geom.Sz(room, 0), Max: geom.Sz(room, ui.Unbounded)})
	x := rowPadX
	c.Place(&r.num, geom.Pt(x, (size.H-ns.H)/2))
	x += ns.W + rowGap
	c.Place(&r.texts, geom.Pt(x, (size.H-ts.H)/2))
	x += room + rowGap
	c.Place(&r.glyph, geom.Pt(x, (size.H-is.H)/2))

	return size
}

// Paint draws the row at its focus scale.
func (r *Row) Paint(c *ui.Context, p *ui.Painter) {
	if r.body == nil {
		r.body = r.paintBody
	}
	r.Item.Scaled(c, p, r.body)
}

// LayoutStyle is `div.f.trow`: the fill and the radius, and the text of the
// three lines it holds.
func (r *Row) LayoutStyle(c *ui.Context) (kit.CSS, string) {
	look := theme.From(c)

	return kit.CSS{Background: kit.CSSColor(r.fill(look)), Radius: "22px", Color: kit.CSSColor(r.ink().Of(look))},
		r.No + "\n" + r.Title + "\n" + r.Sub
}

// Layout lays the two texts one over the other, each as wide as the room.
func (t *rowTexts) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	free := ui.Constraints{Max: geom.Sz(cs.Max.W, ui.Unbounded)}
	block := ui.Constraints{Min: geom.Sz(cs.Max.W, 0), Max: geom.Sz(cs.Max.W, ui.Unbounded)} // `.tt b{display:block}`
	bs := c.Layout(&t.title, block)
	ss := c.Layout(&t.sub, free)
	c.Place(&t.title, geom.Point{})
	c.Place(&t.sub, geom.Pt(0, bs.H))

	return cs.Constrain(geom.Sz(cs.Max.W, bs.H+ss.H))
}

// Layout centers the number in the circle.
func (n *rowNo) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	ls := c.Layout(&n.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	size := cs.Constrain(geom.Sz(rowNoSize, rowNoSize))
	c.Place(&n.label, geom.Pt((size.W-ls.W)/2, (size.H-ls.H)/2))

	return size
}

// LayoutStyle is `span.no`: the number's style, the circle's fill.
func (n *rowNo) LayoutStyle(c *ui.Context) (kit.CSS, string) {
	look := theme.From(c)
	s, txt := n.label.LayoutStyle(c)
	s.Color = kit.CSSColor(n.ink().Of(look))
	s.Background = "rgba(0, 0, 0, 0)"
	if n.row.IsFocused() {
		s.Background = kit.CSSColor(look.T.Red)
	}
	s.Radius = "50%"

	return s, txt
}

// Paint draws the circle: its border and, focused, its fill, then the number.
func (n *rowNo) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	r := n.Bounds()
	edge := look.T.Border // `border:3px solid var(--bd)`
	if n.row.IsFocused() {
		edge = look.T.Red // `#tv .trow.focus .no{background:var(--red);border-color:var(--red)}`
		p.Fill(r, scene.Solid(look.T.Red), r.W/2)
	}
	p.Border(r, geom.Insets{Top: rowNoBorder, Right: rowNoBorder, Bottom: rowNoBorder, Left: rowNoBorder}, edge, r.W/2)
	n.label.Ink = n.ink()
	n.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// rowName is "03 · Stink and Dutty", the number and the title of a lesson as a
// screen reader says them (the lesson row of the phone reads the same).
func rowName(no, title string) string {
	if no == "" {
		return title
	}

	return no + " · " + title
}

// paintBody draws the shadow, the fill and what the row holds.
func (r *Row) paintBody(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	b := r.Bounds()
	if blend := r.Blend(); blend > 0 {
		p.Shadow(b, rowRadius, ui.Shadow{
			Offset: geom.Pt(0, rowDropY*blend), Blur: rowDropBlur * blend, Color: color.RGBA(0, 0, 0, rowDropAlpha*blend),
		})
	}
	p.Fill(b, scene.Solid(r.fill(look)), rowRadius)
	ink := r.ink()
	r.texts.title.Ink = ink
	r.texts.sub.Color = ink.Of(look).WithAlpha(rowSubOpacity) // `opacity:.7` on the span
	r.glyph.Ink = ink
	r.Base.Paint(c, p)
}

// fill is --pr focused, --sf at rest.
func (r *Row) fill(look *theme.Look) color.Color {
	if r.IsFocused() {
		return look.T.Primary
	}

	return look.T.Surface
}

// ink is the color of the text and the triangle: --on focused, --tx at rest.
func (r *Row) ink() theme.Ink {
	if r.IsFocused() {
		return theme.InkOnPrimary
	}

	return theme.InkText
}

// ink is the number's color: #fff on the red of the focused row, else the row's.
func (n *rowNo) ink() theme.Ink {
	if n.row.IsFocused() {
		return theme.InkWhite
	}

	return theme.InkText
}
