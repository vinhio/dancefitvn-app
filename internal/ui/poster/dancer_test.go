package poster_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/poster"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dancerBox is a widget that draws DrawDancer into its own box.
type dancerBox struct {
	ui.Base

	w, h   float32
	ink    color.Color
	stroke float32
}

// Layout takes the size asked for.
func (d *dancerBox) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Sz(d.w, d.h))
}

// Paint draws the figure over the box.
func (d *dancerBox) Paint(_ *ui.Context, p *ui.Painter) {
	poster.DrawDancer(p, d.Bounds(), d.ink, d.stroke)
}

// TestDrawDancerDrawsTheFigure checks, on both phones, that the splash's
// 190 high dancer puts ink at its head (52, 17 of the 100 box) and on its body
// line, centred in a wider box, and nowhere outside its square.
func TestDrawDancerDrawsTheFigure(t *testing.T) {
	t.Parallel()

	ink := color.RGB(255, 208, 40)
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		d := &dancerBox{w: 300, h: 190, ink: ink, stroke: 9}
		root := &ui.Box{Padding: geom.Insets{Left: 20, Top: 100}}
		root.Add(d)
		tt := kittest.Mount(t, ph, root)
		tt.SetScale(1)
		img := tt.Snapshot()
		at := func(x, y float32) color.Color {
			o := int(y)*img.Stride + int(x)*4

			return color.RGB(img.Pix[o], img.Pix[o+1], img.Pix[o+2])
		}
		// The square is 190 wide, centred in the 300: its left edge is at 20 + 55.
		left, top, k := float32(20+55), float32(100), float32(1.9)
		head := at(left+52*k, top+17*k)
		assert.Equal(t, ink, head, "the head")
		assert.Equal(t, ink, at(left+48*k, top+47*k), "the body line from (50,30) to (46,58)")
		assert.NotEqual(t, ink, at(left+5*k, top+50*k), "empty corner of the square")
		assert.NotEqual(t, ink, at(20+20, top+50*k), "left of the square (the box is wider)")
	})
}

// TestDrawDancerEmptyBoxDrawsNothing checks that a box with no room draws no
// ink.
func TestDrawDancerEmptyBoxDrawsNothing(t *testing.T) {
	t.Parallel()

	ph := kittest.Phones[1]
	d := &dancerBox{w: 0, h: 0, ink: color.RGB(255, 208, 40), stroke: 9}
	root := &ui.Box{}
	root.Add(d)
	tt := kittest.Mount(t, ph, root)
	require.NotNil(t, tt.Snapshot())
	assert.Zero(t, d.Bounds().W)
}

// TestDrawDancerDoesNotAllocate checks that a steady frame of the figure
// allocates nothing.
func TestDrawDancerDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		d := &dancerBox{w: 190, h: 190, ink: color.RGB(255, 208, 40), stroke: 9}
		root := &ui.Box{}
		root.Add(d)
		tt := kittest.Mount(t, ph, root)
		for range 3 {
			tt.Elapse(kittest.Frame)
		}
		assert.Zero(t, testing.AllocsPerRun(20, func() {
			root.Repaint()
			tt.Elapse(kittest.Frame)
		}), ph.Name)
	}
}
