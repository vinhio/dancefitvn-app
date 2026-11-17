// Adapted from jplayer-tv/internal/app/backend_other.go at 781bee5.

//go:build !(ios && tvos)

package app

import (
	"fmt"

	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// tvDefaults fails with an error wrapping core.ErrUnsupported: there is no
// NSUserDefaults of tvOS here (tvBackend then keeps the files alone).
func tvDefaults() (store.Backend, error) {
	return nil, fmt.Errorf("app: tv defaults: %w", core.ErrUnsupported)
}
