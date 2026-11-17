//go:build tvos

// Adapted from jplayer-tv/internal/ui/kit/card.go at 781bee5 (a card that is a
// focus Item holding its art and painted under the Item's scale with the focus
// shadow behind it); the look is `tposter` and `.f.focus` of the prototype
// (index.html:644, :181), not jPlayer's card.

package tvkit

import (
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// PosterRadius is `style="border-radius:20px"` of the `.f` that holds a poster
// (`tposter` :644, the grid's `.f` :672): the ring of the focus follows it.
const PosterRadius float32 = 20

// PosterTile is a poster that takes the focus: `<div class="f" data-fid=...
// style="border-radius:20px">` around `poster(cid, w, h, {r:20, ts})`. At rest
// it is the poster; focused it grows to 1.06 about its centre with a ring of
// 6 in --pr and a drop shadow of 0 26 50 rgba(0,0,0,.6) (`#tv .f.focus`, :181),
// which a Raise container paints over what is next to it (F-12).
//
// Set Key to the `data-fid` (it is also the Name layout.json knows the tile by)
// and OnSelect to the `data-tgo` or `data-tact`.
type PosterTile struct {
	Item
	// Poster is the cover; it is the tile's only child.
	Poster *poster.Poster

	w, h float32
	body func(c *ui.Context, p *ui.Painter) // paintBody as a method value, made once
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewPosterTile returns a tile of key showing a poster of spec at w by h DIPs.
func NewPosterTile(key string, spec poster.Spec, w, h float32) *PosterTile {
	t := &PosterTile{Poster: poster.New(spec, w, h), w: w, h: h}
	t.Key, t.Name, t.Item.Label = key, key, spec.Title
	t.FocusScale = FocusScale
	t.Add(t.Poster)

	return t
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetReading names the tile for a screen reader: the course's name, which the
// poster paints in only as "DANCEFIT SET 35" (the poster is decoration), and
// what is worth hearing after it. The tile is one element, a button.
func (t *PosterTile) SetReading(name, about string) {
	t.Item.Label, t.Item.Detail = name, about
}

// Layout is the poster's size.
func (t *PosterTile) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(t.w, t.h))
	c.Layout(t.Poster, ui.Tight(size))
	c.Place(t.Poster, geom.Point{})

	return size
}

// Paint draws the tile at its focus scale.
func (t *PosterTile) Paint(c *ui.Context, p *ui.Painter) {
	if t.body == nil {
		t.body = t.paintBody
	}
	t.Item.Scaled(c, p, t.body)
}

// LayoutStyle is `div.f` of the tile: its radius and the two lines of the
// poster's title as the capture reads them.
func (t *PosterTile) LayoutStyle(*ui.Context) (kit.CSS, string) {
	return kit.CSS{Radius: "20px", Background: "rgba(0, 0, 0, 0)"}, t.titleText()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// titleText is "DANCEFIT\nSET 35", the text of the poster's title.
func (t *PosterTile) titleText() string {
	if t.Poster.Spec().NoTitle || t.Poster.Spec().Title == "" {
		return ""
	}

	return "DANCEFIT\n" + t.Poster.Spec().Title
}

// paintBody draws the ring and the shadow behind the poster, then the poster.
func (t *PosterTile) paintBody(c *ui.Context, p *ui.Painter) {
	FocusShadow(p, t.Bounds(), PosterRadius, t.Blend(), theme.From(c).T.Primary)
	t.Base.Paint(c, p)
}
