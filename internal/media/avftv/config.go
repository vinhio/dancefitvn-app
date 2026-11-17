// Adapted from jplayer-tv/internal/media/avf/config.go at 781bee5.

package avftv

import (
	"dancefitvn-app/internal/clock"
	"fmt"
	"log/slog"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/geom"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Plane is a system layer shown under the app's view: the app's adapter
// over gDesk's underlay API (platform.UnderlayCanvas), which avftv does
// not import. UI thread only.
type Plane interface {
	// SetFrame places the layer, in DIPs of the view (UIKit points).
	SetFrame(r geom.Rect)
	// SetHidden hides or shows the layer.
	SetHidden(hidden bool)
	// Remove takes the layer out and releases it; later calls do nothing.
	Remove()
}

// Quality is the playback quality limit, the app's "Chất lượng video"
// setting (jplayer-tv's media.Quality, kept here: DanceFITVN's media
// package has none yet).
type Quality int

// Qualities. QualityUnknown is the zero value and means no limit.
const (
	QualityUnknown Quality = iota
	QualityAuto
	Quality1080
	Quality720
	Quality480
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Config is what New needs.
type Config struct {
	// Clock is the UI thread's clock: it drives the 250 ms time tick;
	// required.
	Clock clock.Clock
	// Post runs fn on the UI thread later and reports false once the app
	// has ended (host.App.Enqueue); required.
	Post func(fn func()) bool
	// Underlay puts layer (an AVPlayerLayer) under the app's view;
	// required (the picture is the native plane B, D4).
	Underlay func(layer uintptr) (Plane, error)
	// Log receives the engine's diagnostics; nil means slog.Default().
	Log *slog.Logger
	// OnInterruption, when set, is told on the UI thread of the audio
	// session's interruptions (a call, Siri, another app's audio): began
	// when one starts; when it ends, began false and shouldResume when
	// the system says the app may play again
	// (AVAudioSessionInterruptionOptionShouldResume). Interruptions
	// reported together arrive in order. Nil: the engine does not
	// observe them. Optional.
	OnInterruption func(began, shouldResume bool)
}

// Stats are the engine's live counts of what it holds, for leak tests: a
// steady engine holds at most one item and its observations.
type Stats struct {
	Items        int    // AVPlayerItems retained
	Observations int    // key-value observations and notification registrations in place
	Layers       int    // AVPlayerLayers retained
	Loads        uint64 // items made since New
	Ticks        uint64 // time ticks run since New
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// String returns "auto", "1080", "720", "480" or "unknown".
func (q Quality) String() string {
	switch q {
	case QualityAuto:
		return "auto"
	case Quality1080:
		return "1080"
	case Quality720:
		return "720"
	case Quality480:
		return "480"
	case QualityUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// MaxHeight returns the most lines q allows, 0 for no limit.
func (q Quality) MaxHeight() int {
	switch q {
	case Quality1080:
		return 1080
	case Quality720:
		return 720
	case Quality480:
		return 480
	case QualityAuto, QualityUnknown:
		return 0
	default:
		return 0
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// normalize checks c and returns it with its defaults: slog.Default() for
// a nil Log. A missing Clock, Post or Underlay wraps core.ErrInvalid.
func (c Config) normalize() (Config, error) {
	if c.Clock == nil {
		return c, fmt.Errorf("avftv: config: no clock: %w", core.ErrInvalid)
	}
	if c.Post == nil {
		return c, fmt.Errorf("avftv: config: no post function: %w", core.ErrInvalid)
	}
	if c.Underlay == nil {
		return c, fmt.Errorf("avftv: config: native video needs an underlay: %w", core.ErrInvalid)
	}
	if c.Log == nil {
		c.Log = slog.Default()
	}

	return c, nil
}
