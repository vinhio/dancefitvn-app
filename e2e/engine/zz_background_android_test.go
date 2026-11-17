// Adapted from jplayer-mobile/e2e/engine/zz_background_android_test.go at
// 49de3c2.
// Changed: the mock's long HLS (480×854, 24 fps, with sound) and the
// sample MP3 for clip720p60.mp4 and tone.mp3; the position read with
// ReadTime (no loop on the end: the long playlist outlasts the phases);
// the screen-off phase left out on a real phone (onPhone: its keyguard is
// its owner's), and the procs lines left out; the audio's own test as the
// iPhone's zz_background_ios_test.go.

//go:build android && cgo

package engine

// The file's name runs it last: it sends the app to the background and
// back, through devtest-android.sh's driver.

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/amp"
	"fmt"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// q4Limit is the most CPU of the process, in percent of a core, in the
// background with a video playing (jPlayer's Q4 of M9).
const q4Limit = 3.0

// cpuWindow is the CPU time the process used in a window of wall time.
type cpuWindow struct{ wall, cpu time.Duration }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestZZBackgroundAudio plays the sample MP3 with the media service in
// the foreground (as the app's Now Playing has it), sends the app to the
// background for real (the Home key) for the phase, and checks that it
// played on there, no time told meanwhile; back in front the tick tells
// the time at once.
func TestZZBackgroundAudio(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	lc := watchLifecycle(t, h)
	nowPlaying(t, h)
	token := h.loadSrc(t, media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	phase := min(phaseLength(), 25*time.Second)
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
	t.Logf("amp-measure: background audio: %v in the background, played %.2f s (%.2f → %.2f), %d ticks there",
		phase, p1-p0, p0, p1, ticks)
	assert.GreaterOrEqual(t, p1-p0, phase.Seconds()-3, "the MP3 played on in the background")
	assert.Zero(t, ticks, "no time told in the background")
}

// TestZZBackgroundVideo measures what the lesson's video playing in the
// background costs: its frames watched in front; then the app in the
// background for real (the Home key; on the emulator the screen off too),
// the media service in the foreground. The player gives its Surface up
// there (the video has sound), so the decoder of the picture stops while
// the sound plays on. The numbers go to the log; the test fails over
// q4Limit.
func TestZZBackgroundVideo(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	lc := watchLifecycle(t, h)
	frames := h.watchFrames(t)
	nowPlaying(t, h)
	phase := phaseLength()
	token := h.load(t, theMedia.Long("l_3503", 40, time.Now().Add(time.Hour)), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	awaitSeq(t, frames, 0, frameWithin)
	played := func(d time.Duration) (cpuWindow, float64, amp.Stats) {
		t0, s0 := h.readTime(t, token), stats(t, h)
		w := measureCPU(d)
		t1, s1 := h.readTime(t, token), stats(t, h)
		s1.Frames -= s0.Frames
		s1.CopiedFrames -= s0.CopiedFrames
		s1.DroppedFrames -= s0.DroppedFrames

		return w, t1 - t0, s1
	}
	fg, fgPlayed, fgStats := played(phase)

	lc.goBackground(t)
	home, homePlayed, homeStats := played(phase)

	off, offPlayed, offStats := cpuWindow{}, phase.Seconds(), amp.Stats{}
	if !onPhone() {
		_, ok := driver(t, "sleep")
		require.True(t, ok)
		off, offPlayed, offStats = played(phase)
		_, ok = driver(t, "wake")
		require.True(t, ok)
	}

	drainSeqs(frames)
	at := lc.goForeground(t)
	awaitSeq(t, frames, 0, 3*time.Second)
	back := time.Since(at)

	line := fmt.Sprintf("amp-bg: phase %v; foreground %.1f%% (played %.1f s, frames %d copied %d); "+
		"background %.1f%% (played %.1f s, frames %d copied %d dropped %d); "+
		"screen off %.1f%% (played %.1f s, frames %d copied %d dropped %d; skipped on a phone: %v); first frame back %v",
		phase, fg.percent(), fgPlayed, fgStats.Frames, fgStats.CopiedFrames,
		home.percent(), homePlayed, homeStats.Frames, homeStats.CopiedFrames, homeStats.DroppedFrames,
		off.percent(), offPlayed, offStats.Frames, offStats.CopiedFrames, offStats.DroppedFrames, onPhone(),
		back.Round(time.Millisecond))
	fmt.Println(line)
	t.Log(line)
	for _, p := range []struct {
		name   string
		played float64
	}{{"foreground", fgPlayed}, {"background", homePlayed}, {"screen off", offPlayed}} {
		assert.InDelta(t, phase.Seconds(), p.played, 0.15*phase.Seconds()+1, "%s: the media plays on", p.name)
	}
	assert.LessOrEqual(t, homeStats.CopiedFrames, int64(1), "no frame converted in the background")
	assert.LessOrEqual(t, homeStats.Frames, int64(3), "the decoder of the picture stopped in the background")
	assert.LessOrEqual(t, offStats.Frames, int64(3), "the decoder of the picture stopped, screen off")
	assert.LessOrEqual(t, home.percent(), q4Limit, "a video in the background, the app's CPU")
	assert.LessOrEqual(t, off.percent(), q4Limit, "a video with the screen off, the app's CPU")
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// nowPlaying puts the media service in the foreground as the app's Now
// Playing does (nowplaying.Set), until the test ends: without it the
// system freezes the process in the background.
func nowPlaying(t *testing.T, h *harness) {
	t.Helper()

	var np platform.NowPlayingDriver
	h.onUI(t, func() {
		var ok bool
		np, ok = h.app.Driver().(platform.NowPlayingDriver)
		require.True(t, ok, "the Android driver has Now Playing")
		require.NoError(t, np.SetNowPlaying(platform.NowPlaying{
			Title: "e2e", Duration: 60 * time.Second, Rate: 1, State: platform.PlaybackPlaying,
		}))
	})
	t.Cleanup(func() { h.onUI(t, func() { _ = np.ClearNowPlaying() }) })
}

// measureCPU returns the CPU time the process used in a window of d, on
// the test's goroutine, which only waits meanwhile.
func measureCPU(d time.Duration) cpuWindow {
	c0, t0 := cpuTicks(), time.Now()
	wait(d)

	return cpuWindow{wall: time.Since(t0), cpu: time.Duration(cpuTicks()-c0) * 10 * time.Millisecond}
}

// percent is the window's CPU time as a percent of one core.
func (w cpuWindow) percent() float64 {
	if w.wall <= 0 {
		return 0
	}

	return 100 * float64(w.cpu) / float64(w.wall)
}
