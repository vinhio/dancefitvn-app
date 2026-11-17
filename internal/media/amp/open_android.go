// Adapted from jplayer-mobile/internal/media/amp/open_android.go at 49de3c2.

//go:build android && cgo

package amp

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// opened is what the opener made of a load: the player, the proxy of its
// listeners and the reader, all that it made even when it failed, the
// player's state after the calls, the calls the state machine refused, and
// why it failed, nil when the player is preparing.
type opened struct {
	mp      jni.Object // global
	proxy   *jni.Proxy
	reader  *reader
	state   enginekit.MPState
	refused int
	err     error
}

// openFrame is the room of the local frame the opener pushes for each load
// it opens.
const openFrame = 16

// errNoVM is a load the opener could not open: its thread has no VM.
var errNoVM = fmt.Errorf("amp: the opener has no Java VM: %w", core.ErrUnsupported)

// mainTID is the id of Java's main thread: the process's own, on Linux.
var mainTID = os.Getpid()

// timeRing keeps the last loadRing times of something another thread
// adds, without a lock.
type timeRing struct {
	v [loadRing]atomic.Int64
	n atomic.Int64
}

// offMain counts the listeners' calls that came on another thread than
// Java's main one (Stats.OffMainEvents): MediaPlayer posts its events to
// the Looper of the thread that made it, or the main one when that thread
// has none, as the opener's has not.
var offMain atomic.Int64

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// opener makes the players of the loads, on a thread of its own attached
// to the VM for its life (jni.Do), so the UI thread never waits for
// MediaPlayer's constructor and setDataSource (R3). Only the newest load
// waits for it (next): one that a later Load replaced before the opener
// took it is never opened. It ends when Close closes openWake.
func (e *engine) opener() {
	defer close(e.openDone)
	err := jni.Do(func(env jni.Env) error {
		for range e.openWake {
			if l := e.next.Swap(nil); l != nil {
				e.openOne(env, l)
			}
		}

		return nil
	})
	if err != nil { // no VM: every load fails
		for range e.openWake {
			if l := e.next.Swap(nil); l != nil {
				e.deliver(l, &opened{err: errNoVM})
				e.releasing.Add(-1)
			}
		}
	}
	if l := e.next.Swap(nil); l != nil {
		e.releasing.Add(-1)
	}
}

// applyOpen takes the opener's result of l, on the UI thread: the player
// is l's, preparing (onPrepared sets its volume), the rotation and the
// preflight start, and the events that came before replay. A player that
// could not be made, or set to its source, fails the load (EventError, as
// Load returned it before R3). A load that went meanwhile was freed by
// release, or by the opener.
func (e *engine) applyOpen(l *load) {
	l.mu.Lock()
	r, dropped := l.res, l.dropped
	l.res = nil
	l.mu.Unlock()
	if dropped || r == nil {
		return
	}
	if e.cur != l || !l.opening { // not so: release drops what it does not keep
		e.hand(r.held())

		return
	}
	l.opening = false
	e.refused += r.refused
	l.mp, l.proxy, l.reader, l.state = r.mp, r.proxy, r.reader, r.state
	switch {
	case r.err == nil:
	case errors.Is(r.err, errCleartext):
		e.fail(l, fmt.Errorf("%w: %s: %w", errCleartext, l.src.URL, media.ErrSource))

		return
	default:
		e.fail(l, enginekit.OpenError(enginekit.PhaseOpening, l.local, l.src.URL, r.err))

		return
	}
	if !l.src.AudioOnly && l.local {
		e.readRotation(l) // a link's waits for onPrepared: no second fetch of a dead one
	}
	e.startPreflight(l)
	switch {
	case l.earlyErr:
		e.onError(l, l.earlyWhat, l.earlyExtra)
	case l.earlyPrepared:
		e.onPrepared(l)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// openOne opens l, unless it went before the opener took it, in a local
// frame of its own, and hands the result over. Opener's thread.
func (e *engine) openOne(env jni.Env, l *load) {
	defer e.releasing.Add(-1)
	l.mu.Lock()
	dropped := l.dropped
	l.mu.Unlock()
	if dropped {
		return
	}
	t0 := time.Now()
	framed := env.PushLocalFrame(openFrame) == nil
	r := e.open(env, l)
	if framed {
		env.PopLocalFrame(0)
	}
	e.opens.add(time.Since(t0))
	e.deliver(l, r)
}

// deliver hands r to l and has the UI thread apply it; to the releaser
// when l went meanwhile. Opener's thread.
func (e *engine) deliver(l *load, r *opened) {
	l.mu.Lock()
	if l.dropped {
		l.mu.Unlock()
		e.hand(r.held())

		return
	}
	l.res = r
	l.mu.Unlock()
	e.cfg.Dispatch(l.openedFn)
}

// held is what r holds, for the releaser.
func (r *opened) held() held {
	return held{mp: r.mp, proxy: r.proxy, reader: r.reader, call: r.state.Allows(enginekit.CallRelease)}
}

// call makes the call c of the player r made, method m, when its state
// allows it, and follows the state, as engine.call does on the UI thread.
func (r *opened) call(env jni.Env, c enginekit.MPCall, m jni.MethodID, args ...jni.Value) error {
	if r.mp == 0 || !r.state.Allows(c) {
		r.refused++

		return fmt.Errorf("amp: call %d in state %d: %w", c, r.state, core.ErrInvalid)
	}
	if err := env.CallVoid(r.mp, m, args...); err != nil {
		return err
	}
	r.state = r.state.After(c)

	return nil
}

// countThread counts a listener's call off Java's main thread. It
// allocates nothing.
func countThread() {
	if syscall.Gettid() != mainTID {
		offMain.Add(1)
	}
}

// add keeps one more time.
func (r *timeRing) add(t time.Duration) {
	r.v[(r.n.Add(1)-1)%loadRing].Store(int64(t))
}

// list returns the times kept.
func (r *timeRing) list() []time.Duration {
	n := min(int(r.n.Load()), loadRing)
	out := make([]time.Duration, n)
	for i := range n {
		out[i] = time.Duration(r.v[i].Load())
	}

	return out
}
