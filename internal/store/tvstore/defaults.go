// Adapted from jplayer-tv/internal/store/tvstore/defaults.go at 781bee5.

package tvstore

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Prefix is the namespace of the app's keys in NSUserDefaults (local/plans/
// P7.md §2): the preferences live under Prefix + "prefs" (store.PrefsKey),
// the progress queue under Prefix + "progress-queue", and so on.
const Prefix = "dancefitvn.v1."

// Defaults keeps each key's bytes as an NSData in NSUserDefaults'
// standardUserDefaults, under prefix + key. NSUserDefaults is thread-safe,
// so Defaults is too; it may be called from any goroutine.
type Defaults struct {
	prefix string
}
