// Adapted from jplayer-tv/internal/media/avf/observe_tvos.go at 781bee5.

//go:build ios && tvos

package avftv

import (
	"sync/atomic"

	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// The bits a callback sets for the drain to read.
const (
	flagChanged     uint32 = 1 << iota // a key-value observation fired, or an Engine method asks for a read
	flagFailedToEnd                    // AVPlayerItemFailedToPlayToEndTimeNotification
	flagStalled                        // AVPlayerItemPlaybackStalledNotification
	flagErrorLog                       // AVPlayerItemNewErrorLogEntryNotification
	flagEnded                          // AVPlayerItemDidPlayToEndTimeNotification (added for DanceFITVN)
)

// Which callback fired, for callbackCounts.
const (
	callChanged = iota
	callFailed
	callStalled
	callErrorLog
	callEnded
	callInterruption
	callKinds
)

// kvoOptions are the options of every key-value observation: none. The
// engine reads the current values on the UI thread ("notify, then read"),
// so it needs no change dictionary carrying the new value.
const kvoOptions = 0

// slots is how many engines may live at once in a process (one in the
// app; tests make a few in turn).
const slots = 8

// registry maps an observer instance to its engine for the callbacks,
// which receive only the observer: a fixed array scanned linearly, read
// with atomics from any thread, so a callback neither locks nor
// allocates. A slot's engine is cleared before its observer is released.
var registry [slots]struct {
	obs atomic.Uintptr
	e   atomic.Pointer[Engine]
}

// callbacks counts the callbacks of the process by kind, for tests and
// diagnostics.
var callbacks [callKinds]atomic.Uint64

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onChanged is observeValueForKeyPath:ofObject:change:context:, the
// key-value observing callback, as a function of the receiver alone (the
// key path, object, change and context are not read). It runs on the
// main queue or synchronously inside the engine's own sends.
func onChanged(self objc.ID) { notifyFrom(self, callChanged, flagChanged) }

// onFailed is itemFailed:, for AVPlayerItemFailedToPlayToEndTimeNotification.
// Item notifications may come on any thread (AVPlayerItem.h:23).
func onFailed(self objc.ID) { notifyFrom(self, callFailed, flagFailedToEnd) }

// onStalled is itemStalled:, for AVPlayerItemPlaybackStalledNotification:
// a nudge to read again; timeControlStatus carries the state.
func onStalled(self objc.ID) { notifyFrom(self, callStalled, flagStalled) }

// onErrorLog is itemErrorLog:, for AVPlayerItemNewErrorLogEntryNotification:
// counted, never an event.
func onErrorLog(self objc.ID) { notifyFrom(self, callErrorLog, flagErrorLog) }

// onEnded is itemEnded:, for AVPlayerItemDidPlayToEndTimeNotification:
// the item played to its end (EventEnded). It may come on any thread.
func onEnded(self objc.ID) { notifyFrom(self, callEnded, flagEnded) }

// notifyFrom finds the engine of the observer self and notifies it. It
// locks nothing and allocates nothing.
func notifyFrom(self objc.ID, kind int, flag uint32) {
	callbacks[kind].Add(1)
	for i := range registry {
		if registry[i].obs.Load() == uintptr(self) {
			if e := registry[i].e.Load(); e != nil {
				e.notify(flag)
			}

			return
		}
	}
}

// register gives e a slot for its observer; false when all are taken.
func register(e *Engine, obs objc.ID) (int, bool) {
	for i := range registry {
		if registry[i].obs.CompareAndSwap(0, uintptr(obs)) {
			registry[i].e.Store(e)

			return i, true
		}
	}

	return -1, false
}

// unregister frees slot i: the engine first, so that a late callback finds
// no engine, then the observer.
func unregister(i int) {
	if i < 0 || i >= slots {
		return
	}
	registry[i].e.Store(nil)
	registry[i].obs.Store(0)
}

// notify records flag and, unless a drain is already queued, posts one to
// the UI thread. Any thread; it locks nothing and allocates nothing
// (Post may, through the app's queue).
func (e *Engine) notify(flag uint32) {
	e.bits.Or(flag)
	if flag == flagErrorLog {
		e.errorLogs.Add(1)
	}
	if e.pending.CompareAndSwap(false, true) {
		e.post(e.drainFn)
	}
}

// observePlayer starts observing the player's timeControlStatus and
// status. UI thread.
func (e *Engine) observePlayer() {
	objc.Call(e.player, sel.addKVO, uintptr(e.obs), uintptr(av.keyTimeControl), kvoOptions, 0)
	objc.Call(e.player, sel.addKVO, uintptr(e.obs), uintptr(av.keyStatus), kvoOptions, 0)
	e.observations += 2
}

// unobservePlayer stops observing the player. UI thread.
func (e *Engine) unobservePlayer() {
	objc.Call(e.player, sel.removeKVO, uintptr(e.obs), uintptr(av.keyTimeControl), 0)
	objc.Call(e.player, sel.removeKVO, uintptr(e.obs), uintptr(av.keyStatus), 0)
	e.observations -= 2
}

// observeItem starts observing item's status and its four
// notifications. UI thread.
func (e *Engine) observeItem(item objc.ID) {
	objc.Call(item, sel.addKVO, uintptr(e.obs), uintptr(av.keyStatus), kvoOptions, 0)
	center := objc.Call(av.center, sel.defaultCenter)
	for _, n := range [...]struct {
		sel  objc.SEL
		name objc.ID
	}{
		{sel.itemFailed, av.noteFailed}, {sel.itemStalled, av.noteStalled}, {sel.itemErrorLog, av.noteErrorLog},
		{sel.itemEnded, av.noteEnded},
	} {
		objc.Call(center, sel.addNote, uintptr(e.obs), uintptr(n.sel), uintptr(n.name), uintptr(item))
	}
	e.observations += 5
}

// unobserveItem stops every observation of item, before it is released.
// UI thread.
func (e *Engine) unobserveItem(item objc.ID) {
	objc.Call(item, sel.removeKVO, uintptr(e.obs), uintptr(av.keyStatus), 0)
	center := objc.Call(av.center, sel.defaultCenter)
	objc.Call(center, sel.removeNote, uintptr(e.obs), 0, uintptr(item))
	e.observations -= 5
}

// drain runs on the UI thread, posted by notify: it reads the player's
// state and acts on it. The item becoming ready tells the length, takes
// the start position asked and plays if asked (onReady); the first frame
// on the screen tells EventVideoSize once (checkPicture); the end of the
// item tells EventEnded once (onEnded); the machine's state changes are
// published (emitted). After Stop or Close, or before Load, it reads
// nothing. It allocates nothing unless it publishes.
func (e *Engine) drain() {
	e.pending.Store(false)
	b := e.bits.Swap(0)
	if e.closed || e.item == 0 || !e.m.loaded {
		return
	}
	r := reading{
		item:        int(objc.Call(e.item, sel.status)),
		player:      int(objc.Call(e.player, sel.status)),
		control:     int(objc.Call(e.player, sel.timeControlStatus)),
		failedToEnd: b&flagFailedToEnd != 0,
	}
	e.control = r.control
	if !e.ready && !e.failed && r.item == itemReady && r.player != playerFailed {
		e.onReady()
	}
	if !e.pictured {
		e.checkPicture()
	}
	if b&flagEnded != 0 {
		e.onEnded()
	}
	s, changed := e.m.step(r)
	if !changed {
		return
	}
	e.emitted(s, r)
}
