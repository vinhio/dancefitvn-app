// Adapted from jplayer-mobile/e2e/engine/amp_android_test.go and
// amp_wifi_android_test.go at 49de3c2.
// Changed: the media are the mock's (the lesson's HLS, 480×854 at 24 fps
// with sound, 12 s, and the sample MP3, 30 s) served on 127.0.0.1 by
// e2emedia, for jPlayer's local tone.mp3, clip.mp4 and clip720p60.mp4;
// the groups of avf_ios_test.go added (speeds of the sheet read back with
// amp.PlaybackParams, ±10 seeks, A–B loop, expired links, mirror by
// screenshot, the paused start's frame); the prober, the rotated and
// broken clips, sources.json and dumpsys wifi left out; the keep-up
// measured at 1× and 1.5× in a view that paints the frames.

//go:build android && cgo

package engine

import (
	"context"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/amp"
	"dancefitvn-app/internal/media/enginekit"
	"fmt"
	"image"
	"math"
	"os"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The speeds of the options sheet (docs/plan/04-media-engine.md §2.2).
var rates = []float64{0.5, 0.75, 1, 1.25, 1.5}

// The measures' margins.
const (
	// rateSlack is how far the speed measured may be from the rate set,
	// as a share of it (P4.2: ±3 %, as P4.1).
	rateSlack = 0.03
	// hlsRateSlack is rateSlack for the HLS over the 3 s window: NuPlayer's
	// HLS position runs up to 8 % fast in the first seconds after a seek and
	// play at 0.5× and 0.75× (Galaxy A17, API 36: 0.54, 0.78), then slower
	// (8 s: 0.49, 0.73); the rate set reads back exact and the MP3 plays at
	// it to 0.1 %. TestAMPRatesLong measures the long window.
	hlsRateSlack = 0.10
	// longRateSlack is the long window's margin: its position stalls a
	// little at the segments' ends (8 s at 1×: 0.970).
	longRateSlack = 0.05
	// seekSlack is how far a seek may land from its target (SEEK_CLOSEST).
	seekSlack = 0.05
	// playingWithin is how soon a load of the local mock plays.
	playingWithin = 5 * time.Second
	// firstFrameBackAMP is how soon a frame must come once the app is back
	// in the foreground, the Surface given back to a player that had none in
	// the background.
	firstFrameBackAMP = 2 * time.Second
	// keepUp is the share of the frames copied the view must paint
	// (Q-P0.7-3: shown/copied), and at 1× of the source's the engine must
	// copy; above 1× MediaPlayer's renderer drops late frames itself.
	keepUp = 0.95
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestAMPPlaysHLS plays the lesson's HLS from the mock's server: its
// length and size, playing within playingWithin, RGBA frames of 480×854,
// and the times from Load to playing and to the first frame.
func TestAMPPlaysHLS(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	setStatus("amp: HLS")
	frames := h.watchFrames(t)
	start := time.Now()
	token := h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	var dur, size media.Event
	h.await(t, func(ev media.Event) bool {
		switch ev.Kind {
		case media.EventDuration:
			dur = ev
		case media.EventVideoSize:
			size = ev
		default:
		}

		return dur.Kind != media.EventUnknown && size.Kind != media.EventUnknown
	}, loadWithin)
	assert.Equal(t, token, dur.Token)
	assert.InDelta(t, hlsLength, dur.Duration, 0.1)
	assert.Equal(t, [2]int{hlsW, hlsH}, [2]int{size.Width, size.Height})
	h.await(t, state(media.StatePlaying), playingWithin)
	playing := time.Since(start)
	awaitSeq(t, frames, 0, frameWithin)
	first := time.Since(start)
	h.onUI(t, func() {
		img, seq := h.eng.Video().Frame(0)
		require.NotNil(t, img)
		assert.Positive(t, seq)
		assert.Equal(t, [2]int{hlsW, hlsH}, [2]int{img.Width, img.Height})
		assert.False(t, img.BGRA, "RGBA")
	})
	s := stats(t, h)
	assert.Zero(t, s.BadFrames)
	t.Logf("amp-measure: hls load→playing %v, load→first frame %v; frames %d copied %d, convert p50 %v p95 %v",
		playing.Round(time.Millisecond), first.Round(time.Millisecond), s.Frames, s.CopiedFrames, s.ConvertP50, s.ConvertP95)
}

// TestAMPPlaysMP3 plays the sample MP3 as audio only: its length, the
// time going on, no frame, and the end after a seek near it.
func TestAMPPlaysMP3(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	setStatus("amp: MP3")
	start := time.Now()
	h.loadSrc(t, media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Auto: true})
	dur := h.await(t, kind(media.EventDuration), loadWithin)
	assert.InDelta(t, mp3Length, dur.Duration, 0.3)
	h.await(t, state(media.StatePlaying), playingWithin)
	playing := time.Since(start)
	t0 := h.await(t, func(ev media.Event) bool { return ev.Kind == media.EventTime && ev.Time > 0.2 }, 2*time.Second).Time
	t1 := h.await(t, func(ev media.Event) bool { return ev.Kind == media.EventTime && ev.Time > t0+0.2 }, 2*time.Second).Time
	assert.Greater(t, t1, t0)
	h.onUI(t, func() {
		img, _ := h.eng.Video().Frame(0)
		assert.Nil(t, img, "no picture for audio")
	})
	h.onUI(t, func() { require.NoError(t, h.eng.Seek(mp3Length-2)) })
	h.await(t, kind(media.EventEnded), 5*time.Second)
	t.Logf("amp-measure: mp3 load→playing %v", playing.Round(time.Millisecond))
}

// TestAMPRates plays the HLS and the MP3 at each speed of the sheet and
// measures the position against the wall clock over 3 s, from 1 s after
// Playing (the positions of the first 4 s are logged): the MP3 within
// ±3 % of the rate, the HLS within hlsRateSlack; the player's
// PlaybackParams read back the speed and the pitch kept at 1.
func TestAMPRates(t *testing.T) { //nolint:paralleltest // one app a process
	for _, src := range []media.Source{{URL: mediaURL(hlsMaster)}, {URL: mediaURL(mp3Sample), AudioOnly: true}} {
		name := "hls"
		if src.AudioOnly {
			name = "mp3"
		}
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			token := h.loadSrc(t, src, media.LoadOptions{})
			h.await(t, state(media.StatePaused), loadWithin)
			for _, r := range rates {
				setStatus(fmt.Sprintf("amp: %s at %.2fx", name, r))
				h.onUI(t, func() {
					require.NoError(t, h.eng.Pause())
					require.NoError(t, h.eng.Seek(0.5))
					require.NoError(t, h.eng.SetRate(r))
					require.NoError(t, h.eng.Play())
				})
				h.await(t, state(media.StatePlaying), playingWithin)
				w := time.Now()
				var trace []string
				var p0, p1 float64
				var w0, w1 time.Time
				for i := range 17 { // every 250 ms for 4 s: the first 3 s window starts 1 s in
					if i > 0 {
						wait(time.Until(w.Add(time.Duration(i) * 250 * time.Millisecond)))
					}
					p, at := h.readTime(t, token), time.Now()
					trace = append(trace, fmt.Sprintf("%.0fms:%.3f", at.Sub(w).Seconds()*1000, p))
					switch i {
					case 4:
						p0, w0 = p, at
					case 16:
						p1, w1 = p, at
					default:
					}
				}
				speed := (p1 - p0) / w1.Sub(w0).Seconds()
				t.Logf("amp-measure: %s rate %.2f: positions after Playing %v", name, r, trace)
				var sp, pitch float32
				h.onUI(t, func() {
					var ok bool
					sp, pitch, ok = amp.PlaybackParams(h.eng)
					require.True(t, ok)
				})
				t.Logf("amp-measure: %s rate %.2f: speed %.3f (Δpos %.3f / Δwall %.3f), PlaybackParams speed %.2f pitch %.2f",
					name, r, speed, p1-p0, w1.Sub(w0).Seconds(), sp, pitch)
				slack := rateSlack
				if !src.AudioOnly {
					slack = hlsRateSlack
				}
				assert.InDelta(t, r, speed, r*slack, "speed at %.2f", r)
				assert.InDelta(t, r, float64(sp), 0.001)
				assert.InDelta(t, 1, float64(pitch), 0.001, "the pitch kept")
				h.onUI(t, func() { require.NoError(t, h.eng.Pause()) })
				h.await(t, state(media.StatePaused), 2*time.Second)
			}
		})
	}
}

// TestAMPRatesLong plays the long HLS (the lesson four times) at each
// speed for 8 s, from 1 s after Playing: the position's speed and the
// video's (frames from the decoder, 24 a second of media) over the long
// window, logged; the position's within longRateSlack.
func TestAMPRatesLong(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	token := h.load(t, theMedia.Long("l_3503", 4, time.Now().Add(time.Hour)), media.LoadOptions{})
	h.await(t, state(media.StatePaused), loadWithin)
	for _, r := range rates {
		setStatus(fmt.Sprintf("amp: long hls at %.2fx", r))
		h.onUI(t, func() {
			require.NoError(t, h.eng.Pause())
			require.NoError(t, h.eng.Seek(0.5))
			require.NoError(t, h.eng.SetRate(r))
			require.NoError(t, h.eng.Play())
		})
		h.await(t, state(media.StatePlaying), playingWithin)
		wait(time.Second)
		p0, s0, w0 := h.readTime(t, token), stats(t, h), time.Now()
		wait(8 * time.Second)
		p1, s1, w1 := h.readTime(t, token), stats(t, h), time.Now()
		wall := w1.Sub(w0).Seconds()
		speed := (p1 - p0) / wall
		video := float64(s1.Frames-s0.Frames) / 24 / wall
		t.Logf("amp-measure: long hls rate %.2f: position speed %.3f (Δpos %.3f / Δwall %.3f), video speed %.3f (%d frames)",
			r, speed, p1-p0, wall, video, s1.Frames-s0.Frames)
		assert.InDelta(t, r, speed, r*longRateSlack, "speed at %.2f", r)
		h.onUI(t, func() { require.NoError(t, h.eng.Pause()) })
		h.await(t, state(media.StatePaused), 2*time.Second)
	}
}

// TestAMPRateWhilePausedDoesNotStart sets the speed of a paused player:
// it stays paused (setPlaybackParams would start it), the position does
// not move, and it plays at 1.5× once played.
func TestAMPRateWhilePausedDoesNotStart(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	token := h.load(t, mediaURL(hlsMaster), media.LoadOptions{})
	h.await(t, state(media.StatePaused), loadWithin)
	h.onUI(t, func() { require.NoError(t, h.eng.SetRate(1.5)) })
	h.expectNone(t, state(media.StatePlaying), 1500*time.Millisecond)
	assert.InDelta(t, 0, h.readTime(t, token), seekSlack, "the position did not move")
	h.onUI(t, func() { require.NoError(t, h.eng.Play()) })
	h.await(t, state(media.StatePlaying), playingWithin)
	wait(300 * time.Millisecond)
	p0, w0 := h.readTime(t, token), time.Now()
	wait(2 * time.Second)
	p1, w1 := h.readTime(t, token), time.Now()
	speed := (p1 - p0) / w1.Sub(w0).Seconds()
	t.Logf("amp-measure: rate set paused, then played: speed %.3f", speed)
	assert.InDelta(t, 1.5, speed, 1.5*rateSlack)
}

// TestAMPSeekWhilePausedShowsAFrame loads the HLS paused at 1 s: its frame
// shows; seeks of +10 s and −10 s, as the ±10 buttons do, land on 11 and
// 1 (read at once and after the frame) and show their frame each; paused,
// no more frames come.
func TestAMPSeekWhilePausedShowsAFrame(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	frames := h.watchFrames(t)
	token := h.load(t, mediaURL(hlsMaster), media.LoadOptions{Start: 1})
	h.await(t, state(media.StatePaused), loadWithin)
	assert.InDelta(t, 1, h.readTime(t, token), seekSlack)
	s0 := awaitSeq(t, frames, 0, frameWithin)
	for _, step := range []struct{ by, want float64 }{{10, 11}, {-10, 1}} {
		var at float64
		h.onUI(t, func() {
			pos, ok := h.eng.(media.TimeReader).ReadTime(token)
			require.True(t, ok)
			require.NoError(t, h.eng.Seek(pos+step.by))
			at, _ = h.eng.(media.TimeReader).ReadTime(token)
		})
		assert.InDelta(t, step.want, at, seekSlack, "at once, seek %+.0f", step.by)
		s0 = awaitSeq(t, frames, s0, frameWithin)
		wait(200 * time.Millisecond)
		got := h.readTime(t, token)
		t.Logf("amp-measure: seek %+.0f paused: at once %.3f, after the frame %.3f", step.by, at, got)
		assert.InDelta(t, step.want, got, seekSlack, "after the seek %+.0f", step.by)
	}
	drainSeqs(frames)
	var seq uint64
	h.onUI(t, func() { _, seq = h.eng.Video().Frame(0) })
	window(frames, 2*time.Second)
	var later uint64
	h.onUI(t, func() { _, later = h.eng.Video().Frame(0) })
	assert.LessOrEqual(t, later, seq+1, "a paused video went on drawing")
}

// TestAMPABLoop plays the HLS with an A–B loop of 2 s to 6 s kept by the
// test as the player does (the tick's time ≥ B seeks to A): it jumps back
// at least twice in 9 s, each time to A, and the time never runs past B by
// more than a tick; once the loop is off the media plays past B.
func TestAMPABLoop(t *testing.T) { //nolint:paralleltest // one app a process
	const a, b = 2.0, 6.0
	h := newHarness(t)
	token := h.load(t, mediaURL(hlsMaster), media.LoadOptions{Start: a})
	h.await(t, state(media.StatePaused), loadWithin)
	var (
		jumps   int
		landed  []float64
		highest float64
		armed   bool
	)
	var unsub func()
	h.onUI(t, func() {
		unsub = h.eng.Events().Subscribe(func(ev media.Event) {
			if ev.Kind != media.EventTime || ev.Token != token {
				return
			}
			highest = math.Max(highest, ev.Time)
			if armed {
				armed = false
				landed = append(landed, ev.Time)
			}
			if ev.Time >= b {
				jumps++
				armed = true
				theApp.Enqueue(func() { _ = h.eng.Seek(a) })
			}
		})
		require.NoError(t, h.eng.Play())
	})
	h.await(t, state(media.StatePlaying), playingWithin)
	wait(9 * time.Second)
	var gotJumps int
	var gotLanded []float64
	var gotHighest float64
	h.onUI(t, func() {
		unsub()
		gotJumps, gotLanded, gotHighest = jumps, append([]float64(nil), landed...), highest
	})
	t.Logf("amp-measure: A–B %.0f→%.0f s: %d jumps in 9 s, landed at %v, highest time %.3f", a, b, gotJumps, gotLanded, gotHighest)
	assert.GreaterOrEqual(t, gotJumps, 2)
	for _, l := range gotLanded {
		assert.InDelta(t, a, l, 0.3, "landed on A (the first tick after the seek)")
	}
	assert.Less(t, gotHighest, b+0.3, "no further than a tick past B")
	h.await(t, func(ev media.Event) bool { return ev.Kind == media.EventTime && ev.Time > b+0.5 }, 6*time.Second)
}

// TestAMPEndIsNotPaused plays the HLS to its end: Ended, with no Paused
// before it, once; Play plays again from the start.
func TestAMPEndIsNotPaused(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true, Start: hlsLength - 3})
	h.await(t, state(media.StatePlaying), playingWithin)
	ev := h.await(t, func(ev media.Event) bool {
		return ev.Kind == media.EventEnded || (ev.Kind == media.EventState && ev.State == media.StatePaused)
	}, 6*time.Second)
	assert.Equal(t, media.EventEnded, ev.Kind, "Paused came before Ended")
	h.expectNone(t, func(ev media.Event) bool {
		return ev.Kind == media.EventEnded || (ev.Kind == media.EventState && ev.State == media.StatePlaying)
	}, 2*time.Second)
	h.onUI(t, func() { require.NoError(t, h.eng.Play()) })
	h.await(t, state(media.StatePlaying), 3*time.Second)
	assert.Less(t, h.tickTime(t), 1.0, "played again from the start")
}

// TestAMPStaleEventsDropped loads the MP3 then the HLS at once: every
// event after is the HLS's.
func TestAMPStaleEventsDropped(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	h.loadSrc(t, media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Auto: true})
	b := h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	var stale []media.Event
	h.await(t, func(ev media.Event) bool {
		if ev.Token != b {
			stale = append(stale, ev)
		}

		return ev.Kind == media.EventState && ev.State == media.StatePlaying && ev.Token == b
	}, loadWithin)
	h.expectNone(t, func(ev media.Event) bool { return ev.Token != b }, 2*time.Second)
	assert.Empty(t, stale)
}

// TestAMPExpiredURL plays the signed links of the gate (the CDN's part):
// a link expired before the load fails with an error event, and so does a
// long stream whose links are revoked while it plays, at the seek that
// needs a segment not fetched yet. The class of each error is logged: the
// player asks for a new link on media.ErrNetwork (04 §2.1).
func TestAMPExpiredURL(t *testing.T) { //nolint:paralleltest // one app a process
	t.Run("at load", func(t *testing.T) {
		h := newHarness(t)
		start := time.Now()
		h.load(t, theMedia.Signed(hlsMaster, time.Now().Add(-time.Minute)), media.LoadOptions{Auto: true})
		ev := h.await(t, kind(media.EventError), loadWithin)
		require.Error(t, ev.Err)
		_, refused := theMedia.Counts()
		t.Logf("amp-measure: expired at load: error after %v: %v (class %s), gate refused %d",
			time.Since(start).Round(time.Millisecond), ev.Err, classOf(ev.Err), refused)
		h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
		h.await(t, state(media.StatePlaying), playingWithin)
	})
	t.Run("while playing", func(t *testing.T) {
		h := newHarness(t)
		t.Cleanup(func() { theMedia.Revoke(false) })
		token := h.load(t, theMedia.Long("l_3503", 20, time.Now().Add(time.Hour)), media.LoadOptions{Auto: true})
		h.await(t, state(media.StatePlaying), playingWithin)
		served0, _ := theMedia.Counts()
		theMedia.Revoke(true)
		start := time.Now()
		h.onUI(t, func() { require.NoError(t, h.eng.Seek(200)) })
		var seen []string
		ev, ok := h.awaitOr(func(ev media.Event) bool {
			if ev.Kind == media.EventState {
				seen = append(seen, fmt.Sprint(ev.State))
			}

			return ev.Kind == media.EventError && ev.Token == token
		}, 30*time.Second)
		_, refused := theMedia.Counts()
		t.Logf("amp-measure: revoked while playing: gate served %d before; error %t after %v: %v (class %s), states %v, gate refused %d",
			served0, ok, time.Since(start).Round(time.Millisecond), ev.Err, classOf(ev.Err), seen, refused)
		require.True(t, ok, "an error event once the links are refused")
		require.Error(t, ev.Err)
	})
}

// TestAMPMirror shows a paused frame in a view of the window, plain and
// mirrored (the view's flipped draw, 04 §2.3), screenshots both through
// the driver (adb screencap), and checks that the mirrored picture is the
// plain one turned about its vertical axis.
func TestAMPMirror(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	v := addView(t, h)
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Start: 3})
	h.await(t, state(media.StatePaused), loadWithin)
	v.awaitFrames(t, 1, frameWithin, "the paused frame")
	v.awaitStill(t, h)
	plain := screenshot(t, "mirror-off")
	h.onUI(t, func() { v.setMirror(true) })
	v.awaitStill(t, h)
	mirrored := screenshot(t, "mirror-on")
	var frame image.Rectangle
	h.onUI(t, func() {
		r := fitContain(hlsW, hlsH, v.WindowFrame())
		scale := float32(plain.Bounds().Dx()) / theScreen.WindowFrame().W
		frame = image.Rect(int(r.X*scale)+2, int(r.Y*scale)+2, int((r.X+r.W)*scale)-2, int((r.Y+r.H)*scale)-2)
	})
	same := lumaDiff(mirrored, plain, frame, false)
	flipped := lumaDiff(mirrored, plain, frame, true)
	t.Logf("amp-measure: mirror: picture %v px of %v, mean |Δluma| mirrored vs plain %.2f, vs plain flipped %.2f",
		frame, plain.Bounds().Size(), same, flipped)
	assert.Less(t, flipped, 4.0, "the mirrored picture is the plain one flipped")
	assert.Greater(t, same, 2*flipped+1, "and not the plain one")
}

// TestAMPStartPausedShowsItsFrame loads the HLS paused from 6 s (the
// video screen's resume): the frame shown is the one at 6 s, the same as
// a seek to 6 s while paused brings, not the first.
func TestAMPStartPausedShowsItsFrame(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	frames := h.watchFrames(t)
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Start: 6})
	h.await(t, state(media.StatePaused), loadWithin)
	s := awaitSeq(t, frames, 0, frameWithin)
	window(frames, 500*time.Millisecond)
	atLoad := frameSum(t, h)
	h.onUI(t, func() { require.NoError(t, h.eng.Seek(0)) })
	s = awaitSeq(t, frames, s, frameWithin)
	window(frames, 500*time.Millisecond)
	atZero := frameSum(t, h)
	h.onUI(t, func() { require.NoError(t, h.eng.Seek(6)) })
	awaitSeq(t, frames, s, frameWithin)
	window(frames, 500*time.Millisecond)
	atSix := frameSum(t, h)
	t.Logf("amp-measure: paused load from 6 s: frame sum %d, after a seek to 0 %d, to 6 s %d", atLoad, atZero, atSix)
	assert.NotEqual(t, atZero, atSix, "the frames at 0 and 6 s differ")
	assert.Equal(t, atSix, atLoad, "the paused load shows the frame of its start")
}

// TestAMPStateMachine calls the engine in every state the player may: Play,
// Seek, volume and Pause while preparing, Play after the end. No call
// reaches the player in a wrong state (onError -38).
func TestAMPStateMachine(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	src := media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}
	h.onUI(t, func() {
		_, err := h.eng.Load(context.Background(), src, media.LoadOptions{})
		require.NoError(t, err)
		require.NoError(t, h.eng.SetRate(1.5))
		require.NoError(t, h.eng.SetVolume(0.5))
		require.NoError(t, h.eng.SetMuted(true))
		require.NoError(t, h.eng.Play())
		require.NoError(t, h.eng.Seek(5))
		require.NoError(t, h.eng.Pause())
		require.NoError(t, h.eng.Play())
		require.NoError(t, h.eng.SetMuted(false))
	})
	first := h.await(t, kind(media.EventTime), loadWithin)
	assert.InDelta(t, 5, first.Time, 0.05)
	h.await(t, state(media.StatePlaying), 3*time.Second)
	h.onUI(t, func() { require.NoError(t, h.eng.Seek(mp3Length-0.5)) })
	h.await(t, kind(media.EventEnded), 5*time.Second)
	h.onUI(t, func() { require.NoError(t, h.eng.Play()) })
	h.await(t, state(media.StatePlaying), 3*time.Second)
	tick := h.await(t, kind(media.EventTime), 2*time.Second)
	assert.Less(t, tick.Time, 3.0, "Play after the end starts over")
	h.onUI(t, func() { require.NoError(t, h.eng.Pause()) })
	h.await(t, state(media.StatePaused), 2*time.Second)
	s := stats(t, h)
	assert.Zero(t, s.WrongState, "a call in a wrong state reached the player")
	assert.Zero(t, s.Refused, "the engine tried a call the state forbids")
}

// TestAMPTickDoesNotAllocate ticks the playing HLS on a fake clock: a tick
// (getCurrentPosition, EventTime) and a ReadTime allocate nothing.
func TestAMPTickDoesNotAllocate(t *testing.T) { //nolint:paralleltest // one app a process
	fake := clock.NewFake(time.Now())
	h := newHarnessWith(t, media.Config{Dispatch: dispatch, Clock: fake})
	token := h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	var allocs, readAllocs float64
	var cost []time.Duration
	h.onUI(t, func() {
		rt := h.eng.(media.TimeReader)
		fake.Advance(enginekit.TickEvery) // warm
		_, _ = rt.ReadTime(token)
		defer debug.SetGCPercent(debug.SetGCPercent(-1))
		allocs = testing.AllocsPerRun(200, func() { fake.Advance(enginekit.TickEvery) })
		readAllocs = testing.AllocsPerRun(200, func() { _, _ = rt.ReadTime(token) })
		for range 200 {
			s := time.Now()
			fake.Advance(enginekit.TickEvery)
			cost = append(cost, time.Since(s))
		}
	})
	p50, p95 := percentiles(cost)
	t.Logf("amp-measure: tick p50 %v p95 %v, %.2f allocs per tick, %.2f per ReadTime", p50, p95, allocs, readAllocs)
	assert.Zero(t, allocs, "allocations per tick")
	assert.Zero(t, readAllocs, "allocations per ReadTime")
	h.drain()
}

// TestAMPVolumeDoesNotAllocate sets the volume and the mute of a playing
// engine: the calls into Java are funcs made once, so none allocates.
func TestAMPVolumeDoesNotAllocate(t *testing.T) { //nolint:paralleltest // one app a process
	fake := clock.NewFake(time.Now())
	h := newHarnessWith(t, media.Config{Dispatch: dispatch, Clock: fake})
	h.loadSrc(t, media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	var allocs, muteAllocs float64
	h.onUI(t, func() {
		require.NoError(t, h.eng.SetVolume(0.5)) // warm
		defer debug.SetGCPercent(debug.SetGCPercent(-1))
		n := 0
		allocs = testing.AllocsPerRun(200, func() {
			n++
			_ = h.eng.SetVolume(float64(n%8) / 8)
		})
		muteAllocs = testing.AllocsPerRun(100, func() {
			n++
			_ = h.eng.SetMuted(n%2 == 0)
		})
		require.NoError(t, h.eng.SetMuted(false))
		require.NoError(t, h.eng.SetVolume(1))
	})
	t.Logf("amp-measure: SetVolume %v allocations; SetMuted %v allocations", allocs, muteAllocs)
	assert.Zero(t, allocs, "SetVolume")
	assert.Zero(t, muteAllocs, "SetMuted")
	h.drain()
}

// TestAMPPauseSeekDoesNotAllocate pauses and plays a playing engine, and
// seeks it, as the transport does: Pause, Play and Seek hand Java funcs
// made once, and the player's own callbacks allocate nothing either.
func TestAMPPauseSeekDoesNotAllocate(t *testing.T) { //nolint:paralleltest // one app a process
	fake := clock.NewFake(time.Now())
	h := newHarnessWith(t, media.Config{Dispatch: dispatch, Clock: fake})
	h.loadSrc(t, media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	var pauseAllocs, seekAllocs float64
	h.onUI(t, func() {
		require.NoError(t, h.eng.Pause()) // warm
		require.NoError(t, h.eng.Play())
		require.NoError(t, h.eng.Seek(1))
		defer debug.SetGCPercent(debug.SetGCPercent(-1))
		pauseAllocs = testing.AllocsPerRun(50, func() {
			_ = h.eng.Pause()
			_ = h.eng.Play()
		})
		n := 0
		seekAllocs = testing.AllocsPerRun(50, func() {
			n++
			_ = h.eng.Seek(float64(1 + n%10))
		})
	})
	t.Logf("amp-measure: Pause+Play %v allocations; Seek %v allocations", pauseAllocs, seekAllocs)
	assert.Zero(t, pauseAllocs, "Pause and Play")
	assert.Zero(t, seekAllocs, "Seek")
	assert.Zero(t, stats(t, h).WrongState, "no call in a wrong state")
	h.drain()
}

// TestAMPFrameDoesNotAllocate copies frames of the HLS into a watched
// Surface without a tick (a fake clock): the Go heap does not grow with
// them.
func TestAMPFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarnessWith(t, media.Config{Dispatch: dispatch, Clock: clock.NewFake(time.Now())})
	var unwatch func()
	h.onUI(t, func() { unwatch = h.eng.Video().Watch() })
	t.Cleanup(func() { h.onUI(t, unwatch) })
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	h.idle(time.Second) // the three images reach the frame's size
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	s0, m0 := stats(t, h), mallocs()
	h.idle(3 * time.Second)
	m1, s1 := mallocs(), stats(t, h)
	n := s1.CopiedFrames - s0.CopiedFrames
	t.Logf("amp-measure: %d frames copied in 3 s, %d mallocs in the process (%.3f a frame), convert p50 %v p95 %v",
		n, m1-m0, float64(m1-m0)/float64(max(n, 1)), s1.ConvertP50, s1.ConvertP95)
	require.Greater(t, n, int64(60))
	assert.Less(t, float64(m1-m0)/float64(n), 0.2, "frames allocate")
}

// TestAMPConvertGolden checks the C conversion (NEON, scaling) against
// enginekit's reference, for I420, NV12 and NV21 with padded rows.
func TestAMPConvertGolden(t *testing.T) { //nolint:paralleltest // one app a process
	for _, layout := range []string{"I420", "NV12", "NV21"} {
		for _, bt709 := range []bool{false, true} {
			w, h := 70, 38 // a NEON step and a scalar tail
			y, u, v, ys, uvs, ps := synthYUV(w, h, layout, 16)
			want := make([]byte, 4*w*h)
			enginekit.YUV420ToRGBA(want, 4*w, w, h, y, ys, u, v, uvs, ps, bt709)
			got := make([]byte, 4*w*h)
			_, err := amp.ConvertYUV420(got, 4*w, w, h, w, h, y, ys, u, v, uvs, ps, bt709)
			require.NoError(t, err)
			assert.LessOrEqual(t, maxDiff(want, got), 3, "%s 709=%v", layout, bt709)
			hw, hh := w/2, h/2
			half := make([]byte, 4*hw*hh)
			_, err = amp.ConvertYUV420(half, 4*hw, hw, hh, w, h, y, ys, u, v, uvs, ps, bt709)
			require.NoError(t, err)
			worst := 0
			for r := range hh {
				for c := range hw {
					for k := range 3 {
						d := int(half[4*(r*hw+c)+k]) - int(want[4*((2*r+1)*w+2*c+1)+k])
						worst = max(worst, d, -d)
					}
				}
			}
			assert.LessOrEqual(t, worst, 3, "scaled %s 709=%v", layout, bt709)
		}
	}
}

// TestAMPConvertCost times the conversion of 480×854 (the lesson's),
// 1280×720 frames, and 1920×1080 scaled to 1280×720, of each layout.
func TestAMPConvertCost(t *testing.T) { //nolint:paralleltest // one app a process
	for _, c := range []struct{ w, h, dw, dh int }{{480, 854, 480, 854}, {1280, 720, 1280, 720}, {1920, 1080, 1280, 720}} {
		for _, layout := range []string{"I420", "NV12", "NV21"} {
			y, u, v, ys, uvs, ps := synthYUV(c.w, c.h, layout, 64)
			dst := make([]byte, 4*c.dw*c.dh)
			times := make([]time.Duration, 100)
			for i := range times {
				d, err := amp.ConvertYUV420(dst, 4*c.dw, c.dw, c.dh, c.w, c.h, y, ys, u, v, uvs, ps, c.h >= 720)
				require.NoError(t, err)
				times[i] = d
			}
			slices.Sort(times)
			t.Logf("amp-measure: convert %dx%d->%dx%d %s: p50 %v p95 %v", c.w, c.h, c.dw, c.dh, layout, times[50], times[95])
		}
	}
}

// TestAMPFocus takes the focus from the session with requests of the
// test's own: a transient one is InterruptBegin, then InterruptEnd that
// resumes when it is given back; a lasting one is InterruptBegin and
// InterruptEnd that does not.
func TestAMPFocus(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	got := h.sessionEvents(t)
	h.onUI(t, func() { require.NoError(t, h.sess.Activate()) })
	other := newFocusRequest(t, 2) // AUDIOFOCUS_GAIN_TRANSIENT
	request(t, other)
	assert.Equal(t, media.SessionEvent{Kind: media.InterruptBegin}, awaitSession(t, got))
	abandon(t, other)
	assert.Equal(t, media.SessionEvent{Kind: media.InterruptEnd, ShouldResume: true}, awaitSession(t, got))
	lasting := newFocusRequest(t, 1) // AUDIOFOCUS_GAIN
	request(t, lasting)
	assert.Equal(t, media.SessionEvent{Kind: media.InterruptBegin}, awaitSession(t, got))
	assert.Equal(t, media.SessionEvent{Kind: media.InterruptEnd}, awaitSession(t, got))
	abandon(t, lasting)
	h.onUI(t, func() { require.NoError(t, h.sess.Activate()) })
}

// TestAMPFocusAgainAfterTransientLoss checks that a play during a loss
// for a while asks for the focus again rather than waiting for the
// system's gain, and that the gain given back later resumes nothing.
func TestAMPFocusAgainAfterTransientLoss(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	got := h.sessionEvents(t)
	h.onUI(t, func() { require.NoError(t, h.sess.Activate()) })
	var before amp.SessionStats
	h.onUI(t, func() {
		var ok bool
		before, ok = amp.SessionStatsOf(h.sess)
		require.True(t, ok)
	})
	h.onUI(t, func() { require.NoError(t, h.sess.Activate()) })
	h.onUI(t, func() {
		st, _ := amp.SessionStatsOf(h.sess)
		assert.Equal(t, before.Requests, st.Requests, "held: no new request")
	})
	other := newFocusRequest(t, 2) // AUDIOFOCUS_GAIN_TRANSIENT
	request(t, other)
	assert.Equal(t, media.SessionEvent{Kind: media.InterruptBegin}, awaitSession(t, got))
	h.onUI(t, func() { require.NoError(t, h.sess.Activate(), "the focus is granted again") })
	h.onUI(t, func() {
		st, _ := amp.SessionStatsOf(h.sess)
		assert.Equal(t, before.Requests+1, st.Requests, "a play during the loss requests the focus")
		assert.True(t, st.Held)
	})
	abandon(t, other)
	select {
	case ev := <-got:
		assert.NotEqual(t, media.SessionEvent{Kind: media.InterruptEnd, ShouldResume: true}, ev, "no resume after the user played")
	case <-time.After(time.Second):
	}
	h.onUI(t, func() { require.NoError(t, h.sess.Activate()) })
}

// TestAMPNoisyReceiverRegistered sends the tests' noisy broadcast to the
// app: the session reports Noisy (the system's own is protected).
func TestAMPNoisyReceiverRegistered(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	got := h.sessionEvents(t)
	require.NoError(t, jni.Do(func(env jni.Env) error {
		ic, err := env.FindClass("android/content/Intent")
		if err != nil {
			return err
		}
		ctor, err := env.Method(ic, "<init>", "(Ljava/lang/String;)V")
		if err != nil {
			return err
		}
		setPackage, err := env.Method(ic, "setPackage", "(Ljava/lang/String;)Landroid/content/Intent;")
		if err != nil {
			return err
		}
		cc, err := env.FindClass("android/content/Context")
		if err != nil {
			return err
		}
		send, err := env.Method(cc, "sendBroadcast", "(Landroid/content/Intent;)V")
		if err != nil {
			return err
		}
		action, err := env.NewString(testNoisyAction)
		if err != nil {
			return err
		}
		pkg, err := env.NewString(appID)
		if err != nil {
			return err
		}
		intent, err := env.NewObject(ic, ctor, jni.Ref(action))
		if err != nil {
			return err
		}
		if _, err := env.CallObject(intent, setPackage, jni.Ref(pkg)); err != nil {
			return err
		}

		return env.CallVoid(jni.Context(), send, jni.Ref(intent))
	}))
	assert.Equal(t, media.SessionEvent{Kind: media.Noisy}, awaitSession(t, got))
}

// TestAMPPlaybackKeepsUp plays the HLS in a view that paints each frame,
// at 1× and 1.5×: how many of the source's frames the decoder gave, the
// engine copied and the view painted (≥ keepUp of those copied; at 1× the
// copies ≥ keepUp of the source), the conversion's cost and the process's
// CPU.
func TestAMPPlaybackKeepsUp(t *testing.T) { //nolint:paralleltest // one app a process
	for _, r := range []float64{1, 1.5} {
		t.Run(strconv.FormatFloat(r, 'f', -1, 64)+"x", func(t *testing.T) {
			h := newHarness(t)
			v := addView(t, h)
			token := h.load(t, theMedia.Long("l_3503", 4, time.Now().Add(time.Hour)), media.LoadOptions{})
			h.await(t, state(media.StatePaused), loadWithin)
			h.onUI(t, func() {
				require.NoError(t, h.eng.SetRate(r))
				require.NoError(t, h.eng.Play())
			})
			h.await(t, state(media.StatePlaying), playingWithin)
			v.awaitFrames(t, 12, frameWithin, "frames before the measure")
			v.flush()
			var paints0 int
			h.onUI(t, func() { paints0 = v.paints })
			p0, cpu0, w0, s0 := h.readTime(t, token), cpuTicks(), time.Now(), stats(t, h)
			painted := 0
			deadline := time.NewTimer(8 * time.Second)
			for done := false; !done; {
				select {
				case <-v.frames:
					painted++
				case <-deadline.C:
					done = true
				}
			}
			p1, cpu1, w1, s1 := h.readTime(t, token), cpuTicks(), time.Now(), stats(t, h)
			var paints1 int
			h.onUI(t, func() { paints1 = v.paints })
			wall := w1.Sub(w0).Seconds()
			source := (p1 - p0) * 24
			copied := float64(s1.CopiedFrames - s0.CopiedFrames)
			cpu := float64(cpu1-cpu0) / 100 / wall * 100
			t.Logf("amp-measure: keep-up %.1fx: %.1f s of media in %.1f s: %.0f source frames, %d from the decoder, %.0f copied (%.3f), %d painted (%.3f), %d paints (%.1f fps); dropped %d; convert p50 %v p95 %v; CPU %.1f%% of a core; RSS %d MB",
				r, p1-p0, wall, source, s1.Frames-s0.Frames, copied, copied/source, painted, float64(painted)/source,
				paints1-paints0, float64(paints1-paints0)/wall, s1.DroppedFrames-s0.DroppedFrames,
				s1.ConvertP50, s1.ConvertP95, cpu, s1.RSS>>20)
			assert.GreaterOrEqual(t, float64(painted), keepUp*copied, "painted of copied (Q-P0.7-3)")
			if r == 1 {
				assert.GreaterOrEqual(t, copied, keepUp*source, "copied of the source at 1x")
			}
		})
	}
}

// TestAMPSwitchCost switches between the MP3 and the HLS twenty times:
// what a load and an unload cost the UI thread, and every reference,
// proxy and reader of a load released after it.
func TestAMPSwitchCost(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	frames := h.watchFrames(t)
	base := settled(t, h)
	for i := range 20 {
		src := media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}
		if i%2 == 1 {
			src = media.Source{URL: mediaURL(hlsMaster)}
		}
		h.loadSrc(t, src, media.LoadOptions{Auto: true})
		h.await(t, state(media.StatePlaying), loadWithin)
		if !src.AudioOnly {
			awaitSeq(t, frames, 0, frameWithin)
		}
	}
	h.onUI(t, func() { require.NoError(t, h.eng.Stop()) })
	s := settled(t, h)
	m := memSnapshot(t, h)
	t.Logf("amp-measure: load p50 %v p95 %v max %v; unload p50 %v p95 %v max %v; globals %d→%d proxies %d→%d readers %d→%d; goHeap %.2f MiB RSS %.1f MiB",
		pct(s.Loads, 50), pct(s.Loads, 95), pct(s.Loads, 100), pct(s.Unloads, 50), pct(s.Unloads, 95), pct(s.Unloads, 100),
		base.LiveGlobalRefs, s.LiveGlobalRefs, base.LiveProxies, s.LiveProxies, base.LiveReaders, s.LiveReaders,
		mib(m["goHeap"]), mib(m["RSS"]))
	assert.Equal(t, base.LiveGlobalRefs, s.LiveGlobalRefs)
	assert.Equal(t, base.LiveProxies, s.LiveProxies)
	assert.Equal(t, base.LiveReaders, s.LiveReaders)
	assert.Zero(t, s.WrongState)
}

// TestAMPBackground tells the engine the app went to the background while
// the HLS (a video with sound) plays: no tick, no copies, the images freed
// and the player's Surface taken away while the sound plays on; in the
// foreground the Surface is given back, the time told at once and never
// back behind it, and frames come again within firstFrameBackAMP.
func TestAMPBackground(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	frames := h.watchFrames(t)
	h.load(t, theMedia.Long("l_3503", 4, time.Now().Add(time.Hour)), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	awaitSeq(t, frames, 0, frameWithin)
	before := h.await(t, kind(media.EventTime), 2*time.Second)
	bg, ok := h.eng.(media.Backgrounder)
	require.True(t, ok)
	h.onUI(t, func() { bg.SetBackground(true) })
	s0 := stats(t, h)
	assert.True(t, s0.Detached, "the Surface taken away")
	assert.False(t, s0.HoldsImages, "the images freed")
	h.drain()
	evs := h.window(2 * time.Second)
	s1 := stats(t, h)
	assert.False(t, slices.ContainsFunc(evs, func(ev media.Event) bool { return ev.Kind == media.EventTime }), "a tick in the background")
	assert.LessOrEqual(t, s1.CopiedFrames-s0.CopiedFrames, int64(0), "frames copied in the background")
	assert.LessOrEqual(t, s1.Frames-s0.Frames, int64(3), "the decoder of the picture runs on in the background")
	drainSeqs(frames)
	start := time.Now()
	h.onUI(t, func() { bg.SetBackground(false) })
	back := h.await(t, kind(media.EventTime), 300*time.Millisecond)
	assert.Greater(t, back.Time, before.Time+1.5, "the player did not play on")
	awaitSeq(t, frames, 0, firstFrameBackAMP)
	took := time.Since(start)
	s2 := stats(t, h)
	assert.False(t, s2.Detached)
	assert.Equal(t, s0.Detaches, s2.Attaches, "the Surface given back")
	assert.True(t, s2.HoldsImages, "the images made again")
	for _, ev := range h.window(time.Second) {
		if ev.Kind == media.EventTime {
			assert.GreaterOrEqual(t, ev.Time, back.Time-0.05, "the time went back (the key frame before)")
		}
	}
	t.Logf("amp-measure: background hooks: 2 s there, played %.2f s, decoder frames %d copied %d there; first frame %v after the foreground",
		back.Time-before.Time, s1.Frames-s0.Frames, s1.CopiedFrames-s0.CopiedFrames, took.Round(time.Millisecond))
	assert.Zero(t, s2.WrongState, "a call in a wrong state reached the player")
}

// TestAMPWifiLock checks the engine's Wi-Fi lock: held while a link (here
// the loopback server's) plays, let go as it pauses, stops, or the engine
// closes. The system's own answer (WifiLock.isHeld) is read at each step.
func TestAMPWifiLock(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	held := func() bool { return stats(t, h).WifiLock }
	src := media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}
	h.loadSrc(t, src, media.LoadOptions{Auto: true})
	assert.True(t, held(), "held while a link prepares to play")
	h.await(t, state(media.StatePlaying), playingWithin)
	assert.True(t, held(), "held while a link plays")
	h.onUI(t, func() { require.NoError(t, h.eng.Pause()) })
	h.await(t, state(media.StatePaused), 5*time.Second)
	assert.False(t, held(), "let go as it pauses")
	h.onUI(t, func() { require.NoError(t, h.eng.Play()) })
	h.await(t, state(media.StatePlaying), 5*time.Second)
	assert.True(t, held(), "held again as it plays")
	h.onUI(t, func() { require.NoError(t, h.eng.Stop()) })
	assert.False(t, held(), "let go as it stops")
	h.loadSrc(t, src, media.LoadOptions{})
	assert.False(t, held(), "not held for a link loaded paused")
	h.await(t, state(media.StatePaused), loadWithin)
	h.onUI(t, func() { require.NoError(t, h.eng.Play()) })
	h.await(t, state(media.StatePlaying), 5*time.Second)
	require.True(t, held())
	h.onUI(t, func() { require.NoError(t, h.eng.Close()) })
	assert.False(t, held(), "let go as the engine closes")
}

// TestAMPHandoffLock plays the end of the MP3: as it completes, the
// hand-off lock keeps the CPU awake (MediaPlayer let go of its own), and
// the next media starting lets go of it, as does a stop.
func TestAMPHandoffLock(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	handoff := func() bool { return stats(t, h).HandoffLock }
	mp3 := media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}
	h.loadSrc(t, mp3, media.LoadOptions{Auto: true, Start: mp3Length - 1.5})
	h.await(t, state(media.StatePlaying), playingWithin)
	assert.False(t, handoff(), "not held while playing")
	h.await(t, kind(media.EventEnded), loadWithin)
	assert.True(t, handoff(), "held from the end")
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	assert.False(t, handoff(), "let go as the next media plays")
	h.loadSrc(t, mp3, media.LoadOptions{Auto: true, Start: mp3Length - 1.5})
	h.await(t, kind(media.EventEnded), loadWithin)
	require.True(t, handoff())
	h.onUI(t, func() { require.NoError(t, h.eng.Stop()) })
	assert.False(t, handoff(), "let go as the engine stops")
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// newHarnessWith is newHarness on a configuration of the test's own (a
// fake clock).
func newHarnessWith(t *testing.T, cfg media.Config) *harness {
	t.Helper()

	h := &harness{app: theApp, events: make(chan media.Event, eventBuffer)}
	var err error
	h.onUI(t, func() {
		h.eng, h.sess, err = newPlatform(cfg)
		if err != nil {
			return
		}
		unsub := h.eng.Events().Subscribe(func(ev media.Event) {
			select {
			case h.events <- ev:
			default:
				h.dropped++
			}
		})
		t.Cleanup(func() { h.onUI(t, func() { unsub(); _ = h.eng.Close() }) })
	})
	require.NoError(t, err)

	return h
}

// window returns the events that arrive in d: a measuring window.
func (h *harness) window(d time.Duration) []media.Event {
	var out []media.Event
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		select {
		case ev := <-h.events:
			out = append(out, ev)
		case <-timer.C:
			return out
		}
	}
}

// idle drops the events that arrive in d without keeping them: a
// measuring window that allocates nothing per event.
func (h *harness) idle(d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		select {
		case <-h.events:
		case <-timer.C:
			return
		}
	}
}

// watchFrames watches the engine's Surface until the test ends; the
// returned channel gets the sequence number of each frame notified.
func (h *harness) watchFrames(t *testing.T) <-chan uint64 {
	t.Helper()

	ch := make(chan uint64, 1024)
	var unwatch, unsub func()
	h.onUI(t, func() {
		v := h.eng.Video()
		unwatch = v.Watch()
		unsub = v.Frames().Subscribe(func(seq uint64) {
			select {
			case ch <- seq:
			default:
			}
		})
	})
	t.Cleanup(func() { h.onUI(t, func() { unsub(); unwatch() }) })

	return ch
}

// sessionEvents subscribes to the session until the test ends.
func (h *harness) sessionEvents(t *testing.T) <-chan media.SessionEvent {
	t.Helper()

	ch := make(chan media.SessionEvent, 64)
	var unsub func()
	h.onUI(t, func() {
		unsub = h.sess.Events().Subscribe(func(ev media.SessionEvent) {
			select {
			case ch <- ev:
			default:
			}
		})
	})
	t.Cleanup(func() { h.onUI(t, unsub) })

	return ch
}

// awaitSeq returns the first frame number notified over after, failing
// after within.
func awaitSeq(t *testing.T, ch <-chan uint64, after uint64, within time.Duration) uint64 {
	t.Helper()

	timer := time.NewTimer(within)
	defer timer.Stop()
	for {
		select {
		case seq := <-ch:
			if seq > after {
				return seq
			}
		case <-timer.C:
			require.FailNow(t, "no frame", "after %d within %v", after, within)
		}
	}
}

// drainSeqs drops the frame numbers queued.
func drainSeqs(ch <-chan uint64) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// window drops the frame numbers that come in d: a measuring window.
func window(ch <-chan uint64, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		select {
		case <-ch:
		case <-timer.C:
			return
		}
	}
}

// awaitSession returns the next session event, failing after 5 s.
func awaitSession(t *testing.T, ch <-chan media.SessionEvent) media.SessionEvent {
	t.Helper()

	select {
	case ev := <-ch:
		return ev
	case <-time.After(5 * time.Second):
		require.FailNow(t, "no session event")
	}

	return media.SessionEvent{}
}

// stats reads the engine's counters on the UI thread.
func stats(t *testing.T, h *harness) amp.Stats {
	t.Helper()

	var s amp.Stats
	h.onUI(t, func() {
		var ok bool
		s, ok = amp.StatsOf(h.eng)
		require.True(t, ok)
	})

	return s
}

// settled reads the engine's counters once the releaser has freed every
// load handed to it, failing after 2 s.
func settled(t *testing.T, h *harness) amp.Stats {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		s := stats(t, h)
		if s.Releasing == 0 {
			return s
		}
		require.True(t, time.Now().Before(deadline), "%d loads not released", s.Releasing)
		time.Sleep(10 * time.Millisecond) // polls the releaser, within the deadline
	}
}

// frameSum is a checksum of the pixels of the frame shown.
func frameSum(t *testing.T, h *harness) uint64 {
	t.Helper()

	var sum uint64
	h.onUI(t, func() {
		img, _ := h.eng.Video().Frame(0)
		require.NotNil(t, img)
		for i := 0; i < len(img.Pix); i += 7 {
			sum = sum*31 + uint64(img.Pix[i])
		}
	})

	return sum
}

// focusRequest is an AudioFocusRequest of the test's own, a global
// reference, with the AudioManager.
type focusRequest struct{ am, req jni.Object }

// newFocusRequest makes a request of the focus for media of kind gain.
func newFocusRequest(t *testing.T, gain int32) focusRequest {
	t.Helper()

	var f focusRequest
	require.NoError(t, jni.Do(func(env jni.Env) error {
		cc, err := env.FindClass("android/content/Context")
		if err != nil {
			return err
		}
		get, err := env.Method(cc, "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;")
		if err != nil {
			return err
		}
		name, err := env.NewString("audio")
		if err != nil {
			return err
		}
		am, err := env.CallObject(jni.Context(), get, jni.Ref(name))
		if err != nil {
			return err
		}
		ab, err := env.FindClass("android/media/AudioAttributes$Builder")
		if err != nil {
			return err
		}
		abNew, err := env.Method(ab, "<init>", "()V")
		if err != nil {
			return err
		}
		usage, err := env.Method(ab, "setUsage", "(I)Landroid/media/AudioAttributes$Builder;")
		if err != nil {
			return err
		}
		abBuild, err := env.Method(ab, "build", "()Landroid/media/AudioAttributes;")
		if err != nil {
			return err
		}
		b, err := env.NewObject(ab, abNew)
		if err != nil {
			return err
		}
		if _, err := env.CallObject(b, usage, jni.Int(1)); err != nil { // USAGE_MEDIA
			return err
		}
		attrs, err := env.CallObject(b, abBuild)
		if err != nil {
			return err
		}
		fb, err := env.FindClass("android/media/AudioFocusRequest$Builder")
		if err != nil {
			return err
		}
		fbNew, err := env.Method(fb, "<init>", "(I)V")
		if err != nil {
			return err
		}
		setAttrs, err := env.Method(fb, "setAudioAttributes", "(Landroid/media/AudioAttributes;)Landroid/media/AudioFocusRequest$Builder;")
		if err != nil {
			return err
		}
		fbBuild, err := env.Method(fb, "build", "()Landroid/media/AudioFocusRequest;")
		if err != nil {
			return err
		}
		fo, err := env.NewObject(fb, fbNew, jni.Int(gain))
		if err != nil {
			return err
		}
		if _, err := env.CallObject(fo, setAttrs, jni.Ref(attrs)); err != nil {
			return err
		}
		req, err := env.CallObject(fo, fbBuild)
		if err != nil {
			return err
		}
		f.am, f.req = env.NewGlobalRef(am), env.NewGlobalRef(req)

		return nil
	}))
	t.Cleanup(func() {
		_ = jni.Do(func(env jni.Env) error {
			env.DeleteGlobalRef(f.am)
			env.DeleteGlobalRef(f.req)

			return nil
		})
	})

	return f
}

// request asks for the focus with f; it must be granted.
func request(t *testing.T, f focusRequest) {
	t.Helper()

	require.Equal(t, int32(1), focusCall(t, f, "requestAudioFocus"), "focus not granted")
}

// abandon gives the focus of f back.
func abandon(t *testing.T, f focusRequest) {
	t.Helper()

	focusCall(t, f, "abandonAudioFocusRequest")
}

// focusCall calls an AudioManager method taking an AudioFocusRequest.
func focusCall(t *testing.T, f focusRequest, name string) int32 {
	t.Helper()

	var r int32
	require.NoError(t, jni.Do(func(env jni.Env) error {
		c, err := env.FindClass("android/media/AudioManager")
		if err != nil {
			return err
		}
		m, err := env.Method(c, name, "(Landroid/media/AudioFocusRequest;)I")
		if err != nil {
			return err
		}
		r, err = env.CallInt(f.am, m, jni.Ref(f.req))

		return err
	}))

	return r
}

// synthYUV makes a w by h 4:2:0 picture in a layout, rows padded to a
// multiple of pad: the planes, their row strides and the chroma pixel
// stride.
func synthYUV(w, h int, layout string, pad int) (y, u, v []byte, ys, uvs, ps int) {
	cw, ch := (w+1)/2, (h+1)/2
	ys = (w + pad - 1) / pad * pad
	y = make([]byte, ys*h)
	for i := range y {
		y[i] = byte(16 + (i*7)%220)
	}
	switch layout {
	case "I420":
		uvs, ps = (cw+pad-1)/pad*pad, 1
		u, v = make([]byte, uvs*ch), make([]byte, uvs*ch)
		for i := range u {
			u[i], v[i] = byte(16+(i*13)%225), byte(16+(i*29)%225)
		}
	default:
		uvs, ps = (2*cw+pad-1)/pad*pad, 2
		c := make([]byte, uvs*ch+1)
		for i := range c {
			c[i] = byte(16 + (i*13)%225)
		}
		if layout == "NV12" {
			u, v = c, c[1:]
		} else {
			v, u = c, c[1:]
		}
	}

	return y, u, v, ys, uvs, ps
}

// maxDiff returns the largest difference of two RGBA buffers' colours.
func maxDiff(a, b []byte) int {
	worst := 0
	for i := range a {
		if i%4 == 3 {
			continue
		}
		d := int(a[i]) - int(b[i])
		worst = max(worst, d, -d)
	}

	return worst
}

// mallocs returns the process's heap allocations so far.
func mallocs() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return m.Mallocs
}

// cpuTicks returns the process's user and system time from
// /proc/self/stat, in clock ticks (USER_HZ, 100 a second).
func cpuTicks() int64 {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0
	}
	s := string(data)
	if i := strings.LastIndexByte(s, ')'); i >= 0 {
		s = s[i+1:]
	}
	f := strings.Fields(s)
	if len(f) < 13 { // state is f[0]; utime and stime are fields 14 and 15 of the line
		return 0
	}
	ut, _ := strconv.ParseInt(f[11], 10, 64)
	st, _ := strconv.ParseInt(f[12], 10, 64)

	return ut + st
}

// pct returns the p-th percentile of ds.
func pct(ds []time.Duration, p int) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := slices.Clone(ds)
	slices.Sort(s)

	return s[min(len(s)-1, len(s)*p/100)]
}
