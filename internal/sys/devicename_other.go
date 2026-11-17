//go:build !ios

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// deviceName has no UIDevice to ask outside iOS and tvOS.
func deviceName() (string, error) {
	return "", fmt.Errorf("sys: device name: %w", core.ErrUnsupported)
}
