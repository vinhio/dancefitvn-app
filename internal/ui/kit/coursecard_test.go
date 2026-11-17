package kit_test

import (
	"testing"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/poster"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// set35Card returns the card of M03's first course.
func set35Card() *kit.CourseCard {
	c := model.Course{ID: "set-35", Name: "DanceFIT Set 35"}
	card := &kit.CourseCard{
		Poster: poster.New(poster.ForCourse(c), 156, 100),
		Name:   c.Name, Quote: "“Cùng nhảy cùng hát”", Tag: "THƯỜNG · 10 BÀI", Category: kit.CardRegular,
	}
	card.Pressable.Name = "go.detail:set35"

	return card
}

// TestCourseCardBoxesMatchPrototype compares the card, its poster and its quote
// with M03: 156 x 183 with the poster 156 x 100 on top, padded 10 12 12.
func TestCourseCardBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		card := set35Card()
		want := protoBox(t, "M03", "go.detail:set35")
		mountStageReal(t, ph, placeAt(card, want.X, want.Y, 0))
		rectNear(t, want, frameOf(card), 0, "go.detail:set35")
		rectNear(t, protoBox(t, "M03", "div.poster#2"), frameOf(part(t, card, "div.poster")), 0, "poster")
		rectNear(t, protoBox(t, "M03", "div.muted#4"), frameOf(part(t, card, "div.muted")), 1, "quote")
		// the name: 14/700 block, 17 tall, 10 under the poster
		var name geom.Rect
		// (unnamed, as the capture does not record it: the title is the Label
		// that holds the card's name)
		for ch := range card.Children() {
			if l, ok := ch.(*kit.Label); ok && l.Text == card.Name {
				name = frameOf(ch)
			}
		}
		assert.InDelta(t, float64(want.Y+110), float64(name.Y), .01)
		assert.InDelta(t, 17, float64(name.H), 0)
		assert.InDelta(t, float64(want.X+12), float64(name.X), .01)
		assert.InDelta(t, 132, float64(name.W), .01)
	})
}

// TestCourseCardWithoutAQuoteKeepsItsHeight covers `&nbsp;`: a blank line.
func TestCourseCardWithoutAQuoteKeepsItsHeight(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		card := set35Card()
		card.Quote = ""
		mountStageReal(t, ph, placeAt(card, 20, 512, 0))
		assert.InDelta(t, 183, float64(frameOf(card).H), 0)
		assert.InDelta(t, 15, float64(frameOf(part(t, card, "div.muted")).H), 0)
	})
}

// TestCourseCardColors checks the surface, the clip of the poster to the card's
// radius, and the category line's color.
func TestCourseCardColors(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		card := set35Card()
		tt := mountStageReal(t, ph, placeAt(card, 20, 512, 0))
		img := tt.Snapshot()
		pixelNear(t, img, 20+78, 512+150, look.T.Surface, 0, "--sf")
		pixelNear(t, img, 20, 512, look.T.Bg, 12, "rounded 16: the poster does not poke out of the corner")
		pixelNear(t, img, 20+78, 512+183-3, look.T.Surface, 0, "12 of padding at the bottom")

		tag := func() *kit.Label {
			var l *kit.Label
			for ch := range card.Children() {
				if x, ok := ch.(*kit.Label); ok {
					l = x
				}
			}
			return l
		}
		assert.Equal(t, look.C.CatRegular, tag().Color)
		card.Category = kit.CardSpecial
		card.Relayout()
		tt.Elapse(kittest.Frame)
		assert.Equal(t, look.C.CatSpecial, tag().Color)
		card.Category = kit.CardIntro
		card.Relayout()
		tt.Elapse(kittest.Frame)
		assert.Equal(t, look.C.CatIntro, tag().Color)
		assert.Equal(t, "THƯỜNG · 10 BÀI", tag().Text)
	})
}

// TestCourseCardPress checks the tap and the description.
func TestCourseCardPress(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		taps := 0
		card := set35Card()
		card.OnPress = func() { taps++ }
		tt := mountStage(t, ph, placeAt(card, 20, 512, 0))
		require.NoError(t, tt.Click("DanceFIT Set 35"))
		assert.Equal(t, 1, taps)
		card.Width = 200
		card.Relayout()
		tt.Elapse(kittest.Frame)
		assert.InDelta(t, 200, float64(frameOf(card).W), 0)
	})
}

// TestCourseCardDoesNotAllocate checks a steady frame of a strip of cards.
func TestCourseCardDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		tt := mountStage(t, ph, placeAt(set35Card(), 20, 100, 0), placeAt(set35Card(), 188, 100, 0))
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a steady frame", ph.Name)
	}
}
