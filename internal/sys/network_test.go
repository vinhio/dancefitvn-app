// Adapted from jplayer-mobile/internal/sys/network_test.go at 49de3c2.

package sys

import (
	"runtime"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWatchNetworkOffPhone checks that off a phone there is no network to
// watch, and that a nil Network, which the app keeps then, knows none.
func TestWatchNetworkOffPhone(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		t.Skip("the watch runs on the simulator and the emulator (e2e/sys)")
	}
	n, err := WatchNetwork(func(func()) { t.Error("dispatched") })
	require.ErrorIs(t, err, core.ErrUnsupported)
	assert.Nil(t, n)
	assert.False(t, n.Cellular())
	assert.Equal(t, NetworkUnknown, n.Kind())
	n.Close()
}

// TestNetworkClosed checks that a closed Network knows no network and
// closes once.
func TestNetworkClosed(t *testing.T) {
	t.Parallel()

	n := &Network{}
	n.Close()
	n.Close()
	assert.False(t, n.Cellular())
	assert.Equal(t, NetworkUnknown, n.Kind())
}

// TestNetworkChangedLogsOnce checks that only a change of kind goes to the
// dispatch, and nothing once closed.
func TestNetworkChangedLogsOnce(t *testing.T) {
	t.Parallel()

	var got int
	n := &Network{dispatch: func(func()) { got++ }}
	for _, step := range []struct {
		kind NetworkKind
		want int
	}{
		{NetworkWiFi, 1},
		{NetworkWiFi, 1},
		{NetworkCellular, 2},
		{NetworkCellular, 2},
		{NetworkNone, 3},
		{NetworkWiFi, 4},
	} {
		n.changed(step.kind)
		assert.Equal(t, step.want, got, step.kind.String())
	}
	n.Close()
	n.changed(NetworkCellular)
	assert.Equal(t, 4, got, "closed")
}

// TestNetworkOnChange checks that OnChange's function hears each change
// of kind on the dispatch, until it is removed or the Network closed, and
// that a nil Network takes one.
func TestNetworkOnChange(t *testing.T) {
	t.Parallel()

	var heard []NetworkKind
	n := &Network{dispatch: func(fn func()) { fn() }}
	n.OnChange(func(k NetworkKind) { heard = append(heard, k) })
	n.changed(NetworkNone)
	n.changed(NetworkNone)
	n.changed(NetworkWiFi)
	assert.Equal(t, []NetworkKind{NetworkNone, NetworkWiFi}, heard)
	n.OnChange(nil)
	n.changed(NetworkCellular)
	assert.Len(t, heard, 2, "removed")
	n.OnChange(func(k NetworkKind) { heard = append(heard, k) })
	n.Close()
	n.changed(NetworkWiFi)
	assert.Len(t, heard, 2, "closed")

	var none *Network
	none.OnChange(func(NetworkKind) { t.Error("told") })
}

// TestNetworkKindString checks the kinds' names, unknown for a value out
// of range.
func TestNetworkKindString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind NetworkKind
		want string
	}{
		{NetworkUnknown, "unknown"},
		{NetworkNone, "none"},
		{NetworkWiFi, "wifi"},
		{NetworkCellular, "cellular"},
		{NetworkWired, "wired"},
		{NetworkOther, "other"},
		{NetworkKind(99), "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, tc.kind.String())
		})
	}
}

// TestCellularDoesNotAllocate checks that the player's check of each link
// it loads allocates nothing: on a nil Network, a closed one and, off a
// phone, the platform's answer. The phones' are counted in e2e/sys.
func TestCellularDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun panics in a parallel test
	var none *Network
	closed := &Network{}
	closed.Close()
	open := &Network{}
	for name, n := range map[string]*Network{"nil": none, "closed": closed, "open": open} {
		allocs := testing.AllocsPerRun(1000, func() { _ = n.Cellular() })
		assert.Zero(t, allocs, name)
	}
}

// BenchmarkCellular measures the check of a link off a phone (the
// platform's answer); on the phones e2e/sys measures it.
func BenchmarkCellular(b *testing.B) {
	n := &Network{}
	b.ReportAllocs()
	for b.Loop() {
		_ = n.Cellular()
	}
}

// TestKindDoesNotAllocate is the gate of Network.Kind, which the downloads
// ask before each transfer and the player at each link: no allocation on a
// nil, a closed and an open Network.
func TestKindDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun panics in a parallel test
	var none *Network
	closed := &Network{}
	closed.Close()
	open := &Network{}
	for name, n := range map[string]*Network{"nil": none, "closed": closed, "open": open} {
		allocs := testing.AllocsPerRun(1000, func() { _ = n.Kind() })
		assert.Zero(t, allocs, name)
	}
}

// BenchmarkKind measures Network.Kind off a phone.
func BenchmarkKind(b *testing.B) {
	n := &Network{}
	b.ReportAllocs()
	for b.Loop() {
		_ = n.Kind()
	}
}
