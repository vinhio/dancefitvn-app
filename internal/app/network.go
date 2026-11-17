// Adapted from jplayer-mobile/internal/app/network.go at 49de3c2.
// Changed: jPlayer's developer variable JPLAYER_CELLULAR (every network
// taken for mobile data) is DFV_NETWORK here, with the kinds none, wifi,
// cellular and auto, read at launch and again, every second, from a file
// of the app's data folder (scripts/net.sh), since the iOS simulator has no
// airplane mode (local/plans/P5.md F-6); the kind drives the app's offline
// state and the downloads' rules (03 section 2, Q-P5-2) instead of the
// player's links. A build that ships reads none of it.

package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dancefitvn-app/internal/sys"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// networkEnv names the variable of a developer's build that forces the
// kind of network at launch: none, wifi, cellular, or auto (the system's).
const networkEnv = "DFV_NETWORK"

// networkFile is the file, under the data folder, a developer's build reads
// every networkPoll for the kind to force (scripts/net.sh writes it); it
// wins over networkEnv while it exists, "auto" giving the system's kind
// back.
const networkFile = "dev/network"

// networkPoll is how often a developer's build reads networkFile.
const networkPoll = time.Second

// noForce is a devNetwork that forces no kind.
const noForce = -1

// netWatch is what the services read of the device's network: its kind
// now, and its changes on the UI thread. *sys.Network is one, nil
// included (it knows no network); a test gives a fake.
type netWatch interface {
	// Kind is the kind of network now; it allocates nothing.
	Kind() sys.NetworkKind
	// OnChange has fn told each change of kind, on the UI thread.
	OnChange(fn func(sys.NetworkKind))
	// Close stops the watch.
	Close()
}

// devNetwork is the system's network with the kind a developer forces
// over it (networkEnv, networkFile). Safe for concurrent use.
type devNetwork struct {
	sys      netWatch
	dispatch func(func())
	file     string
	logf     func(format string, args ...any)

	forced atomic.Int32                          // the kind forced, noForce for the system's
	env    int32                                 // the kind of networkEnv, noForce for none
	fn     atomic.Pointer[func(sys.NetworkKind)] // OnChange's function
	stop   chan struct{}
	once   sync.Once
}

var (
	_ netWatch = (*sys.Network)(nil)
	_ netWatch = (*devNetwork)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// watchNetwork starts watching the device's network (sys.WatchNetwork, its
// changes told through dispatch) and returns it; a nil *sys.Network where
// the system has none (the Mac, a test), which knows no network and never
// changes. A developer's build puts the kind of forced (networkEnv) and of
// the file devFile (networkFile) over it. Close it as the window closes.
func watchNetwork(dispatch func(func()), logf func(format string, args ...any), devFile, forced string) netWatch {
	n, err := sys.WatchNetwork(dispatch)
	if err != nil && !errors.Is(err, core.ErrUnsupported) {
		logf("dancefitvn: network: %v; downloads use any network", err)
	}
	if !devBuild {
		return n
	}
	d := newDevNetwork(n, dispatch, devFile, forced, logf)
	if d.env != noForce {
		logf("dancefitvn: network: %s forces %s", networkEnv, sys.NetworkKind(d.env))
	}
	go d.run()

	return d
}

// newDevNetwork returns the network of base with the kind of forced (a
// value of networkEnv) and of file over it; it reads file once, and polls
// it only once run is started.
func newDevNetwork(base netWatch, dispatch func(func()), file, forced string, logf func(format string, args ...any)) *devNetwork {
	d := &devNetwork{sys: base, dispatch: dispatch, file: file, logf: logf, stop: make(chan struct{})}
	d.env = parseNetwork(forced)
	d.forced.Store(d.env)
	d.poll()

	return d
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Kind is the kind forced, else the system's.
func (d *devNetwork) Kind() sys.NetworkKind {
	if f := d.forced.Load(); f != noForce {
		return sys.NetworkKind(f)
	}

	return d.sys.Kind()
}

// OnChange has fn told each change of the kind Kind reports: the system's
// while none is forced, and each change of the kind forced.
func (d *devNetwork) OnChange(fn func(sys.NetworkKind)) {
	if fn == nil {
		d.fn.Store(nil)
		d.sys.OnChange(nil)

		return
	}
	d.fn.Store(&fn)
	d.sys.OnChange(func(k sys.NetworkKind) {
		if d.forced.Load() == noForce {
			fn(k)
		}
	})
}

// Close stops the poll and the system's watch.
func (d *devNetwork) Close() {
	d.once.Do(func() { close(d.stop) })
	d.sys.Close()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// run polls the file every networkPoll until Close.
func (d *devNetwork) run() {
	t := time.NewTicker(networkPoll)
	defer t.Stop()
	for {
		select {
		case <-d.stop:
			return
		case <-t.C:
			d.poll()
		}
	}
}

// poll reads the file and, when the kind it forces changed, tells the new
// kind on the UI thread. A missing file leaves the kind of networkEnv.
func (d *devNetwork) poll() {
	want := d.env
	if raw, err := os.ReadFile(d.file); err == nil {
		want = parseNetwork(string(raw))
	}
	if d.forced.Swap(want) == want {
		return
	}
	k := d.Kind()
	if want == noForce {
		d.logf("dancefitvn: network: the system's again: %s (%s)", k, d.file)
	} else {
		d.logf("dancefitvn: network: forced to %s (%s)", k, d.file)
	}
	d.dispatch(func() {
		if fn := d.fn.Load(); fn != nil {
			(*fn)(k)
		}
	})
}

// parseNetwork is the kind a value of networkEnv or of networkFile forces:
// none, wifi or cellular; noForce for auto, empty or anything else.
func parseNetwork(v string) int32 {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "none", "off", "airplane":
		return int32(sys.NetworkNone)
	case "wifi", "wi-fi":
		return int32(sys.NetworkWiFi)
	case "cellular", "data", "mobile":
		return int32(sys.NetworkCellular)
	default:
		return noForce
	}
}

// devNetworkFile returns where networkFile is in the data folder of the
// device, "" when there is none.
func devNetworkFile() string {
	data, err := sys.DataDir()
	if err != nil {
		return ""
	}

	return filepath.Join(data, filepath.FromSlash(networkFile))
}
