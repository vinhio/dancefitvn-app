package kit_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScrubPressSeeks checks that a press anywhere on the bar asks for that share of
// the width, once, and that nothing follows the finger (no drag, Q-P4-15).
func TestScrubPressSeeks(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var got []float32
		s := kit.NewScrub("")
		s.OnSeek = func(share float32) { got = append(got, share) }
		tt := mountStage(t, ph, placeAt(s, 20, 700, 353))
		f := frameOf(s)
		assert.Equal(t, geom.Sz(353, 18), f.Size())
		assert.Equal(t, "scrub", s.Name)

		tt.ClickAt(geom.Pt(f.X+f.W/4, f.Y+9))
		require.Len(t, got, 1)
		assert.InDelta(t, .25, got[0], 1e-4)
		tt.ClickAt(geom.Pt(f.X+0, f.Y+9))
		assert.InDelta(t, 0, got[1], 1e-4)
		tt.ClickAt(geom.Pt(f.X+f.W-.5, f.Y+9))
		assert.InDelta(t, 1, got[2], .01)
		got = got[:0]
		tt.Drag(geom.Pt(f.X+20, f.Y+9), geom.Pt(f.X+200, f.Y+9))
		assert.Len(t, got, 1, "a drag is one press")
	})
}

// TestScrubPaints checks the track, the fill, the knob and the band in pixels.
func TestScrubPaints(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewScrub("")
		tt := mountStage(t, ph, placeAt(s, 20, 700, 353))
		f := frameOf(s)
		y := f.Y + 9
		tt.Frame()
		track := vvPixel(tt, geom.Pt(f.X+200, y))
		assert.Greater(t, track.R, uint8(60), "white at .3 over --bg")

		s.SetPos(.5)
		s.SetBand(true, .1, .3)
		tt.Frame()
		fill := vvPixel(tt, geom.Pt(f.X+100, y))
		assert.Equal(t, uint8(255), fill.R, "--pr")
		assert.Equal(t, uint8(0xd0), fill.G)
		knob := vvPixel(tt, geom.Pt(f.X+f.W/2, f.Y+3))
		assert.Equal(t, uint8(255), knob.R, "the knob is 14 high, from 2")
		above := vvPixel(tt, geom.Pt(f.X+100, f.Y+1))
		assert.NotEqual(t, uint8(255), above.R, "the fill is 4 high: nothing at 1")
		band := vvPixel(tt, geom.Pt(f.X+f.W*.2, f.Y+6))
		assert.Greater(t, band.R, band.G, "the band is red at .7")
		on, a, b := s.Band()
		assert.True(t, on)
		assert.InDelta(t, .1, a, 1e-6)
		assert.InDelta(t, .3, b, 1e-6)
		s.SetBand(false, 0, 0)
		tt.Frame()
		gone := vvPixel(tt, geom.Pt(f.X+f.W*.2, f.Y+6))
		assert.NotEqual(t, band, gone)
	})
}

// TestScrubSetPosDoesNotAllocate checks that moving the bar and painting it does not
// allocate.
func TestScrubSetPosDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	s := kit.NewScrub("")
	tt := mountStage(t, kittest.Phones[0], placeAt(s, 20, 700, 353))
	p := float32(0)
	step := func() {
		p += .003
		if p > 1 {
			p = 0
		}
		s.SetPos(p)
		s.SetBand(true, .2, .4)
		tt.Frame()
	}
	for range 10 {
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(100, step))
}

// TestBeatsBoxesAndLit checks the bar of eight beats: 24 tall, equal cells 5 apart,
// the numbers 1 to 8, and --red under white for the lit one.
func TestBeatsBoxesAndLit(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		b := kit.NewBeats()
		tt := mountStageReal(t, ph, placeAt(b, 20, 680, 353))
		f := frameOf(b)
		assert.Equal(t, geom.Sz(353, 24), f.Size())
		cw := float32((353 - 7*5) / 8.0)
		for i := range kit.BeatCount {
			x := f.X + float32(i)*(cw+5) + cw/2
			c := vvPixel(tt, geom.Pt(x, f.Y+2))
			if i == 0 {
				assert.Equal(t, uint8(0xe5), c.R, "cell 1 is lit: --red")
				assert.Equal(t, uint8(0x38), c.G)
			} else {
				assert.NotEqual(t, uint8(0xe5), c.R, "cell %d is not lit", i+1)
			}
		}
		assert.True(t, tt.Shows("1") && tt.Shows("8"), "numbered 1 to 8")
		b.SetLit(5)
		tt.Frame()
		assert.Equal(t, 5, b.Lit())
		c := vvPixel(tt, geom.Pt(f.X+5*(cw+5)+cw/2, f.Y+2))
		assert.Equal(t, uint8(0xe5), c.R)
		first := vvPixel(tt, geom.Pt(f.X+cw/2, f.Y+2))
		assert.NotEqual(t, uint8(0xe5), first.R, "the first one is off again")
		b.SetLit(-1)
		tt.Frame()
		assert.Equal(t, -1, b.Lit(), "none lit for a value out of range")
	})
}

// TestBeatsSetLitDoesNotAllocate checks a new lit cell and its paint allocate nothing.
func TestBeatsSetLitDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	b := kit.NewBeats()
	tt := mountStage(t, kittest.Phones[0], placeAt(b, 20, 680, 353))
	i := 0
	step := func() {
		i = (i + 1) % kit.BeatCount
		b.SetLit(i)
		tt.Frame()
	}
	for range 10 {
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(100, step))
}

// TestSideButtonBoxesAndState checks a button of the side column: 46 by 62, the icon
// 22 in the middle of the disc, black at .45 and white, --pr and --on when on.
func TestSideButtonBoxesAndState(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		b := kit.NewSideButton(icon.Mirror, "Gương")
		b.Name = "mirror"
		presses := 0
		b.OnPress = func() { presses++ }
		tt := mountStageReal(t, ph, placeAt(b, 333, 378, 0))
		f := frameOf(b)
		assert.Equal(t, geom.Sz(46, 62), f.Size())
		ic := frameOf(part(t, b, "svg.i"))
		assert.Equal(t, geom.Rect{X: 345, Y: 390, W: 22, H: 22}, ic)

		off := vvPixel(tt, geom.Pt(f.X+5, f.Y+23))
		assert.Less(t, off.R, uint8(20), "black at .45 over --bg")
		b.SetOn(true)
		tt.Frame()
		on := vvPixel(tt, geom.Pt(f.X+5, f.Y+23))
		assert.Equal(t, uint8(255), on.R, "--pr")
		assert.Equal(t, uint8(0xd0), on.G)
		corner := vvPixel(tt, geom.Pt(f.X+1, f.Y+1))
		assert.NotEqual(t, uint8(255), corner.R, "a disc, not a square")

		tt.ClickAt(f.Center())
		assert.Equal(t, 1, presses)
		tt.ClickAt(geom.Pt(f.X+23, f.Y+57))
		assert.Equal(t, 2, presses, "the label is part of the button")

		wide := kit.NewSideButton(icon.Rep, "Một nhãn rất dài hơn nút")
		kittest.MountReal(t, ph, newStage(placeAt(wide, 200, 200, 0)))
		assert.Greater(t, frameOf(wide).W, float32(46), "a longer label widens the button")
	})
}

// TestDiscButtonPlay checks the play button: 58, --pr, a glyph 26 wide in --on, with no
// name for the glyph, and the press.
func TestDiscButtonPlay(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		d := kit.NewDiscButton(icon.Play, "Phát")
		d.Name = "togglePlay"
		presses := 0
		d.OnPress = func() { presses++ }
		tt := mountStage(t, ph, placeAt(d, 167.2, 760, 0))
		f := frameOf(d)
		assert.Equal(t, geom.Sz(58, 58), f.Size())
		c := vvPixel(tt, geom.Pt(f.X+4, f.Y+29))
		assert.Equal(t, uint8(255), c.R, "--pr")
		corner := vvPixel(tt, geom.Pt(f.X+1, f.Y+1))
		assert.NotEqual(t, uint8(255), corner.R)
		var glyph geom.Rect
		for ch := range d.Children() {
			glyph = frameOf(ch)
			assert.Empty(t, ch.Node().Name, "the PLAY svg has no class, layout.json does not record it")
		}
		assert.Equal(t, geom.Sz(26, 26), glyph.Size())
		assert.Equal(t, f.Center(), glyph.Center())
		tt.ClickAt(f.Center())
		assert.Equal(t, 1, presses)
		d.SetIcon(icon.Pause)
		tt.Elapse(time.Millisecond)
		assert.Equal(t, icon.Pause, d.Icon)
	})
}

// TestStagePaints checks the K5 stage in pixels: the gradient from #3A1A1C at the top
// left, the glow over the middle, the six stripes of the wall, the floor's black over
// the bottom 28 %.
func TestStagePaints(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewStage()
		tt := kittest.Mount(t, ph, newStage(placeAt(sized(s, ph.Size.W, ph.Size.H), 0, 0, 0)))
		w, h := ph.Size.W, ph.Size.H
		tl := vvPixel(tt, geom.Pt(1, 1))
		assert.InDelta(t, 0x3a, float64(tl.R), 6, "the top left is the first color of the gradient")
		assert.InDelta(t, 0x1a, float64(tl.G), 6)
		glow := vvPixel(tt, geom.Pt(w*.5, h*.35))
		assert.Greater(t, glow.G, uint8(60), "the yellow glow lifts the green over the middle")
		wallX := vvPixel(tt, geom.Pt(w*.1, h*.5))
		stripe := vvPixel(tt, geom.Pt(w*.1575, h*.5))
		assert.Greater(t, stripe.R, wallX.R, "a stripe of white at .05 from 15.5 to 16 %")
		last := vvPixel(tt, geom.Pt(w*.9575, h*.5))
		near := vvPixel(tt, geom.Pt(w*.93, h*.5))
		assert.Greater(t, last.R, near.R, "the sixth stripe, at 95.5 %")
		wall := vvPixel(tt, geom.Pt(w*.1, h*.71))
		floor := vvPixel(tt, geom.Pt(w*.1, h*.73))
		assert.Less(t, floor.R, wall.R, "the floor darkens from 72 %")
	})
}

// TestButtonsDrawTheChangeAtOnce checks the buttons of the video's controls that
// change without a layout (the symbol and the ink keep their size): the icon child
// carries the new symbol or ink after the very next frame, with nothing else moving
// (a video paused, a load that failed).
func TestButtonsDrawTheChangeAtOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		d := kit.NewDiscButton(icon.Pause, "Tạm dừng")
		side := kit.NewSideButton(icon.Mirror, "Gương")
		side.Name = "mirror"
		ib := kit.NewIconButton(icon.Eye, "Hiện mật khẩu")
		tt := mountStage(t, ph, placeAt(d, 167, 700, 0), placeAt(side, 300, 378, 0), placeAt(ib, 20, 300, 0))
		tt.Frame()
		glyphOf := func(w ui.Widget) *kit.Icon {
			for c := range w.Node().Children() {
				if ic, ok := c.(*kit.Icon); ok {
					return ic
				}
			}
			require.Fail(t, "no icon child")

			return nil
		}

		d.SetIcon(icon.Play)
		tt.Frame()
		assert.Equal(t, icon.Play, glyphOf(d).Glyph, "the disc draws PLAY")
		d.SetIcon(icon.Pause)
		tt.Frame()
		assert.Equal(t, icon.Pause, glyphOf(d).Glyph)

		require.Equal(t, theme.InkWhite, glyphOf(side).Ink)
		side.SetOn(true)
		tt.Frame()
		assert.Equal(t, theme.InkOnPrimary, glyphOf(side).Ink, "the icon is --on over the --pr disc")
		side.SetOn(false)
		tt.Frame()
		assert.Equal(t, theme.InkWhite, glyphOf(side).Ink)

		ib.SetIcon(icon.Speed)
		tt.Frame()
		assert.Equal(t, icon.Speed, glyphOf(ib).Glyph)
	})
}
