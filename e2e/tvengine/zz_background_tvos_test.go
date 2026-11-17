// Adapted from jplayer-mobile/e2e/engine/zz_background_ios_test.go at 49de3c2 (the background measure) and jplayer-tv/scripts/tvos-avf-test.sh at 781bee5 (the background and foreground requests).

//go:build ios && tvos

package tvengine

// The file's name runs it last: it sends the app to the background and
// back, through the script.

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avftv"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tvPhase is how long the app stays in the background.
const tvPhase = 8 * time.Second

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestZZBackground plays the MP3, then the HLS, sends the app to the
// background for real (the TV Settings app in front) for tvPhase each, the
// engine told of it as the app's player does (media.Backgrounder, P7.5a),
// and measures what played there: the sound plays on (the audio
// background mode, TV09), the video pauses at once and tells Paused
// (tvOS would stop it about 2 s later without a word, Q-P4.1-2). Back in
// the front the engine still answers: the time, a pause, a play.
//
// On an Apple TV (scripts/tvos-avf-test.sh device-e2e) the app with its
// video paused plays nothing in the background, so tvOS suspends it until
// the script brings it back: the time at the move to the background is
// read in the lifecycle callback itself, before the suspension, and the
// move is remembered (wentBack) rather than awaited as the state of now.
func TestZZBackground(t *testing.T) { //nolint:paralleltest // one app a process
	var mu sync.Mutex
	life := platform.LifecycleActive
	var remove func()
	var eng media.Backgrounder // the engine of the subtest running; UI thread
	var reader *harness        // its harness and load, for the time at the move; UI thread
	var reading uint64
	wentBack, atBack := false, 0.0 // the move to the background and the time then; under mu
	onUI(t, func() {
		remove = tv.app.OnLifecycle(func(s platform.LifecycleState) {
			mu.Lock()
			life = s
			mu.Unlock()
			if eng == nil {
				return
			}
			switch s {
			case platform.LifecycleBackground:
				eng.SetBackground(true)
				if reader != nil {
					v, _ := reader.eng.ReadTime(reading)
					mu.Lock()
					wentBack, atBack = true, v
					mu.Unlock()
				}
			case platform.LifecycleActive:
				eng.SetBackground(false)
			case platform.LifecycleUnknown, platform.LifecycleInactive:
			default:
			}
		})
	})
	t.Cleanup(func() { onUI(t, remove) })
	went := func() bool {
		mu.Lock()
		defer mu.Unlock()

		return wentBack
	}
	is := func(s platform.LifecycleState) func() bool {
		return func() bool {
			mu.Lock()
			defer mu.Unlock()

			return life == s
		}
	}
	for _, src := range []media.Source{{URL: mediaURL(mp3Sample), AudioOnly: true}, {URL: mediaURL(hlsMaster)}} {
		name := "hls"
		if src.AudioOnly {
			name = "mp3"
		}
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			onUI(t, func() { eng = h.eng })
			t.Cleanup(func() { onUI(t, func() { eng, reader = nil, nil }) })
			token := h.load(t, src, media.LoadOptions{Auto: true})
			h.await(t, state(media.StatePlaying), playingWithin)
			onUI(t, func() {
				reader, reading = h, token
				mu.Lock()
				wentBack = false
				mu.Unlock()
			})
			p0 := h.readTime(t, token)
			if !ask("background", 30*time.Second) {
				t.Skip("needs scripts/tvos-avf-test.sh")
			}
			waitUntil(t, 15*time.Second, "the background", went)
			mu.Lock()
			pb := atBack
			mu.Unlock()
			wait(tvPhase)
			var p1 float64
			var read bool
			onUI(t, func() { p1, read = h.eng.ReadTime(token) })
			var states []string
			paused := false
			for len(h.events) > 0 {
				if ev := <-h.events; ev.Kind == media.EventState {
					states = append(states, fmt.Sprint(ev.State))
					paused = paused || ev.State == media.StatePaused
				}
			}
			require.True(t, ask("foreground", 30*time.Second))
			waitUntil(t, 15*time.Second, "active again", is(platform.LifecycleActive))
			p2 := h.readTime(t, token)
			measure("tv-background", "src", name, "phase", tvPhase, "played_there", fmt.Sprintf("%.2f", p1-p0),
				"there_since_lifecycle", fmt.Sprintf("%.2f", p1-pb), "read", read, "states_there", fmt.Sprint(states),
				"position_back", fmt.Sprintf("%.2f", p2))
			assert.True(t, read, "the time read in the background")
			if src.AudioOnly {
				assert.False(t, paused, "a sound plays on: %v", states)
				assert.Greater(t, p1-p0, (tvPhase - time.Second).Seconds(), "the sound played in the background")
				onUI(t, func() { require.NoError(t, h.eng.Pause()) })
				h.await(t, state(media.StatePaused), 3*time.Second)
			} else {
				assert.True(t, paused, "the video told Paused in the background: %v", states)
				assert.Less(t, p1-pb, 0.1, "the video paused as the app left")
				assert.InDelta(t, p1, p2, 0.1, "and stayed paused")
				h.expectNone(t, state(media.StatePlaying), 500*time.Millisecond)
			}
			served0, _ := tv.media.Counts()
			onUI(t, func() { require.NoError(t, h.eng.Seek(0)) })
			onUI(t, func() { require.NoError(t, h.eng.Play()) })
			var seen []string
			_, ok := h.awaitOr(func(ev media.Event) bool {
				seen = append(seen, fmt.Sprintf("%d/%d", ev.Kind, ev.State))

				return ev.Kind == media.EventState && ev.State == media.StatePlaying
			}, playingWithin)
			if !ok {
				// What failed after the return: the engine, or the test's own
				// server on 127.0.0.1 (a suspended app's listener).
				served1, _ := tv.media.Counts()
				var rate, at0, at1 float64
				var ticked bool
				onUI(t, func() { rate, _, _ = avftv.PlayerRate(h.eng); at0, _ = h.eng.ReadTime(token) })
				wait(time.Second)
				onUI(t, func() { at1, _ = h.eng.ReadTime(token); ticked = avftv.Tick(h.eng) })
				_, tickPlays := h.awaitOr(state(media.StatePlaying), time.Second)
				measure("tv-background-return-player", "rate", rate, "pos_advanced_1s", fmt.Sprintf("%.2f", at1-at0),
					"manual_tick", ticked, "playing_after_tick", tickPlays)
				fresh := h.load(t, src, media.LoadOptions{Auto: true})
				_, again := h.awaitOr(func(ev media.Event) bool {
					return ev.Token == fresh && ev.Kind == media.EventState && ev.State == media.StatePlaying
				}, loadWithin)
				served2, refused2 := tv.media.Counts()
				reached := probe(tv.media.URL(hlsMaster))
				measure("tv-background-return-failed", "src", name, "events", fmt.Sprint(seen), "served_during", served1-served0,
					"served_during_fresh_load", served2-served1, "refused", refused2, "requests", tv.media.Requests(),
					"rate", rate, "probe", reached, "fresh_load_plays", again)
				if served2 == served1 && reached == "200 OK" {
					// Apple TV 4K, tvOS 27.2 (local/p8/appletv): after the
					// suspension the system's media loader asks the test's
					// own 127.0.0.1 server for nothing, not even for a new
					// item, while a Go client in the app still gets 200. The
					// app's media come from the CDN; its own return from
					// the background plays on (local/p8/appletv R4).
					t.Skip("the media loader no longer reaches this app's loopback server after a suspension (device)")
				}
			}
			require.True(t, ok, "playing again after the return: %v", seen)
		})
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// probe GETs url with a short limit and returns the status or the error.
func probe(url string) string {
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(url) //nolint:noctx // a diagnostic with its own limit
	if err != nil {
		return "error " + err.Error()
	}
	_ = resp.Body.Close()

	return resp.Status
}
