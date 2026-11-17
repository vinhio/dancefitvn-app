// Adapted from jplayer-tv/internal/media/avf/engine_other.go at 781bee5.

//go:build !(ios && tvos)

package avftv

import (
	"context"
	"dancefitvn-app/internal/media"
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/event"
	"github.com/gdeskdev/desk/foundation/core/geom"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// errUnsupported is what the engine answers off tvOS.
var errUnsupported = fmt.Errorf("avftv: AVFoundation on the Apple TV only: %w", core.ErrUnsupported)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Engine is AVFoundation's engine of the Apple TV, which exists only on
// tvOS: here it is a type the app compiles against, and New never
// returns one.
type Engine struct {
	events event.Topic[media.Event]
}

var (
	_ media.Engine       = (*Engine)(nil)
	_ media.TimeReader   = (*Engine)(nil)
	_ media.Backgrounder = (*Engine)(nil)
)

// New fails with core.ErrUnsupported: the engine is built only with the
// tvos tag.
func New(Config) (*Engine, error) { return nil, errUnsupported }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Load fails with core.ErrUnsupported.
func (e *Engine) Load(context.Context, media.Source, media.LoadOptions) (uint64, error) {
	return 0, errUnsupported
}

// Play fails with core.ErrUnsupported.
func (e *Engine) Play() error { return errUnsupported }

// Pause fails with core.ErrUnsupported.
func (e *Engine) Pause() error { return errUnsupported }

// Stop does nothing.
func (e *Engine) Stop() error { return nil }

// Seek fails with core.ErrUnsupported.
func (e *Engine) Seek(float64) error { return errUnsupported }

// SetVolume fails with core.ErrUnsupported.
func (e *Engine) SetVolume(float64) error { return errUnsupported }

// SetMuted fails with core.ErrUnsupported.
func (e *Engine) SetMuted(bool) error { return errUnsupported }

// SetRate fails with core.ErrUnsupported.
func (e *Engine) SetRate(float64) error { return errUnsupported }

// SetLimits does nothing.
func (e *Engine) SetLimits(Quality) {}

// Place does nothing.
func (e *Engine) Place(geom.Rect) {}

// SetMirror does nothing.
func (e *Engine) SetMirror(bool) {}

// Mirrored returns false.
func (e *Engine) Mirrored() bool { return false }

// SetBackground does nothing.
func (e *Engine) SetBackground(bool) {}

// ReadTime returns false.
func (e *Engine) ReadTime(uint64) (float64, bool) { return 0, false }

// Events returns a topic nothing is published to.
func (e *Engine) Events() *event.Topic[media.Event] { return &e.events }

// Video returns nil.
func (e *Engine) Video() *media.Surface { return nil }

// Close does nothing.
func (e *Engine) Close() error { return nil }

// Stats returns zero counts.
func (e *Engine) Stats() Stats { return Stats{} }

// Tick reports false: no engine exists off tvOS.
func Tick(*Engine) bool { return false }

// PlayerRate reports false: no engine exists off tvOS.
func PlayerRate(*Engine) (rate, defaultRate float64, ok bool) { return 0, 0, false }

// PitchAlgorithm reports false: no engine exists off tvOS.
func PitchAlgorithm(*Engine) (string, bool) { return "", false }

// LayerScaleX reports false: no engine exists off tvOS.
func LayerScaleX(*Engine) (float64, bool) { return 0, false }

// LayerReadyForDisplay reports false: no engine exists off tvOS.
func LayerReadyForDisplay(*Engine) (ready, ok bool) { return false, false }
