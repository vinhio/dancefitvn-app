// Adapted from jplayer-mobile/e2e/sys/network_ios_test.go at 49de3c2.
// Changed: the package's import path. The build tag ios covers the
// Apple TV simulator too, which has the Mac's network as well.

//go:build ios

package sys

import (
	"testing"

	"github.com/stretchr/testify/assert"

	jsys "dancefitvn-app/internal/sys"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// networkSteps checks what the simulator can show: it has the Mac's
// network (Wi-Fi or Ethernet, seen through the simulator as either or as
// another kind) and no mobile data, so Cellular is false. Its network
// cannot be changed from the test: the monitor's answers to a change, and
// mobile data, are for a real iPhone (docs/plan/07 §6, P8).
func networkSteps(t *testing.T, n *jsys.Network) {
	t.Helper()

	k := n.Kind()
	assert.NotEqual(t, jsys.NetworkCellular, k, "the simulator has no mobile data")
	assert.False(t, n.Cellular())
	t.Logf("simulator network: %s (changes and mobile data need an iPhone)", k)
}
