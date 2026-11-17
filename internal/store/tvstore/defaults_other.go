// Adapted from jplayer-tv/internal/store/tvstore/defaults_other.go at 781bee5.

//go:build !(ios && tvos)

package tvstore

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewDefaults fails with an error wrapping core.ErrUnsupported: there is no
// NSUserDefaults of tvOS on this system.
func NewDefaults(string) (*Defaults, error) {
	return nil, fmt.Errorf("tvstore: new defaults: %w", core.ErrUnsupported)
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Load fails with an error wrapping core.ErrUnsupported.
func (*Defaults) Load(key string) ([]byte, error) {
	return nil, fmt.Errorf("tvstore: load %s: %w", key, core.ErrUnsupported)
}

// Save fails with an error wrapping core.ErrUnsupported.
func (*Defaults) Save(key string, _ []byte) error {
	return fmt.Errorf("tvstore: save %s: %w", key, core.ErrUnsupported)
}

// Sync fails with an error wrapping core.ErrUnsupported.
func (*Defaults) Sync() error {
	return fmt.Errorf("tvstore: sync: %w", core.ErrUnsupported)
}
