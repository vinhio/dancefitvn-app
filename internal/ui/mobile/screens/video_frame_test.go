//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/np/video_alloc_test.go at 49de3c2 (a
// producer that fills the back image, publishes and announces, then the display
// paints) and from cmd/avspike/video_test.go (the flip, checked in pixels).

package screens_test

import (
	"image/color"
	"testing"
	"time"

	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The picture the tests publish: 160 by 90 pixels, red on the left half and blue
// on the right.
const (
	frameW, frameH = 160, 90
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// publishHalves makes the producer publish a frame whose left half is red and whose
// right half is blue, and announce it.
func publishHalves(tb testing.TB, s *media.Surface) {
	tb.Helper()
	img := s.Back(frameW, frameH, false)
	require.NotNil(tb, img)
	for y := range frameH {
		for x := range frameW {
			o := y*img.Stride + x*4
			if x < frameW/2 {
				img.Pix[o], img.Pix[o+1], img.Pix[o+2] = 255, 0, 0
			} else {
				img.Pix[o], img.Pix[o+1], img.Pix[o+2] = 0, 0, 255
			}
			img.Pix[o+3] = 255
		}
	}
	s.Publish()
	s.Notify()
}

// pixelAt reads the window's pixel.
func pixelAt(r *shelltest.Rig, p geom.Point) color.RGBA {
	return r.Tester.Snapshot().RGBAAt(int(p.X), int(p.Y))
}

// TestVideoMirrorFlipsThePicture checks "Lật gương" in pixels, on both phones: the
// picture of the engine, fitted whole into the window (Q-P4-2), is flipped left to
// right while the mirror is on, and not once it is off; the bars above and below it
// are black.
func TestVideoMirrorFlipsThePicture(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		pb.Pause()
		r.Settle()
		publishHalves(t, pb.Video())
		r.Tester.Elapse(kittest.Frame)
		mid := ph.Size.H / 2
		left, right := geom.Pt(60, mid), geom.Pt(290, mid)
		assert.True(t, pb.Mirror(), "the prototype starts with the mirror on")
		assert.Equal(t, color.RGBA{R: 0, G: 0, B: 255, A: 255}, pixelAt(r, left), "mirrored: the left of the window shows the right half")
		assert.Equal(t, color.RGBA{R: 255, G: 0, B: 0, A: 255}, pixelAt(r, right), "mirrored: the right shows the left half")
		assert.Equal(t, color.RGBA{A: 255}, pixelAt(r, geom.Pt(60, 200)), "contained: a bar of black above the picture")

		tap(r, named(r.Shell, "mirror", 1))
		r.Tester.Elapse(kittest.Frame)
		assert.False(t, pb.Mirror())
		assert.Equal(t, color.RGBA{R: 255, G: 0, B: 0, A: 255}, pixelAt(r, left), "not mirrored: as the engine drew it")
		assert.Equal(t, color.RGBA{R: 0, G: 0, B: 255, A: 255}, pixelAt(r, right))
	})
}

// TestVideoStageShowsUntilTheFirstFrame checks K5: before the engine has a frame the
// stand-in stage shows (not black); once it has, the picture and the bars of black.
func TestVideoStageShowsUntilTheFirstFrame(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		pb.Pause()
		r.Tester.Elapse(kittest.Frame)
		p := geom.Pt(60, 220)
		assert.NotEqual(t, color.RGBA{A: 255}, pixelAt(r, p), "the stage")
		publishHalves(t, pb.Video())
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, color.RGBA{A: 255}, pixelAt(r, p), "the bar above the picture")
	})
}

// TestVideoScreenPlayingFrameDoesNotAllocate is the gate of a frame of screen 06
// while it plays, on both phones: a new video frame, the 250 ms tick of the player
// (the clock, the bar, the lit beat, the speed) and the paint, with the controls
// shown and with them hidden, allocate nothing.
func TestVideoScreenPlayingFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		for _, hidden := range []bool{false, true} {
			r, pb, v := atVideo(t, ph, false)
			s := pb.Video()
			restart(r, pb)
			if hidden {
				r.Tester.Elapse(ms3s + 300*time.Millisecond)
				require.False(t, v.Shown())
			}
			pos := 10.0
			step := func() {
				img := s.Back(frameW, frameH, true)
				img.Pix[0]++
				s.Publish()
				s.Notify()
				pos += .25
				if pos > 40 {
					pos = 10
				}
				pb.Tick(pos)
				r.Tester.Frame()
			}
			for range 200 { // every second's text and every beat is shaped once
				step()
			}
			name := ph.Name + " shown"
			if hidden {
				name = ph.Name + " hidden"
			}
			assert.Zero(t, testing.AllocsPerRun(100, step), name+": a new frame, a tick and a paint")
			steady := func() {
				r.Tester.Repaint()
			}
			steady()
			assert.Zero(t, testing.AllocsPerRun(100, steady), name+": a paint with nothing new")
		}
	}
}

// BenchmarkVideoScreenFrame is a frame of screen 06 playing: a new picture, a tick of
// the player and the paint, controls shown.
func BenchmarkVideoScreenFrame(b *testing.B) {
	for _, ph := range kittest.Phones {
		b.Run(ph.Name, func(b *testing.B) {
			r, pb, _ := atVideo(b, ph, false)
			s := pb.Video()
			restart(r, pb)
			pos := 10.0
			step := func() {
				img := s.Back(frameW, frameH, true)
				img.Pix[0]++
				s.Publish()
				s.Notify()
				pos += .25
				if pos > 40 {
					pos = 10
				}
				pb.Tick(pos)
				r.Tester.Frame()
			}
			for range 200 {
				step()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				step()
			}
		})
	}
}
