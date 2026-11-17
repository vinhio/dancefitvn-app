// Adapted from jplayer-tv/e2e/storecheck/slots_tvos.go at 781bee5.

//go:build ios && tvos

package main

import (
	"fmt"
	"path/filepath"

	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/store/tvstore"
	"dancefitvn-app/internal/sys"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// installID is the app's: a file in sys.SecretDir, under Library/Caches
// on tvOS.
var installID = sys.InstallID

// openSlots returns NSUserDefaults under tvstore.Prefix as the primary and
// <sys.DataDir()>/state (Library/Caches/com.jivecode.dancefitvn/state) as
// the copy, the places the app's tvBackend uses, and says where they are.
func openSlots() (Slots, string, error) {
	primary, err := tvstore.NewDefaults(tvstore.Prefix)
	if err != nil {
		return Slots{}, "", err
	}
	data, err := sys.DataDir()
	if err != nil {
		return Slots{}, "", fmt.Errorf("tvstore: data dir: %w", err)
	}
	dir := filepath.Join(data, "state")
	where := fmt.Sprintf("primary=NSUserDefaults/%s<key> copy=%s/<key>.json", tvstore.Prefix, dir)

	return Slots{Primary: primary, Copy: store.NewDir(dir)}, where, nil
}
