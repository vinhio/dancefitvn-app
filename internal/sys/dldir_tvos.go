//go:build ios && tvos

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// audioBase is no folder on tvOS: the TV downloads nothing (PLAN, the TV
// screens have no offline audio).
func audioBase() (string, error) {
	return "", fmt.Errorf("no downloads on tvOS: %w", core.ErrUnsupported)
}
