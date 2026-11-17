// Adapted from jplayer-mobile/internal/sys/keepawake.go at 49de3c2. The
// iPhone side goes through gDesk's power.KeepAwake (uikit's PowerDriver)
// instead of setting the idle timer itself; Android keeps jPlayer's
// FLAG_KEEP_SCREEN_ON while gDesk has no PowerDriver there (fallback F8 of
// docs/plan/06-gdesk-extensions.md, proposal G9).

package sys

import (
	"fmt"
	"sync/atomic"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// awake is what the last KeepAwake asked for, kept for ReapplyKeepAwake:
// the screen kept on or left to the system's sleep timer. Written on the
// UI thread, read there and on Android's main thread.
var awake atomic.Bool

// keepAwakeReason is what the system may show of why the screen stays on.
const keepAwakeReason = "Video playing" //nolint:unused // used by ios files (keepawake_ios.go)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// KeepAwake keeps the screen on while on is set (a video playing with the
// app in front, Q-P4-6), and gives it back to the system's sleep timer
// once it is not. It is idempotent: asking again for what holds does
// nothing. The value is remembered, so that ReapplyKeepAwake sets it
// again on a window the system made since.
//
// On iOS it holds gDesk's power.KeepAwake on d (UIApplication's idle
// timer, which lasts for the process) and releases it. On Android it
// queues, on Java's main thread, the activity's Window.addFlags or
// clearFlags of FLAG_KEEP_SCREEN_ON, which last for that activity's
// window only: while the app has no activity, the value waits for
// ReapplyKeepAwake.
//
// Call it on the UI thread. d is the app's driver. Elsewhere than iOS and
// Android it remembers on and fails with core.ErrUnsupported.
func KeepAwake(d platform.Driver, on bool) error {
	if d == nil {
		return fmt.Errorf("sys: keep awake: no driver: %w", core.ErrInvalid)
	}
	awake.Store(on)

	return keepAwake(d, on, false)
}

// ReapplyKeepAwake sets again what the last KeepAwake asked for, on the
// window the app has now: Android makes a new activity, whose window
// lacks the flag, after the old one was destroyed. The app calls it each
// time it becomes active. Call it on the UI thread. Elsewhere than iOS
// and Android it fails with core.ErrUnsupported.
func ReapplyKeepAwake(d platform.Driver) error {
	if d == nil {
		return fmt.Errorf("sys: keep awake: no driver: %w", core.ErrInvalid)
	}

	return keepAwake(d, awake.Load(), true)
}

// KeepingAwake reports what the last KeepAwake asked for: whether the
// screen is meant to stay on.
func KeepingAwake() bool { return awake.Load() }
