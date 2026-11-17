// Adapted from jplayer-tv/internal/media/avf/engine_tvos_test.go at 781bee5 and jplayer-tv/internal/media/avf/plane_tvos_test.go at 781bee5 (play, pause, stop, the plane under the canvas, the counts kept flat, the drain without allocation), with the seek, speed and end tests of jplayer-mobile/e2e/engine/avf_ios_test.go at 49de3c2.

//go:build ios && tvos

package tvengine

import (
	"context"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avftv"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The speeds of the options sheet (docs/plan/04-media-engine.md §2.2).
var rates = []float64{0.5, 0.75, 1, 1.25, 1.5}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestTVPlaysHLS plays the lesson's HLS on the native plane: its length,
// playing within playingWithin, the pitch kept (TimeDomain), one layer
// under the canvas, shown at the pane's frame.
func TestTVPlaysHLS(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	start := time.Now()
	token := h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true})
	dur := h.await(t, kind(media.EventDuration), loadWithin)
	assert.Equal(t, token, dur.Token)
	assert.InDelta(t, hlsLength, dur.Duration, 0.1)
	h.await(t, state(media.StatePlaying), playingWithin)
	playing := time.Since(start)
	waitUntil(t, 2*time.Second, "the plane shown", func() bool { p := lastPlane(); return p != nil && !p.hidden })
	onUI(t, func() {
		pitch, ok := avftv.PitchAlgorithm(h.eng)
		assert.True(t, ok)
		assert.Equal(t, "TimeDomain", pitch)
		st := h.eng.Stats()
		assert.Equal(t, 1, st.Items)
		assert.Equal(t, 1, st.Layers)
		p := lastPlane()
		assert.Equal(t, tv.root.pane.WindowFrame(), p.frame, "the plane at the pane's frame")
	})
	measure("tv-hls", "load_to_playing", playing.Round(time.Millisecond))
}

// TestTVPlaysMP3 plays the sample MP3 as audio only: its length, the time
// going on, and the end after a seek near it.
func TestTVPlaysMP3(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	start := time.Now()
	h.load(t, media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Auto: true})
	dur := h.await(t, kind(media.EventDuration), loadWithin)
	assert.InDelta(t, mp3Length, dur.Duration, 0.3)
	h.await(t, state(media.StatePlaying), playingWithin)
	playing := time.Since(start)
	t0 := h.await(t, func(ev media.Event) bool { return ev.Kind == media.EventTime && ev.Time > 0.2 }, 2*time.Second).Time
	t1 := h.await(t, func(ev media.Event) bool { return ev.Kind == media.EventTime && ev.Time > t0+0.2 }, 2*time.Second).Time
	assert.Greater(t, t1, t0)
	assert.Nil(t, h.eng.Video(), "the view draws no frame on the TV")
	onUI(t, func() { require.NoError(t, h.eng.Seek(mp3Length-2)) })
	h.await(t, kind(media.EventEnded), 5*time.Second)
	measure("tv-mp3", "load_to_playing", playing.Round(time.Millisecond))
}

// TestTVRates plays the HLS and the MP3 at each speed of the sheet and
// measures the position against the wall clock over 3 s: within ±3 % of
// the rate, the player's rate and defaultRate read back as set.
func TestTVRates(t *testing.T) { //nolint:paralleltest // one app a process
	for _, src := range []media.Source{{URL: mediaURL(hlsMaster)}, {URL: mediaURL(mp3Sample), AudioOnly: true}} {
		name := "hls"
		if src.AudioOnly {
			name = "mp3"
		}
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			token := h.load(t, src, media.LoadOptions{})
			h.await(t, state(media.StatePaused), loadWithin)
			for _, r := range rates {
				onUI(t, func() {
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
				onUI(t, func() {
					var ok bool
					rate, def, ok = avftv.PlayerRate(h.eng)
					require.True(t, ok)
				})
				measure("tv-rate", "src", name, "rate", r, "speed", fmt.Sprintf("%.3f", speed),
					"player_rate", fmt.Sprintf("%.2f", rate), "default_rate", fmt.Sprintf("%.2f", def))
				assert.InDelta(t, r, speed, r*rateSlack, "speed at %.2f", r)
				assert.InDelta(t, r, rate, 0.001)
				assert.InDelta(t, r, def, 0.001)
				onUI(t, func() { require.NoError(t, h.eng.Pause()) })
				h.await(t, state(media.StatePaused), 2*time.Second)
			}
		})
	}
}

// TestTVRateWhilePaused checks that a speed set while paused does not
// start the media, and that it plays at that speed once played.
func TestTVRateWhilePaused(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	token := h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{})
	h.await(t, state(media.StatePaused), loadWithin)
	onUI(t, func() { require.NoError(t, h.eng.SetRate(1.5)) })
	h.expectNone(t, state(media.StatePlaying), time.Second)
	assert.InDelta(t, 0, h.readTime(t, token), seekSlack, "the position did not move")
	onUI(t, func() {
		rate, def, ok := avftv.PlayerRate(h.eng)
		require.True(t, ok)
		assert.Zero(t, rate, "paused")
		assert.InDelta(t, 1.5, def, 0.001, "the rate play uses")
	})
	onUI(t, func() { require.NoError(t, h.eng.Play()) })
	h.await(t, state(media.StatePlaying), playingWithin)
	wait(300 * time.Millisecond)
	p0, w0 := h.readTime(t, token), time.Now()
	wait(2 * time.Second)
	p1, w1 := h.readTime(t, token), time.Now()
	speed := (p1 - p0) / w1.Sub(w0).Seconds()
	measure("tv-rate-set-paused", "speed", fmt.Sprintf("%.3f", speed))
	assert.InDelta(t, 1.5, speed, 1.5*rateSlack)
}

// TestTVSeekPaused seeks a paused video by +10 s and −10 s from 1 s, as
// the ±10 buttons of the remote's screen do: exact (11.0 and 1.0), read
// at once and after the seek finished.
func TestTVSeekPaused(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	token := h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Start: 1})
	h.await(t, state(media.StatePaused), loadWithin)
	assert.InDelta(t, 1, h.readTime(t, token), seekSlack)
	for _, step := range []struct {
		by, want float64
	}{{10, 11}, {-10, 1}} {
		var at float64
		onUI(t, func() {
			pos, ok := h.eng.ReadTime(token)
			require.True(t, ok)
			require.NoError(t, h.eng.Seek(pos+step.by))
			at, _ = h.eng.ReadTime(token)
		})
		wait(time.Second)
		got := h.readTime(t, token)
		measure("tv-seek-paused", "by", step.by, "at_once", fmt.Sprintf("%.3f", at), "after", fmt.Sprintf("%.3f", got))
		assert.InDelta(t, step.want, at, seekSlack, "at once, seek %+.0f", step.by)
		assert.InDelta(t, step.want, got, seekSlack, "after the seek %+.0f", step.by)
	}
}

// TestTVABLoop plays the HLS with an A–B loop of 2 s to 6 s kept by the
// test as the player will (the tick's time ≥ B seeks to A): at least two
// jumps in 9 s, each to A exactly, never more than a tick past B; once
// the loop is off the media plays past B.
func TestTVABLoop(t *testing.T) { //nolint:paralleltest // one app a process
	const a, b = 2.0, 6.0
	h := newHarness(t)
	token := h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Start: a})
	h.await(t, state(media.StatePaused), loadWithin)
	var (
		jumps   int
		landed  []float64
		highest float64
		armed   bool
	)
	var unsub func()
	onUI(t, func() {
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
				tv.app.Enqueue(func() { _ = h.eng.Seek(a) })
			}
		})
		require.NoError(t, h.eng.Play())
	})
	h.await(t, state(media.StatePlaying), playingWithin)
	wait(9 * time.Second)
	var gotJumps int
	var gotLanded []float64
	var gotHighest float64
	onUI(t, func() {
		unsub()
		gotJumps, gotLanded, gotHighest = jumps, append([]float64(nil), landed...), highest
	})
	measure("tv-ab-loop", "a", a, "b", b, "jumps_in_9s", gotJumps, "landed", fmt.Sprint(gotLanded), "highest", fmt.Sprintf("%.3f", gotHighest))
	assert.GreaterOrEqual(t, gotJumps, 2)
	for _, l := range gotLanded {
		assert.InDelta(t, a, l, seekSlack, "landed on A")
	}
	assert.Less(t, gotHighest, b+0.3, "no further than a tick past B")
	h.await(t, func(ev media.Event) bool { return ev.Kind == media.EventTime && ev.Time > b+0.5 }, 6*time.Second)
}

// TestTVEndOnce plays the HLS to its end: the last time before Ended, no
// pause at the end, Ended once, and Play plays again from the start.
func TestTVEndOnce(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true, Start: hlsLength - 3})
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
	// Ended once, and no Playing after it: a reading of the player taken
	// as the end came still said playing (fixed in avftv's onEnded).
	h.expectNone(t, func(ev media.Event) bool {
		return ev.Kind == media.EventEnded || (ev.Kind == media.EventState && ev.State == media.StatePlaying)
	}, 2*time.Second)
	onUI(t, func() { require.NoError(t, h.eng.Play()) })
	var after []string
	_, ok := h.awaitOr(func(ev media.Event) bool {
		after = append(after, fmt.Sprintf("%d/%d/%.2f", ev.Kind, ev.State, ev.Time))

		return ev.Kind == media.EventState && ev.State == media.StatePlaying
	}, 3*time.Second)
	if !ok {
		var control int
		var reason string
		onUI(t, func() { control, reason = playerWaiting() })
		measure("tv-end-play-again", "events", fmt.Sprint(after), "control", control, "reason", reason)
	}
	require.True(t, ok, "Playing after Play at the end")
	assert.Less(t, h.tickTime(t), 1.0, "played again from the start")
}

// TestTVStaleEventsDropped loads two media in a row: no event of the first
// comes after the second's load, and the engine holds one item.
func TestTVStaleEventsDropped(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	var a, b uint64
	onUI(t, func() {
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
	onUI(t, func() { assert.Equal(t, 1, h.eng.Stats().Items) })
}

// TestTVExpiredURL plays the gate's signed links (the CDN's part): a link
// expired before the load fails with an error event, and so does a long
// stream whose links are revoked while it plays, at the seek that needs a
// segment not fetched yet. The class of each error is measured: the
// player asks for a new link on an engine error (04 §2.1, P4.3).
func TestTVExpiredURL(t *testing.T) { //nolint:paralleltest // one app a process
	t.Run("at load", func(t *testing.T) {
		h := newHarness(t)
		start := time.Now()
		h.load(t, media.Source{URL: tv.media.Signed(hlsMaster, time.Now().Add(-time.Minute))}, media.LoadOptions{Auto: true})
		ev := h.await(t, kind(media.EventError), loadWithin)
		require.Error(t, ev.Err)
		measure("tv-expired-at-load", "after", time.Since(start).Round(time.Millisecond), "class", classOf(ev.Err), "err", fmt.Sprintf("%q", ev.Err.Error()))
		onUI(t, func() { require.Error(t, h.eng.Play(), "a failed load does not play") })
		h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true})
		h.await(t, state(media.StatePlaying), playingWithin)
	})
	t.Run("while playing", func(t *testing.T) {
		h := newHarness(t)
		t.Cleanup(func() { tv.media.Revoke(false) })
		token := h.load(t, media.Source{URL: tv.media.Long("l_3503", 20, time.Now().Add(time.Hour))}, media.LoadOptions{Auto: true})
		h.await(t, state(media.StatePlaying), playingWithin)
		served0, _ := tv.media.Counts()
		tv.media.Revoke(true)
		start := time.Now()
		onUI(t, func() { require.NoError(t, h.eng.Seek(200)) })
		var seen []string
		ev, ok := h.awaitOr(func(ev media.Event) bool {
			if ev.Kind == media.EventState {
				seen = append(seen, fmt.Sprint(ev.State))
			}

			return ev.Kind == media.EventError && ev.Token == token
		}, 30*time.Second)
		_, refused := tv.media.Counts()
		measure("tv-revoked-while-playing", "error", ok, "after", time.Since(start).Round(time.Millisecond), "class", classOf(ev.Err),
			"states", fmt.Sprint(seen), "served_before", served0, "refused", refused)
		require.True(t, ok, "an error event once the links are refused")
		require.Error(t, ev.Err)
	})
}

// TestTVMirror mirrors the paused picture: the layer's transform.scale.x
// reads −1, then 1, and the screenshot of the plane mirrored is the plain
// one turned about its vertical axis.
func TestTVMirror(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Start: 3})
	h.await(t, state(media.StatePaused), loadWithin)
	waitUntil(t, 2*time.Second, "the plane shown", func() bool { p := lastPlane(); return p != nil && !p.hidden })
	wait(time.Second) // the paused frame composited
	plain, ok := shot(t, "mirror-off")
	if !ok {
		t.Skip("needs scripts/tvos-avf-test.sh")
	}
	onUI(t, func() {
		h.eng.SetMirror(true)
		x, ok := avftv.LayerScaleX(h.eng)
		require.True(t, ok)
		assert.InDelta(t, -1, x, 1e-9)
		assert.True(t, h.eng.Mirrored())
	})
	wait(500 * time.Millisecond)
	mirrored, _ := shot(t, "mirror-on")
	var frame image.Rectangle
	onUI(t, func() {
		r := fitContain(480, 854, tv.root.pane.WindowFrame())
		scale := float32(plain.Bounds().Dx()) / full.W
		frame = image.Rect(int(r.X*scale)+2, int(r.Y*scale)+2, int((r.X+r.W)*scale)-2, int((r.Y+r.H)*scale)-2)
		h.eng.SetMirror(false)
		x, ok := avftv.LayerScaleX(h.eng)
		require.True(t, ok)
		assert.InDelta(t, 1, x, 1e-9)
	})
	same := lumaDiff(mirrored, plain, frame, false)
	flipped := lumaDiff(mirrored, plain, frame, true)
	measure("tv-mirror", "frame", frame, "diff_plain", fmt.Sprintf("%.2f", same), "diff_flipped", fmt.Sprintf("%.2f", flipped))
	assert.Less(t, flipped, 4.0, "the mirrored picture is the plain one flipped")
	assert.Greater(t, same, 2*flipped+1, "and not the plain one")
}

// TestTVPlace moves the pane's frame: the plane follows it; an empty Place
// hides it, a later one shows it again at the new frame.
func TestTVPlace(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	waitUntil(t, 2*time.Second, "the plane shown", func() bool { p := lastPlane(); return p != nil && !p.hidden })
	r := geom.XYWH(100, 100, 360, 640)
	onUI(t, func() {
		h.eng.Place(r)
		p := lastPlane()
		assert.Equal(t, r, p.frame)
		assert.False(t, p.hidden)
		h.eng.Place(geom.Rect{})
		assert.True(t, p.hidden, "an empty Place hides it")
		h.eng.Place(tv.root.pane.WindowFrame())
		assert.False(t, p.hidden)
		assert.Equal(t, tv.root.pane.WindowFrame(), p.frame)
		assert.Equal(t, 1, h.eng.Stats().Layers, "one layer all along")
	})
	h.expectNone(t, kind(media.EventError), 500*time.Millisecond)
}

// TestTVNoLeakOverLoads loads the HLS and the MP3 three times each, in
// turn, playing each: one item and one layer after each, the
// observations flat, and neither the Go heap nor the footprint growing
// from the first round to the third beyond a margin.
func TestTVNoLeakOverLoads(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	type round struct {
		st         avftv.Stats
		heap, foot int64
	}
	var rounds []round
	for i := range 3 {
		h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true})
		h.await(t, state(media.StatePlaying), playingWithin)
		h.load(t, media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Auto: true})
		h.await(t, state(media.StatePlaying), playingWithin)
		wait(500 * time.Millisecond)
		var r round
		onUI(t, func() { r.st = h.eng.Stats() })
		r.heap, r.foot = goHeap(), footprint()
		rounds = append(rounds, r)
		measure("tv-leaks", "round", i+1, "items", r.st.Items, "observations", r.st.Observations, "layers", r.st.Layers,
			"loads", r.st.Loads, "go_heap_mib", fmt.Sprintf("%.2f", mib(r.heap)), "footprint_mib", fmt.Sprintf("%.1f", mib(r.foot)))
		assert.Equal(t, 1, r.st.Items)
		assert.Equal(t, 1, r.st.Layers)
	}
	assert.Equal(t, rounds[0].st.Observations, rounds[2].st.Observations, "the observations flat")
	assert.Less(t, rounds[2].heap-rounds[0].heap, int64(2<<20), "the Go heap, round 1 to 3")
	assert.Less(t, rounds[2].foot-rounds[0].foot, int64(16<<20), "the footprint, round 1 to 3")
}

// TestTVTickDoesNotAllocate checks the 250 ms tick of a playing video,
// ReadTime and the player's rate read: no allocation (the garbage
// collector off, as the pools of objc.Call and objc.Invocation refill once
// after each collection).
func TestTVTickDoesNotAllocate(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	token := h.load(t, media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	h.await(t, kind(media.EventTime), time.Second)
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	var tick, read, place float64
	var cost []time.Duration
	onUI(t, func() {
		r := tv.root.pane.WindowFrame()
		for range 20 {
			avftv.Tick(h.eng)
			_, _ = h.eng.ReadTime(token)
			h.eng.Place(r)
		}
		tick = testing.AllocsPerRun(200, func() { avftv.Tick(h.eng) })
		read = testing.AllocsPerRun(200, func() { _, _ = h.eng.ReadTime(token) })
		place = testing.AllocsPerRun(200, func() { h.eng.Place(r) })
		for range 200 {
			s := time.Now()
			avftv.Tick(h.eng)
			cost = append(cost, time.Since(s))
		}
	})
	p50, p95 := percentiles(cost)
	measure("tv-tick", "p50", p50, "p95", p95, "allocs_tick", tick, "allocs_read_time", read, "allocs_place", place)
	assert.Zero(t, tick, "allocations per tick")
	assert.Zero(t, read, "allocations per ReadTime")
	assert.Zero(t, place, "allocations per Place unchanged")
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// playerWaiting reads the timeControlStatus of the player on the newest
// plane's layer and why it waits. UI thread.
func playerWaiting() (control int, reason string) {
	p := lastPlane()
	if p == nil {
		return -1, ""
	}
	objc.WithPool(func() {
		pl := objc.Call(objc.ID(p.layer), objc.Sel("player"))
		if pl == 0 {
			control = -2

			return
		}
		control = int(objc.Call(pl, objc.Sel("timeControlStatus")))
		reason = objc.GoString(objc.Call(pl, objc.Sel("reasonForWaitingToPlay")))
	})

	return control, reason
}

// shot asks the script for a screenshot and returns it decoded; false
// without the script.
func shot(t *testing.T, name string) (image.Image, bool) {
	t.Helper()

	_ = os.Remove(filepath.Join(os.TempDir(), "avf-shot-"+name+".png")) // the app's data outlives a run
	if !ask("screenshot "+name, 30*time.Second) {
		return nil, false
	}
	f, err := os.Open(filepath.Join(os.TempDir(), "avf-shot-"+name+".png")) //nolint:gosec // G304: the script's copy
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	require.NoError(t, err)

	return img, true
}

// fitContain returns the largest rectangle of w×h's proportions centred in
// box (object-fit: contain), as AVPlayerLayer's resizeAspect.
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

// lumaDiff is the mean absolute difference of the grey levels of a and b
// over r, b read mirrored about r's vertical axis when flip.
func lumaDiff(a, b image.Image, r image.Rectangle, flip bool) float64 {
	var sum, n float64
	for y := r.Min.Y; y < r.Max.Y; y += 2 {
		for x := r.Min.X; x < r.Max.X; x += 2 {
			bx := x
			if flip {
				bx = r.Max.X - 1 - (x - r.Min.X)
			}
			la, lb := luma(a, x, y), luma(b, bx, y)
			sum += math.Abs(la - lb)
			n++
		}
	}
	if n == 0 {
		return 0
	}

	return sum / n
}

// luma is the grey level of img at x, y, 0 to 255.
func luma(img image.Image, x, y int) float64 {
	r, g, b, _ := img.At(x, y).RGBA()

	return (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 257
}

// classOf names the media sentinel err wraps, "none" when it wraps none.
func classOf(err error) string {
	for _, c := range []struct {
		name string
		err  error
	}{
		{"ErrNetwork", media.ErrNetwork}, {"ErrSource", media.ErrSource}, {"ErrDecode", media.ErrDecode},
		{"ErrMissingFile", media.ErrMissingFile}, {"ErrAborted", media.ErrAborted},
	} {
		if errors.Is(err, c.err) {
			return c.name
		}
	}

	return "none"
}
