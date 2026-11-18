//go:build tvos

package tvkit_test

import (
	"fmt"
	"image/color"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the two lists the Slide carries (index.html:672-679, layout
// of TV05 and TV06), as the fixtures lay them out.
const (
	// gridCol, gridRowPitch, gridPadTop: `#tvGrid{grid-template-columns:
	// repeat(3,540px);gap:40px 60px;padding:10px 0 60px}`, cells 304 + 24 + the
	// 34 and 26 of the caption and the line below it + 8 + the 40 of the gap: a
	// row every 436 (TV05 cg0 y 360, cg3 y 796).
	gridColW, gridCellH, gridRowPitch, gridPadTop float32 = 540, 304, 436, 10
	// listRow, listPitch, listPad are `.trow` 88 tall with 12 below it in
	// `#tvList{padding:8px}`; headH is the heading, 26 px and a 24 margin, as
	// TV06 places the first row (dl0 y 173 in a clip at 110).
	listRow, listPitch, listPad, headH float32 = 88, 100, 8, 55
)

// gridContent is `#tvGrid`: n cells of 540 by 304 in three columns.
type gridContent struct {
	ui.Base
	cells []*swatch
}

// listContent is `#tvList`: n rows 100 apart.
type listContent struct {
	ui.Base
	rows []*swatch
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newGrid returns the grid of n cells named cg0.. .
func newGrid(n int) *gridContent {
	g := &gridContent{}
	for i := range n {
		s := &swatch{col: color.RGBA{uint8(40 + 10*i), 90, 160, 255}, w: gridColW, h: gridCellH}
		s.Key, s.Name = fmt.Sprintf("cg%d", i), fmt.Sprintf("cg%d", i)
		g.cells = append(g.cells, s)
		g.Add(s)
	}

	return g
}

// newList returns the list of n rows named dl0.. , w wide.
func newList(n int, w float32) *listContent {
	l := &listContent{}
	for i := range n {
		s := &swatch{col: color.RGBA{200, uint8(20 * i), 60, 255}, w: w - 2*listPad, h: listRow}
		s.Key, s.Name = fmt.Sprintf("dl%d", i), fmt.Sprintf("dl%d", i)
		l.rows = append(l.rows, s)
		l.Add(s)
	}

	return l
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout puts the cells in three columns, 60 apart, a row every 436.
func (g *gridContent) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	rows := (len(g.cells) + 2) / 3
	for i, s := range g.cells {
		c.Layout(s, ui.Tight(geom.Sz(gridColW, gridCellH)))
		c.Place(s, geom.Pt(float32(i%3)*(gridColW+60), gridPadTop+float32(i/3)*gridRowPitch))
	}

	return cs.Constrain(geom.Sz(cs.Max.W, gridPadTop+float32(rows)*gridRowPitch+20))
}

// Layout stacks the rows 100 apart in a padding of 8.
func (l *listContent) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	for i, s := range l.rows {
		c.Layout(s, ui.Tight(geom.Sz(cs.Max.W-2*listPad, listRow)))
		c.Place(s, geom.Pt(listPad, listPad+float32(i)*listPitch))
	}

	return cs.Constrain(geom.Sz(cs.Max.W, 2*listPad+float32(len(l.rows))*listPitch))
}

// gridRig is TV05's grid in a Slide at (90, 350), 1830 by 730.
func gridRig(t testing.TB) (*tvkit.Slide, *gridContent, *ui.Tester) {
	t.Helper()
	g := newGrid(15)
	sl := tvkit.NewSlide(nil, g)
	root := &stage{}
	root.putSized(sl, 90, 350, 1830, 730)
	tt := mount(t, root)

	return sl, g, tt
}

// listRig is TV06's list in a Slide at (960, 110), 870 by 970, under a heading.
func listRig(t testing.TB) (*tvkit.Slide, *listContent, *swatch, *ui.Tester) {
	t.Helper()
	l := newList(10, 870)
	head := &swatch{col: color.RGBA{255, 208, 40, 255}, w: 870, h: headH}
	sl := tvkit.NewSlide(head, l)
	root := &stage{}
	root.putSized(sl, 960, 110, 870, 970)
	tt := mount(t, root)

	return sl, l, head, tt
}

// TestSlideOffsetsOfTV05 pins the offsets of the courses grid: row i puts
// `max(0, top - (730 - 304 - 80))` with top = 10 + 436 i (F-10).
func TestSlideOffsetsOfTV05(t *testing.T) {
	t.Parallel()
	sl, g, tt := gridRig(t)
	require.Zero(t, sl.Offset())
	want := []float32{0, 100, 536, 972, 1408}
	for row, w := range want {
		tt.View().Focus(g.cells[row*3+1])
		assert.Equal(t, w, sl.Offset(), "row %d", row)
		tt.Elapse(tvkit.SlideTime)
		// the geometry the focus picks by is final: the cell stands at its
		// document place less the offset
		f := g.cells[row*3+1].Node().WindowFrame()
		assert.InDelta(t, 350+gridPadTop+float32(row)*gridRowPitch-w, f.Y, .01, "row %d y", row)
	}
	// going back up lowers the offset by the same rule; sideways keeps it
	tt.View().Focus(g.cells[3])
	assert.Equal(t, float32(100), sl.Offset())
	tt.View().Focus(g.cells[4])
	assert.Equal(t, float32(100), sl.Offset())
}

// TestSlideSpanKeepsMoreThanTheItem: with Span the offset keeps the span in
// view in place of the item's own height (K45): a span of 396 (tile 304 and the
// two lines under it) puts row 1 at 446 - (730 - 396 - 80) = 192; the bleed
// widens the clip without moving the box.
func TestSlideSpanKeepsMoreThanTheItem(t *testing.T) {
	t.Parallel()
	sl, g, tt := gridRig(t)
	sl.Span = func(ui.Widget) float32 { return 396 }
	sl.BleedLeft, sl.BleedTop = 24, 20
	box := sl.WindowFrame()
	tt.View().Focus(g.cells[3])
	assert.Equal(t, float32(192), sl.Offset())
	tt.View().Focus(g.cells[0])
	assert.Zero(t, sl.Offset())
	assert.Equal(t, box, sl.WindowFrame(), "the bleed is the clip's, not the box's")
}

// TestSlideOffsetsOfTV06: the rows of the lesson list at 55 + 8 + 100 i give
// 61 for dl8 and 161 for dl9, relative to a clip 970 tall (F-10).
func TestSlideOffsetsOfTV06(t *testing.T) {
	t.Parallel()
	sl, l, _, tt := listRig(t)
	for i := range 8 {
		tt.View().Focus(l.rows[i])
		assert.Zero(t, sl.Offset(), "dl%d fits", i)
	}
	tt.View().Focus(l.rows[8])
	assert.Equal(t, float32(61), sl.Offset())
	tt.View().Focus(l.rows[9])
	assert.Equal(t, float32(161), sl.Offset(), "unclamped: a Scroll would stop at 109")
}

// TestSlideGlides: the offset is final at once, the picture glides there over
// 250 ms with CSS `ease`; a focus outside the content leaves it alone.
func TestSlideGlides(t *testing.T) {
	t.Parallel()
	sl, g, tt := gridRig(t)
	tt.View().Focus(g.cells[3])
	require.Equal(t, float32(100), sl.Offset())
	assert.True(t, sl.Gliding())
	assert.Zero(t, sl.Shown(), "the glide starts where it was")
	tt.Elapse(125 * time.Millisecond)
	mid := sl.Shown()
	assert.Greater(t, mid, float32(50), "CSS ease is past the middle at half the time (0.8)")
	assert.Less(t, mid, float32(100))
	tt.Elapse(125 * time.Millisecond)
	assert.Equal(t, float32(100), sl.Shown())
	assert.False(t, sl.Gliding())
	assert.Equal(t, 250*time.Millisecond, tvkit.SlideTime)

	// the focus leaves the grid: the offset stays (index.html:729 acts only when
	// the grid contains the element)
	other := &swatch{w: 100, h: 100}
	other.Key = "cf0"
	root := tt.View().Root().(*stage)
	root.put(other, 0, 0)
	tt.Elapse(tvtest.Frame)
	tt.View().Focus(other)
	assert.Equal(t, float32(100), sl.Offset())
}

// TestSlideReplacingTheContentJumps: a re-render starts at the offset of the
// focused item, with no glide, and at 0 when the focus is elsewhere.
func TestSlideReplacingTheContentJumps(t *testing.T) {
	t.Parallel()
	sl, g, tt := gridRig(t)
	tt.View().Focus(g.cells[9])
	tt.Elapse(tvkit.SlideTime)
	require.Equal(t, float32(972), sl.Offset())

	fresh := newGrid(15)
	sl.SetContent(fresh)
	tt.Elapse(tvtest.Frame)
	assert.Zero(t, sl.Offset(), "the focus is gone with the old content: 0")
	assert.Zero(t, sl.Shown())
	assert.False(t, sl.Gliding())

	tt.View().Focus(fresh.cells[12])
	tt.Elapse(tvkit.SlideTime)
	require.Equal(t, float32(1408), sl.Offset())
	// the same cell of a content replaced while it holds the focus: no glide
	again := newGrid(15)
	sl.SetContent(again)
	tt.Elapse(tvtest.Frame)
	assert.Zero(t, sl.Offset())
	assert.Same(t, again, sl.Content())
}

// TestSlideDrawsTheListOverTheHeading: the clip holds the heading and the
// list, the list slides over it (F-10), and nothing shows outside the clip.
func TestSlideDrawsTheListOverTheHeading(t *testing.T) {
	t.Parallel()
	sl, l, _, tt := listRig(t)
	tt.View().Focus(l.rows[9])
	tt.Elapse(tvkit.SlideTime + tvtest.Frame)
	require.Equal(t, float32(161), sl.Offset())
	img := tt.Snapshot()
	at := func(x, y int) color.RGBA {
		r, g, b, a := img.At(x, y).RGBA()

		return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
	}
	// dl1 stands at 63 + 100 - 161 = 2 in the clip: it covers the heading's pixels
	y := 110 + 20
	assert.Equal(t, color.RGBA{200, 20, 60, 255}, at(960+100, y), "dl1 over the heading")
	assert.Equal(t, color.RGBA{255, 208, 40, 255}, at(960+4, y), "the heading is at the side of the row (padding 8)")
	assert.Zero(t, at(960+100, 109).A, "above the clip")
}

// TestSlideFollowsKeys: the keys of the grid walk the cells and the offset
// follows them.
func TestSlideFollowsKeys(t *testing.T) {
	t.Parallel()
	sl, g, tt := gridRig(t)
	tt.View().Focus(g.cells[0])
	for row := 1; row < 5; row++ {
		require.True(t, tt.Key(keys.KeyDown, 0))
		assert.Same(t, g.cells[row*3], tt.View().Focused())
		assert.Equal(t, []float32{0, 100, 536, 972, 1408}[row], sl.Offset())
	}
	assert.False(t, tt.Key(keys.KeyDown, 0))
	require.True(t, tt.Key(keys.KeyUp, 0))
	assert.Equal(t, float32(972), sl.Offset())
}

// TestSlideGlideDoesNotAllocate: a Down and an Up along the grid with the
// frames of the glide allocate nothing.
func TestSlideGlideDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	_, g, tt := gridRig(t)
	tt.View().Focus(g.cells[0])
	tt.Elapse(time.Second)
	down := true
	step := func() {
		k := keys.KeyDown
		if !down {
			k = keys.KeyUp
		}
		down = !down
		tt.Key(k, 0)
		tt.Elapse(40 * time.Millisecond)
	}
	for range 12 {
		step()
	}
	assert.InDelta(t, 0, testing.AllocsPerRun(50, step), 0)
}

// BenchmarkTVFocusMove is a Down/Up pair along a grid of 15 cells in a Slide.
func BenchmarkTVFocusMove(b *testing.B) {
	_, g, tt := gridRig(b)
	tt.View().Focus(g.cells[0])
	tt.Elapse(time.Second)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		k := keys.KeyDown
		if i%2 == 1 {
			k = keys.KeyUp
		}
		tt.Key(k, 0)
		tt.Elapse(40 * time.Millisecond)
	}
}
