// Adapted from jplayer-mobile/internal/media/avf/engine_other.go at 49de3c2.

//go:build !ios || tvos || !cgo

package avf

import (
	"dancefitvn-app/internal/media"
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// errUnsupported is what every constructor answers off iOS.
var errUnsupported = fmt.Errorf("avf: AVFoundation on the iPhone only: %w", core.ErrUnsupported)

// Stats are an engine's counters (see the iOS build).
type Stats struct {
	LiveItems, Observers, Frames, Pulls int
	Outputs, NextOutputs, NextGiveUps   int
	Players, VideoTracksOff             int
	FootprintBytes                      int64
}

// SessionStats are a session's counters (see the iOS build).
type SessionStats struct {
	Activations         int
	Active              bool
	Resets, Suspensions int
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// New reports core.ErrUnsupported: the engine is iOS only.
func New(media.Config, media.Session) (media.Engine, error) { return nil, errUnsupported }

// NewSession reports core.ErrUnsupported: the session is iOS only.
func NewSession(media.Config) (media.Session, error) { return nil, errUnsupported }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// StatsOf reports false: no engine of this package exists off iOS.
func StatsOf(media.Engine) (Stats, bool) { return Stats{}, false }

// SessionStatsOf reports false: no session of this package exists off iOS.
func SessionStatsOf(media.Session) (SessionStats, bool) { return SessionStats{}, false }

// Tick reports false: no engine of this package exists off iOS. For device
// tests only (testhooks_ios.go).
func Tick(media.Engine) bool { return false }

// SetLateOutput reports false: no engine of this package exists off iOS.
// For device tests only (testhooks_ios.go).
func SetLateOutput(media.Engine, bool) bool { return false }

// SetKeepVideoInBackground reports false: no engine of this package exists
// off iOS. For device measures only (testhooks_ios.go).
func SetKeepVideoInBackground(media.Engine, bool) bool { return false }

// VideoTracks reports false: no engine of this package exists off iOS. For
// device tests only (testhooks_ios.go).
func VideoTracks(media.Engine) (total, enabled int, ok bool) { return 0, 0, false }

// SetVariantSizing reports false: no engine of this package exists off iOS.
// For device tests only (testhooks_ios.go).
func SetVariantSizing(media.Engine, bool) bool { return false }

// SetNextStarved reports false: no engine of this package exists off iOS.
// For device tests only (testhooks_ios.go).
func SetNextStarved(media.Engine, bool) bool { return false }

// PlayerRate reports false: no engine of this package exists off iOS. For
// device measures only (testhooks_ios.go).
func PlayerRate(media.Engine) (rate, defaultRate float64, ok bool) { return 0, 0, false }

// PitchAlgorithm reports false: no engine of this package exists off iOS.
// For device measures only (testhooks_ios.go).
func PitchAlgorithm(media.Engine) (string, bool) { return "", false }

// SetMaxResolution reports false: no engine of this package exists off
// iOS (quality_ios.go).
func SetMaxResolution(media.Engine, int, int) bool { return false }

// MaxResolution reports false: no engine of this package exists off iOS.
// For device tests only (quality_ios.go).
func MaxResolution(media.Engine) (w, h float64, ok bool) { return 0, 0, false }

// ErrorLog reports false: no engine of this package exists off iOS. For
// device tests only (testhooks_ios.go).
func ErrorLog(media.Engine) (n int, domain string, code int, comment string, ok bool) {
	return 0, "", 0, "", false
}
