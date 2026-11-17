// Adapted from jplayer-mobile/e2e/sys/other_test.go at 49de3c2.

//go:build !ios && !(android && cgo)

package sys

import "testing"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestSysNeedsADevice marks the package as device-only: the services run
// on the iOS simulator and the Android emulator, through e2e/devtest-*.sh.
func TestSysNeedsADevice(t *testing.T) {
	t.Parallel()

	t.Skip("the system service tests run on a simulator or an emulator: see e2e/README.md")
}
