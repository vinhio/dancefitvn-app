//go:build !ios

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// excludeFromBackup reports core.ErrUnsupported: only iOS (and tvOS) has
// the flag; Android keeps the secret folder in its no-backup folder.
func excludeFromBackup(string) error {
	return fmt.Errorf("backup flag: %w", core.ErrUnsupported)
}

// excludedFromBackup reports core.ErrUnsupported, as excludeFromBackup.
func excludedFromBackup(string) (bool, error) {
	return false, fmt.Errorf("backup flag: %w", core.ErrUnsupported)
}
