// Adapted from jplayer-tv/internal/ui/kit/shadowcache_test.go at 781bee5.

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
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// cachedShadowed draws a text over its shadow at (10, 10) of its box, from a
// ShadowCache or, when plain is set, as DrawTextShadow does: the blurred layer.
type cachedShadowed struct {
	ui.Base
	face   text.Face
	shadow kit.TextShadow
	plain  bool
	layout *text.Layout
	cache  kit.ShadowCache
}

func (s *cachedShadowed) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	s.layout = c.TextLayout("DANCEFIT SET 35", s.face, 0)

	return cs.Constrain(geom.Sz(s.layout.Size.W+60, s.layout.Size.H+60))
}

func (s *cachedShadowed) Paint(c *ui.Context, p *ui.Painter) {
	if s.plain {
		kit.DrawTextShadow(p, s.layout, geom.Pt(30, 30), s.shadow, color.White)

		return
	}
	s.cache.Draw(c, p, s.layout, geom.Pt(30, 30), s.shadow, color.White)
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// shadowFrame draws the widget and returns a copy of its box.
func shadowFrame(t *testing.T, ph kittest.Phone, scale float32, w *cachedShadowed) *image.RGBA {
	t.Helper()
	tt := mountAt(t, ph, w, 0)
	if scale != 0 {
		tt.SetScale(scale)
		tt.Repaint()
	}
	snap := tt.Snapshot()
	sc := tt.View().Context().Scale()
	f := w.Frame()
	x0, y0, x1, y1 := int(f.X*sc), int(f.Y*sc), int((f.X+f.W)*sc), int((f.Y+f.H)*sc)
	out := image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	for y := y0; y < y1; y++ {
		copy(out.Pix[(y-y0)*out.Stride:(y-y0+1)*out.Stride], snap.Pix[y*snap.Stride+4*x0:y*snap.Stride+4*x1])
	}

	return out
}

// TestShadowCacheDrawsTheLayersPicture: the picture of the cached shadow is the
// one of the blurred layer, at the scales of the phones and of the 4K stage
// (scale 2), the poster title's shadow and a strong one.
func TestShadowCacheDrawsTheLayersPicture(t *testing.T) {
	t.Parallel()

	shadows := map[string]kit.TextShadow{
		"poster title": kit.PosterTitleShadow,
		"strong":       {Offset: geom.Pt(0, 6), Blur: 16, Color: color.RGBA(255, 0, 0, .9)},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for name, sh := range shadows {
			for _, scale := range []float32{0, 2} {
				face := theme.NewFaces(ph.Naming).Face(theme.RoleDisplay, 40, 800)
				plain := shadowFrame(t, ph, scale, &cachedShadowed{face: face, shadow: sh, plain: true})
				cached := shadowFrame(t, ph, scale, &cachedShadowed{face: face, shadow: sh})
				require.Equal(t, plain.Bounds(), cached.Bounds())
				var sum, n, worst int
				for i := range plain.Pix {
					d := int(plain.Pix[i]) - int(cached.Pix[i])
					d = max(d, -d)
					sum += d
					n++
					worst = max(worst, d)
				}
				mean := float64(sum) / float64(n)
				assert.Less(t, mean, 0.5, "%s at scale %v: the mean difference of the channels", name, scale)
				assert.LessOrEqual(t, worst, 12, "%s at scale %v: the largest difference", name, scale)
			}
		}
	})
}

// TestShadowCacheSharesAndRendersOnce: two widgets of the same text, face and
// shadow share one picture, and a widget draws it again with no new render.
func TestShadowCacheSharesAndRendersOnce(t *testing.T) { //nolint:paralleltest // the process's cache is shared
	kit.ResetShadowPics()
	ph := kittest.Phones[0]
	face := theme.NewFaces(ph.Naming).Face(theme.RoleDisplay, 40, 800)
	a := &cachedShadowed{face: face, shadow: kit.PosterTitleShadow}
	b := &cachedShadowed{face: face, shadow: kit.PosterTitleShadow}
	tt := mountAt(t, ph, ui.Column(0, a, b), 0) // one view: one shaper, which the key holds
	tt.Repaint()
	assert.Equal(t, 1, a.cache.Renders()+b.cache.Renders(), "one picture for the two")
	tt.Repaint()
	tt.Repaint()
	assert.Equal(t, 1, a.cache.Renders()+b.cache.Renders(), "the next frames draw the picture")
	require.NotNil(t, a.cache.Picture())
	assert.Same(t, a.cache.Picture(), b.cache.Picture())
	assert.Equal(t, 1, kit.ShadowPicsLen())
}

// TestShadowCacheEvictsTheLeastUsed: the count and the bytes of the pictures
// are bounded, the least recently used goes first.
func TestShadowCacheEvictsTheLeastUsed(t *testing.T) {
	t.Parallel()

	c := kit.NewPicCache(3, 1000)
	for _, name := range []string{"a", "b", "c"} {
		c.Put(name, 100)
	}
	assert.True(t, c.Get("a"), "a is used again")
	c.Put("d", 100)
	assert.False(t, c.Get("b"), "b was the least recently used")
	assert.True(t, c.Get("a"))
	assert.Equal(t, 3, c.Len())
	c.Put("big", 5000)
	assert.False(t, c.Get("big"), "a picture over the byte budget is not kept")
	assert.Equal(t, 3, c.Len())
	assert.Equal(t, 300, c.Bytes())
}

func TestShadowCacheDrawDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		face := theme.NewFaces(ph.Naming).Face(theme.RoleDisplay, 20, 800)
		w := &cachedShadowed{face: face, shadow: kit.PosterTitleShadow}
		tt := mountAt(t, ph, w, 0)
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), ph.Name)
	}
}
