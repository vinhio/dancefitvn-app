// Adapted from jplayer-mobile/e2e/engine/other_test.go at 49de3c2.

//go:build (!ios || tvos) && (!android || !cgo)

package engine

import "testing"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestEngineNeedsADevice marks the package as device-only: the engines run
// on the iOS simulator (e2e/devtest-ios.sh) and on Android
// (e2e/devtest-android.sh).
func TestEngineNeedsADevice(t *testing.T) {
	t.Parallel()

	t.Skip("the engine tests run on the iOS simulator and on Android: see e2e/README.md")
}
