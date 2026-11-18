package kit_test

import (
	"testing"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/poster"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// set35Tile returns the grid tile of the course of M03's first card.
func set35Tile() *kit.CourseTile {
	c := model.Course{ID: "set-35", Name: "DanceFIT Set 35"}
	ct := &kit.CourseTile{
		Poster: poster.New(poster.ForCourse(c), 156, 100),
		Name:   c.Name, Tag: "THƯỜNG · 10 BÀI", Category: kit.CardRegular,
	}
	ct.Pressable.Name = "go.detail:set35"

	return ct
}

// TestCourseTileScalesThePosterToItsWidth checks K39: the poster is as wide as the tile
// with the 156:100 aspect, the name 10 under it, 12 in from each side, and the tile as
// tall as the poster, 10, one line of 17, 6, the strut's 20 and 12.
func TestCourseTileScalesThePosterToItsWidth(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, w := range []float32{170.5, 180, 156} {
			ct := set35Tile()
			mountStageReal(t, ph, placeAt(ct, 20, 300, w))
			f := frameOf(ct)
			assert.InDelta(t, float64(w), float64(f.W), 0.001, "width")
			po := frameOf(ct.Poster)
			assert.InDelta(t, float64(w), float64(po.W), 0.001, "poster width")
			assert.InDelta(t, float64(w*100/156), float64(po.H), 0.001, "poster 156:100")
			assert.InDelta(t, float64(po.H+10+17+6+20+12), float64(f.H), 0.5, "height")
			assert.InDelta(t, float64(kit.PosterHeight(w)), float64(po.H), 0.001)
		}
	})
}

// TestCourseTileNameTakesTwoLinesAndNoMore checks the name: one line short, two lines
// with an ellipsis when long (a row's evenness is checked on Home).
func TestCourseTileNameTakesTwoLinesAndNoMore(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		short, long := set35Tile(), set35Tile()
		long.Name = "DanceFIT Set 34 Một Tên Khoá Học Rất Dài Không Thể Vừa Trong Một Dòng Hay Hai Dòng Của Cột Này"
		mountStageReal(t, ph, placeAt(short, 20, 300, 170.5), placeAt(long, 202.5, 300, 170.5))
		assert.InDelta(t, 17, float64(frameOf(nameOf(t, short)).H), 0.01)
		assert.InDelta(t, 34, float64(frameOf(nameOf(t, long)).H), 0.01, "two lines, then no more")
		assert.InDelta(t, 17, float64(frameOf(long).H-frameOf(short).H), 0.5)
	})
}

// TestCourseTileColorsAndPress checks the category line's colors and the tap.
func TestCourseTileColorsAndPress(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		taps := 0
		ct := set35Tile()
		ct.OnPress = func() { taps++ }
		tt := mountStageReal(t, ph, placeAt(ct, 20, 300, 170.5))
		require.NoError(t, tt.Click("DanceFIT Set 35"))
		assert.Equal(t, 1, taps)
		tag := func() *kit.Label { return tagOf(t, ct) }
		assert.Equal(t, look.C.CatRegular, tag().Color)
		ct.Category = kit.CardSpecial
		ct.Relayout()
		tt.Elapse(kittest.Frame)
		assert.Equal(t, look.C.CatSpecial, tag().Color)
		ct.Category = kit.CardIntro
		ct.Relayout()
		tt.Elapse(kittest.Frame)
		assert.Equal(t, look.C.CatIntro, tag().Color)
		assert.Equal(t, "THƯỜNG · 10 BÀI", tag().Text)
		assert.Equal(t, "DanceFIT Set 35", ct.AccessibleLabel())
	})
}

// TestCourseTileDoesNotAllocate checks a steady frame of two tiles.
func TestCourseTileDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		tt := mountStageReal(t, ph, placeAt(set35Tile(), 20, 100, 170.5), placeAt(set35Tile(), 202.5, 100, 170.5))
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a steady frame", ph.Name)
	}
}

// nameOf returns the name label of a tile: the Label holding its Name.
func nameOf(t testing.TB, ct *kit.CourseTile) *kit.Label {
	t.Helper()
	for ch := range ct.Children() {
		if l, ok := ch.(*kit.Label); ok && l.Text == ct.Name {
			return l
		}
	}
	require.FailNow(t, "no name label")

	return nil
}

// tagOf returns the category label of a tile: the last Label.
func tagOf(t testing.TB, ct *kit.CourseTile) *kit.Label {
	t.Helper()
	var tag *kit.Label
	for ch := range ct.Children() {
		if l, ok := ch.(*kit.Label); ok {
			tag = l
		}
	}
	require.NotNil(t, tag)

	return tag
}
