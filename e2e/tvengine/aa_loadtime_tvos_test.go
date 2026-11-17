// Adapted from jplayer-tv/internal/media/avf/engine_tvos_test.go at 781bee5 (TestTimeToPlaying: timing on the UI thread, measure lines).

//go:build ios && tvos

package tvengine

// The file's name runs it first: the first Load of the process is the one
// R-P4-6 measured at 67–104 ms in P0.

import (
	"context"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avftv"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// loadBudget is what a Load may block the UI thread for: two frames at
// 60 Hz (R-P4-6).
const loadBudget = 33 * time.Millisecond

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestAALoadUIThreadTime times, on the UI thread, avftv.New and ten Loads
// (the lesson's HLS and the MP3 in turn, each played to Playing): the first
// Load of the process against the warm ones, and the first Load of a
// second engine. The first is the one the TV07 entry blocks on.
func TestAALoadUIThreadTime(t *testing.T) { //nolint:paralleltest // one app a process
	var newTook time.Duration
	h := &harness{events: make(chan media.Event, eventBuffer)}
	onUI(t, func() {
		start := time.Now()
		eng, err := avftv.New(avftv.Config{Clock: &viewClock{view: tv.view}, Post: tv.app.Enqueue, Underlay: underlay})
		newTook = time.Since(start)
		require.NoError(t, err)
		h.eng = eng
		h.eng.Events().Subscribe(func(ev media.Event) {
			select {
			case h.events <- ev:
			default:
			}
		})
		tv.root.pane.show(eng)
	})
	t.Cleanup(func() {
		onUI(t, func() {
			tv.root.pane.show(nil)
			assert.NoError(t, h.eng.Close())
		})
	})
	waitUntil(t, 3*time.Second, "the pane placed the plane", func() bool { return lastPlane() != nil })
	var took []time.Duration
	for i := range 10 {
		src := media.Source{URL: mediaURL(hlsMaster)}
		if i%2 == 1 {
			src = media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}
		}
		var d time.Duration
		onUI(t, func() {
			start := time.Now()
			_, err := h.eng.Load(context.Background(), src, media.LoadOptions{Auto: true})
			d = time.Since(start)
			require.NoError(t, err)
		})
		took = append(took, d)
		h.await(t, state(media.StatePlaying), loadWithin)
		h.drain()
	}
	second := newHarness(t)
	var secondTook time.Duration
	onUI(t, func() {
		start := time.Now()
		_, err := second.eng.Load(context.Background(), media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true})
		secondTook = time.Since(start)
		require.NoError(t, err)
	})
	second.await(t, state(media.StatePlaying), loadWithin)
	p50, p95 := percentiles(took[1:])
	measure("tv-load-ui-thread", "new", newTook.Round(time.Microsecond), "first", took[0].Round(time.Microsecond),
		"warm_p50", p50.Round(time.Microsecond), "warm_p95", p95.Round(time.Microsecond),
		"second_engine_first", secondTook.Round(time.Microsecond), "all", fmt.Sprint(took))
	assert.Less(t, took[0], loadBudget, "the first Load of the process on the UI thread")
	assert.Less(t, p95, loadBudget, "a warm Load on the UI thread")
}
