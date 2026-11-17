//go:build tvos

// Adapted from jplayer-tv/internal/app/app.go at 781bee5 (underlay,
// underlayOn): the wire's view in place of jPlayer's App, avftv's Plane.

package app

import (
	"fmt"

	"dancefitvn-app/internal/media/avftv"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// underlay puts layer (avftv's AVPlayerLayer) under the view's canvas
// through gDesk's underlays. Before attach, or before the view's first
// frame, there is no canvas yet: core.ErrInvalid, and the engine tries
// again later. A canvas without underlays wraps core.ErrUnsupported: the
// video then plays without a picture, which only happens off tvOS. UI
// thread.
func (w *tvWire) underlay(layer uintptr) (avftv.Plane, error) {
	w.mu.Lock()
	v := w.view
	w.mu.Unlock()
	if v == nil {
		return nil, fmt.Errorf("app: underlay: no view yet: %w", core.ErrInvalid)
	}

	return underlayOn(v.Canvas(), layer)
}

// underlayOn puts layer under c, a view's canvas (underlay).
func underlayOn(c platform.Canvas, layer uintptr) (avftv.Plane, error) {
	if c == nil {
		return nil, fmt.Errorf("app: underlay: no canvas yet: %w", core.ErrInvalid)
	}
	uc, ok := c.(platform.UnderlayCanvas)
	if !ok {
		return nil, fmt.Errorf("app: underlay: the canvas takes no underlay: %w", core.ErrUnsupported)
	}
	u, err := uc.AddUnderlay(layer)
	if err != nil {
		return nil, fmt.Errorf("app: underlay: %w", err)
	}

	return u, nil
}
