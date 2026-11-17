// Adapted from jplayer-tv/e2e/storecheck/assert_tvos_test.go at 781bee5.

//go:build ios && tvos

package main

import (
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/store/tvstore"
)

// tvstore.Defaults satisfies the store's interfaces by its method set
// alone (tvstore does not import store).
var (
	_ store.Backend = (*tvstore.Defaults)(nil)
	_ store.Syncer  = (*tvstore.Defaults)(nil)
)
