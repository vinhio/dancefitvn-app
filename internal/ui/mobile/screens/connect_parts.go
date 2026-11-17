//go:build !tvos

// Adapted from dancefitvn-app/internal/ui/mobile/kitdemo/pages_connect.go (the
// boxes and widgets of M16 that the demo lays out at absolute positions, here the
// flow that gives them) and from the dashed disc of downloads_empty.go (a block
// of a round shape with an icon centred in it). The numbers are those of MS.connect
// (index.html:496-502), each from its selector.

package screens

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of the unlinked state of MS.connect (index.html:497-500), in DIPs.
const (
	// connTVW, connTVH, connTVRadius, connTVBorder, connTVTop are the TV picture
	// `margin:18px auto 0;width:200px;height:122px;border-radius:12px;border:2px solid
	// var(--bd)` (:497).
	connTVW, connTVH, connTVRadius, connTVBorder, connTVTop float32 = 200, 122, 12, 2, 18
	// connTVEyebrow is the `.eb` of "Mã trên TV" at `font-size:9px`; connTVCode the
	// `.d.acc` at `font-size:30px;letter-spacing:4px;margin-top:6px`; connTVCodeSpacing
	// its spacing and connTVCodeTop its margin.
	connTVEyebrow, connTVCode, connTVCodeSpacing, connTVCodeTop float32 = 9, 30, 4, 6
	// connTitleSize, connTitleTop are the `.d` at `font-size:26px;margin-top:28px`.
	connTitleSize, connTitleTop float32 = 26, 28
	// connLeadSize, connLeadLine, connLeadTop, connLeadBottom are the `p.muted` at
	// `font-size:14px;margin:10px 0 20px;line-height:1.5`.
	connLeadSize, connLeadLine, connLeadTop, connLeadBottom float32 = 14, 1.5, 10, 20
)

// The measures of the linked state of MS.connect (index.html:497), in DIPs.
const (
	// connOkTop, connOkSize are the disc `margin:0 auto` under the wrapper's
	// `margin-top:80px`, 96 across; connOkIcon, connOkStroke its `ic('check',48,3)`.
	connOkTop, connOkSize, connOkIcon, connOkStroke float32 = 80, 96, 48, 3
	// connDoneSize, connDoneTop are the `.d` at `font-size:28px;margin-top:28px`.
	connDoneSize, connDoneTop float32 = 28, 28
	// connUserSize, connUserTop, connUserBottom are the `p.muted` at the page's own
	// 16 px, `margin:12px 0 26px`.
	connUserSize, connUserTop, connUserBottom float32 = 16, 12, 26
)

// connButtonSide, connButtonBottom are the fixed button's `left:20px;right:20px;
// bottom:38px` (:500).
const connButtonSide, connButtonBottom float32 = 20, 38

// The colours MS.connect writes as literals (they are not in :root).
var (
	// connTVInk is the TV picture's `background:#0B0B0C` (:497).
	connTVInk = color.RGB(0x0b, 0x0b, 0x0c)
	// connOkInk is the check's `color:#0B0C0E` over the --ok disc (:497).
	connOkInk = color.RGB(0x0b, 0x0c, 0x0e)
)

// The names layout.json gives the parts that are not a kit widget's.
const (
	nameConnLead = "p.muted" // the `<p class="muted">` under the title and the one of the linked state
	nameLinkTV   = "linkTV"  // data-act="linkTV"
	nameUnlinkTV = "unlinkTV"
)

// tvPicture is the picture of the TV that shows the code (:497): a 200 x 122
// rounded box on #0B0B0C with a 2 --bd border, centred in the column, holding the
// eyebrow "Mã trên TV" over the sample code "7K4-M2P" in --pr, both centred in it
// (`display:flex;flex-direction:column;align-items:center;justify-content:center`).
type tvPicture struct {
	ui.Base
	// Decor: the picture of the TV with a sample code is an illustration; the field
	// under it has the label "Mã trên TV" and the code typed.
	kit.Decor

	eyebrow *kit.Label
	code    *kit.Label
}

// okDisc is the 96 --ok disc with the check of the linked state (:497), centred in
// the column.
type okDisc struct {
	ui.Base

	check *kit.Icon
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newTVPicture returns the picture with its two texts.
func newTVPicture(eyebrow, code string) *tvPicture {
	t := &tvPicture{eyebrow: kit.Eyebrow(eyebrow), code: accent(kit.Display(code, connTVCode))}
	t.eyebrow.Size = connTVEyebrow
	t.code.Spacing = connTVCodeSpacing
	t.Add(t.eyebrow, t.code)

	return t
}

// newOkDisc returns the disc with the check in #0B0C0E.
func newOkDisc() *okDisc {
	d := &okDisc{check: kit.NewIcon(icon.Check, connOkIcon, theme.InkText)}
	d.check.Stroke, d.check.Color = connOkStroke, connOkInk
	d.Add(d.check)

	return d
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout puts the picture in the middle of the column; its two texts are centred in
// it, the code `margin-top:6px` under the eyebrow.
func (t *tvPicture) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(cs.Max.W, connTVH))
	box := geom.XYWH((size.W-connTVW)/2, 0, connTVW, connTVH)
	loose := ui.Loose(geom.Sz(connTVW, ui.Unbounded))
	es := c.Layout(t.eyebrow, loose)
	cs2 := c.Layout(t.code, loose)
	h := es.H + connTVCodeTop + cs2.H
	y := box.Y + connTVBorder + (connTVH-2*connTVBorder-h)/2
	c.Place(t.eyebrow, geom.Pt(box.X+(connTVW-es.W)/2, y))
	c.Place(t.code, geom.Pt(box.X+(connTVW-cs2.W)/2, y+es.H+connTVCodeTop))

	return size
}

// Paint draws the box and its border under the texts.
func (t *tvPicture) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	b := t.Bounds()
	r := geom.XYWH((b.W-connTVW)/2, 0, connTVW, connTVH)
	p.Fill(r, scene.Solid(connTVInk), connTVRadius)
	p.Border(r, geom.Uniform(connTVBorder), look.T.Border, connTVRadius)
	t.Base.Paint(c, p)
}

// Layout puts the check in the middle of the disc, which is in the middle of the
// column.
func (d *okDisc) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(cs.Max.W, connOkSize))
	c.Layout(d.check, ui.Tight(geom.Sz(connOkIcon, connOkIcon)))
	c.Place(d.check, geom.Pt((size.W-connOkIcon)/2, (connOkSize-connOkIcon)/2))

	return size
}

// Paint draws the --ok disc under the check.
func (d *okDisc) Paint(c *ui.Context, p *ui.Painter) {
	b := d.Bounds()
	p.Fill(geom.XYWH((b.W-connOkSize)/2, 0, connOkSize, connOkSize), scene.Solid(theme.From(c).T.Success), connOkSize/2)
	d.Base.Paint(c, p)
}
