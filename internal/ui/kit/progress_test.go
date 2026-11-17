package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
)

// TestProgressBoxMatchesPrototype compares a bar with the `div.prog` of M03:
// 257 x 5 at (36, 347), on a track of white at .3.
func TestProgressBoxMatchesPrototype(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		want := protoBox(t, "M03", "div.prog")
		p := &kit.Progress{Value: .45, Track: color.RGBA(255, 255, 255, .3)}
		tt := mountStage(t, ph, placeAt(p, want.X, want.Y, want.W))
		rectNear(t, want, frameOf(p), 0, "div.prog")
		assert.Equal(t, "div.prog", p.Name)
		img := tt.Snapshot()
		fillEnd := int(want.X + want.W*.45)
		y := int(want.Y) + 2
		pixelNear(t, img, int(want.X)+20, y, look.T.Primary, 2, "fill")
		pixelNear(t, img, fillEnd-3, y, look.T.Primary, 2, "fill end")
		pixelNear(t, img, fillEnd+3, y, over(color.RGBA(255, 255, 255, .3), look.T.Bg), 3, "track")
		// 3 rounded: the corner pixel of the fill is mostly cut away
		corner, inner := pixelAt(img, int(want.X), int(want.Y)), pixelAt(img, int(want.X)+20, y)
		assert.Less(t, int(corner.R)+int(corner.G), (int(inner.R)+int(inner.G))*3/5, "rounded corner")
	})
}

// TestProgressValue checks SetValue's clamp, the height and the default colors.
func TestProgressValue(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		// :452 `height:8px`.
		p := &kit.Progress{Height: 8}
		tt := mountStage(t, ph, placeAt(p, 10, 10, 200))
		assert.InDelta(t, 8, frameOf(p).H, 0)
		pixelNear(t, tt.Snapshot(), 100, 14, look.T.Surface2, 2, "empty: the track")
		p.SetValue(-3)
		assert.InDelta(t, 0, p.Value, 0)
		p.SetValue(7)
		assert.InDelta(t, 1, p.Value, 0)
		tt.Elapse(kittest.Frame)
		pixelNear(t, tt.Snapshot(), 100, 14, look.T.Primary, 2, "full: the fill")
		p.SetValue(.5)
		tt.Elapse(kittest.Frame)
		pixelNear(t, tt.Snapshot(), 50, 14, look.T.Primary, 2, "half: fill")
		pixelNear(t, tt.Snapshot(), 150, 14, look.T.Surface2, 2, "half: track")
		var now float64
		for _, n := range tt.Semantics().Nodes {
			if n.Role == platform.RoleText && n.Max == 1 {
				now = n.Now
			}
		}
		assert.InDelta(t, .5, now, 0)
	})
}

// TestProgressDoesNotAllocate checks that SetValue and a repaint allocate
// nothing.
func TestProgressDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		p := &kit.Progress{}
		tt := mountStage(t, ph, placeAt(p, 10, 10, 200))
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a repaint", ph.Name)
		v := float32(0)
		assert.Zero(t, testing.AllocsPerRun(100, func() { v += .01; p.SetValue(v) }), "%s: SetValue", ph.Name)
	}
}

// TestProgressReadingDoesNotAllocate checks that the percent of a bar and a Spoken
// that did not change allocate nothing: the reading is made when it changes, never
// per frame.
func TestProgressReadingDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	p := &kit.Progress{Label: "Vị trí phát"}
	p.SetSpoken("01:12 / 03:40")
	var n platform.SemanticNode
	share := float32(0)
	assert.Zero(t, testing.AllocsPerRun(100, func() { share += .01; _ = kit.Percent(share) }), "Percent")
	assert.Zero(t, testing.AllocsPerRun(100, func() { p.SetSpoken("01:12 / 03:40") }), "the same reading")
	assert.Zero(t, testing.AllocsPerRun(100, func() { p.Describe(nil, &n) }), "Describe")
	p.Spoken = ""
	assert.Zero(t, testing.AllocsPerRun(100, func() { p.Describe(nil, &n) }), "Describe, the percent")
}
