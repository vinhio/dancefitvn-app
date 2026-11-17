// Adapted from jplayer-tv/internal/app/backend_tvos.go at 781bee5.

//go:build ios && tvos

package app

import (
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/store/tvstore"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// tvDefaults opens NSUserDefaults under tvstore.Prefix, the primary of
// tvBackend's Mirror.
func tvDefaults() (store.Backend, error) {
	d, err := tvstore.NewDefaults(tvstore.Prefix)
	if err != nil {
		return nil, err
	}

	return d, nil
}
