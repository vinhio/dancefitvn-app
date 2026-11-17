// Adapted from jplayer-tv/internal/media/avf/engine_tvos_test.go at 781bee5 (TestTimeToPlaying: timing on the UI thread).

//go:build ios && !tvos

package engine

// The file's name runs it first: the first session, engine and Load of
// the process.

import (
	"context"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avf"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestAALoadUIThreadTime times, on the UI thread, the first
// avf.NewSession, avf.New and Load of the process (the HLS, played), then
// a warm Load, for the record (the TV's measure is R-P4-6's; the iPhone's
// is its twin).
func TestAALoadUIThreadTime(t *testing.T) { //nolint:paralleltest // one app a process
	var sessTook, newTook, firstTook, warmTook time.Duration
	var eng media.Engine
	events := make(chan media.Event, eventBuffer)
	require.NoError(t, theApp.Invoke(func() {
		cfg := media.Config{Dispatch: dispatch, Clock: clock.System(dispatch)}
		start := time.Now()
		sessionOnce.Do(func() { theSession, sessionErr = avf.NewSession(cfg) })
		sessTook = time.Since(start)
		require.NoError(t, sessionErr)
		start = time.Now()
		var err error
		eng, err = avf.New(cfg, theSession)
		newTook = time.Since(start)
		require.NoError(t, err)
		eng.Events().Subscribe(func(ev media.Event) {
			select {
			case events <- ev:
			default:
			}
		})
	}))
	t.Cleanup(func() { require.NoError(t, theApp.Invoke(func() { _ = eng.Close() })) })
	h := &harness{app: theApp, eng: eng, sess: theSession, events: events}
	for _, took := range []*time.Duration{&firstTook, &warmTook} {
		h.onUI(t, func() {
			start := time.Now()
			_, err := eng.Load(context.Background(), media.Source{URL: mediaURL(hlsMaster)}, media.LoadOptions{Auto: true})
			*took = time.Since(start)
			require.NoError(t, err)
		})
		h.await(t, state(media.StatePlaying), loadWithin)
		h.drain()
	}
	t.Logf("avf-measure: ios ui thread: NewSession %v, New %v, first Load(Auto) %v, warm Load(Auto) %v",
		sessTook.Round(time.Microsecond), newTook.Round(time.Microsecond), firstTook.Round(time.Microsecond), warmTook.Round(time.Microsecond))
}
