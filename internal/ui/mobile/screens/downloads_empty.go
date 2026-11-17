//go:build !tvos

// The empty screen is the second return of MS.downloads (index.html:451-453)
// and its M12 capture (local/shots/prototype-kit/mobile/M12.layout.json).

package screens

import (
	"math"

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

// The measures of screen 12 (index.html:452-453), in DIPs.
const (
	// emptyTop is the block's `margin-top:60px` under the title.
	emptyTop float32 = 60
	// emptyDisc, emptyDiscIcon, emptyDiscStroke are the 140 disc and its
	// `ic('hp',60,1.6)`; emptyDashOn, emptyDashOff the 1 px dashes Chrome draws
	// for a `1px dashed` border (3 on, 2 off, the gap bent to fit the circle).
	emptyDisc, emptyDiscIcon, emptyDiscStroke float32 = 140, 60, 1.6
	emptyDashOn, emptyDashOff                 float32 = 3, 2
	// emptyTitleTop, emptyTitleSize are the `.d` at `font-size:24px;margin-top:30px`.
	emptyTitleTop, emptyTitleSize float32 = 30, 24
	// emptyBodyTop, emptyBodySize, emptyBodyLine, emptyBodyBottom are the `p.muted`
	// at `font-size:14px;margin:12px 0 24px;line-height:1.5`.
	emptyBodyTop, emptyBodySize, emptyBodyLine, emptyBodyBottom float32 = 12, 14, 1.5, 24
	// emptyButtonW is `style="width:230px;margin:0 auto"`.
	emptyButtonW float32 = 230
)

// emptyBlock is screen 12 under the title: the dashed disc with the headphones, "Chưa
// có file âm thanh", the line under it and the button "Xem khoá học".
type emptyBlock struct {
	ui.Base

	disc   *dashedDisc
	title  *kit.Label
	body   *kit.Label
	button *kit.Button
}

// dashedDisc is `<div style="width:140px;height:140px;border-radius:50%;background:
// var(--sf);border:1px dashed var(--bd)">` with the 60 headphones in --pr.
type dashedDisc struct {
	ui.Base

	icon   *kit.Icon
	ring   scene.Path
	dashes []float32
	built  bool
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newEmptyBlock returns the empty screen's block; the button goes to the
// courses (`data-go="courses" data-root="1"`).
func newEmptyBlock(e *env.Env) *emptyBlock {
	b := &emptyBlock{
		disc:   &dashedDisc{icon: kit.NewIcon(icon.Hp, emptyDiscIcon, theme.InkPrimary)},
		title:  heading(kit.Display(e.T("downloads.emptyTitle"), emptyTitleSize), 1),
		body:   kit.Muted(e.T("downloads.emptyBody"), emptyBodySize),
		button: kit.NewButton(kit.ButtonP, e.T("downloads.browse"), icon.Grid),
	}
	b.disc.icon.Stroke = emptyDiscStroke
	b.disc.Add(b.disc.icon)
	b.title.Align = ui.TextAlignCenter
	b.body.LineHeight, b.body.Align, b.body.Wrap, b.body.Name = emptyBodyLine, ui.TextAlignCenter, true, "p.muted"
	b.button.Name = "go.courses"
	b.button.OnPress = func() { e.Act.Go(nav.Route{Screen: env.RouteCourses}, true) }
	b.Add(b.disc, b.title, b.body, b.button)

	return b
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout centers the disc, the title and the line, and the 230 button, one under
// the other.
func (b *emptyBlock) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	col := ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}
	c.Layout(b.disc, ui.Tight(geom.Sz(emptyDisc, emptyDisc)))
	c.Place(b.disc, geom.Pt((w-emptyDisc)/2, 0))
	y := emptyDisc + emptyTitleTop
	ts := c.Layout(b.title, col)
	c.Place(b.title, geom.Pt(0, y))
	y += ts.H + emptyBodyTop
	bs := c.Layout(b.body, col)
	c.Place(b.body, geom.Pt(0, y))
	y += bs.H + emptyBodyBottom
	ks := c.Layout(b.button, ui.Constraints{Max: geom.Sz(min(emptyButtonW, w), ui.Unbounded)})
	c.Place(b.button, geom.Pt((w-ks.W)/2, y))

	return cs.Constrain(geom.Sz(w, y+ks.H))
}

// Layout centers the headphones in the disc.
func (d *dashedDisc) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(d.icon, ui.Tight(geom.Sz(emptyDiscIcon, emptyDiscIcon)))
	c.Place(d.icon, geom.Pt((emptyDisc-emptyDiscIcon)/2, (emptyDisc-emptyDiscIcon)/2))

	return cs.Constrain(geom.Sz(emptyDisc, emptyDisc))
}

// Paint draws the --sf disc and, inside its edge, the dashed ring of the border.
func (d *dashedDisc) Paint(c *ui.Context, p *ui.Painter) {
	t := &theme.From(c).T
	r := d.Bounds()
	p.Fill(r, scene.Solid(t.Surface), r.W/2)
	if !d.built {
		d.build(r)
	}
	p.StrokePath(&d.ring, scene.Stroke{Width: 1, Dashes: d.dashes}, scene.Solid(t.Border))
	d.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// build makes the ring: a circle half a DIP inside the edge, dashed 3 on and 2 off
// with the gap spread so that a whole number of dashes fills it, as Chrome does
// for a border that closes on itself.
func (d *dashedDisc) build(r geom.Rect) {
	d.built = true
	radius := r.W/2 - .5
	d.ring.Circle(r.W/2, r.H/2, radius)
	n := math.Round(float64(2*math.Pi*radius) / float64(emptyDashOn+emptyDashOff))
	gap := float32(float64(2*math.Pi*radius)/n) - emptyDashOn
	d.dashes = []float32{emptyDashOn, gap}
}
