// Adapted from jplayer-mobile/internal/app/media_ios.go at 49de3c2 (the
// prober dropped; the session is warmed off the UI thread, warmMedia).

//go:build ios && !tvos && cgo

package app

import (
	"fmt"
	"time"

	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avf"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mediaLate makes the system's playback after the first frame
// (lateMedia): AVAudioSession takes ~60 ms (local/p4/engine-ios.md),
// which the first frame would wait for.
const mediaLate = true

// qualityLimited: the engine caps the HLS variant (limitResolution), so
// the quality row of screen 07 shows (K21).
const qualityLimited = true

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newPlatformMedia makes AVFoundation's playback (internal/media/avf):
// the process's audio session, made once, as its observers stay, waited
// for when the warm-up is under way; and an engine of its own over it.
// logf logs. Call it on the UI thread.
func newPlatformMedia(cfg media.Config, logf func(string, ...any)) (platformMedia, error) {
	t0 := time.Now()
	sess, err := processSession(cfg, avf.NewSession, logf)
	if err != nil {
		return platformMedia{}, fmt.Errorf("audio session: %w", err)
	}
	t1 := time.Now()
	eng, err := avf.New(cfg, sess)
	if err != nil {
		return platformMedia{sess: sess}, fmt.Errorf("engine: %w", err)
	}
	logf("dancefitvn: media made: session %.1f ms, engine %.1f ms", ms(t1.Sub(t0)), ms(time.Since(t1)))

	return platformMedia{name: "avf", eng: eng, sess: sess}, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// warmMedia starts making the process's audio session off the UI thread
// (avf.NewSession: AVAudioSession is thread-safe, and the session's
// observers post on cfg.Dispatch); ready runs on the UI thread once it
// is done, and logf there.
func warmMedia(cfg media.Config, ready func(), logf func(string, ...any)) {
	warmSession(cfg, avf.NewSession, ready, logf)
}

// limitResolution caps the HLS variant of eng, avf's, at w×h pixels
// (AVPlayerItem.preferredMaximumResolution, Q-P4-3, K21); 0×0 lifts it.
// It reports false for another engine (the fake). UI thread.
func limitResolution(eng media.Engine, w, h int) bool { return avf.SetMaxResolution(eng, w, h) }
