// Adapted from jplayer-mobile/internal/sys/network_ios.go at 49de3c2.
// Changed: the queue's label. The build tag ios covers tvOS too (GOOS=ios
// with -tags tvos); the block helpers are in block_ios.go.

//go:build ios

package sys

import (
	"fmt"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/dl"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The libraries of the path monitor: Network.framework's C API (path.h,
// path_monitor.h, interface.h of the SDK) and libdispatch, in libSystem.
const (
	pathNetwork   = "/System/Library/Frameworks/Network.framework/Network"
	pathLibSystem = "/usr/lib/libSystem.B.dylib"
)

// nw_path_status_t of Network.framework's path.h.
const (
	nwPathStatusInvalid     = 0
	nwPathStatusSatisfied   = 1
	nwPathStatusUnsatisfied = 2
	nwPathStatusSatisfiable = 3
)

// nw_interface_type_t of Network.framework's interface.h (other 0,
// loopback 4, which a path of neither kind below is).
const (
	nwInterfaceWiFi     = 1
	nwInterfaceCellular = 2
	nwInterfaceWired    = 3
)

// netOS is the iOS side of a Network: it is one of the watchers of the
// process's path monitor.
type netOS struct{ n *Network }

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// monitor is the process's path monitor, which every Network watches: it
// runs while one does, and is made again for the next one after the last
// has closed. Its C functions are bound, its queue and its update block
// made, at the first watch and kept for the process, as each C callback
// is made once. The block runs on the queue, a serial one, so the
// monitor's answers come in order.
var monitor struct {
	once sync.Once
	err  error

	create, setHandler, setQueue, start, cancel dl.Fn // nw_path_monitor_*
	status, usesType                            dl.Fn // nw_path_get_status, nw_path_uses_interface_type
	queue                                       objc.ID
	handler                                     objc.ID // the update block, never released

	mu       sync.Mutex
	running  objc.ID // the nw_path_monitor_t, 0 while none runs
	watchers []*Network

	kind atomic.Int32 // the NetworkKind of the last path, NetworkUnknown while none runs
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// start adds n to the monitor's watchers, starting the monitor for the
// first: nw_path_monitor_create, its update handler and queue set before
// nw_path_monitor_start, as path_monitor.h asks. Its first path comes on
// the queue at once.
func (o *netOS) start(n *Network) error {
	monitor.once.Do(setupMonitor)
	if monitor.err != nil {
		return monitor.err
	}
	o.n = n
	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	monitor.watchers = append(monitor.watchers, n)
	if monitor.running != 0 {
		return nil
	}
	m := objc.ID(monitor.create.Call())
	if m == 0 {
		monitor.watchers = monitor.watchers[:0]

		return fmt.Errorf("sys: network: nw_path_monitor_create failed: %w", core.ErrUnsupported)
	}
	monitor.setHandler.Call(uintptr(m), uintptr(monitor.handler))
	monitor.setQueue.Call(uintptr(m), uintptr(monitor.queue))
	monitor.start.Call(uintptr(m))
	monitor.running = m

	return nil
}

// kind returns the kind of the monitor's last path.
func (*netOS) kind() NetworkKind { return NetworkKind(monitor.kind.Load()) }

// stop removes the watcher; the last one cancels the monitor and
// releases it (nw_path_monitor_create's object, a retained OS object):
// no update comes after nw_path_monitor_cancel.
func (o *netOS) stop() {
	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	monitor.watchers = slices.DeleteFunc(monitor.watchers, func(w *Network) bool { return w == o.n })
	if len(monitor.watchers) > 0 || monitor.running == 0 {
		return
	}
	monitor.cancel.Call(uintptr(monitor.running))
	objc.Release(monitor.running)
	monitor.running = 0
	monitor.kind.Store(int32(NetworkUnknown))
}

// setupMonitor binds the C functions and makes the queue and the block,
// once; its failure is kept.
func setupMonitor() {
	m := &monitor
	if err := objc.Init(); err != nil {
		m.err = fmt.Errorf("sys: network: %w", err)

		return
	}
	nw, err := dl.Open(pathNetwork)
	if err != nil {
		m.err = fmt.Errorf("sys: network: %w", err)

		return
	}
	if err := nw.Bind(
		dl.Sym{Fn: &m.create, Name: "nw_path_monitor_create"},
		dl.Sym{Fn: &m.setHandler, Name: "nw_path_monitor_set_update_handler"},
		dl.Sym{Fn: &m.setQueue, Name: "nw_path_monitor_set_queue"},
		dl.Sym{Fn: &m.start, Name: "nw_path_monitor_start"},
		dl.Sym{Fn: &m.cancel, Name: "nw_path_monitor_cancel"},
		dl.Sym{Fn: &m.status, Name: "nw_path_get_status"},
		dl.Sym{Fn: &m.usesType, Name: "nw_path_uses_interface_type"},
	); err != nil {
		m.err = fmt.Errorf("sys: network: %w", err)

		return
	}
	libSystem, err := dl.Open(pathLibSystem)
	if err != nil {
		m.err = fmt.Errorf("sys: network: %w", err)

		return
	}
	var queueCreate dl.Fn
	if err := libSystem.Bind(dl.Sym{Fn: &queueCreate, Name: "dispatch_queue_create"}); err != nil {
		m.err = fmt.Errorf("sys: network: %w", err)

		return
	}
	// A serial queue (DISPATCH_QUEUE_SERIAL is NULL), kept for the process.
	m.queue = objc.ID(queueCreate.Call(dl.Lit("com.jivecode.dancefitvn.network\x00"), 0))
	if m.queue == 0 {
		m.err = fmt.Errorf("sys: network: dispatch_queue_create failed: %w", core.ErrUnsupported)

		return
	}
	// void (^nw_path_monitor_update_handler_t)(nw_path_t path)
	m.handler = newBlock(func(args []reflect.Value) { pathUpdated(idArg(args[0])) }, idType)
}

// pathUpdated takes the monitor's new path, on its queue: the path is
// the block's argument, borrowed for the call.
func pathUpdated(path objc.ID) {
	k := pathKind(path)
	monitor.mu.Lock()
	if monitor.running == 0 {
		monitor.mu.Unlock()

		return
	}
	monitor.kind.Store(int32(k))
	watchers := slices.Clone(monitor.watchers)
	monitor.mu.Unlock()
	for _, w := range watchers {
		w.changed(k)
	}
}

// pathKind returns the kind of network path goes through: none unless it
// is satisfied (or satisfiable: a connection would bring its interface
// up), and then by the interfaces it uses, mobile data first, so that a
// path using it beside another (a VPN over it) counts as mobile data.
func pathKind(path objc.ID) NetworkKind {
	if path == 0 {
		return NetworkUnknown
	}
	switch monitor.status.Call(uintptr(path)) & 0xffffffff {
	case nwPathStatusSatisfied, nwPathStatusSatisfiable:
	case nwPathStatusUnsatisfied:
		return NetworkNone
	case nwPathStatusInvalid:
		return NetworkUnknown
	default:
		return NetworkUnknown
	}
	switch {
	case usesInterface(path, nwInterfaceCellular):
		return NetworkCellular
	case usesInterface(path, nwInterfaceWiFi):
		return NetworkWiFi
	case usesInterface(path, nwInterfaceWired):
		return NetworkWired
	default:
		return NetworkOther
	}
}

// usesInterface calls nw_path_uses_interface_type, whose C bool is the
// low byte of the result register.
func usesInterface(path objc.ID, t uintptr) bool {
	return uint8(monitor.usesType.Call(uintptr(path), t)) != 0
}
