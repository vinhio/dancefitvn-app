// Adapted from jplayer-mobile/e2e/engine/other_test.go at 49de3c2.

//go:build !(ios && tvos)

package tvengine

import "testing"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestTVEngineNeedsADevice marks the package as device-only: the engine
// runs on the Apple TV simulator, through scripts/tvos-avf-test.sh e2e.
func TestTVEngineNeedsADevice(t *testing.T) {
	t.Parallel()

	t.Skip("the Apple TV engine tests run on the tvOS simulator: see e2e/README.md")
}
