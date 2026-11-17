// Adapted from jplayer-mobile/internal/sys/network_other.go at 49de3c2.

//go:build !ios && !android

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// netOS is the watch of a platform without one.
type netOS struct{}

// errNoNetwork is WatchNetwork's error off a phone.
var errNoNetwork = fmt.Errorf("sys: network: %w", core.ErrUnsupported)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// start has no network to watch off a phone.
func (*netOS) start(*Network) error { return errNoNetwork }

// kind knows no network off a phone.
func (*netOS) kind() NetworkKind { return NetworkUnknown }

// stop has nothing to stop off a phone.
func (*netOS) stop() {}
