//go:build ios

// Adapted from jplayer-mobile/internal/sys/keepawake_ios.go at 49de3c2:
// the idle timer is gDesk's now (bridge/system/power, uikit's counted
// holds), so this file only keeps the one hold of the app.

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/bridge/system/power"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// idleHold is the release of the app's hold on the idle timer, nil while
// it holds none. UI thread only, which is the main thread on iOS.
var idleHold func()

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// keepAwake takes or releases the app's hold (power.KeepAwake). The timer
// is the process's and lasts across its scenes, so force (the app active
// again) releases and takes the hold again: uikit sets the timer at the
// first hold and the last release, which puts the value back should the
// system have changed it while the app was away.
func keepAwake(d platform.Driver, on, force bool) error {
	switch {
	case on && idleHold != nil && !force, !on && idleHold == nil:
		return nil
	case !on:
		idleHold()
		idleHold = nil

		return nil
	default:
	}
	if idleHold != nil {
		idleHold()
		idleHold = nil
	}
	release, err := power.KeepAwake(d, keepAwakeReason, true)
	if err != nil {
		return fmt.Errorf("sys: keep awake: %w", err)
	}
	idleHold = release

	return nil
}
