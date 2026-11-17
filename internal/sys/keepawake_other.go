//go:build !ios && !android

// Adapted from jplayer-mobile/internal/sys/keepawake_other.go at 49de3c2.

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// errNoKeepAwake is KeepAwake's error off a phone.
var errNoKeepAwake = fmt.Errorf("sys: keep awake: %w", core.ErrUnsupported)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// keepAwake has no screen to keep on off a phone.
func keepAwake(platform.Driver, bool, bool) error { return errNoKeepAwake }
