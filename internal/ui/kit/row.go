// Adapted from jplayer-mobile/internal/ui/kit/row.go at 49de3c2 (the Row's
// structure, Describe and AccessibleLabel; its long-press, swipe and reorder
// are not in the prototype).

package kit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of .row (index.html:113-117).
const (
	// rowMinHeight, rowGap are `.row{min-height:52px;gap:12px}` (:113).
	rowMinHeight, rowGap float32 = 52, 12
	// rowBorder is `.row{border-bottom:1px solid var(--bd)}` (:113), which
	// `.row:last-child` drops (:114).
	rowBorder float32 = 1
	// rowIco, rowIcoRadius are `.row .ico{width:36px;height:36px;
	// border-radius:10px}` (:115).
	rowIco, rowIcoRadius float32 = 36, 10
	// rowTitleSize, rowTitleWeight are `.row .tt b{font-size:15px;
	// font-weight:600}` (:116).
	rowTitleSize float32 = 15
	// rowSubSize is `.row .tt span{font-size:12px}` (:116).
	rowSubSize float32 = 12
	// rowValSize is `.row .val{font-size:14px}` (:117).
	rowValSize float32 = 14
	// rowChevron is `ic('cr',20)` (:468).
	rowChevron float32 = 20
	// rowWidth is the width of a row given no width limit.
	rowWidth float32 = 325
)

// The names layout.json gives the parts of a row.
const (
	nameRowIco   = "span.ico"
	nameRowTT    = "span.tt"
	nameRowTitle = "b"
	nameRowVal   = "span.val"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Row is `.row` (index.html:113-117): a line of a settings card or a sheet, at
// least 52 tall, with an optional 36 square icon, a title (15/600) over an
// optional sub line (12, --tx2), an optional value (14, --tx2) and a trailing
// chevron or Switch, 12 apart, and a 1 px --bd line under it unless it is the
// last. The Row takes the press; a Switch in it only shows the state.
//
// Set the Name to the data key of the prototype (`go.info`, `pref:wifi`).
type Row struct {
	ui.Pressable
	// Icon is the symbol of the 36 square before the text; icon.NameUnknown
	// for none (the square is left out too).
	Icon icon.Name
	// Title is the first line, 15/600.
	Title string
	// Sub is the second line, 12 --tx2; empty for none.
	Sub string
	// Value is the text before the trailing widget, 14 --tx2.
	Value string
	// Chevron adds the 20 DIP `cr` icon at the end, when Trailing is nil.
	Chevron bool
	// Trailing, when set, ends the row: a Switch.
	Trailing ui.Widget
	// Last drops the line under the row (`.row:last-child`).
	Last bool

	ico      rowIcoBox
	text     rowText
	val      Label
	chev     Icon
	attached bool
	trail    ui.Widget // the Trailing that is a child
}

// rowIcoBox is `.row .ico`: a rounded --sf2 square holding the icon.
type rowIcoBox struct {
	ui.Base
	icon Icon
}

// rowText is `.row .tt`: the title over the sub line.
type rowText struct {
	ui.Base
	title, sub Label
	// strut is the line box of the page's 16 px text, which the sub line sits
	// in; hasSub tells whether there is one.
	strut  theme.LineBox
	hasSub bool
}

var _ ui.Labeler = (*Row)(nil)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewRow returns a row with an icon, a title and a sub line.
func NewRow(icn icon.Name, title, sub string) *Row {
	return &Row{Icon: icn, Title: title, Sub: sub}
}

// SetTitle changes the title, laying the row out again; the same text changes
// nothing.
func (r *Row) SetTitle(s string) {
	if s != r.Title {
		r.Title = s
		r.Relayout()
	}
}

// SetValue changes the value text, laying the row out again; the same text
// changes nothing.
func (r *Row) SetValue(s string) {
	if s != r.Value {
		r.Value = s
		r.Relayout()
	}
}

// AccessibleLabel returns the title.
func (r *Row) AccessibleLabel() string { return r.Title }

// Describe describes the row as a button named by its title; as a switch,
// on or off, when its trailing widget is a Switch.
func (r *Row) Describe(c *ui.Context, n *platform.SemanticNode) {
	r.Pressable.Describe(c, n)
	n.Value = r.Value
	if s, ok := r.Trailing.(*Switch); ok {
		n.Role, n.Toggle = platform.RoleSwitch, platform.ToggledOff
		if s.On {
			n.Toggle = platform.ToggledOn
		}
	}
}

// Layout fills the width it gets and is at least 52 tall; the items are
// centered in the height, the text taking the width the others leave.
func (r *Row) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	r.init()
	w := rowWidth
	if cs.Max.W < ui.Unbounded {
		w = cs.Max.W
	}
	border := rowBorder
	if r.Last {
		border = 0
	}
	r.syncParts()
	loose := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	hasIco := r.Icon != icon.NameUnknown
	avail := w
	if hasIco {
		avail -= rowIco + rowGap
	}
	var vs, ts geom.Size
	if r.Value != "" {
		vs = c.Layout(&r.val, loose)
		avail -= vs.W + rowGap
	}
	if r.trail != nil {
		ts = c.Layout(r.trail, loose)
		avail -= ts.W + rowGap
	}
	r.text.strut = strut(c)
	r.text.hasSub = r.Sub != ""
	r.text.title.SetText(r.Title)
	r.text.sub.SetText(r.Sub)
	r.text.sub.Hidden = r.Sub == ""
	tt := c.Layout(&r.text, ui.Constraints{Max: geom.Sz(max(avail, 0), ui.Unbounded)})

	content := max(tt.H, vs.H, ts.H)
	if hasIco {
		content = max(content, rowIco)
	}
	h := max(rowMinHeight, content+border)
	ch := h - border
	size := cs.Constrain(geom.Sz(w, h))
	x := float32(0)
	if hasIco {
		c.Layout(&r.ico, ui.Tight(geom.Sz(rowIco, rowIco)))
		c.Place(&r.ico, geom.Pt(0, centeredY(ch, rowIco)))
		x = rowIco + rowGap
	}
	c.Place(&r.text, geom.Pt(x, centeredY(ch, tt.H)))
	end := size.W
	if r.trail != nil {
		end -= ts.W
		c.Place(r.trail, geom.Pt(end, centeredY(ch, ts.H)))
		end -= rowGap
	}
	if r.Value != "" {
		end -= vs.W
		c.Place(&r.val, geom.Pt(end, centeredY(ch, vs.H)))
	}

	return size
}

// Paint draws the line under the row and the children.
func (r *Row) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	b := r.Bounds()
	if !r.Last {
		p.Fill(geom.XYWH(0, b.H-rowBorder, b.W, rowBorder), scene.Solid(look.T.Border), 0)
	}
	if r.FocusVisible() {
		focusRing(p, b, 0, look.T.Primary)
	}
	r.Base.Paint(c, p)
}

// Layout makes the square the size it gets and centers the icon in it.
func (b *rowIcoBox) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(rowIco, rowIco))
	s := c.Layout(&b.icon, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&b.icon, geom.Pt(centeredY(size.W, s.W), centeredY(size.H, s.H)))

	return size
}

// Paint draws the rounded --sf2 square under the icon.
func (b *rowIcoBox) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(b.Bounds(), scene.Solid(theme.From(c).T.Surface2), rowIcoRadius)
	b.Base.Paint(c, p)
}

// Layout stacks the title and the sub line. The title is a block of its own
// height. The sub line is an inline run that sits in a line box of the 16 px
// strut: 20 tall with its baseline 16 down, so a 12 px sub is 4 below the top of
// that box and the whole text is title + 20 tall.
func (t *rowText) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	loose := ui.Loose(geom.Sz(w, ui.Unbounded))
	// the title is `display:block`: as wide as the text block.
	ts := c.Layout(&t.title, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(&t.title, geom.Point{})
	h := ts.H
	if t.hasSub {
		c.Layout(&t.sub, loose)
		c.Place(&t.sub, geom.Pt(0, h+t.strut.Baseline-t.sub.Baseline()))
		h += t.strut.Height
	}

	return cs.Constrain(geom.Sz(w, h))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// init adds the parts the first time and gives them the names of layout.json.
func (r *Row) init() {
	if r.attached {
		return
	}
	r.attached = true
	r.Look = noLook
	r.ico.Name, r.text.Name, r.val.Name, r.text.title.Name, r.chev.Name = nameRowIco, nameRowTT, nameRowVal, nameRowTitle, iconName
	r.ico.icon.Name = iconName
	r.ico.Add(&r.ico.icon)
	r.text.Add(&r.text.title, &r.text.sub)
	r.Add(&r.ico, &r.text, &r.val)
	labelStyle{Role: theme.RoleBody, Size: rowTitleSize, Weight: 600, Ink: theme.InkText, Wrap: true}.apply(&r.text.title)
	labelStyle{Role: theme.RoleBody, Size: rowSubSize, Ink: theme.InkText2, Wrap: true}.apply(&r.text.sub)
	labelStyle{Role: theme.RoleBody, Size: rowValSize, Ink: theme.InkText2}.apply(&r.val)
}

// syncParts makes the children match the fields: the icon, the value, the
// trailing widget or chevron.
func (r *Row) syncParts() {
	r.ico.Hidden = r.Icon == icon.NameUnknown
	r.ico.icon.Glyph, r.ico.icon.Size, r.ico.icon.Ink = r.Icon, defaultIconSize, theme.InkText
	r.val.Hidden = r.Value == ""
	r.val.SetText(r.Value)
	r.chev.Glyph, r.chev.Size, r.chev.Ink = icon.Cr, rowChevron, theme.InkText
	want := r.Trailing
	if want == nil && r.Chevron {
		want = &r.chev
	}
	if r.trail != want {
		if r.trail != nil {
			r.Remove(r.trail)
		}
		if r.trail = want; r.trail != nil {
			r.Add(r.trail)
		}
	}
}
