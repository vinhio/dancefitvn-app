//go:build !ios

// Adapted from jplayer-mobile/internal/sys/a11ydump_other.go at 49de3c2.

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// accessibilityDump has no system's tree to read: Android's content has
// no accessibility bridge (G10), a desktop's is read by its own tools.
func accessibilityDump(platform.Canvas) ([]A11yElement, error) {
	return nil, fmt.Errorf("sys: accessibility dump: %w", core.ErrUnsupported)
}
