//go:build !ios

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// idleTimerDisabled has no UIApplication to ask outside iOS and tvOS.
func idleTimerDisabled() (bool, error) {
	return false, fmt.Errorf("sys: idle timer: %w", core.ErrUnsupported)
}
