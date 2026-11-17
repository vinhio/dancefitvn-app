// Adapted from jplayer-mobile/e2e/engine/smoke_test.go at 49de3c2.

//go:build (ios && !tvos) || (android && cgo)

package engine

import (
	"dancefitvn-app/internal/media"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestSmokeAppRuns checks that the app is up, the tests run off the UI
// thread, onUI runs on it, and the media server listens.
func TestSmokeAppRuns(t *testing.T) { //nolint:paralleltest // one app a process
	require.NotNil(t, theApp)
	require.NotNil(t, theView)
	require.NotNil(t, theMedia)
	setStatus("smoke")

	var onUI bool
	require.NoError(t, theApp.Invoke(func() { onUI = theApp.Driver().OnUIThread() }))
	assert.True(t, onUI, "Invoke runs on the UI thread")
	assert.False(t, theApp.Driver().OnUIThread(), "the test goroutine is not the UI thread")
	assert.Contains(t, mediaURL(hlsMaster), "http://127.0.0.1:")
}

// TestSmokeMedia checks that the script put the mock's media where the
// server serves them.
func TestSmokeMedia(t *testing.T) { //nolint:paralleltest // one app a process
	for _, name := range []string{hlsMaster, hlsMedia, "hls/l_3503/480p_000.ts", mp3Sample} {
		_, err := os.Stat(filepath.Join(mediaDir(), filepath.FromSlash(name)))
		require.NoError(t, err, "test media %s: run through e2e/devtest-ios.sh", name)
	}
}

// TestSmokeHarness checks newHarness: the engine answers a load of the
// MP3 with a state event.
func TestSmokeHarness(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	h.loadSrc(t, media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), 10*time.Second)
	assert.Positive(t, memSnapshot(t, h)["goHeap"])
}
