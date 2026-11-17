// Adapted from jplayer-tv/internal/media/avf/interrupt_tvos.go at 781bee5.

//go:build ios && tvos

package avftv

import (
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onInterruption is sessionInterrupted:, for
// AVAudioSessionInterruptionNotification: it reads the notification's
// type and options, queues the event for the engine of the observer self
// and posts its drain unless one is queued. The notification may come on
// any thread; the dictionary and number reads are thread-safe and return
// unowned objects. Rare, so purego's callback allocating for the
// notification argument does not matter; nothing is locked but the
// queue's short lock.
func onInterruption(self objc.ID, _ objc.SEL, note objc.ID) {
	callbacks[callInterruption].Add(1)
	e := engineOf(self)
	if e == nil || note == 0 {
		return
	}
	info := objc.Call(note, sel.userInfo)
	if info == 0 {
		return
	}
	typ := objc.Call(info, sel.objectForKey, uintptr(av.keyInterruptionType))
	if typ == 0 {
		return
	}
	var opts uint64
	if o := objc.Call(info, sel.objectForKey, uintptr(av.keyInterruptionOption)); o != 0 {
		opts = uint64(objc.Call(o, sel.unsignedIntegerValue))
	}
	ev, ok := decodeInterruption(uint64(objc.Call(typ, sel.unsignedIntegerValue)), opts)
	if !ok {
		return
	}
	if e.intr.push(ev) {
		e.post(e.intrFn)
	}
}

// engineOf finds the engine of the observer self in the registry, nil
// when its slot is cleared. Any thread; it locks nothing.
func engineOf(self objc.ID) *Engine {
	for i := range registry {
		if registry[i].obs.Load() == uintptr(self) {
			return registry[i].e.Load()
		}
	}

	return nil
}

// observeSession starts receiving the shared audio session's
// interruptions. UI thread.
func (e *Engine) observeSession() {
	if e.sessionObserved || e.obs == 0 {
		return
	}
	session := objc.Call(av.session, sel.sharedInstance)
	if session == 0 {
		e.log.Warn("avf: no audio session, interruptions not observed")

		return
	}
	center := objc.Call(av.center, sel.defaultCenter)
	objc.Call(center, sel.addNote, uintptr(e.obs), uintptr(sel.sessionInterrupted), uintptr(av.noteInterruption), uintptr(session))
	e.sessionObserved = true
	e.observations++
}

// unobserveSession stops receiving the interruptions, before the
// observer leaves the registry. UI thread.
func (e *Engine) unobserveSession() {
	if !e.sessionObserved {
		return
	}
	e.sessionObserved = false
	e.observations--
	session := objc.Call(av.session, sel.sharedInstance)
	center := objc.Call(av.center, sel.defaultCenter)
	objc.Call(center, sel.removeNote, uintptr(e.obs), uintptr(av.noteInterruption), uintptr(session))
}

// drainInterruptions runs on the UI thread, posted by onInterruption: it
// tells Config.OnInterruption of the interruptions waiting, in order;
// after Close, of none. It allocates nothing.
func (e *Engine) drainInterruptions() { e.intr.deliver(e.closed, e.cfg.OnInterruption) }
