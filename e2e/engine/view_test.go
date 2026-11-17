// Adapted from jplayer-mobile/e2e/engine/avf_ios_test.go at 49de3c2 (loneView) and cmd/avspike/video.go (framePane: the fitted, mirrored draw).

//go:build (ios && !tvos) || (android && cgo)

package engine

import (
	"dancefitvn-app/internal/media"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The view's size: the lesson's 9:16, small enough to leave the status
// line above it.
const viewW, viewH = 270, 480

// videoView shows an engine's surface alone in the window, where nothing
// else moves, as the video screen will (P4.4 kit.VideoView): black, the
// newest frame fitted inside, mirrored about its vertical axis when asked
// (04 §2.3, a flipped draw: the engine knows nothing of it). While the
// surface is Active it pulls at each paint and asks for the next; else a
// paint follows a frame the surface announces. It counts its paints and
// hands each new frame it drew to frames. UI thread.
type videoView struct {
	ui.Base
	surf   *media.Surface
	mirror bool
	// seen is the last sequence number drawn; paints how many times the
	// view painted.
	seen   uint64
	paints int
	// frames receives each new sequence number drawn, unless it is full.
	frames chan uint64

	unwatch, unsub func()
	// p, img and dst are what drawImage reads inside Transform; drawFn
	// and onFrameFn are made once, so no paint allocates a closure.
	p         *ui.Painter
	img       *scene.Image
	dst       geom.Rect
	drawFn    func()
	onFrameFn func(uint64)
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// addView puts a videoView of the engine's surface in the window, taken
// out when the test ends, and waits until it stopped painting.
func addView(t *testing.T, h *harness) *videoView {
	t.Helper()

	v := &videoView{surf: h.eng.Video(), frames: make(chan uint64, 4096)}
	v.Name = "video"
	v.drawFn = v.drawImage
	v.onFrameFn = func(uint64) { v.Repaint() }
	h.onUI(t, func() { theScreen.Add(v) })
	t.Cleanup(func() { h.onUI(t, func() { theScreen.Remove(v) }) })
	v.awaitStill(t, h)

	return v
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout takes the view's 9:16 box.
func (v *videoView) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Size{W: viewW, H: viewH})
}

// Mounted starts the interest in the surface's frames.
func (v *videoView) Mounted(*ui.Context) {
	if v.surf == nil || v.unwatch != nil {
		return
	}
	v.unwatch = v.surf.Watch()
	v.unsub = v.surf.Frames().Subscribe(v.onFrameFn)
}

// Unmounted ends it.
func (v *videoView) Unmounted(*ui.Context) {
	if v.unwatch == nil {
		return
	}
	v.unsub()
	v.unwatch()
	v.unsub, v.unwatch = nil, nil
}

// Paint draws black, then the newest frame fitted inside, mirrored when
// asked, and hands on the frame if it is new.
func (v *videoView) Paint(c *ui.Context, p *ui.Painter) {
	b := v.Bounds()
	p.Fill(b, scene.Solid(color.RGB(0, 0, 0)), 0)
	v.paints++
	s := v.surf
	if s == nil {
		return
	}
	if s.Active() && s.Pulls() {
		s.Pull()
		v.Repaint()
	}
	img, seq := s.Frame(paintNumber(c))
	if img != nil {
		v.drawFrame(p, img, b)
	}
	if seq != 0 && seq != v.seen {
		v.seen = seq
		select {
		case v.frames <- seq:
		default:
		}
	}
}

// setMirror mirrors the picture, or not, and repaints. UI thread.
func (v *videoView) setMirror(on bool) {
	v.mirror = on
	v.Repaint()
}

// awaitStill waits until the view has painted at least once and then not
// for stillFor, so that what paints it next is the engine.
func (v *videoView) awaitStill(t *testing.T, h *harness) {
	t.Helper()

	tick := time.NewTicker(stillFor / 6)
	defer tick.Stop()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	last, since := -1, time.Now()
	for {
		var n int
		h.onUI(t, func() { n = v.paints })
		if n != last {
			last, since = n, time.Now()
		} else if n > 0 && time.Since(since) >= stillFor {
			return
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			require.FailNow(t, "the view does not stop painting", "%d paints", n)
		}
	}
}

// awaitFrames waits until the view has drawn n new frames.
func (v *videoView) awaitFrames(t *testing.T, n int, within time.Duration, what string) {
	t.Helper()

	timer := time.NewTimer(within)
	defer timer.Stop()
	for got := 0; got < n; {
		select {
		case <-v.frames:
			got++
		case <-timer.C:
			require.FailNow(t, "the view drew no frame: "+what, "%d of %d within %v", got, n, within)
		}
	}
}

// awaitNoFrame fails the test when the view draws a new frame within d.
func (v *videoView) awaitNoFrame(t *testing.T, d time.Duration, what string) {
	t.Helper()

	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case seq := <-v.frames:
		assert.Fail(t, "the view drew a frame "+what, "sequence %d", seq)
	case <-timer.C:
	}
}

// flush drops the frames handed on so far.
func (v *videoView) flush() {
	for len(v.frames) > 0 {
		<-v.frames
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// drawFrame draws img fitted inside box, mirrored about the vertical axis
// through the centre of the picture when asked: x' = 2·cx − x, a literal
// affine map, no allocation.
func (v *videoView) drawFrame(p *ui.Painter, img *scene.Image, box geom.Rect) {
	dst := fitContain(img.Width, img.Height, box)
	if dst.Empty() {
		return
	}
	if !v.mirror {
		p.Image(img, dst, 1)

		return
	}
	v.p, v.img, v.dst = p, img, dst
	p.Transform(scene.Affine{XX: -1, YY: 1, X0: 2 * (dst.X + dst.W/2)}, v.drawFn)
	v.p, v.img = nil, nil
}

// drawImage draws the image of this paint at dst, inside drawFrame's
// Transform.
func (v *videoView) drawImage() { v.p.Image(v.img, v.dst, 1) }

// paintNumber is the number of the paint under way, for Surface.Frame.
func paintNumber(c *ui.Context) uint64 {
	if c == nil || c.View() == nil {
		return 0
	}

	return uint64(c.View().FramesDrawn()) //nolint:gosec // G115: a count of frames, never negative
}

// fitContain returns the largest rectangle of w×h's proportions centred in
// box (object-fit: contain); empty for an empty image.
func fitContain(w, h int, box geom.Rect) geom.Rect {
	if w <= 0 || h <= 0 || box.Empty() {
		return geom.Rect{}
	}
	sw, sh := box.W, box.W*float32(h)/float32(w)
	if sh > box.H {
		sw, sh = box.H*float32(w)/float32(h), box.H
	}

	return geom.XYWH(box.X+(box.W-sw)/2, box.Y+(box.H-sh)/2, sw, sh)
}
