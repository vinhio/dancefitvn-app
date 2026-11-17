// Adapted from jplayer-tv/e2e/storecheck/slots_other.go at 781bee5.

//go:build !(ios && tvos)

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// installID fails with an error wrapping core.ErrUnsupported: a run on a
// Mac must not make an install id in the user's folders.
func installID() (string, error) {
	return "", fmt.Errorf("tvstore: install id: %w", core.ErrUnsupported)
}

// openSlots returns two Dir backends under os.TempDir()/dancefitvn-tvstore,
// primary/ and copy/: there is no NSUserDefaults of tvOS here, and a run
// on a Mac must not write into the user's caches.
func openSlots() (Slots, string, error) {
	dir := filepath.Join(os.TempDir(), "dancefitvn-tvstore")
	where := fmt.Sprintf("primary=%s copy=%s", filepath.Join(dir, "primary"), filepath.Join(dir, "copy"))

	return Slots{
		Primary: store.NewDir(filepath.Join(dir, "primary")),
		Copy:    store.NewDir(filepath.Join(dir, "copy")),
	}, where, nil
}
