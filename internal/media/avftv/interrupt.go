// Adapted from jplayer-tv/internal/media/avf/interrupt.go at 781bee5.

package avftv

import "sync"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The values of an AVAudioSessionInterruptionNotification's userInfo, as
// AVFAudio's headers number them (AVAudioSessionTypes.h).
const (
	// interruptionEnded is AVAudioSessionInterruptionTypeEnded.
	interruptionEnded = 0
	// interruptionBegan is AVAudioSessionInterruptionTypeBegan.
	interruptionBegan = 1
	// interruptionShouldResume is the bit of
	// AVAudioSessionInterruptionOptionShouldResume in the options.
	interruptionShouldResume = 1
)

// interruptCap is how many interruptions wait for the UI thread at most;
// more before a drain replace the last one.
const interruptCap = 8

// interruptEvent is one interruption as the UI thread is told of it. The
// zero value, interruptEventUnknown, is none: OnInterruption is told
// nothing began and nothing may resume.
type interruptEvent uint8

// The interruption events.
const (
	// interruptEventUnknown: no interruption (a type decode does not know).
	interruptEventUnknown interruptEvent = iota
	// interruptBegan: an interruption began.
	interruptBegan
	// interruptEnded: it ended; the app should stay paused.
	interruptEnded
	// interruptEndedResume: it ended and the app may play again.
	interruptEndedResume
)

// interruptQueue keeps the interruptions a notification reported, from
// any thread, until the UI thread's drain takes them, in order. It locks
// briefly and allocates nothing.
type interruptQueue struct {
	mu     sync.Mutex
	ev     [interruptCap]interruptEvent
	n      int
	posted bool // a drain is queued and has not taken the events yet
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// decodeInterruption reads an interruption notification's type and
// options (AVAudioSessionInterruptionTypeKey, …OptionKey): began for
// TypeBegan; for TypeEnded, resume when the ShouldResume bit is set. A
// type it does not know gives ok false.
func decodeInterruption(typ, opts uint64) (ev interruptEvent, ok bool) {
	switch typ {
	case interruptionBegan:
		return interruptBegan, true
	case interruptionEnded:
		if opts&interruptionShouldResume != 0 {
			return interruptEndedResume, true
		}

		return interruptEnded, true
	default:
		return interruptEventUnknown, false
	}
}

// split returns what OnInterruption is told of ev.
func (ev interruptEvent) split() (began, shouldResume bool) {
	switch ev {
	case interruptBegan:
		return true, false
	case interruptEndedResume:
		return false, true
	case interruptEnded, interruptEventUnknown:
		return false, false
	default:
		return false, false
	}
}

// push records ev and tells whether the caller must post a drain: none is
// queued yet. When the queue is full, ev replaces the last event. Any
// thread.
func (q *interruptQueue) push(ev interruptEvent) (post bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.n == interruptCap {
		q.ev[interruptCap-1] = ev
	} else {
		q.ev[q.n] = ev
		q.n++
	}
	if q.posted {
		return false
	}
	q.posted = true

	return true
}

// deliver takes the events waiting and, unless closed or fn is nil, tells
// fn of each in order, outside the lock. The drain's work, on the UI
// thread; it allocates nothing.
func (q *interruptQueue) deliver(closed bool, fn func(began, shouldResume bool)) {
	var buf [interruptCap]interruptEvent
	q.mu.Lock()
	n := copy(buf[:], q.ev[:q.n])
	q.n, q.posted = 0, false
	q.mu.Unlock()
	if closed || fn == nil {
		return
	}
	for _, ev := range buf[:n] {
		fn(ev.split())
	}
}
