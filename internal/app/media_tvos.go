//go:build tvos

// The Apple TV's playback: avftv (docs/plan/04 section 1, from
// jplayer-tv/internal/media/avf at 781bee5), whose picture is the native
// plane under the view (underlay_tv.go). The pattern is media_ios.go.

package app

import (
	"fmt"

	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avftv"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newTVPlatformMedia makes avftv's engine on cfg's clock: post is the
// host's Enqueue, underlay puts its layer under the view, and sess hears
// its audio session's interruptions. Off tvOS avftv.New fails with
// core.ErrUnsupported and makeMedia plays on the fake engine; sess stays
// the session then. UI thread.
func newTVPlatformMedia(cfg media.Config, post func(fn func()) bool, underlay func(layer uintptr) (avftv.Plane, error),
	sess *tvSession,
) (platformMedia, error) {
	eng, err := avftv.New(avftv.Config{Clock: cfg.Clock, Post: post, Underlay: underlay, OnInterruption: sess.interrupted})
	if err != nil {
		return platformMedia{sess: sess}, fmt.Errorf("engine: %w", err)
	}

	return platformMedia{name: "avftv", eng: eng, sess: sess}, nil
}
