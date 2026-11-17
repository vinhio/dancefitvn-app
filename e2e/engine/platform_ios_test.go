// Adapted from jplayer-mobile/e2e/engine/platform_ios_test.go at 49de3c2.

//go:build ios && !tvos

package engine

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avf"
	"os"
	"sync"
	"testing"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The audio session lives as long as the process (its observer is never
// removed), so the tests share one, made by the first newPlatform.
var (
	sessionOnce sync.Once
	sessionErr  error
	theSession  media.Session
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// newPlatform returns the iOS engine and the process's session for cfg.
// UI thread.
func newPlatform(cfg media.Config) (media.Engine, media.Session, error) {
	sessionOnce.Do(func() { theSession, sessionErr = avf.NewSession(cfg) })
	if sessionErr != nil {
		return nil, nil, sessionErr
	}
	eng, err := avf.New(cfg, theSession)
	if err != nil {
		return nil, nil, err
	}

	return eng, theSession, nil
}

// statsOf returns the engine's counters (LiveItems, Observers, Frames,
// Pulls, Players, FootprintBytes) and the session's (Activations), nil for
// another engine.
func statsOf(eng media.Engine) map[string]int64 {
	st, ok := avf.StatsOf(eng)
	if !ok {
		return nil
	}
	out := map[string]int64{
		"LiveItems": int64(st.LiveItems), "Observers": int64(st.Observers), "Players": int64(st.Players),
		"Frames": int64(st.Frames), "Pulls": int64(st.Pulls), "Outputs": int64(st.Outputs), "FootprintBytes": st.FootprintBytes,
	}
	if ss, ok := avf.SessionStatsOf(theSession); ok {
		out["Activations"] = int64(ss.Activations)
	}

	return out
}

// tickTime runs a tick now and returns the time it tells.
func (h *harness) tickTime(t *testing.T) float64 {
	t.Helper()

	h.drain()
	h.onUI(t, func() { avf.Tick(h.eng) })

	return h.await(t, kind(media.EventTime), time.Second).Time
}

// replyDir is where devtest-ios.sh's driver writes its answers: the app's
// temporary folder.
func replyDir() string { return os.TempDir() }
