//go:build tvos

package tvkit

import "github.com/gdeskdev/desk/framework/ui"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Raise is a container that paints the child holding the focus last, over its
// siblings: `#tv .f.focus{z-index:5;position:relative}` (index.html:181,
// F-12). The shadow and the ring of a focused poster show over its caption and
// over the next row, and a focused row's shadow over the next row. It lays its
// children out as ui.Base does; give it a layout of its own by embedding it
// and calling PaintRaised from Paint.
type Raise struct {
	ui.Base
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Paint paints the children, the one with the focus inside it last.
func (r *Raise) Paint(c *ui.Context, p *ui.Painter) { PaintRaised(c, p, &r.Base) }

// PaintRaised paints the children of b in order, except that the child holding
// the focus, or an item inside it, is painted last. With no focus inside b it
// is Base.Paint. It allocates nothing.
func PaintRaised(c *ui.Context, p *ui.Painter, b *ui.Base) {
	top := raised(b)
	if top == nil {
		b.Paint(c, p)

		return
	}
	for w := range b.Children() {
		if w != top {
			p.Child(c, w)
		}
	}
	p.Child(c, top)
}

// PaintOver paints the children of b in order with top last, over the rest: the
// nav bar of a root screen, which `.tnav` puts over the hero and the grid that
// slides under it, though the tree lists it first so that a screen reader reads
// the page from the top (reading order is tree order, P8.3b). It allocates
// nothing.
func PaintOver(c *ui.Context, p *ui.Painter, b *ui.Base, top ui.Widget) {
	for w := range b.Children() {
		if w != top {
			p.Child(c, w)
		}
	}
	if top != nil {
		p.Child(c, top)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// raised returns the child of b that holds the focus or has it inside, nil
// when b does not.
func raised(b *ui.Base) ui.Widget {
	v := b.View()
	if v == nil || !b.FocusWithin() {
		return nil
	}
	self := b.Self()
	for w := v.Focused(); w != nil; w = w.Node().Parent() {
		if w.Node().Parent() == self {
			return w
		}
	}

	return nil
}
