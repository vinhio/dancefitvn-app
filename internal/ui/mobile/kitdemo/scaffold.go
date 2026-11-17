//go:build !tvos

package kitdemo

import (
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// item is a child of a vstack with the room above it.
type item struct {
	w      ui.Widget
	before float32
	// center puts a widget narrower than the room in the middle (`margin:0 auto`).
	center bool
}

// vstack stacks its children, `before` apart from the one above, each as wide
// as the room: a column with margins, the way the prototype's block flow puts
// the content of a sheet.
type vstack struct {
	ui.Base

	items []item
}

// spacer is a box of a fixed height that draws nothing: a part the kit does
// not own yet.
type spacer struct {
	ui.Base

	h float32
}

// disc is a round plate with an icon in it: the 64 trash disc of the delete
// sheet (:520).
type disc struct {
	ui.Base

	size float32
	fill func() scene.Paint
	icon *kit.Icon
}

// infoLine is `<p class="muted" style="font-size:12px">${ic('info',14)} text</p>`
// (:517): a 14 icon, the space after it, and the muted text.
type infoLine struct {
	ui.Base

	icon *kit.Icon
	text *kit.Label
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newVStack returns a stack of the items.
func newVStack(items ...item) *vstack {
	s := &vstack{items: items}
	for _, it := range items {
		s.Add(it.w)
	}

	return s
}

// newDisc returns the 64 disc of the sheet that asks to delete: red at .16
// with the trash icon in --dg (:520).
func newDisc() *disc {
	d := &disc{size: 64, icon: kit.NewIcon(icon.Trash, 28, theme.InkDanger)}
	d.fill = func() scene.Paint { return scene.Solid(tokens.Dark.Danger.WithAlpha(.16)) }
	d.Add(d.icon)

	return d
}

// newInfoLine returns the info line with the text.
func newInfoLine(text string) *infoLine {
	l := &infoLine{icon: kit.NewIcon(icon.Info, 14, theme.InkText2), text: kit.Muted(text, 12)}
	l.Name = "p.muted"
	l.Add(l.icon, l.text)

	return l
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout stacks the items across the room.
func (s *vstack) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	y := float32(0)
	for _, it := range s.items {
		y += it.before
		cs := ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}
		if it.center {
			cs.Min = geom.Size{}
		}
		sz := c.Layout(it.w, cs)
		x := float32(0)
		if it.center {
			x = (w - sz.W) / 2
		}
		c.Place(it.w, geom.Pt(x, y))
		y += sz.H
	}

	return cs.Constrain(geom.Sz(w, y))
}

// Layout takes the width it gets and its height.
func (b *spacer) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Sz(cs.Max.W, b.h))
}

// Layout makes a square and centers the icon.
func (d *disc) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(d.icon, ui.Loose(geom.Sz(d.size, d.size)))
	is := d.icon.Frame().Size()
	c.Place(d.icon, geom.Pt((d.size-is.W)/2, (d.size-is.H)/2))

	return cs.Constrain(geom.Sz(d.size, d.size))
}

// Paint draws the disc under the icon.
func (d *disc) Paint(c *ui.Context, p *ui.Painter) {
	r := d.Bounds()
	p.Fill(r, d.fill(), d.size/2)
	d.Base.Paint(c, p)
}

// Layout puts the icon, then the text on the icon's baseline row: 14 and the
// 4.4 DIP space of a 12 px Inter after it, the line 17 tall.
func (l *infoLine) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	is := c.Layout(l.icon, ui.Loose(geom.Sz(14, 14)))
	c.Place(l.icon, geom.Pt(0, 0))
	ts := c.Layout(l.text, ui.Loose(geom.Sz(max(w-is.W-4, 0), ui.Unbounded)))
	c.Place(l.text, geom.Pt(is.W+4, 0))

	return cs.Constrain(geom.Sz(w, max(is.H, ts.H, 17)))
}
