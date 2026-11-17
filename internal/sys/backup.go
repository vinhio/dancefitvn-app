package sys

import (
	"errors"
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// ExcludeFromBackup flags the file or folder at path with
// NSURLIsExcludedFromBackupKey, so that iCloud and computer backups skip
// it (and, for a folder, all it holds). It is iOS's (and tvOS's); other
// systems answer core.ErrUnsupported (Android's no-backup folder needs no
// flag). It runs on any goroutine.
func ExcludeFromBackup(path string) error {
	if err := excludeFromBackup(path); err != nil {
		return fmt.Errorf("sys: exclude %s from backup: %w", path, err)
	}

	return nil
}

// ExcludedFromBackup reads back the flag ExcludeFromBackup sets, afresh
// (not from the URL's cache): for probes and tests on a device. Other
// systems than iOS answer core.ErrUnsupported.
func ExcludedFromBackup(path string) (bool, error) {
	ok, err := excludedFromBackup(path)
	if err != nil {
		return false, fmt.Errorf("sys: read backup flag of %s: %w", path, err)
	}

	return ok, nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// excludeIfSupported is ExcludeFromBackup where the system has the
// flag, and nothing elsewhere.
func excludeIfSupported(path string) error {
	err := ExcludeFromBackup(path)
	if errors.Is(err, core.ErrUnsupported) {
		return nil
	}

	return err
}
