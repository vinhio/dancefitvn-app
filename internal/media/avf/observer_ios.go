// Adapted from jplayer-mobile/internal/media/avf/observer_ios.go at 49de3c2.

//go:build ios && !tvos && cgo

package avf

import (
	"dancefitvn-app/internal/media"
	"log"
	"sync"

	"github.com/gdeskdev/desk/foundation/ffi/cf"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// registry maps each observer object to what it observes for: a load of an
// engine (JPAVFObserver, one per Load) or a session (JPAVFSessionObserver).
// The callbacks may run on any thread, hence the lock. Key-value observing
// of AVPlayer and AVPlayerItem comes on the main queue (AVPlayer.h:49,
// AVPlayerItem.h:76), which is the UI thread of gDesk on iOS, and is handled
// at once; anything else goes through the engine's Dispatch.
var registry struct {
	mu       sync.Mutex
	loads    map[objc.ID]*load
	sessions map[objc.ID]*session
	engines  map[objc.ID]*engine // the observers of the scene, one an engine
}

// nsError is what an NSError tells, read on the thread that has it.
type nsError struct {
	domain, underDomain string
	code, underCode     int
	text                string
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// watchLoad records that obs observes for l.
func watchLoad(obs objc.ID, l *load) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.loads == nil {
		registry.loads = map[objc.ID]*load{}
	}
	registry.loads[obs] = l
}

// unwatchLoad forgets obs.
func unwatchLoad(obs objc.ID) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	delete(registry.loads, obs)
}

// watchSession records that obs observes for s.
func watchSession(obs objc.ID, s *session) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if registry.sessions == nil {
		registry.sessions = map[objc.ID]*session{}
	}
	registry.sessions[obs] = s
}

// watchEngine records that obs observes the scene for e; nil forgets obs.
func watchEngine(obs objc.ID, e *engine) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if e == nil {
		delete(registry.engines, obs)

		return
	}
	if registry.engines == nil {
		registry.engines = map[objc.ID]*engine{}
	}
	registry.engines[obs] = e
}

// observers counts the load observers of e alive.
func observers(e *engine) int {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	n := 0
	for _, l := range registry.loads {
		if l.e == e {
			n++
		}
	}

	return n
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// observed is -observeValueForKeyPath:ofObject:change:context: of
// JPAVFObserver: the item's status or the player's timeControlStatus
// changed. Both are read again on the UI thread, so which one changed does
// not matter.
func observed(self objc.ID, _ objc.SEL, _, _, _ objc.ID, _ uintptr) {
	l := loadOf(self)
	if l == nil {
		return
	}
	if objc.IsMainThread() {
		l.e.sync(l)

		return
	}
	l.e.cfg.Dispatch(func() { l.e.sync(l) })
}

// notified is -jpEnded: and -jpFailed: of JPAVFObserver: the item played to
// its end, or failed to (its error read here, on the thread posting).
func notified(self objc.ID, note objc.ID, ended bool) {
	l := loadOf(self)
	if l == nil {
		return
	}
	var cause nsError
	if !ended {
		objc.WithPool(func() {
			info := objc.Call(note, av.sel.userInfo)
			if info != 0 {
				cause = describe(objc.Call(info, av.sel.objectForKey, uintptr(av.failedErrorKey)))
			}
		})
	}
	run := func() {
		if ended {
			l.e.onEnded(l)
		} else {
			l.e.onPlayFailed(l, cause)
		}
	}
	if objc.IsMainThread() {
		run()

		return
	}
	l.e.cfg.Dispatch(run)
}

// sessionNotified is -jpInterruption: and -jpRouteChange: of
// JPAVFSessionObserver. The user info is read on the thread posting (route
// changes come on a secondary one); the event is published on the UI
// thread. An interruption that began because the system suspended the app
// (its reason AppWasSuspended, or the older WasSuspended flag: M4.5 D5) is
// no call nor other app: nothing played while the app was suspended, so
// the player is not told; the session only learns that it is inactive.
func sessionNotified(self objc.ID, note objc.ID, interruption bool) {
	s := sessionOf(self)
	if s == nil {
		return
	}
	var ev media.SessionEvent
	var reason int64
	suspended := false
	objc.WithPool(func() {
		info := objc.Call(note, av.sel.userInfo)
		if interruption {
			ev.Kind = media.InterruptEnd
			if intOf(info, av.interruptionType) == interruptionBegan {
				ev.Kind = media.InterruptBegin
				reason = intOf(info, av.interruptionReason)
				suspended = reason == reasonAppSuspended || intOf(info, av.interruptionSuspended) != 0
			}
			ev.ShouldResume = ev.Kind == media.InterruptEnd && intOf(info, av.interruptionOption)&interruptionResume != 0

			return
		}
		if intOf(info, av.routeReason) == routeOldDeviceAbsent {
			ev.Kind = media.Noisy
		}
	})
	if ev.Kind == media.InterruptBegin {
		log.Printf("dancefitvn: avf interruption began (reason %d, suspended %t)", reason, suspended)
	}
	if suspended {
		s.cfg.Dispatch(s.onSuspendedFn)

		return
	}
	if ev.Kind == media.SessionEventUnknown {
		return
	}
	s.cfg.Dispatch(func() { s.onEvent(ev) })
}

// sessionReset is -jpMediaReset: of JPAVFSessionObserver: the system's
// media services were reset (AVAudioSessionMediaServicesWereResetNotification),
// which every AVFoundation object of the app outlived; handled on the UI
// thread.
func sessionReset(self objc.ID) {
	s := sessionOf(self)
	if s == nil {
		return
	}
	log.Printf("dancefitvn: avf media services were reset")
	s.cfg.Dispatch(s.onResetFn)
}

// enteringForeground is -jpWillEnterForeground: of JPAVFObserver: the
// app's scene leaves the background (UISceneWillEnterForegroundNotification,
// on the main thread), ahead of the lifecycle's Active and so of
// SetBackground(false).
func enteringForeground(self objc.ID) {
	registry.mu.Lock()
	e := registry.engines[self]
	registry.mu.Unlock()
	if e == nil {
		return
	}
	if objc.IsMainThread() {
		e.leavingBackground()

		return
	}
	e.cfg.Dispatch(e.leavingBackground)
}

// sessionOf returns the session obs observes for, nil when none.
func sessionOf(obs objc.ID) *session {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	return registry.sessions[obs]
}

// loadOf returns the load obs observes for, nil when it was unloaded.
func loadOf(obs objc.ID) *load {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	return registry.loads[obs]
}

// intOf reads the NSNumber of key in the dictionary info, 0 when missing.
func intOf(info, key objc.ID) int64 {
	if info == 0 {
		return 0
	}
	v, _ := cf.IntValue(cf.Ref(objc.Call(info, av.sel.objectForKey, uintptr(key))))

	return v
}

// describe reads an NSError: its domain and code, those of the error under
// NSUnderlyingErrorKey, and its text, for the log. A nil error gives the
// zero nsError. In a pool.
func describe(e objc.ID) nsError {
	if e == 0 {
		return nsError{}
	}
	d := nsError{
		domain: objc.GoString(objc.Call(e, av.sel.domain)),
		code:   int(objc.Call(e, av.sel.code)),
		text:   objc.GoString(objc.Call(e, av.sel.localizedDescription)),
	}
	if info := objc.Call(e, av.sel.userInfo); info != 0 {
		if under := objc.Call(info, av.sel.objectForKey, uintptr(av.underlyingErrorKey)); under != 0 {
			d.underDomain = objc.GoString(objc.Call(under, av.sel.domain))
			d.underCode = int(objc.Call(under, av.sel.code))
		}
	}

	return d
}
