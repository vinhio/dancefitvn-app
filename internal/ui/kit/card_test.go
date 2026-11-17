package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCardBoxesMatchPrototype lays the three empty cards of the loading screen
// (`flex:1;height:72px`, gap 8, index.html:372) at 20,396 and compares them with
// M03B: 112.33 wide, 72 tall.
func TestCardBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		cards := make([]*kit.Card, 3)
		row := ui.Row(8)
		row.Align = ui.CrossStretch
		for i := range cards {
			cards[i] = kit.NewCard(nil)
			cards[i].Height = 72
			cards[i].Grow = 1
			row.Add(cards[i])
		}
		mountStageReal(t, ph, placeAt(row, 20, 396, 353))
		for i, key := range []string{"div.card", "div.card#2", "div.card#3"} {
			want := protoBox(t, "M03B", key)
			if ph.Size.W != 393 {
				// the row is 353 wide on every phone here; only the iOS boxes are the prototype's
				assert.InDelta(t, 72, frameOf(cards[i]).H, 0, key)

				continue
			}
			rectNear(t, want, frameOf(cards[i]), .1, key)
		}
	})
}

// TestCardHugsItsContentWithPaddingAndBorder checks the size of a card around a
// child: the room's width, the content's height plus the padding, and the 1 px
// border of the locked course's card on each side.
func TestCardHugsItsContentWithPaddingAndBorder(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		tests := []struct {
			name   string
			border bool
			pad    geom.Insets
			wantH  float32
		}{
			{"padding 14", false, geom.Uniform(14), 30 + 28},
			{"padding 16 and border", true, geom.Uniform(16), 30 + 32 + 2},
			{"no padding", false, geom.Insets{}, 30},
		}
		for _, tc := range tests {
			inner := &ui.Box{Height: 30}
			card := kit.NewCard(inner)
			card.Padding, card.Border = tc.pad, tc.border
			mountStage(t, ph, placeAt(card, 20, 100, 353))
			f := frameOf(card)
			assert.InDelta(t, 353, f.W, 0, tc.name)
			assert.InDelta(t, tc.wantH, f.H, 0, tc.name)
			in := frameOf(inner)
			b := float32(0)
			if tc.border {
				b = 1
			}
			assert.InDelta(t, 20+tc.pad.Left+b, in.X, 0, tc.name+": content x")
			assert.InDelta(t, 100+tc.pad.Top+b, in.Y, 0, tc.name+": content y")
			assert.InDelta(t, 353-tc.pad.Left-tc.pad.Right-2*b, in.W, 0, tc.name+": content width")
		}
	})
}

// TestCardPaintsSurfaceAndBorder checks the fill (--sf, 16 rounded) and the 1 px
// --bd border, and the computed style the dump records.
func TestCardPaintsSurfaceAndBorder(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		look := palette()
		plain, bordered := kit.NewCard(nil), kit.NewCard(nil)
		plain.Height, bordered.Height, bordered.Border = 72, 72, true
		tt := mountStage(t, ph, placeAt(plain, 20, 100, 100), placeAt(bordered, 150, 100, 100))
		img := tt.Snapshot()
		pixelNear(t, img, 70, 136, look.T.Surface, 0, "fill")
		pixelNear(t, img, 200, 136, look.T.Surface, 0, "fill inside the border")
		pixelNear(t, img, 150, 136, look.T.Border, 0, "border")
		pixelNear(t, img, 20, 136, look.T.Surface, 0, "no border on the plain card")
		pixelNear(t, img, 21, 101, look.T.Bg, 0, "the corner is rounded")

		ctx := tt.View().Context()
		s, txt := plain.LayoutStyle(ctx)
		assert.Equal(t, "rgb(31, 33, 37)", s.Background)
		assert.Equal(t, "16px", s.Radius)
		assert.Empty(t, s.Border, "a card with no border records none")
		assert.Empty(t, txt)
		s, _ = bordered.LayoutStyle(ctx)
		assert.Equal(t, "1px rgb(51, 54, 61)", s.Border)
		assert.Equal(t, "div.card", plain.Name)

		lay := layoutdump.Dump(ctx, tt.View().Root(), "t", "mobile", ph.Size, nil)
		_, ok := lay.Find("div.card")
		require.True(t, ok)
	})
}

// TestCardPaintDoesNotAllocate checks that a steady repaint of cards allocates
// nothing.
func TestCardPaintDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		c := kit.NewCard(&ui.Box{Height: 20})
		c.Padding, c.Border = geom.Uniform(14), true
		tt := mountStage(t, ph, placeAt(c, 20, 100, 353))
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), ph.Name)
	}
}
