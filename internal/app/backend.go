// Adapted from jplayer-tv/internal/app/backend_tvos.go at 781bee5
// (defaultBackend, slotName): split so that the Mirror's wiring builds and
// is tested on the Mac, the NSUserDefaults part alone tagged.

package app

import (
	"dancefitvn-app/internal/store"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tvStateFolder is the folder, under sys.DataDir, of the TV's small
// documents (as stateFolder on the phone): on tvOS it lies in
// Library/Caches, which the system may empty.
const tvStateFolder = "state"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// tvBackend returns the backend of the TV's state (local/plans/P7.md
// Q-P7-6): every key the TV wire saves through it (store.PrefsKey,
// progress.QueueKey, and any other document of its state folder) is kept
// twice by a store.Mirror, in NSUserDefaults under tvstore.Prefix + key
// (the primary, the one storage tvOS promises to keep) and in
// <dir>/<key>.json (the copy, a store.Dir; dir is DataDir/state, in
// Library/Caches). After the system purges the Caches the primary serves
// the next launch and the next save writes the file again; the copy
// serves when NSUserDefaults lost its last write (jPlayer TV T0.5).
//
// Without NSUserDefaults (off tvOS, or Foundation unreachable) the primary
// is a store.Memory, so the files keep the same envelopes, and logf says
// why once. A slot that fails while the other works goes to logf too.
// Tokens and the install id are not here: they stay in sys.SecretDir (F1)
// until G1.
func tvBackend(dir string, logf func(format string, args ...any)) *store.Mirror {
	primary, err := tvDefaults()

	return newTVBackend(primary, err, dir, logf)
}

// newTVBackend is tvBackend over a given primary, or its error.
func newTVBackend(primary store.Backend, err error, dir string, logf func(format string, args ...any)) *store.Mirror {
	if err != nil || primary == nil {
		logf("dancefitvn: store: no NSUserDefaults, the state folder alone (%s): %v", dir, err)
		primary = store.NewMemory()
	}

	return store.NewMirror(primary, store.NewDir(dir), store.WithSlotError(func(s store.Slot, err error) {
		logf("dancefitvn: store: %s slot failed: %v", slotName(s), err)
	}))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// slotName names a slot of the Mirror for the log.
func slotName(s store.Slot) string {
	switch s {
	case store.SlotPrimary:
		return "NSUserDefaults"
	case store.SlotCopy:
		return "file"
	default:
		return "unknown"
	}
}
