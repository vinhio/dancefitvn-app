package store

import (
	"slices"
	"sync"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Online is the app's view of the network: online until a call learns
// otherwise. Set has the signature of api.Options.OnNetwork, so
// `OnNetwork: online.Set` feeds it. It is safe for concurrent use.
type Online struct {
	mu      sync.Mutex
	offline bool
	subs    []func(online bool)
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewOnline returns an Online that reads as online.
func NewOnline() *Online { return &Online{} }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Online reports whether the network is thought to be up.
func (o *Online) Online() bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	return !o.offline
}

// Set records the network state and tells the subscribers when it changed.
// They run on the calling goroutine, after the lock is released, so one may
// call Online.
func (o *Online) Set(online bool) {
	o.mu.Lock()
	changed := o.offline == online
	o.offline = !online
	subs := slices.Clone(o.subs)
	o.mu.Unlock()

	if !changed {
		return
	}
	for _, fn := range subs {
		fn(online)
	}
}

// Subscribe adds fn to those told of a change of state.
func (o *Online) Subscribe(fn func(online bool)) {
	o.mu.Lock()
	o.subs = append(o.subs, fn)
	o.mu.Unlock()
}
