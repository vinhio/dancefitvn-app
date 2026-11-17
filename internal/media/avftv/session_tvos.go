// Adapted from jplayer-tv/internal/media/avf/session_tvos.go at 781bee5.

//go:build ios && tvos

package avftv

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// activateSession sets the audio session's category to playback (mode
// default) the first time, and activates it unless it is active (04 §5,
// OQ-T4-4). Failures are logged, not fatal: playback goes on. The session
// counts as active only once setActive: succeeded, so a failed activation
// is tried again at the next Play. While warmSession does it off the UI
// thread, it leaves it to that.
func (e *Engine) activateSession() {
	if e.sessionOn || e.warming {
		return
	}
	session := objc.Call(av.session, sel.sharedInstance)
	if session == 0 {
		e.log.Warn("avf: no audio session")

		return
	}
	if !e.sessionSet {
		e.sessionSet = true
		e.nsErr = 0
		if !objc.CallBool(session, sel.setCategory, uintptr(av.category), uintptr(av.mode), 0,
			uintptr(unsafe.Pointer(&e.nsErr))) { //nolint:gosec // G103: a field of a heap object
			e.log.Warn("avf: audio session category", "err", sessionError(e.nsErr))
		}
	}
	if !e.setActive(session, true) {
		e.log.Warn("avf: audio session activate", "err", sessionError(e.nsErr))

		return
	}
	e.sessionOn = true
}

// warmSession sets the category and activates the audio session on a
// goroutine of its own, once, at New: the first activation blocks for
// 90–100 ms (tvOS 27 simulator), which the first Play paid on the UI
// thread (R-P4-6, P4.1), and Apple advises activating off the main
// thread. The result comes back to the UI thread through Post: on success
// the session counts as set and active; on a failure the next Play tries
// again there (activateSession). UI thread.
func (e *Engine) warmSession() {
	if e.sessionOn || e.warming {
		return
	}
	e.warming = true
	post := e.post
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		var category, active bool
		var catErr, actErr string
		objc.WithPool(func() {
			session := objc.Call(av.session, sel.sharedInstance)
			if session == 0 {
				return
			}
			nsErr := new(objc.ID) // the NSError out-parameter, on the heap (docs/conventions.md §7)
			category = objc.CallBool(session, sel.setCategory, uintptr(av.category), uintptr(av.mode), 0,
				uintptr(unsafe.Pointer(nsErr))) //nolint:gosec // G103: a heap object, written during the call
			if !category {
				catErr = sessionError(*nsErr)
			}
			*nsErr = 0
			active = objc.CallBool(session, sel.setActive, 1, uintptr(unsafe.Pointer(nsErr))) //nolint:gosec // G103: as above
			if !active {
				actErr = sessionError(*nsErr)
			}
		})
		post(func() { e.warmed(category, active, catErr, actErr) })
	}()
}

// warmed takes warmSession's result on the UI thread.
func (e *Engine) warmed(category, active bool, catErr, actErr string) {
	e.warming = false
	if e.closed {
		return
	}
	if catErr != "" {
		e.log.Warn("avf: audio session category", "err", catErr)
	}
	if actErr != "" {
		e.log.Warn("avf: audio session activate", "err", actErr)
	}
	e.sessionSet = e.sessionSet || category
	e.sessionOn = e.sessionOn || active
}

// deactivateSession deactivates the audio session if Play activated it.
func (e *Engine) deactivateSession() {
	if !e.sessionOn {
		return
	}
	e.sessionOn = false
	session := objc.Call(av.session, sel.sharedInstance)
	if session == 0 {
		return
	}
	if !e.setActive(session, false) {
		e.log.Warn("avf: audio session deactivate", "err", sessionError(e.nsErr))
	}
}

// sessionSetActive is setActive:error: on session, its NSError left in
// e.nsErr; the default of Engine.setActive.
func (e *Engine) sessionSetActive(session objc.ID, on bool) bool {
	flag := uintptr(0)
	if on {
		flag = 1
	}
	e.nsErr = 0

	return objc.CallBool(session, sel.setActive, flag, uintptr(unsafe.Pointer(&e.nsErr))) //nolint:gosec // G103: a field of a heap object
}

// sessionError describes an audio session's NSError by domain and code.
func sessionError(nsErr objc.ID) string {
	if nsErr == 0 {
		return "no error object"
	}
	var out string
	objc.WithPool(func() {
		out = fmt.Sprintf("%s %d", cleanDomain(objc.GoString(objc.Call(nsErr, sel.domain))), int(objc.Call(nsErr, sel.code)))
	})

	return out
}
