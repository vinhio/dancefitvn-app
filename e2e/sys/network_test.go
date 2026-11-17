// Adapted from jplayer-mobile/e2e/sys/network_test.go at 49de3c2.
// Changed: the package's import path.

//go:build ios || (android && cgo)

package sys

import (
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jsys "dancefitvn-app/internal/sys"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestNetworkCellular watches the network, waits for its first kind (iOS
// answers on the monitor's queue), checks the downloads' check of the
// network on the UI thread allocates nothing, and has the platform change
// the network where it can (networkSteps: the emulator turns Wi-Fi,
// mobile data and airplane mode off and on; the simulator has the Mac's
// network and no mobile data). After Close the Network knows none, and a new watch starts again.
func TestNetworkCellular(t *testing.T) { //nolint:paralleltest // one app a process
	var (
		n   *jsys.Network
		err error
	)
	onUI(t, func() { n, err = jsys.WatchNetwork(dispatch) })
	require.NoError(t, err)
	defer onUI(t, n.Close)
	eventually(t, within, "the first kind", func() bool { return n.Kind() != jsys.NetworkUnknown })
	t.Logf("network: %s, cellular %v", n.Kind(), n.Cellular())

	var (
		allocs float64
		took   time.Duration
	)
	onUI(t, func() {
		for range 20 {
			_ = n.Cellular()
		}
		defer debug.SetGCPercent(debug.SetGCPercent(-1))
		allocs = testing.AllocsPerRun(200, func() { _ = n.Cellular() })
		t0 := time.Now()
		for range 200 {
			_ = n.Cellular()
		}
		took = time.Since(t0) / 200
	})
	t.Logf("Cellular: %v a call, %v allocations", took, allocs)
	assert.Zero(t, allocs, "allocations of Cellular")

	networkSteps(t, n)

	onUI(t, n.Close)
	assert.False(t, n.Cellular(), "closed")
	assert.Equal(t, jsys.NetworkUnknown, n.Kind(), "closed")

	// A watch after the last closed starts the system's watch again.
	var again *jsys.Network
	onUI(t, func() { again, err = jsys.WatchNetwork(dispatch) })
	require.NoError(t, err)
	defer onUI(t, again.Close)
	eventually(t, within, "the first kind of a second watch", func() bool { return again.Kind() != jsys.NetworkUnknown })
	assert.Equal(t, jsys.NetworkUnknown, n.Kind(), "the first stays closed")
}
