// Adapted from jplayer-mobile/e2e/engine/platform_android_test.go at 49de3c2.
// Changed: the package's import paths; no prober; tickTime, replyDir and
// onPhone added (the iOS side's tickTime calls avf.Tick).

//go:build android && cgo

package engine

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/amp"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// testNoisyAction is the broadcast the tests send for headphones pulled
// out: the system's AUDIO_BECOMING_NOISY is protected, so the session
// also takes this one (amp.NoisyActions).
const testNoisyAction = appID + ".TEST_AUDIO_BECOMING_NOISY"

// The process's audio session: the app keeps one for its life, and so do
// the tests, whatever engine they make.
var (
	sessionOnce sync.Once
	theSession  media.Session
	sessionErr  error
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// newPlatform returns the Android engine and the process's session for
// cfg. UI thread.
func newPlatform(cfg media.Config) (media.Engine, media.Session, error) {
	sess, err := platformSession()
	if err != nil {
		return nil, nil, err
	}
	eng, err := amp.New(cfg, sess)
	if err != nil {
		return nil, nil, fmt.Errorf("e2e: %w", err)
	}

	return eng, sess, nil
}

// statsOf returns the engine's counters (LiveProxies, LiveGlobalRefs,
// LiveReaders, frames, RSS...), nil for another engine.
func statsOf(eng media.Engine) map[string]int64 {
	s, ok := amp.StatsOf(eng)
	if !ok {
		return nil
	}

	return map[string]int64{
		"LiveGlobalRefs": int64(s.LiveGlobalRefs),
		"LiveProxies":    int64(s.LiveProxies),
		"LiveReaders":    int64(s.LiveReaders),
		"Frames":         s.Frames,
		"CopiedFrames":   s.CopiedFrames,
		"DroppedFrames":  s.DroppedFrames,
		"BadFrames":      s.BadFrames,
		"Releasing":      int64(s.Releasing),
		"WrongState":     int64(s.WrongState),
		"Refused":        int64(s.Refused),
		"ConvertP50ns":   int64(s.ConvertP50),
		"ConvertP95ns":   int64(s.ConvertP95),
		"NativeHeap":     s.NativeHeap,
		"RSS":            s.RSS,
	}
}

// platformSession returns the process's session, made at the first call
// (UI thread) with the tests' noisy action.
func platformSession() (media.Session, error) {
	sessionOnce.Do(func() {
		amp.NoisyActions = append(amp.NoisyActions, testNoisyAction)
		theSession, sessionErr = amp.NewSession(media.Config{Dispatch: dispatch})
		if sessionErr != nil {
			sessionErr = fmt.Errorf("e2e: %w", sessionErr)
		}
	})

	return theSession, sessionErr
}

// tickTime returns the time of the engine's next tick (every
// enginekit.TickEvery on the system clock): amp has no tick to run by hand.
func (h *harness) tickTime(t *testing.T) float64 {
	t.Helper()

	h.drain()

	return h.await(t, kind(media.EventTime), time.Second).Time
}

// replyDir is where devtest-android.sh's driver pushes its answers: the
// media folder.
func replyDir() string { return mediaDir() }

// onPhone tells whether the tests run on a real phone, where the driver
// changes nothing of the system (devtest-android.sh pushes phone.txt):
// no screen off there.
func onPhone() bool {
	_, err := os.Stat(filepath.Join(mediaDir(), "phone.txt"))

	return err == nil
}
