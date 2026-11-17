// Adapted from jplayer-tv/internal/store/backend.go at 781bee5.

package store

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Backend keeps the store's bytes by key; the store owns the format (JSON).
// Load wraps core.ErrNotFound when the key is absent. Calls come from the UI
// thread.
type Backend interface {
	Load(key string) ([]byte, error)
	Save(key string, data []byte) error
}

// Syncer is a Backend that can push its writes to durable storage now
// (NSUserDefaults' synchronize).
type Syncer interface {
	Sync() error
}
