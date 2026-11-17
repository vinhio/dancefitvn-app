//go:build tvos

package tvkit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the player's next-lesson row `pn*` (index.html:688), written
// inline over `.trow`.
const (
	// NextRowHeight is `style="height:110px;..."`.
	NextRowHeight float32 = 110
	// nextNoSize, nextNoWeight are the number's `font-family:var(--disp);
	// font-size:40px;font-weight:800`.
	nextNoSize   float32 = 40
	nextNoWeight         = 800
	// nextTitleSize, nextTitleMaxW are `b` at `font-size:28px;max-width:330px`
	// with `white-space:nowrap;overflow:hidden;text-overflow:ellipsis`.
	nextTitleSize, nextTitleMaxW float32 = 28, 330
	// nextSubSize is the span's `font-size:22px` (its opacity .7 is the rule's).
	nextSubSize float32 = 22
)

// The numbers of the settings row `s*` (index.html:711), written inline over
// `.trow`.
const (
	// SettingRowHeight is `style="height:96px;margin-bottom:20px"`.
	SettingRowHeight float32 = 96
	// settingIcon is `ic(r[0],36)`.
	settingIcon float32 = 36
	// settingValueSize, settingValueOpacity are the value's `<span style=
	// "font-size:28px;opacity:.7">`.
	settingValueSize, settingValueOpacity float32 = 28, .7
)

// NextRow is the player's `.f.trow` of the lessons that follow (`pn*`,
// index.html:688): 110 tall, the number in Barlow 40/800 --pr, the title (28/700
// on one line, 330 wide at most) over its line of tag and length. The prototype's
// inline background (--sf) and number color (--pr) beat the focus rule, so a
// focused row keeps both and only its text turns --on (F-9, PrototypeInlineFocus);
// it is scaled to 1.02 with the shadow of 0 20 40 .5. A Raise container paints it
// over the next row.
type NextRow struct {
	Item
	// No is the number, "04"; Title the title; Sub the line under it.
	No, Title, Sub string

	num   kit.Label
	texts nextTexts
	body  func(c *ui.Context, p *ui.Painter) // paintBody as a method value, made once
}

// nextTexts is `span.tt` of a next row: the title, capped at 330, over its line.
type nextTexts struct {
	ui.Base
	title, sub kit.Label
}

// SettingRow is the settings' `.f.trow` (`s*`, index.html:711): 96 tall, the icon
// at 36, the title (30/700) taking the room and the value (28 at .7) at the right.
// Focused it is --pr with --on, scaled to 1.02 with the row's shadow. A Danger row
// has `color:var(--dg)` inline, which the focus rule does not beat: it keeps its
// red text and icon on --pr (F-9, PrototypeInlineFocus).
type SettingRow struct {
	Item
	// Title is the bold text; Value the text at the right; Icon the symbol.
	Title, Value string
	Icon         icon.Name
	// Danger is the logout row's red.
	Danger bool

	glyph kit.Icon
	texts settingTexts
	value kit.Label
	body  func(c *ui.Context, p *ui.Painter) // paintBody as a method value, made once
}

// settingTexts is `span.tt` of a setting row: the title only.
type settingTexts struct {
	ui.Base
	title kit.Label
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewNextRow returns the row of key with the number, the title and the line.
func NewNextRow(key, no, title, sub string) *NextRow {
	r := &NextRow{No: no, Title: title, Sub: sub}
	r.Key, r.Name, r.Item.Label, r.Detail = key, key, title, sub
	r.FocusScale = RowFocusScale
	r.num.Role, r.num.Size, r.num.Weight, r.num.Text = theme.RoleDisplay, nextNoSize, nextNoWeight, no
	r.texts.Name = nameRowTexts
	r.texts.title.Name = nameRowTitle
	r.texts.title.Role, r.texts.title.Size, r.texts.title.Weight = theme.RoleBody, nextTitleSize, rowTitleWeight
	r.texts.title.Text, r.texts.title.Ellipsis = title, true
	r.texts.sub.Role, r.texts.sub.Size, r.texts.sub.Text, r.texts.sub.Ellipsis = theme.RoleBody, nextSubSize, sub, true
	r.texts.Add(&r.texts.title, &r.texts.sub)
	r.Add(&r.num, &r.texts)

	return r
}

// NewSettingRow returns the row of key with the icon, the title and the value.
func NewSettingRow(key string, icn icon.Name, title, value string) *SettingRow {
	r := &SettingRow{Title: title, Value: value, Icon: icn}
	r.Key, r.Name, r.Item.Label, r.Detail = key, key, title, value
	r.FocusScale = RowFocusScale
	r.glyph.Glyph, r.glyph.Size, r.glyph.Name = icn, settingIcon, "svg.i"
	r.texts.Name = nameRowTexts
	r.texts.title.Name = nameRowTitle
	r.texts.title.Role, r.texts.title.Size, r.texts.title.Weight = theme.RoleBody, rowTitleSize, rowTitleWeight
	r.texts.title.Text = title
	r.texts.Add(&r.texts.title)
	r.value.Role, r.value.Size, r.value.Text = theme.RoleBody, settingValueSize, value
	r.Add(&r.glyph, &r.texts, &r.value)

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetSub changes the line under the title; the same text changes nothing.
func (r *NextRow) SetSub(s string) {
	if s != r.Sub {
		r.Sub, r.Detail = s, s
		r.texts.sub.SetText(s)
		r.Relayout()
	}
}

// Describe describes the row as a button named by its number and title, valued
// by the line under the title.
func (r *NextRow) Describe(c *ui.Context, n *platform.SemanticNode) {
	r.Item.Describe(c, n)
	n.Label = rowName(r.No, r.Title)
}

// Layout is the width it gets and 110: the number, then the texts that take what
// is left, each centred on the height.
func (r *NextRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(cs.Max.W, NextRowHeight))
	ns := c.Layout(&r.num, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	room := max(size.W-2*rowPadX-ns.W-rowGap, 0) // `.tt{flex:1}`
	ts := c.Layout(&r.texts, ui.Constraints{Min: geom.Sz(room, 0), Max: geom.Sz(room, ui.Unbounded)})
	c.Place(&r.num, geom.Pt(rowPadX, (size.H-ns.H)/2))
	c.Place(&r.texts, geom.Pt(rowPadX+ns.W+rowGap, (size.H-ts.H)/2))

	return size
}

// Paint draws the row at its focus scale.
func (r *NextRow) Paint(c *ui.Context, p *ui.Painter) {
	if r.body == nil {
		r.body = r.paintBody
	}
	r.Item.Scaled(c, p, r.body)
}

// LayoutStyle is `div.f.trow` of the next row: the fill and the radius, and the
// lines it holds.
func (r *NextRow) LayoutStyle(c *ui.Context) (kit.CSS, string) {
	look := theme.From(c)

	return kit.CSS{Background: kit.CSSColor(look.T.Surface), Radius: "22px", Color: kit.CSSColor(r.ink().Of(look))},
		r.No + "\n" + r.Title + "\n" + r.Sub
}

// Layout stacks the title, capped at 330 wide, over its line.
func (t *nextTexts) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := min(cs.Max.W, nextTitleMaxW)
	bs := c.Layout(&t.title, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}) // `b{display:block}`
	ss := c.Layout(&t.sub, ui.Loose(geom.Sz(cs.Max.W, ui.Unbounded)))
	c.Place(&t.title, geom.Point{})
	c.Place(&t.sub, geom.Pt(0, bs.H))

	return cs.Constrain(geom.Sz(cs.Max.W, bs.H+ss.H))
}

// SetValue changes the text at the right; the same text changes nothing.
func (r *SettingRow) SetValue(s string) {
	if s != r.Value {
		r.Value, r.Detail = s, s
		r.value.SetText(s)
		r.Relayout()
	}
}

// Layout is the width it gets and 96: the icon, the title that takes what is left
// and the value, 26 apart, centred on the height. An empty value still takes its
// gap (an empty flex item).
func (r *SettingRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(cs.Max.W, SettingRowHeight))
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	is := c.Layout(&r.glyph, ui.Tight(geom.Sz(settingIcon, settingIcon)))
	vw, vh := float32(0), float32(0)
	if r.Value != "" {
		vs := c.Layout(&r.value, free)
		vw, vh = vs.W, vs.H
	}
	room := max(size.W-2*rowPadX-is.W-vw-2*rowGap, 0)
	ts := c.Layout(&r.texts, ui.Constraints{Min: geom.Sz(room, 0), Max: geom.Sz(room, ui.Unbounded)})
	c.Place(&r.glyph, geom.Pt(rowPadX, (size.H-is.H)/2))
	c.Place(&r.texts, geom.Pt(rowPadX+is.W+rowGap, (size.H-ts.H)/2))
	if r.Value != "" {
		c.Place(&r.value, geom.Pt(size.W-rowPadX-vw, (size.H-vh)/2))
	}

	return size
}

// Paint draws the row at its focus scale.
func (r *SettingRow) Paint(c *ui.Context, p *ui.Painter) {
	if r.body == nil {
		r.body = r.paintBody
	}
	r.Item.Scaled(c, p, r.body)
}

// LayoutStyle is `div.f.trow` of a setting: the fill and the radius, and the two
// texts it holds.
func (r *SettingRow) LayoutStyle(c *ui.Context) (kit.CSS, string) {
	look := theme.From(c)
	txt := r.Title
	if r.Value != "" {
		txt += "\n" + r.Value
	}

	return kit.CSS{Background: kit.CSSColor(r.fill(look)), Radius: "22px", Color: kit.CSSColor(r.ink().Of(look))}, txt
}

// Layout is the title's line.
func (t *settingTexts) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	free := ui.Constraints{Min: geom.Sz(cs.Max.W, 0), Max: geom.Sz(cs.Max.W, ui.Unbounded)} // `b{display:block}`
	bs := c.Layout(&t.title, free)
	c.Place(&t.title, geom.Point{})

	return cs.Constrain(geom.Sz(cs.Max.W, bs.H))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// rowShadow paints the focus shadow of a `.trow`, 0 20 40 rgba(0,0,0,.5), for
// the transition progress blend.
func rowShadow(p *ui.Painter, b geom.Rect, blend float32) {
	if blend <= 0 {
		return
	}
	p.Shadow(b, rowRadius, ui.Shadow{
		Offset: geom.Pt(0, rowDropY*blend), Blur: rowDropBlur * blend, Color: color.RGBA(0, 0, 0, rowDropAlpha*blend),
	})
}

// paintBody draws the shadow, the --sf fill and what the row holds.
func (r *NextRow) paintBody(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	b := r.Bounds()
	rowShadow(p, b, r.Blend())
	fill := look.T.Surface // `background:#1F2125` inline, which is --sf
	if r.IsFocused() && !PrototypeInlineFocus {
		fill = look.T.Primary
	}
	p.Fill(b, scene.Solid(fill), rowRadius)
	ink := r.ink()
	r.texts.title.Ink = ink
	r.texts.sub.Color = ink.Of(look).WithAlpha(rowSubOpacity) // `.tt span{opacity:.7}`
	if r.IsFocused() && !PrototypeInlineFocus {
		r.num.Ink = theme.InkOnPrimary
	} else {
		r.num.Ink = theme.InkPrimary // `color:var(--pr)` inline
	}
	r.Base.Paint(c, p)
}

// ink is the text's color: --on focused, --tx at rest.
func (r *NextRow) ink() theme.Ink {
	if r.IsFocused() {
		return theme.InkOnPrimary
	}

	return theme.InkText
}

// paintBody draws the shadow, the fill and what the row holds.
func (r *SettingRow) paintBody(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	b := r.Bounds()
	rowShadow(p, b, r.Blend())
	p.Fill(b, scene.Solid(r.fill(look)), rowRadius)
	ink := r.ink()
	r.glyph.Ink = ink
	r.texts.title.Ink = ink
	r.value.Color = ink.Of(look).WithAlpha(settingValueOpacity)
	r.Base.Paint(c, p)
}

// fill is --pr focused, --sf at rest.
func (r *SettingRow) fill(look *theme.Look) color.Color {
	if r.IsFocused() {
		return look.T.Primary
	}

	return look.T.Surface
}

// ink is --on focused, --tx at rest; a Danger row keeps --dg.
func (r *SettingRow) ink() theme.Ink {
	switch {
	case r.Danger && (!r.IsFocused() || PrototypeInlineFocus):
		return theme.InkDanger
	case r.IsFocused():
		return theme.InkOnPrimary
	default:
		return theme.InkText
	}
}
