package kit_test

import (
	"testing"

	"dancefitvn-app/internal/avatar"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// splitImage returns a w x h picture whose left half is red and right half blue.
func splitImage(t testing.TB, w, h int) *scene.Image {
	t.Helper()
	img, err := scene.NewImage(w, h)
	require.NoError(t, err)
	for y := range h {
		for x := range w {
			o := y*img.Stride + 4*x
			if x < w/2 {
				img.Pix[o], img.Pix[o+3] = 255, 255
			} else {
				img.Pix[o+2], img.Pix[o+3] = 255, 255
			}
		}
	}
	img.Version = 1

	return img
}

// TestCropViewBoxesMatchPrototype checks the frame: 353 wide in the sheet, 230 tall
// (`.crop`, M15D [20,393,353,230]).
func TestCropViewBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		cv := kit.NewCropView(avatar.Initial)
		mountStage(t, ph, placeAt(cv, 20, 393, 353))
		assert.Equal(t, geom.XYWH(20, 393, 353, 230), frameOf(cv))
		assert.Equal(t, "crop", cv.Name)
		assert.True(t, cv.DragExempt())
	})
}

// TestCropViewPaint checks what the frame shows at zoom 1: the picture's centred
// square in the 260 box, #2A2D33 around it, the mask (black at .55) outside the hole of
// radius 100, the ring (white, 2.5, inside the 200 circle), and the ring's inside clear.
func TestCropViewPaint(t *testing.T) {
	t.Parallel()
	frame := color.RGB(0x2A, 0x2D, 0x33)
	black := color.RGBA(0, 0, 0, .55)
	red, blue := color.RGB(255, 0, 0), color.RGB(0, 0, 255)
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		// a 200 x 100 picture: its centred square is 100 x 100, red left, blue right
		cv := kit.NewCropView(avatar.Picked)
		cv.SetImage(splitImage(t, 200, 100))
		tt := mountStage(t, ph, placeAt(cv, 20, 393, 353))
		img := tt.Snapshot()
		cx, cy := 20+176, 393+115 // the ring's centre, from the frame's 176.5, 115
		pixelNear(t, img, cx-60, cy, red, 4, "inside the ring, left of the middle")
		pixelNear(t, img, cx+60, cy, blue, 4, "inside the ring, right of the middle")
		// the box is 260 across, from the frame's 46.5 to 306.5; the hole is 100 round
		pixelNear(t, img, 20+60, cy, over(black, red), 6, "inside the box, under the mask")
		pixelNear(t, img, 20+30, cy, over(black, frame), 6, "left of the box: the frame, under the mask")
		pixelNear(t, img, 20+330, cy, over(black, frame), 6, "right of the box")
		pixelNear(t, img, cx-20, 393+5, over(black, red), 6, "the top of the frame, the box over it, outside the ring")
		pixelNear(t, img, cx-100+1, cy, color.White, 12, "the ring's left edge")
		pixelNear(t, img, cx, cy-99, color.White, 12, "the ring's top")
		pixelNear(t, img, cx-95, cy, red, 12, "just inside the ring's stroke")
		// the rounded corner of the frame: 16 round, so its corner pixel is the stage's
		pixelNear(t, img, 20, 393, kittestBg(), 8, "the frame's corner is rounded")
	})
}

// kittestBg is --bg, what the stage paints under the frame.
func kittestBg() color.Color { return palette().T.Bg }

// TestCropViewMovesAndZooms checks the picture's place: moved by (x, y) and scaled by
// the zoom about its centre.
func TestCropViewMovesAndZooms(t *testing.T) {
	t.Parallel()
	red, blue := color.RGB(255, 0, 0), color.RGB(0, 0, 255)
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		cv := kit.NewCropView(avatar.View{Zoom: 1, X: 50, Y: 0})
		cv.SetImage(splitImage(t, 100, 100)) // red | blue split at the box's middle, 130
		tt := mountStage(t, ph, placeAt(cv, 20, 393, 353))
		cx, cy := 20+176, 393+115
		// moved 50 right: the split is at 176.5 + 50 = 226.5
		pixelNear(t, tt.Snapshot(), cx+40, cy, red, 4, "left of the moved split, inside the ring")
		pixelNear(t, tt.Snapshot(), cx+60, cy, blue, 4, "right of it")
		cv.SetView(avatar.View{Zoom: 2, X: -50, Y: 0}) // the box is 520 wide, its split at 176.5 - 50
		tt.Elapse(kittest.Frame)
		pixelNear(t, tt.Snapshot(), cx-60, cy, red, 4, "zoomed, left of the split")
		pixelNear(t, tt.Snapshot(), cx-40, cy, blue, 4, "zoomed, right of the split")
	})
}

// TestCropViewFigure checks the picture when there is none: the person figure over a
// square of #DCD8D2, not a disc, cut by the frame, under the same mask.
func TestCropViewFigure(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		cv := kit.NewCropView(avatar.Initial)
		tt := mountStage(t, ph, placeAt(cv, 20, 393, 353))
		img := tt.Snapshot()
		cx, cy := 20+176, 393+115
		// the corner of the 312 box (zoom 1.2) inside the frame is the square's, not a disc's
		pixelNear(t, img, cx-150, 393+3, over(color.RGBA(0, 0, 0, .55), color.RGB(0xdc, 0xd8, 0xd2)), 10, "the figure's square reaches the frame's corner")
		pixelNear(t, img, cx-70, cy-60, color.RGB(0xdc, 0xd8, 0xd2), 10, "the figure's background inside the ring")
	})
}

// TestCropViewDrag checks the finger: each move adds what the pointer moved, with no
// limit; OnMove hears the view; a press takes the pointer (no sheet or scroll follows).
func TestCropViewDrag(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		cv := kit.NewCropView(avatar.View{Zoom: 1.5, X: 5, Y: -5})
		var heard []avatar.View
		cv.OnMove = func(v avatar.View) { heard = append(heard, v) }
		tt := mountStage(t, ph, placeAt(cv, 20, 393, 353))
		c := frameOf(cv).Center()
		tt.TouchDown(c)
		assert.True(t, cv.Dragging())
		tt.TouchMove(c.Add(geom.Pt(10, 20)))
		tt.TouchMove(c.Add(geom.Pt(-300, 400))) // far outside the frame
		tt.TouchUp(c.Add(geom.Pt(-300, 400)))
		assert.False(t, cv.Dragging())
		assert.Equal(t, avatar.View{Zoom: 1.5, X: -295, Y: 395}, cv.View, "no limit, the zoom is the slider's")
		require.Len(t, heard, 2)
		assert.Equal(t, avatar.View{Zoom: 1.5, X: 15, Y: 15}, heard[0])
		// the scroll of a surrounding page does not take it: the press is grabbed
		assert.Equal(t, platform.CursorMove, cv.Cursor())
	})
}

// TestSliderBoxesMatchPrototype checks the slider against the `input[type=range]` of
// M15D: the track is the 6 tall `input` box, the thumb 20 round and red.
func TestSliderBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewSlider(1, 3, .01, 1.2)
		tt := mountStage(t, ph, placeAt(s, 40, 660, 310.13))
		track := part(t, s, "input")
		rectNear(t, geom.XYWH(40, 669, 310.13, 6), frameOf(track), .01, "input")
		img := tt.Snapshot()
		pixelNear(t, img, 40+200, 672, look.T.Surface2, 3, "the track")
		pixelNear(t, img, 79, 670, look.T.Red, 3, "the thumb at 1.2: 10 + .1 of the 290.13 run")
		// the thumb is 20 round: its top at 660 and the corner clear
		pixelNear(t, img, 79, 661, look.T.Red, 6, "the thumb's top")
		pixelNear(t, img, 70, 661, look.T.Bg, 8, "the thumb's corner is round")
		assert.True(t, s.DragExempt())
	})
}

// TestSliderSetsTheValue checks the user's changes: OnChange hears a press and a drag
// at steps of .01, within 1 and 3; SetValue is silent.
func TestSliderSetsTheValue(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewSlider(1, 3, .01, 1.2)
		var heard []float64
		s.OnChange = func(v float64) { heard = append(heard, v) }
		tt := mountStage(t, ph, placeAt(s, 40, 660, 310.13))
		f := frameOf(s)
		at := func(z float64) geom.Point {
			return geom.Pt(f.X+10+float32((z-1)/2)*(f.W-20), f.Y+10)
		}
		tt.Drag(at(2), at(2))
		assert.InDelta(t, 2, s.Value, .0035) // a step of .01 is 1.45 DIPs of the run
		tt.Drag(at(2), geom.Pt(f.Max().X+60, f.Y))
		assert.InDelta(t, 3, s.Value, 0)
		tt.Drag(at(3), geom.Pt(f.X-60, f.Y))
		assert.InDelta(t, 1, s.Value, 0)
		require.NotEmpty(t, heard)
		for _, v := range heard {
			assert.GreaterOrEqual(t, v, 1.0)
			assert.LessOrEqual(t, v, 3.0)
			assert.InDelta(t, 0, (v-1)*100-float64(int((v-1)*100+.5)), 1e-6, "on a step of .01")
		}
		n := len(heard)
		assert.True(t, s.SetValue(2.5))
		assert.Len(t, heard, n, "SetValue is silent")
		tt.View().Focus(s)
		require.True(t, tt.Key(keys.KeyLeft, 0))
		assert.InDelta(t, 2.49, s.Value, 1e-9)
	})
}

// TestTextAreaBoxesAndLook checks the text area: 80 tall, the room's width, --sf with a
// 1 px --bd border that turns --pr with the focus, 14 rounded, the text 15 in.
func TestTextAreaBoxesAndLook(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		a := kit.NewTextArea("")
		a.SetText("Huấn luyện viên DanceFIT & StepX tại Đà Nẵng.")
		tt := mountStage(t, ph, placeAt(a, 20, 212, 353))
		assert.Equal(t, geom.XYWH(20, 212, 353, 80), frameOf(a))
		img := tt.Snapshot()
		pixelNear(t, img, 20+100, 212+70, look.T.Surface, 3, "the box")
		pixelNear(t, img, 20+100, 212, look.T.Border, 6, "the top border")
		tt.View().Focus(a)
		tt.Elapse(kittest.Frame)
		pixelNear(t, tt.Snapshot(), 20+100, 212, look.T.Primary, 6, "the focus ring")
		assert.True(t, a.DragExempt())
		tt.Type("!")
		assert.Contains(t, a.Text(), "!")
	})
}
