// Adapted from jplayer-mobile/e2e/engine/harness_test.go at 49de3c2.

//go:build (ios && !tvos) || (android && cgo)

package engine

import (
	"context"
	"dancefitvn-app/e2e/e2emedia"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/framework/host"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The app TestMain started (main_ios_test.go, main_android_test.go), the
// screen it shows, its
// view, and the media server. Set before the tests run, read-only after.
var (
	theApp    *host.App
	theScreen *screen
	theView   *ui.View
	theMedia  *e2emedia.Server
)

// The mock's media the tests play (e2e/prepare-media.sh): lesson l_3503's
// HLS (480×854, 24 fps, 3 × 4 s) and the sample MP3.
const (
	hlsMaster = "hls/l_3503/master.m3u8"
	hlsMedia  = "hls/l_3503/480p.m3u8"
	mp3Sample = "audio/_sample.mp3"
	// hlsLength and mp3Length are their lengths in seconds.
	hlsLength = 12.0
	mp3Length = 30.0
	// hlsW, hlsH is the picture's size.
	hlsW, hlsH = 480, 854
)

// eventBuffer is how many events a harness holds before it drops (and
// counts) the newest; a test awaiting events drains far sooner.
const eventBuffer = 8192

// The tests' timing.
const (
	loadWithin  = 10 * time.Second
	frameWithin = 3 * time.Second
	pullEvery   = 16 * time.Millisecond // a 60 Hz display
	// stillFor is how long a window must not paint to count as still.
	stillFor = 300 * time.Millisecond
)

// harness is an engine under test, on the app's UI thread.
type harness struct {
	app  *host.App
	eng  media.Engine
	sess media.Session
	// events receives every event of the engine, from the UI thread.
	events chan media.Event
	// dropped counts events that found events full (UI thread only).
	dropped int
}

// screen is the e2e window: a title and a status line.
type screen struct {
	ui.Flex
	status *ui.Text
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newScreen returns the window's content.
func newScreen() *screen {
	s := &screen{status: &ui.Text{Value: "DanceFITVN e2e", Wrap: true, Name: "status"}}
	s.Vertical, s.Gap = true, 12
	s.Align = ui.CrossStretch
	s.Add(&ui.Text{Value: "DanceFITVN e2e", Size: 20}, s.status)

	return s
}

// newHarness returns a harness on the platform's engine, closed when the
// test ends. It skips the test while the engine is not wired
// (core.ErrUnsupported); any other error fails it.
func newHarness(t *testing.T) *harness {
	t.Helper()

	h := &harness{app: theApp, events: make(chan media.Event, eventBuffer)}
	var err error
	h.onUI(t, func() {
		cfg := media.Config{Dispatch: dispatch, Clock: clock.System(dispatch)}
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
	if errors.Is(err, core.ErrUnsupported) {
		t.Skipf("engine not wired yet: %v", err)
	}
	require.NoError(t, err)

	return h
}

// dispatch hands fn to the UI thread, as media.Config.Dispatch asks.
func dispatch(fn func()) { theApp.Enqueue(fn) }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// onUI runs fn on the UI thread and returns when it has.
func (h *harness) onUI(t *testing.T, fn func()) {
	t.Helper()

	require.NoError(t, h.app.Invoke(fn))
}

// load loads url on the UI thread and returns the token; the call must
// succeed (errors that arrive as events are for await).
func (h *harness) load(t *testing.T, url string, opts media.LoadOptions) uint64 {
	t.Helper()

	return h.loadSrc(t, media.Source{URL: url}, opts)
}

// loadSrc is load of a whole source.
func (h *harness) loadSrc(t *testing.T, src media.Source, opts media.LoadOptions) uint64 {
	t.Helper()

	var (
		token uint64
		err   error
	)
	h.onUI(t, func() { token, err = h.eng.Load(context.Background(), src, opts) })
	require.NoError(t, err)

	return token
}

// await returns the first event pred accepts, skipping the others; it fails
// the test after within.
func (h *harness) await(t *testing.T, pred func(media.Event) bool, within time.Duration) media.Event {
	t.Helper()

	timer := time.NewTimer(within)
	defer timer.Stop()
	for {
		select {
		case ev := <-h.events:
			if pred(ev) {
				return ev
			}
		case <-timer.C:
			require.FailNow(t, "no such event", "within %v", within)
		}
	}
}

// awaitOr returns the first event pred accepts, skipping the others, and
// true; the zero event and false after within.
func (h *harness) awaitOr(pred func(media.Event) bool, within time.Duration) (media.Event, bool) {
	timer := time.NewTimer(within)
	defer timer.Stop()
	for {
		select {
		case ev := <-h.events:
			if pred(ev) {
				return ev, true
			}
		case <-timer.C:
			return media.Event{}, false
		}
	}
}

// expectNone fails the test when an event pred accepts comes within d.
func (h *harness) expectNone(t *testing.T, pred func(media.Event) bool, d time.Duration) {
	t.Helper()

	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		select {
		case ev := <-h.events:
			if pred(ev) {
				assert.Fail(t, "unexpected event", "%+v", ev)

				return
			}
		case <-timer.C:
			return
		}
	}
}

// drain drops the events queued so far.
func (h *harness) drain() {
	for {
		select {
		case <-h.events:
		default:
			return
		}
	}
}

// readTime reads the position of the load token now (media.TimeReader).
func (h *harness) readTime(t *testing.T, token uint64) float64 {
	t.Helper()

	rt, ok := h.eng.(media.TimeReader)
	require.True(t, ok, "the engine reads its time when asked")
	var (
		v    float64
		read bool
	)
	h.onUI(t, func() { v, read = rt.ReadTime(token) })
	require.True(t, read, "the time of load %d", token)

	return v
}

// state is a predicate for await: a state event of s.
func state(s media.PlayState) func(media.Event) bool {
	return func(ev media.Event) bool { return ev.Kind == media.EventState && ev.State == s }
}

// kind is a predicate for await: an event of k.
func kind(k media.EventKind) func(media.Event) bool {
	return func(ev media.Event) bool { return ev.Kind == k }
}

// mediaURL returns the link of a file of the media folder on the mock's
// server.
func mediaURL(name string) string { return theMedia.URL(name) }

// setBackground tells the engine the app went to the background, or came
// back.
func setBackground(t *testing.T, h *harness, on bool) {
	t.Helper()

	h.onUI(t, func() {
		b, ok := h.eng.(media.Backgrounder)
		require.True(t, ok)
		b.SetBackground(on)
	})
}

// memSnapshot returns the Go heap in use after a collection ("goHeap",
// bytes), plus the engine's own counters (statsOf).
func memSnapshot(t *testing.T, h *harness) map[string]int64 {
	t.Helper()

	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	out := map[string]int64{"goHeap": int64(ms.HeapAlloc)} //nolint:gosec // G115: a heap size far below 2^63
	h.onUI(t, func() {
		for k, v := range statsOf(h.eng) {
			out[k] = v
		}
	})

	return out
}

// setStatus shows s in the test window (any goroutine).
func setStatus(s string) {
	if theApp == nil || theScreen == nil {
		return
	}
	theApp.Enqueue(func() { theScreen.status.SetValue(s) })
}
