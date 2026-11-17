// Adapted from jplayer-mobile/e2e/engine/zz_background_ios_test.go at 49de3c2.

//go:build ios && !tvos

package engine

// The file's name runs it last: it sends the app to the background and
// back, through devtest-ios.sh's driver.

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avf"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestZZBackgroundAudio plays the sample MP3, sends the app to the
// background for real (Settings in front: devtest-ios.sh's driver) for
// the phase (15 s), and checks that it played on there (UIBackgroundModes
// audio; suspended, the time would stand still), no time told meanwhile;
// back in the front the tick tells the time at once.
func TestZZBackgroundAudio(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	lc := watchLifecycle(t, h)
	token := h.loadSrc(t, media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	phase := phaseLength()
	p0 := h.readTime(t, token)
	lc.goBackground(t)
	h.drain()
	wait(phase)
	p1 := h.readTime(t, token)
	ticks := 0
	for len(h.events) > 0 {
		if ev := <-h.events; ev.Kind == media.EventTime {
			ticks++
		}
	}
	lc.goForeground(t)
	h.await(t, kind(media.EventTime), time.Second)
	t.Logf("avf-measure: background audio: %v in the background, played %.2f s (%.2f → %.2f), %d ticks there",
		phase, p1-p0, p0, p1, ticks)
	assert.GreaterOrEqual(t, p1-p0, phase.Seconds()-3, "the MP3 played on in the background")
	assert.Zero(t, ticks, "no time told in the background")
}

// TestZZBackgroundVideoTracks plays the HLS, sends the app to the
// background for real: the item's video track is disabled there (the
// sound only is decoded), enabled again in the front, where the frames
// come again.
func TestZZBackgroundVideoTracks(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	lc := watchLifecycle(t, h)
	v := addView(t, h)
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	v.awaitFrames(t, 12, frameWithin, "frames before the background")
	lc.goBackground(t)
	var total, enabled int
	h.onUI(t, func() {
		var ok bool
		total, enabled, ok = avf.VideoTracks(h.eng)
		require.True(t, ok)
	})
	assert.Equal(t, 1, total)
	assert.Zero(t, enabled, "the video track off in the background")
	wait(2 * time.Second)
	v.flush()
	at := lc.goForeground(t)
	v.awaitFrames(t, 1, frameWithin, "a frame back in the front")
	back := time.Since(at)
	h.onUI(t, func() { _, enabled, _ = avf.VideoTracks(h.eng) })
	assert.Equal(t, 1, enabled, "enabled in the front")
	v.awaitFrames(t, 12, frameWithin, "frames in the front")
	t.Logf("avf-measure: background video: track off there, first frame %v after active", back.Round(time.Millisecond))
}
