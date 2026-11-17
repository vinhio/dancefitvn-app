// Adapted from jplayer-mobile/internal/app/media_android.go at 49de3c2
// (the prober dropped).

//go:build android && cgo

package app

import (
	"fmt"
	"time"

	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/amp"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mediaLate makes the system's playback after the first frame
// (lateMedia): MediaPlayer's session and engine take 1 to 2 ms, and the
// players are made off the UI thread.
const mediaLate = false

// qualityLimited: MediaPlayer cannot pick an HLS variant, so screen 07 has
// no quality row (K21, Q-P4-3).
const qualityLimited = false

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newPlatformMedia makes MediaPlayer's playback (internal/media/amp): the
// process's audio session, made once on the application context, so it
// outlives the activity; an engine of its own over it, which keeps the
// CPU awake while it plays when the manifest grants WAKE_LOCK (gdesk.json
// android.mediaPlayback). A session that cannot be made is logged (logf)
// and leaves none: the engine plays without the focus. Call it on the UI
// thread, while the activity runs.
func newPlatformMedia(cfg media.Config, logf func(string, ...any)) (platformMedia, error) {
	t0 := time.Now()
	sess, err := processSession(cfg, amp.NewSession, logf)
	if err != nil {
		logf("dancefitvn: media: audio session: %v", err)
		sess = nil
	}
	t1 := time.Now()
	eng, err := amp.New(cfg, sess)
	if err != nil {
		return platformMedia{sess: sess}, fmt.Errorf("engine: %w", err)
	}
	logf("dancefitvn: media made: session %.1f ms, engine %.1f ms", ms(t1.Sub(t0)), ms(time.Since(t1)))

	return platformMedia{name: "amp", eng: eng, sess: sess}, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// warmMedia has nothing to warm: the session is made on the UI thread,
// on the application context, in 1 ms. ready runs at once (posted).
func warmMedia(cfg media.Config, ready func(), _ func(string, ...any)) {
	if ready != nil {
		cfg.Dispatch(ready)
	}
}

// limitResolution has no effect: MediaPlayer cannot pick an HLS variant
// (Q-P4-3, K21). It reports false.
func limitResolution(media.Engine, int, int) bool { return false }
