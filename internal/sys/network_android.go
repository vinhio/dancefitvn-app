// Adapted from jplayer-mobile/internal/sys/network_android.go at 49de3c2.
// Changed: lookupMethod is paths_android.go's (instance methods only).

//go:build android

package sys

import (
	"fmt"
	"log"
	"sync"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The constants of android.jar (platform 35), read with javap -constants:
// Context.CONNECTIVITY_SERVICE, ConnectivityManager.CONNECTIVITY_ACTION
// (a broadcast still sent to receivers registered at run time) and
// NetworkCapabilities' transports.
const (
	connectivityService = "connectivity"
	connectivityAction  = "android.net.conn.CONNECTIVITY_CHANGE"

	transportCellular int32 = 0
	transportWiFi     int32 = 1
	transportEthernet int32 = 3
)

// netOS is the Android side of a Network: the ConnectivityManager it
// asks (a global reference) and the receiver of the changes, logged.
// ConnectivityManager.NetworkCallback, the modern watch, is an abstract
// class, which jni.NewProxy (a java.lang.reflect.Proxy, interfaces only)
// cannot implement without Java of the app's: the kind is asked of the
// system at each call instead, which is exact and cheap, and the
// broadcast only tells the log.
type netOS struct {
	n        *Network
	manager  jni.Object // android.net.ConnectivityManager, global
	receiver *jni.Receiver

	mu    sync.Mutex // one query at a time: query writes got
	query func(jni.Env) error
	got   NetworkKind
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// netIDs are the methods a query calls, looked up at the first watch:
// the system's classes are never unloaded, so their IDs last.
var netIDs struct {
	sync.Mutex
	found bool

	getSystemService jni.MethodID // Context.getSystemService(Ljava/lang/String;)Ljava/lang/Object;
	activeNetwork    jni.MethodID // ConnectivityManager.getActiveNetwork()Landroid/net/Network; (API 23)
	capabilities     jni.MethodID // ConnectivityManager.getNetworkCapabilities(Landroid/net/Network;)Landroid/net/NetworkCapabilities;
	hasTransport     jni.MethodID // NetworkCapabilities.hasTransport(I)Z
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// start gets the ConnectivityManager and registers the receiver of
// CONNECTIVITY_ACTION, which needs gDesk's Java: without it the changes
// go unlogged, and the kind is still asked at each call.
func (o *netOS) start(n *Network) error {
	o.n = n
	o.query = o.ask
	err := jni.Do(func(env jni.Env) error {
		if err := findNetIDs(env); err != nil {
			return err
		}
		ctx := jni.Context()
		if ctx == 0 {
			return fmt.Errorf("no context: %w", core.ErrUnsupported)
		}
		name, err := env.NewString(connectivityService)
		if err != nil {
			return err
		}
		defer env.DeleteLocalRef(name)
		cm, err := env.CallObject(ctx, netIDs.getSystemService, jni.Ref(name))
		if err != nil {
			return err
		}
		if cm == 0 {
			return fmt.Errorf("no ConnectivityManager: %w", core.ErrUnsupported)
		}
		o.manager = env.NewGlobalRef(cm)
		env.DeleteLocalRef(cm)
		if !jni.HasJava() {
			return nil
		}
		r, err := jni.RegisterReceiver(env, []string{connectivityAction}, o.broadcast)
		if err != nil {
			log.Printf("sys: network: no change broadcasts: %v", err)

			return nil
		}
		o.receiver = r

		return nil
	})
	if err != nil {
		o.release()

		return fmt.Errorf("sys: network: %w", err)
	}

	return nil
}

// kind asks the system the kind of the active network, on the calling
// thread (gDesk's UI thread is attached to the VM, where jni.Do and the
// calls allocate nothing; another thread is attached for the call). An
// error counts as unknown, which never holds a link back.
func (o *netOS) kind() NetworkKind {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.got = NetworkUnknown
	if o.manager == 0 {
		return NetworkUnknown
	}
	if err := jni.Do(o.query); err != nil {
		return NetworkUnknown
	}

	return o.got
}

// ask is the query of kind, made a function once: it reads the active
// network's capabilities into got.
func (o *netOS) ask(env jni.Env) error {
	k, err := activeKind(env, o.manager)
	o.got = k

	return err
}

// broadcast logs the kind the system has now, as it tells a change, on
// Java's main thread. The intent's extras (deprecated) are not read.
func (o *netOS) broadcast(env jni.Env, _ jni.Object) {
	o.mu.Lock()
	k, err := activeKind(env, o.manager)
	o.mu.Unlock()
	if err != nil {
		k = NetworkUnknown
	}
	o.n.changed(k)
}

// stop unregisters the receiver and deletes the manager's reference.
func (o *netOS) stop() {
	if o.receiver != nil {
		if err := o.receiver.Close(); err != nil {
			log.Printf("sys: network: %v", err)
		}
		o.receiver = nil
	}
	o.release()
}

// release deletes the manager's global reference, with no query running.
func (o *netOS) release() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.manager == 0 {
		return
	}
	m := o.manager
	o.manager = 0
	_ = jni.Do(func(env jni.Env) error {
		env.DeleteGlobalRef(m)

		return nil
	})
}

// activeKind returns the kind of cm's active network:
// getActiveNetwork (null: none), getNetworkCapabilities (null: a network
// gone meanwhile, none), then its transports, mobile data first so that
// a VPN over it counts as mobile data. It deletes the local references it
// makes; manager is 0 once released.
func activeKind(env jni.Env, cm jni.Object) (NetworkKind, error) {
	if cm == 0 {
		return NetworkUnknown, nil
	}
	net, err := env.CallObject(cm, netIDs.activeNetwork)
	if err != nil {
		return NetworkUnknown, err
	}
	if net == 0 {
		return NetworkNone, nil
	}
	caps, err := env.CallObject(cm, netIDs.capabilities, jni.Ref(net))
	env.DeleteLocalRef(net)
	if err != nil {
		return NetworkUnknown, err
	}
	if caps == 0 {
		return NetworkNone, nil
	}
	defer env.DeleteLocalRef(caps)
	for _, t := range [...]struct {
		transport int32
		kind      NetworkKind
	}{{transportCellular, NetworkCellular}, {transportWiFi, NetworkWiFi}, {transportEthernet, NetworkWired}} {
		has, err := env.CallBool(caps, netIDs.hasTransport, jni.Int(t.transport))
		if err != nil {
			return NetworkUnknown, err
		}
		if has {
			return t.kind, nil
		}
	}

	return NetworkOther, nil
}

// findNetIDs looks the methods of a query up, once.
func findNetIDs(env jni.Env) error {
	netIDs.Lock()
	defer netIDs.Unlock()
	if netIDs.found {
		return nil
	}
	for _, m := range []struct {
		id               *jni.MethodID
		class, name, sig string
	}{
		{&netIDs.getSystemService, "android/content/Context", "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;"},
		{&netIDs.activeNetwork, "android/net/ConnectivityManager", "getActiveNetwork", "()Landroid/net/Network;"},
		{&netIDs.capabilities, "android/net/ConnectivityManager", "getNetworkCapabilities", "(Landroid/net/Network;)Landroid/net/NetworkCapabilities;"},
		{&netIDs.hasTransport, "android/net/NetworkCapabilities", "hasTransport", "(I)Z"},
	} {
		if err := lookupMethod(env, m.id, m.class, m.name, m.sig); err != nil {
			return err
		}
	}
	netIDs.found = true

	return nil
}
