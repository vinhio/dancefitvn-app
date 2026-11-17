// Adapted from jplayer-mobile/internal/sys/network.go at 49de3c2.
// Changed: the doc speaks of the downloads ("Chỉ tải qua Wi‑Fi") instead of
// the player's links.

package sys

import (
	"log"
	"sync/atomic"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// NetworkKind is the kind of network the device's traffic goes through.
type NetworkKind int32

// The kinds of network.
const (
	// NetworkUnknown is a network not known yet: before the system's
	// first answer, or off a phone.
	NetworkUnknown NetworkKind = iota
	// NetworkNone is no usable network.
	NetworkNone
	// NetworkWiFi is a Wi-Fi network.
	NetworkWiFi
	// NetworkCellular is mobile data, which the setting "Chỉ tải qua
	// Wi‑Fi" keeps the downloads off (docs/plan/03, the pref wifi).
	NetworkCellular
	// NetworkWired is a wired Ethernet network.
	NetworkWired
	// NetworkOther is any other usable network: a virtual one, the
	// loopback, a network of a kind the system does not tell.
	NetworkOther
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Network watches the device's network, for the app to know it is
// offline at once and for the downloads to wait for Wi-Fi while
// "Chỉ tải qua Wi‑Fi" is on. Its methods may be called on any goroutine,
// and on a nil Network, which knows no network.
type Network struct {
	dispatch Dispatch
	closed   atomic.Bool
	logged   atomic.Int32                      // the NetworkKind last logged
	onChange atomic.Pointer[func(NetworkKind)] // OnChange's function
	os       netOS                             // the platform's watch
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// WatchNetwork starts watching the device's network. Each change of
// kind is logged through dispatch, on the UI thread. Close the Network
// once it is needed no more.
//
// On iOS it runs a Network.framework path monitor (NWPathMonitor), whose
// answers come later, on a queue of its own: the kind is
// NetworkUnknown until the first, which comes as the monitor starts.
// On Android it asks ConnectivityManager at each call of Cellular or
// Kind (the active network's NetworkCapabilities), and logs the
// changes the system broadcasts (CONNECTIVITY_ACTION). Elsewhere it
// fails with core.ErrUnsupported.
func WatchNetwork(dispatch func(func())) (*Network, error) {
	n := &Network{dispatch: dispatch}
	if err := n.os.start(n); err != nil {
		return nil, err
	}

	return n, nil
}

// Cellular reports whether the device's traffic goes over mobile data
// now. It is false while unknown, without a network, on an error (a
// failing check never holds a download back) and on a nil or closed
// Network. It allocates nothing.
func (n *Network) Cellular() bool { return n.Kind() == NetworkCellular }

// Kind returns the kind of network the device's traffic goes through
// now; NetworkUnknown on a nil or closed Network. It allocates nothing.
func (n *Network) Kind() NetworkKind {
	if n == nil || n.closed.Load() {
		return NetworkUnknown
	}

	return n.os.kind()
}

// OnChange has fn told each change of kind, on the UI thread (through the
// dispatch), after it is logged: the app goes offline at once, probes the
// API as the network comes back and gates the downloads. Nil stops
// telling. Nothing on a nil Network.
func (n *Network) OnChange(fn func(NetworkKind)) {
	if n == nil {
		return
	}
	if fn == nil {
		n.onChange.Store(nil)

		return
	}
	n.onChange.Store(&fn)
}

// Close stops the watch. It may be called more than once.
func (n *Network) Close() {
	if n == nil || n.closed.Swap(true) {
		return
	}
	n.os.stop()
}

// String returns the kind's name.
func (k NetworkKind) String() string {
	switch k {
	case NetworkUnknown:
		return "unknown"
	case NetworkNone:
		return "none"
	case NetworkWiFi:
		return "wifi"
	case NetworkCellular:
		return "cellular"
	case NetworkWired:
		return "wired"
	case NetworkOther:
		return "other"
	default:
		return "unknown"
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// changed logs k through the dispatch, and tells OnChange's function,
// when it is not the kind logged last, from whatever thread the system
// tells it on.
func (n *Network) changed(k NetworkKind) {
	if n.closed.Load() || NetworkKind(n.logged.Swap(int32(k))) == k {
		return
	}
	if n.dispatch == nil {
		log.Printf("sys: network: %s", k)

		return
	}
	n.dispatch(func() {
		log.Printf("sys: network: %s", k)
		if fn := n.onChange.Load(); fn != nil && !n.closed.Load() {
			(*fn)(k)
		}
	})
}
