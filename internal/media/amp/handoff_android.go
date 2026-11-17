// Adapted from jplayer-mobile/internal/media/amp/handoff_android.go at 49de3c2.

//go:build android && cgo

package amp

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// handoffFor is how long the hand-off lock keeps the CPU awake at most:
// the next media of a queue starts well within it, a link included
// (prepareAsync runs meanwhile, under the lock).
const handoffFor = 15 * time.Second

// handoffTag is the name of the hand-off lock, as dumpsys power shows it.
const handoffTag = "jPlayer:next"

// handoff keeps the CPU awake from the end of a media to the start of the
// next (M9): MediaPlayer's wake mode lets go of its lock as the media
// completes, and with the screen off the system may then suspend the CPU
// before the app has loaded and started the next one, which stops a queue
// played in the background. The lock (PowerManager PARTIAL_WAKE_LOCK, not
// reference counted) is taken on Java's main thread in the completion's
// callback, where MediaPlayer has just let go of its own, for handoffFor
// at most, and let go once a media starts (MediaPlayer holds the CPU
// then), or the engine stops or closes. Without WAKE_LOCK it does nothing.
type handoff struct {
	mu   sync.Mutex // guards lock, which Java's main thread reads
	lock jni.Object // global, 0 when none
	held atomic.Bool
	// releaseDo is release's call into Java, made once.
	releaseDo func(jni.Env) error
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newHandoff returns the hand-off lock, made on env when may (the app
// holds WAKE_LOCK); otherwise a lock that does nothing.
func newHandoff(env jni.Env, may bool) (*handoff, error) {
	h := &handoff{}
	h.releaseDo = h.releaseOn
	ctx := jni.Context()
	if !may || ctx == 0 {
		return h, nil
	}
	name, err := env.NewString("power") // Context.POWER_SERVICE
	if err != nil {
		return nil, err
	}
	pm, err := env.CallObject(ctx, ids.getSystemService, jni.Ref(name))
	env.DeleteLocalRef(name)
	if err != nil {
		return nil, err
	}
	if pm == 0 {
		return nil, fmt.Errorf("amp: no PowerManager: %w", core.ErrNotFound)
	}
	defer env.DeleteLocalRef(pm)
	tag, err := env.NewString(handoffTag)
	if err != nil {
		return nil, err
	}
	wl, err := env.CallObject(pm, ids.newWakeLock, jni.Int(partialWakeLock), jni.Ref(tag))
	env.DeleteLocalRef(tag)
	if err != nil {
		return nil, err
	}
	if wl == 0 {
		return nil, fmt.Errorf("amp: newWakeLock: %w", core.ErrNotFound)
	}
	defer env.DeleteLocalRef(wl)
	if err := env.CallVoid(wl, ids.wakeCounted, jni.Bool(false)); err != nil {
		return nil, err
	}
	h.lock = newGlobal(env, wl)

	return h, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// take keeps the CPU awake for handoffFor at most, on env: the completion's
// callback, on Java's main thread.
func (h *handoff) take(env jni.Env) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.lock == 0 {
		return
	}
	if err := env.CallVoid(h.lock, ids.wakeAcquireFor, jni.Long(handoffFor.Milliseconds())); err != nil {
		logf("amp: hand-off lock: %v", err)

		return
	}
	h.held.Store(true)
}

// release lets go of the lock when it was taken. It allocates nothing. UI
// thread.
func (h *handoff) release() {
	if !h.held.Load() {
		return
	}
	if err := jni.Do(h.releaseDo); err != nil {
		logf("amp: hand-off lock release: %v", err)
	}
}

// isHeld asks the system whether the lock is held (Stats). UI thread.
func (h *handoff) isHeld() bool {
	held := false
	_ = jni.Do(func(env jni.Env) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.lock == 0 {
			return nil
		}
		var err error
		held, err = env.CallBool(h.lock, ids.wakeHeld)

		return err
	})

	return held
}

// close lets go of the lock and deletes it, on env.
func (h *handoff) close(env jni.Env) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.lock == 0 {
		return
	}
	_ = env.CallVoid(h.lock, ids.wakeRelease) // not counted: a lock not held is let go of quietly
	h.held.Store(false)
	deleteGlobal(env, h.lock)
	h.lock = 0
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// releaseOn is release's call into Java.
func (h *handoff) releaseOn(env jni.Env) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.held.Store(false)
	if h.lock == 0 {
		return nil
	}

	return env.CallVoid(h.lock, ids.wakeRelease)
}
