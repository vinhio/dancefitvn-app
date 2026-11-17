// Adapted from jplayer-tv/internal/ui/kit/textshadow_test.go at 781bee5.

package kit_test

import (
	"image"
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
)

// shadowed draws a text with kit.DrawTextShadow at (10, 10) of its box.
type shadowed struct {
	ui.Base
	face   text.Face
	shadow kit.TextShadow
	layout *text.Layout
}

func (s *shadowed) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	s.layout = c.TextLayout("DANCEFIT", s.face, 0)

	return cs.Constrain(geom.Sz(s.layout.Size.W+20, s.layout.Size.H+40))
}

func (s *shadowed) Paint(_ *ui.Context, p *ui.Painter) {
	kit.DrawTextShadow(p, s.layout, geom.Pt(10, 10), s.shadow, color.White)
}

func TestPosterTitleShadowIsTheCSSOne(t *testing.T) {
	t.Parallel()

	// .poster .ttl{text-shadow:0 2px 12px rgba(0,0,0,.18)} (index.html:67).
	assert.Equal(t, geom.Pt(0, 2), kit.PosterTitleShadow.Offset)
	assert.InDelta(t, 12, kit.PosterTitleShadow.Blur, 0)
	assert.Equal(t, color.RGBA(0, 0, 0, .18), kit.PosterTitleShadow.Color)
}

// TestDrawTextShadow checks the picture: the shadow's color shows around the
// text, below it by the offset, blurred past the glyphs' edges, and the text
// stays on top in its own color; no shadow draws the text alone.
func TestDrawTextShadow(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		face := theme.NewFaces(ph.Naming).Face(theme.RoleDisplay, 40, 800)
		red := kit.TextShadow{Offset: geom.Pt(0, 8), Blur: 8, Color: color.RGBA(255, 0, 0, .9)}
		draw := func(s kit.TextShadow) *image.RGBA {
			w := &shadowed{face: face, shadow: s}
			tt := mountAt(t, ph, w, 0)
			// The tester reuses its frame: keep a copy of the part under test.
			f := w.Frame()
			out := image.NewRGBA(image.Rect(0, 0, int(f.W), int(f.H)))
			snap := tt.Snapshot()
			for y := range out.Bounds().Dy() {
				copy(out.Pix[y*out.Stride:(y+1)*out.Stride], snap.Pix[y*snap.Stride:y*snap.Stride+out.Stride])
			}

			return out
		}
		plain, shaded := draw(kit.TextShadow{}), draw(red)
		assert.False(t, hasReddish(plain), "no shadow, no red")
		assert.True(t, hasReddish(shaded), "the shadow's color shows")
		assert.True(t, hasPixelNear(shaded, color.White), "the text stays in its own color")
		// Blurred past the text: red where the plain picture has the background.
		bg := plain.Pix[0:3]
		past := 0
		for y := range plain.Bounds().Dy() {
			for x := range plain.Bounds().Dx() {
				i := y*plain.Stride + 4*x
				if absDiff(plain.Pix[i], bg[0])+absDiff(plain.Pix[i+1], bg[1])+absDiff(plain.Pix[i+2], bg[2]) < 6 &&
					int(shaded.Pix[i]) > int(shaded.Pix[i+2])+20 {
					past++
				}
			}
		}
		assert.Greater(t, past, 100, "the blur reaches beyond the glyphs")
		// A nil layout draws nothing and does not panic.
		assert.NotPanics(t, func() { kit.DrawTextShadow(nil, nil, geom.Point{}, red, color.White) })
	})
}

// hasReddish reports whether the image has a pixel that is clearly red.
func hasReddish(img *image.RGBA) bool {
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if int(img.Pix[i]) > int(img.Pix[i+2])+40 && int(img.Pix[i]) > int(img.Pix[i+1])+40 {
			return true
		}
	}

	return false
}

func TestDrawTextShadowDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		face := theme.NewFaces(ph.Naming).Face(theme.RoleDisplay, 20, 800)
		w := &shadowed{face: face, shadow: kit.PosterTitleShadow}
		tt := mountAt(t, ph, w, 0)
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), ph.Name)
	}
}
