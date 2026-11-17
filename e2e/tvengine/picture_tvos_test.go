//go:build ios && tvos

package tvengine

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avftv"
	"fmt"
	"image"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// pictureWithin is how long after Playing the first frame may be told
// (P7.5a: the stage over the picture, K5, hides on it).
const pictureWithin = 2 * time.Second

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestTVFirstFrame checks the picture-ready event of the Apple TV's
// engine (P7.5a): an EventVideoSize, once a load with a picture, of that
// load's token, with the size the item presents, when the layer shows
// its first frame (readyForDisplay), within pictureWithin of Playing;
// none for a sound, none while the view did not place the picture (then
// one as it does), one for a load that stays paused.
func TestTVFirstFrame(t *testing.T) { //nolint:paralleltest // one app a process
	t.Run("auto", func(t *testing.T) {
		h := newHarness(t)
		start := time.Now()
		token := h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true})
		var playing, picture time.Duration
		var size media.Event
		h.await(t, func(ev media.Event) bool {
			switch {
			case ev.Token != token:
			case ev.Kind == media.EventState && ev.State == media.StatePlaying && playing == 0:
				playing = time.Since(start)
			case ev.Kind == media.EventVideoSize:
				picture, size = time.Since(start), ev
			default:
			}

			return playing > 0 && picture > 0
		}, loadWithin)
		var ready, ok bool
		onUI(t, func() { ready, ok = avftv.LayerReadyForDisplay(h.eng) })
		assert.True(t, ok && ready, "the layer is ready for display once told")
		assert.Positive(t, size.Width)
		assert.Positive(t, size.Height)
		assert.InDelta(t, 9.0/16, float64(size.Width)/float64(size.Height), 0.01, "the lesson's 9:16, %dx%d", size.Width, size.Height)
		assert.Less(t, picture-playing, pictureWithin, "the first frame within %v of Playing", pictureWithin)
		var luma float64
		if img, shotOK := shot(t, "first-frame"); shotOK {
			luma = meanLuma(img, paneRect)
			assert.Greater(t, luma, 5.0, "a picture in the pane, not the layer's black")
		}
		measure("tv-first-frame", "load_to_playing", playing.Round(time.Millisecond), "load_to_picture", picture.Round(time.Millisecond),
			"picture_after_playing", (picture - playing).Round(time.Millisecond), "size", fmt.Sprintf("%dx%d", size.Width, size.Height),
			"pane_luma", fmt.Sprintf("%.1f", luma))
		h.expectNone(t, kind(media.EventVideoSize), 2*time.Second)

		// The next load of a picture tells its own first frame.
		again := time.Now()
		next := h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true, Start: 5})
		ev := h.await(t, kind(media.EventVideoSize), loadWithin)
		assert.Equal(t, next, ev.Token, "the new load's token")
		measure("tv-first-frame-next", "load_to_picture", time.Since(again).Round(time.Millisecond))
		h.expectNone(t, kind(media.EventVideoSize), time.Second)
	})
	t.Run("paused", func(t *testing.T) {
		h := newHarness(t)
		start := time.Now()
		token := h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{})
		ev := h.await(t, kind(media.EventVideoSize), loadWithin)
		assert.Equal(t, token, ev.Token)
		measure("tv-first-frame-paused", "load_to_picture", time.Since(start).Round(time.Millisecond))
		h.expectNone(t, state(media.StatePlaying), 500*time.Millisecond)
	})
	t.Run("sound", func(t *testing.T) {
		h := newHarness(t)
		h.load(t, media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Auto: true})
		h.await(t, state(media.StatePlaying), playingWithin)
		h.expectNone(t, kind(media.EventVideoSize), 2*time.Second)
	})
	t.Run("placed later", func(t *testing.T) {
		h := newHarness(t)
		onUI(t, func() { tv.root.pane.show(nil) })
		token := h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true})
		h.await(t, state(media.StatePlaying), playingWithin)
		h.expectNone(t, kind(media.EventVideoSize), time.Second)
		placed := time.Now()
		onUI(t, func() { tv.root.pane.show(h.eng) })
		ev := h.await(t, kind(media.EventVideoSize), pictureWithin)
		assert.Equal(t, token, ev.Token)
		measure("tv-first-frame-placed-later", "place_to_picture", time.Since(placed).Round(time.Millisecond))
	})
	t.Run("stopped", func(t *testing.T) {
		h := newHarness(t)
		h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true})
		onUI(t, func() { require.NoError(t, h.eng.Stop()) })
		h.drain()
		h.expectNone(t, kind(media.EventVideoSize), 2*time.Second)
	})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// meanLuma is the mean grey level of img over r, in points of the window
// (the shot's pixels are its scale times them).
func meanLuma(img image.Image, r geom.Rect) float64 {
	scale := float32(img.Bounds().Dx()) / full.W
	box := image.Rect(int(r.X*scale), int(r.Y*scale), int((r.X+r.W)*scale), int((r.Y+r.H)*scale))
	var sum, n float64
	for y := box.Min.Y; y < box.Max.Y; y += 4 {
		for x := box.Min.X; x < box.Max.X; x += 4 {
			sum += luma(img, x, y)
			n++
		}
	}
	if n == 0 {
		return 0
	}

	return sum / n
}
