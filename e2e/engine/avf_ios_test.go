// Adapted from jplayer-mobile/e2e/engine/avf_ios_test.go at 49de3c2.

//go:build ios && !tvos

package engine

import (
	"context"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avf"
	"fmt"
	"image"
	"math"
	"runtime/debug"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/scene"
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
	// as a share of it (P4.1: ±3 %).
	rateSlack = 0.03
	// seekSlack is how far a seek may land from its target (an exact seek,
	// no tolerance).
	seekSlack = 0.05
	// playingWithin is how soon a load of the local mock plays.
	playingWithin = 3 * time.Second
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestAVFPlaysHLS plays the lesson's HLS from the mock's server: its
// length and size, playing within playingWithin, the pitch kept
// (TimeDomain), BGRA frames of 480×854, and the times from Load to
// playing and to the first frame.
func TestAVFPlaysHLS(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	setStatus("avf: HLS")
	start := time.Now()
	token := h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	dur := h.await(t, kind(media.EventDuration), loadWithin)
	assert.Equal(t, token, dur.Token)
	assert.InDelta(t, hlsLength, dur.Duration, 0.1)
	h.await(t, state(media.StatePlaying), playingWithin)
	playing := time.Since(start)
	var first time.Duration
	var bad string
	ok := pullUntil(t, h, frameWithin, func(img *scene.Image, seq uint64) bool {
		if img == nil || seq == 0 {
			return false
		}
		first = time.Since(start)
		if !img.BGRA || img.Width != hlsW || img.Height != hlsH {
			bad = fmt.Sprintf("a frame of %d×%d, BGRA %t", img.Width, img.Height, img.BGRA)
		}

		return true
	})
	require.True(t, ok, "a frame within %v", frameWithin)
	assert.Empty(t, bad)
	h.onUI(t, func() {
		pitch, ok := avf.PitchAlgorithm(h.eng)
		assert.True(t, ok)
		assert.Equal(t, "TimeDomain", pitch)
	})
	t.Logf("avf-measure: hls load→playing %v, load→first frame %v", playing.Round(time.Millisecond), first.Round(time.Millisecond))
}

// TestAVFPlaysMP3 plays the sample MP3 from the mock's server as audio
// only: its length, the time going on, no frame, and the end after a
// seek near it.
func TestAVFPlaysMP3(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	setStatus("avf: MP3")
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
		assert.False(t, h.eng.Video().Active())
	})
	h.onUI(t, func() { require.NoError(t, h.eng.Seek(mp3Length-2)) })
	h.await(t, kind(media.EventEnded), 5*time.Second)
	t.Logf("avf-measure: mp3 load→playing %v", playing.Round(time.Millisecond))
}

// TestAVFRates plays the HLS and the MP3 at each speed of the sheet and
// measures the position against the wall clock over 3 s: within ±3 % of
// the rate, the player's rate and defaultRate read back as set.
func TestAVFRates(t *testing.T) { //nolint:paralleltest // one app a process
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
				setStatus(fmt.Sprintf("avf: %s at %.2fx", name, r))
				h.onUI(t, func() {
					require.NoError(t, h.eng.Pause())
					require.NoError(t, h.eng.Seek(0.5))
					require.NoError(t, h.eng.SetRate(r))
					require.NoError(t, h.eng.Play())
				})
				h.await(t, state(media.StatePlaying), playingWithin)
				wait(500 * time.Millisecond)
				p0, w0 := h.readTime(t, token), time.Now()
				wait(3 * time.Second)
				p1, w1 := h.readTime(t, token), time.Now()
				speed := (p1 - p0) / w1.Sub(w0).Seconds()
				var rate, def float64
				h.onUI(t, func() {
					var ok bool
					rate, def, ok = avf.PlayerRate(h.eng)
					require.True(t, ok)
				})
				t.Logf("avf-measure: %s rate %.2f: speed %.3f (Δpos %.3f / Δwall %.3f), player rate %.2f defaultRate %.2f",
					name, r, speed, p1-p0, w1.Sub(w0).Seconds(), rate, def)
				assert.InDelta(t, r, speed, r*rateSlack, "speed at %.2f", r)
				assert.InDelta(t, r, rate, 0.001)
				assert.InDelta(t, r, def, 0.001)
				h.onUI(t, func() { require.NoError(t, h.eng.Pause()) })
				h.await(t, state(media.StatePaused), 2*time.Second)
			}
		})
	}
}

// TestAVFRateWhilePaused checks that a speed set while paused does not
// start the media, and that it plays at that speed once played.
func TestAVFRateWhilePaused(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	token := h.load(t, mediaURL(hlsMaster), media.LoadOptions{})
	h.await(t, state(media.StatePaused), loadWithin)
	h.onUI(t, func() { require.NoError(t, h.eng.SetRate(1.5)) })
	h.expectNone(t, state(media.StatePlaying), time.Second)
	assert.InDelta(t, 0, h.readTime(t, token), seekSlack, "the position did not move")
	h.onUI(t, func() {
		rate, def, ok := avf.PlayerRate(h.eng)
		require.True(t, ok)
		assert.Zero(t, rate, "paused")
		assert.InDelta(t, 1.5, def, 0.001, "the rate play uses")
	})
	h.onUI(t, func() { require.NoError(t, h.eng.Play()) })
	h.await(t, state(media.StatePlaying), playingWithin)
	wait(300 * time.Millisecond)
	p0, w0 := h.readTime(t, token), time.Now()
	wait(2 * time.Second)
	p1, w1 := h.readTime(t, token), time.Now()
	speed := (p1 - p0) / w1.Sub(w0).Seconds()
	t.Logf("avf-measure: rate set paused, then played: speed %.3f", speed)
	assert.InDelta(t, 1.5, speed, 1.5*rateSlack)
}

// TestAVFSeekPaused seeks a paused video by +10 s and −10 s from 1 s, as
// the ±10 buttons do: exact (11.0 and 1.0), read at once and after the
// seek finished, and a frame of the new position pulled each time.
func TestAVFSeekPaused(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	token := h.load(t, mediaURL(hlsMaster), media.LoadOptions{Start: 1})
	h.await(t, state(media.StatePaused), loadWithin)
	assert.InDelta(t, 1, h.readTime(t, token), seekSlack)
	var s0 uint64
	require.True(t, pullUntil(t, h, frameWithin, func(img *scene.Image, seq uint64) bool { s0 = seq; return img != nil }),
		"the paused video shows its frame")
	for _, step := range []struct {
		by, want float64
	}{{10, 11}, {-10, 1}} {
		var at float64
		h.onUI(t, func() {
			pos, ok := h.eng.(media.TimeReader).ReadTime(token)
			require.True(t, ok)
			require.NoError(t, h.eng.Seek(pos+step.by))
			at, _ = h.eng.(media.TimeReader).ReadTime(token)
		})
		assert.InDelta(t, step.want, at, seekSlack, "at once, seek %+.0f", step.by)
		require.True(t, pullUntil(t, h, frameWithin, func(_ *scene.Image, seq uint64) bool {
			if seq > s0 {
				s0 = seq

				return true
			}

			return false
		}), "a frame after the seek %+.0f", step.by)
		got := h.readTime(t, token)
		t.Logf("avf-measure: seek %+.0f paused: at once %.3f, after the frame %.3f", step.by, at, got)
		assert.InDelta(t, step.want, got, seekSlack, "after the seek %+.0f", step.by)
	}
	h.onUI(t, func() {
		img, _ := h.eng.Video().Frame(0)
		require.NotNil(t, img)
	})
}

// TestAVFABLoop plays the HLS with an A–B loop of 2 s to 6 s kept by the
// test as the player will (internal/player: the tick's time ≥ B seeks to
// A): it jumps back at least twice in 9 s, each time to A exactly, and the
// time never runs past B by more than a tick; once the loop is off the
// media plays past B.
func TestAVFABLoop(t *testing.T) { //nolint:paralleltest // one app a process
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
	t.Logf("avf-measure: A–B %.0f→%.0f s: %d jumps in 9 s, landed at %v, highest time %.3f", a, b, gotJumps, gotLanded, gotHighest)
	assert.GreaterOrEqual(t, gotJumps, 2)
	for _, l := range gotLanded {
		assert.InDelta(t, a, l, seekSlack, "landed on A")
	}
	assert.Less(t, gotHighest, b+0.3, "no further than a tick past B")
	h.await(t, func(ev media.Event) bool { return ev.Kind == media.EventTime && ev.Time > b+0.5 }, 6*time.Second)
}

// TestAVFEndOnce plays the HLS to its end: the last time comes before
// Ended, no pause is reported at the end, Ended comes once, and Play
// plays again from the start.
func TestAVFEndOnce(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true, Start: hlsLength - 3})
	h.await(t, state(media.StatePlaying), playingWithin)
	var seen []media.Event
	h.await(t, func(ev media.Event) bool {
		seen = append(seen, ev)

		return ev.Kind == media.EventEnded
	}, 6*time.Second)
	for _, ev := range seen {
		assert.False(t, ev.Kind == media.EventState && ev.State == media.StatePaused, "a pause reported at the end: %+v", seen)
	}
	require.GreaterOrEqual(t, len(seen), 2)
	last := seen[len(seen)-2]
	assert.Equal(t, media.EventTime, last.Kind, "the last time comes before Ended")
	assert.InDelta(t, hlsLength, last.Time, 0.2)
	h.expectNone(t, func(ev media.Event) bool {
		return ev.Kind == media.EventEnded || (ev.Kind == media.EventState && ev.State == media.StatePlaying)
	}, 2*time.Second)
	h.onUI(t, func() { require.NoError(t, h.eng.Play()) })
	h.await(t, state(media.StatePlaying), 2*time.Second)
	assert.Less(t, h.tickTime(t), 1.0, "played again from the start")
}

// TestAVFStaleEventsDropped loads two media in a row: no event of the
// first comes after the second's load, and the engine holds one item.
func TestAVFStaleEventsDropped(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	var a, b uint64
	h.onUI(t, func() {
		var err error
		a, err = h.eng.Load(context.Background(), media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Auto: true})
		require.NoError(t, err)
		b, err = h.eng.Load(context.Background(), media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true})
		require.NoError(t, err)
	})
	require.NotEqual(t, a, b)
	var stale []media.Event
	h.await(t, func(ev media.Event) bool {
		if ev.Token != b {
			stale = append(stale, ev)
		}

		return ev.Kind == media.EventState && ev.State == media.StatePlaying && ev.Token == b
	}, loadWithin)
	h.expectNone(t, func(ev media.Event) bool { return ev.Token != b }, time.Second)
	assert.Empty(t, stale)
	h.onUI(t, func() {
		st, ok := avf.StatsOf(h.eng)
		require.True(t, ok)
		assert.Equal(t, 1, st.LiveItems)
		assert.Equal(t, 1, st.Observers)
	})
}

// TestAVFExpiredURL plays the signed links of the gate (the CDN's part):
// a link expired before the load fails with an error event, and so does a
// long stream whose links are revoked while it plays, at the seek that
// needs a segment not fetched yet. The class of each error (media
// sentinel) is logged: the player asks for a new link on media.ErrNetwork
// (04 §2.1, P4.3), so the class decides whether it refreshes.
func TestAVFExpiredURL(t *testing.T) { //nolint:paralleltest // one app a process
	t.Run("at load", func(t *testing.T) {
		h := newHarness(t)
		start := time.Now()
		h.load(t, theMedia.Signed(hlsMaster, time.Now().Add(-time.Minute)), media.LoadOptions{Auto: true})
		ev := h.await(t, kind(media.EventError), loadWithin)
		require.Error(t, ev.Err)
		_, refused := theMedia.Counts()
		t.Logf("avf-measure: expired at load: error after %v: %v (class %s), gate refused %d",
			time.Since(start).Round(time.Millisecond), ev.Err, classOf(ev.Err), refused)
		h.onUI(t, func() { require.Error(t, h.eng.Play(), "a failed load does not play") })
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
		t.Logf("avf-measure: revoked while playing: gate served %d before the revoke", served0)
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
		var logN, logCode int
		var logDomain, logComment string
		h.onUI(t, func() { logN, logDomain, logCode, logComment, _ = avf.ErrorLog(h.eng) })
		t.Logf("avf-measure: revoked while playing: error %t after %v: %v (class %s), states %v, gate refused %d, error log %d entries, last %s %d %q",
			ok, time.Since(start).Round(time.Millisecond), ev.Err, classOf(ev.Err), seen, refused, logN, logDomain, logCode, logComment)
		if !ok {
			p0 := h.readTime(t, token)
			wait(3 * time.Second)
			t.Logf("avf-measure: revoked while playing: position %.2f then %.2f 3 s later", p0, h.readTime(t, token))
		}
		require.True(t, ok, "an error event once the links are refused")
		require.Error(t, ev.Err)
	})
}

// TestAVFMaxResolution caps the item at 480p (854×480, avftv's
// mapping of the quality setting) before a load and checks the item has
// the cap and still plays the lesson; lifted (0×0), the item has none.
func TestAVFMaxResolution(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	h.onUI(t, func() { require.True(t, avf.SetMaxResolution(h.eng, 854, 480)) })
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	require.True(t, pullUntil(t, h, frameWithin, func(img *scene.Image, _ uint64) bool { return img != nil }), "frames under the cap")
	h.onUI(t, func() {
		w, hh, ok := avf.MaxResolution(h.eng)
		require.True(t, ok)
		assert.Equal(t, [2]float64{854, 480}, [2]float64{w, hh})
		require.True(t, avf.SetMaxResolution(h.eng, 0, 0))
		w, hh, ok = avf.MaxResolution(h.eng)
		require.True(t, ok)
		assert.Equal(t, [2]float64{0, 0}, [2]float64{w, hh})
	})
	h.expectNone(t, kind(media.EventError), time.Second)
}

// TestAVFMirror shows a paused frame in a view of the window, plain and
// mirrored (the view's flipped draw, 04 §2.3), screenshots both through
// the driver, and checks that the mirrored picture is the plain one
// turned about its vertical axis.
func TestAVFMirror(t *testing.T) { //nolint:paralleltest // one app a process
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
	t.Logf("avf-measure: mirror: picture %v px, mean |Δluma| mirrored vs plain %.2f, vs plain flipped %.2f", frame, same, flipped)
	assert.Less(t, flipped, 4.0, "the mirrored picture is the plain one flipped")
	assert.Greater(t, same, 2*flipped+1, "and not the plain one")
}

// TestAVFBackground checks the background hooks (media.Backgrounder): no
// time while in the background; back, one time at once and the frame
// shown again, also while paused.
func TestAVFBackground(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	setBackground(t, h, true)
	h.drain()
	h.expectNone(t, kind(media.EventTime), 1200*time.Millisecond)
	setBackground(t, h, false)
	h.await(t, kind(media.EventTime), 100*time.Millisecond)
	var s0 uint64
	require.True(t, pullUntil(t, h, frameWithin, func(_ *scene.Image, seq uint64) bool { s0 = seq; return seq > 0 }))
	h.onUI(t, func() { require.NoError(t, h.eng.Pause()) })
	h.await(t, state(media.StatePaused), 2*time.Second)
	require.True(t, pullUntil(t, h, 2*time.Second, func(_ *scene.Image, seq uint64) bool {
		s0 = seq

		return !h.eng.Video().Active()
	}))
	setBackground(t, h, true)
	setBackground(t, h, false)
	h.await(t, kind(media.EventTime), 100*time.Millisecond)
	assert.True(t, pullUntil(t, h, time.Second, func(_ *scene.Image, seq uint64) bool { return seq > s0 }),
		"a frame back from the background while paused")
}

// TestAVFBackgroundStopsFrames shows a playing video in a view alone in
// its window: in the background the surface is not Active, the view stops
// painting, and for 2 s the engine pulls nothing and no frame comes, even
// through a pause, a seek and a play; back in the foreground the frames
// run again.
func TestAVFBackgroundStopsFrames(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	v := addView(t, h)
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	v.awaitFrames(t, 24, frameWithin, "a second of frames before the background")
	stats := func() avf.Stats {
		var st avf.Stats
		h.onUI(t, func() {
			var ok bool
			st, ok = avf.StatsOf(h.eng)
			require.True(t, ok)
		})

		return st
	}
	setBackground(t, h, true)
	h.onUI(t, func() { assert.False(t, h.eng.Video().Active(), "not Active in the background") })
	v.awaitStill(t, h)
	v.flush()
	var paints0 int
	h.onUI(t, func() { paints0 = v.paints })
	st0 := stats()
	h.onUI(t, func() { require.NoError(t, h.eng.Pause()) })
	h.await(t, state(media.StatePaused), 2*time.Second)
	h.onUI(t, func() { require.NoError(t, h.eng.Seek(4)) })
	h.onUI(t, func() { require.NoError(t, h.eng.Play()) })
	h.await(t, state(media.StatePlaying), 2*time.Second)
	v.awaitNoFrame(t, 2*time.Second, "in the background")
	st1 := stats()
	var paints1 int
	var active bool
	h.onUI(t, func() { paints1, active = v.paints, h.eng.Video().Active() })
	t.Logf("avf-measure: background 2 s: pulls %d, frames %d, paints %d", st1.Pulls-st0.Pulls, st1.Frames-st0.Frames, paints1-paints0)
	assert.False(t, active, "not Active after a pause, a seek and a play in the background")
	assert.Equal(t, st0.Pulls, st1.Pulls, "no pull in the background")
	assert.Equal(t, st0.Frames, st1.Frames, "no frame copied in the background")
	assert.Equal(t, paints0, paints1, "the view did not paint in the background")
	assert.Equal(t, 1, st1.VideoTracksOff, "the video track off in the background")
	setBackground(t, h, false)
	h.onUI(t, func() { assert.True(t, h.eng.Video().Active(), "Active back in the foreground") })
	start := time.Now()
	v.awaitFrames(t, 24, frameWithin, "a second of frames back in the foreground")
	t.Logf("avf-measure: back in the foreground, 24 frames in %v", time.Since(start).Round(time.Millisecond))
}

// TestAVFTickDoesNotAllocate checks the 250 ms tick of a playing video:
// no allocation (the garbage collector off, as the pools of objc.Call and
// objc.Invocation refill once after each collection).
func TestAVFTickDoesNotAllocate(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	token := h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	h.await(t, kind(media.EventTime), time.Second)
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	var allocs, readAllocs float64
	var cost []time.Duration
	h.onUI(t, func() {
		rt := h.eng.(media.TimeReader)
		for range 20 {
			avf.Tick(h.eng)
			_, _ = rt.ReadTime(token)
		}
		allocs = testing.AllocsPerRun(200, func() { avf.Tick(h.eng) })
		readAllocs = testing.AllocsPerRun(200, func() { _, _ = rt.ReadTime(token) })
		for range 200 {
			s := time.Now()
			avf.Tick(h.eng)
			cost = append(cost, time.Since(s))
		}
	})
	assert.Zero(t, allocs, "allocations per tick")
	assert.Zero(t, readAllocs, "allocations per ReadTime")
	p50, p95 := percentiles(cost)
	t.Logf("avf-measure: tick p50 %v p95 %v, %.2f allocs per tick, %.2f per ReadTime", p50, p95, allocs, readAllocs)
}

// TestAVFPullDoesNotAllocate pulls a playing video at 60 Hz for 3 s and
// counts the allocations of each pull, with and without a new frame. The
// counts are the process's (runtime.MemStats), so the engine's tick is
// stopped meanwhile (background on, the video tracks kept enabled so that
// the frames go on coming; measurePulls pulls by hand); the pull without
// a frame is also measured alone, by testing.AllocsPerRun.
func TestAVFPullDoesNotAllocate(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	require.True(t, pullUntil(t, h, frameWithin, func(_ *scene.Image, seq uint64) bool { return seq >= 4 }))
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	h.onUI(t, func() { avf.SetKeepVideoInBackground(h.eng, true) })
	setBackground(t, h, true)
	defer setBackground(t, h, false)
	// The background frees the surface's images: a second of pulls, made
	// whatever Active says, makes the three again.
	pullForce(t, h, time.Second, func(*scene.Image, uint64) {})
	r := measurePulls(t, h, 3*time.Second)
	var empty float64
	h.onUI(t, func() { empty = testing.AllocsPerRun(1000, h.eng.Video().Pull) })
	p50, p95 := percentiles(r.frameCost)
	t.Logf("avf-measure: pulls %d, frames %d, mallocs with a frame %d (in %d pulls), without %d (in %d pulls); AllocsPerRun of 1000 pulls %.3f; pull+copy p50 %v p95 %v",
		r.pulls, r.frames, r.frameMallocs, r.frameDirty, r.emptyMallocs, r.emptyDirty, empty, p50, p95)
	assert.Positive(t, r.frames)
	assert.Zero(t, r.frameMallocs, "allocations of the pulls with a frame")
	assert.Zero(t, r.emptyMallocs, "allocations of the pulls without")
	assert.Zero(t, empty, "allocations of pulls back to back")
}

// TestAVFNoLeakOverLoads loads the HLS and the MP3 three times each, in
// turn, playing each: the engine holds one item, one observer and one
// player after each, and neither the Go heap nor the process's footprint
// grows from the first round to the third beyond a margin.
func TestAVFNoLeakOverLoads(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	var rounds []map[string]int64
	for i := range 3 {
		setStatus(fmt.Sprintf("avf: leaks, round %d", i+1))
		h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
		h.await(t, state(media.StatePlaying), playingWithin)
		require.True(t, pullUntil(t, h, frameWithin, func(img *scene.Image, _ uint64) bool { return img != nil }))
		h.loadSrc(t, media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Auto: true})
		h.await(t, state(media.StatePlaying), playingWithin)
		wait(500 * time.Millisecond)
		m := memSnapshot(t, h)
		rounds = append(rounds, m)
		t.Logf("avf-measure: leaks round %d: items %d observers %d players %d goHeap %.2f MiB footprint %.1f MiB",
			i+1, m["LiveItems"], m["Observers"], m["Players"], mib(m["goHeap"]), mib(m["FootprintBytes"]))
		assert.Equal(t, int64(1), m["LiveItems"])
		assert.Equal(t, int64(1), m["Observers"])
		assert.Equal(t, int64(1), m["Players"])
	}
	assert.Less(t, rounds[2]["goHeap"]-rounds[0]["goHeap"], int64(2<<20), "the Go heap, round 1 to 3")
	assert.Less(t, rounds[2]["FootprintBytes"]-rounds[0]["FootprintBytes"], int64(16<<20), "the footprint, round 1 to 3")
}

// TestAVFStartPausedShowsItsFrame loads the HLS paused from 6 s (the
// video screen's resume) in a view alone: the frame shown is the one at
// 6 s, the same as a seek to 6 s while paused brings, not the item's
// first (its burnt-in timecode differs), and the pulls stop after it.
func TestAVFStartPausedShowsItsFrame(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	v := addView(t, h)
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Start: 6})
	h.await(t, state(media.StatePaused), loadWithin)
	v.awaitFrames(t, 1, frameWithin, "the paused frame")
	v.awaitStill(t, h)
	atLoad := frameSum(t, h)
	h.onUI(t, func() { require.NoError(t, h.eng.Seek(0)) })
	v.awaitFrames(t, 1, frameWithin, "the frame at 0")
	v.awaitStill(t, h)
	atZero := frameSum(t, h)
	h.onUI(t, func() { require.NoError(t, h.eng.Seek(6)) })
	v.awaitFrames(t, 1, frameWithin, "the frame at 6 s")
	v.awaitStill(t, h)
	atSix := frameSum(t, h)
	t.Logf("avf-measure: paused load from 6 s: frame sum %d, after a seek to 0 %d, to 6 s %d", atLoad, atZero, atSix)
	assert.NotEqual(t, atZero, atSix, "the frames at 0 and 6 s differ")
	assert.Equal(t, atSix, atLoad, "the paused load shows the frame of its start")
	h.onUI(t, func() { assert.False(t, h.eng.Video().Active(), "the pulls stopped") })
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
