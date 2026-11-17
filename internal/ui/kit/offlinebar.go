package kit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of .offbar (index.html:132, offbar :414).
const (
	// offbarHeight is `.offbar{height:40px}`.
	offbarHeight float32 = 40
	// offbarPad, offbarGap are `.offbar{padding:0 20px;gap:8px}`.
	offbarPad, offbarGap float32 = 20, 8
	// offbarSize is `.offbar{font-size:13px;font-weight:700}`.
	offbarSize float32 = 13
	// offbarIcon, offbarStroke are `ic('wifi',18,2)` (:414).
	offbarIcon, offbarStroke float32 = 18, 2
)

// nameOffbar is the key layout.json gives the bar.
const nameOffbar = "div.offbar"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// OfflineBar is `.offbar` (index.html:132, :414): the 40 tall strip under the
// status bar that says the app is offline, rgba(255,176,32,.16) with a --warn
// wifi icon (18, stroke 2) and 13/700 --warn text, padded 20, 8 apart. Its
// parent puts it 54 from the top (`top:54px`, over the screen frame).
type OfflineBar struct {
	ui.Base
	// Text is the message.
	Text string

	icon     Icon
	label    Label
	attached bool
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewOfflineBar returns a bar saying text.
func NewOfflineBar(text string) *OfflineBar { return &OfflineBar{Text: text} }

// Layout takes the width it gets and 40 of height, with the icon and the text
// centered on the row.
func (o *OfflineBar) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !o.attached {
		o.attached = true
		o.Name = nameOffbar
		o.icon.Name = iconName
		o.icon.Glyph, o.icon.Size, o.icon.Stroke, o.icon.Ink = icon.Wifi, offbarIcon, offbarStroke, theme.InkWarning
		o.Add(&o.icon, &o.label)
	}
	labelStyle{Role: theme.RoleBody, Size: offbarSize, Weight: 700, Ink: theme.InkWarning, Ellipsis: true}.apply(&o.label)
	o.label.SetText(o.Text)
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = 0
	}
	size := cs.Constrain(geom.Sz(w, offbarHeight))
	is := c.Layout(&o.icon, ui.Tight(geom.Sz(offbarIcon, offbarIcon)))
	c.Place(&o.icon, geom.Pt(offbarPad, centeredY(size.H, is.H)))
	room := max(size.W-2*offbarPad-is.W-offbarGap, 0)
	ls := c.Layout(&o.label, ui.Loose(geom.Sz(room, ui.Unbounded)))
	c.Place(&o.label, geom.Pt(offbarPad+is.W+offbarGap, centeredY(size.H, ls.H)))

	return size
}

// Paint draws the tinted strip, then the icon and the text.
func (o *OfflineBar) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(o.Bounds(), scene.Solid(theme.From(c).C.OfflineBg), 0)
	o.Base.Paint(c, p)
}
