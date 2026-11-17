// Adapted from jplayer-tv/internal/store/tvstore/doc.go at 781bee5.

// Package tvstore keeps the store's bytes in NSUserDefaults on tvOS, the
// only local storage Apple promises a tvOS app keeps (jPlayer TV DT10). Defaults
// satisfies internal/store's Backend and Syncer by its method set alone:
// the package is a leaf and imports nothing of the app.
//
// Only internal/app imports it (archtest); e2e/tvstore, a test harness
// outside the rules, does too. The package builds on every system:
// off tvOS (the build tags ios and tvos together) NewDefaults and every
// method fail with an error wrapping core.ErrUnsupported.
package tvstore
