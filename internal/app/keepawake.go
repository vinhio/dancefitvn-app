// Adapted from jplayer-mobile/internal/app/keepawake.go at 49de3c2: the
// player is DanceFITVN's (Kind, Changes()), the sink sys.KeepAwake on the
// app's driver, and a missing sink (a test, a shot) keeps nothing on.

package app

import (
	"errors"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/sys"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// awakeSink keeps the screen on (sys.KeepAwake); the tests put a fake in
// its place.
type awakeSink interface {
	// Set keeps the screen on, or gives it back to the system's timer.
	Set(on bool) error
	// Reapply sets the last value again on the window the app has now.
	Reapply() error
}

// sysAwake is the awakeSink of the system, on the app's driver.
type sysAwake struct {
	driver func() platform.Driver
}

// keepAwake keeps the screen on while a video plays with the app in
// front (Q-P4-6): on when the player plays a video, off when it pauses,
// ends, fails, goes to an audio lesson, or when the app goes to the
// background. It asks the system only when that changes. It lives on the
// UI thread.
type keepAwake struct {
	sink   awakeSink
	p      *player.Player
	logf   func(string, ...any)
	hidden bool // the app is in the background
	on     bool // what the system was last told
	told   bool // the system was told once
	off    bool // the system cannot keep the screen on, or no sink
	unsub  func()
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newKeepAwake returns what keeps the screen on through sink while p
// plays a video, listening to p until close; a nil sink keeps nothing on.
func newKeepAwake(sink awakeSink, p *player.Player, logf func(string, ...any)) *keepAwake {
	k := &keepAwake{sink: sink, p: p, logf: logf, off: sink == nil}
	k.unsub = p.Changes().Subscribe(k.onChange)

	return k
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Set calls sys.KeepAwake.
func (s sysAwake) Set(on bool) error { return sys.KeepAwake(s.driver(), on) }

// Reapply calls sys.ReapplyKeepAwake.
func (s sysAwake) Reapply() error { return sys.ReapplyKeepAwake(s.driver()) }

// onChange follows the state and the lesson of the player; nothing else
// (a tick is not looked at).
func (k *keepAwake) onChange(c player.Change) {
	switch c.Kind {
	case player.ChangeState, player.ChangeLesson:
		k.update()
	case player.ChangeUnknown, player.ChangeTime, player.ChangeDuration, player.ChangeRate,
		player.ChangeOptions, player.ChangeSeek, player.ChangeEnded, player.ChangeError:
	default:
	}
}

// lifecycle follows the app's standing: in the background the screen is
// let go; active again, it is kept as the player asks, and the value set
// again on the window the system may have made since (an Android
// activity created again).
func (k *keepAwake) lifecycle(state platform.LifecycleState) {
	switch state {
	case platform.LifecycleBackground:
		k.hidden = true
		k.update()
	case platform.LifecycleActive:
		k.hidden = false
		k.update()
		if k.off {
			return
		}
		if err := k.sink.Reapply(); err != nil {
			k.failed(err)
		}
	case platform.LifecycleUnknown, platform.LifecycleInactive:
	default:
	}
}

// close stops listening to the player and lets the screen go.
func (k *keepAwake) close() {
	if k.unsub != nil {
		k.unsub()
		k.unsub = nil
	}
	k.set(false)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// update tells the system what the player asks for now. A load keeps
// what was asked for its video (the one before), lets go for an audio.
func (k *keepAwake) update() {
	video := k.p.Kind() == player.KindVideo && !k.hidden
	on := false
	switch k.p.State() {
	case player.StatePlaying:
		on = video
	case player.StateLoading:
		on = video && k.on
	case player.StateUnknown, player.StateIdle, player.StatePaused, player.StateEnded, player.StateError:
	default:
	}
	k.set(on)
}

// set tells the system on, unless it was told so already. A failure (no
// window yet) leaves it to the next change.
func (k *keepAwake) set(on bool) {
	if k.off || (k.told && k.on == on) {
		return
	}
	if err := k.sink.Set(on); err != nil {
		k.failed(err)

		return
	}
	k.on, k.told = on, true
}

// failed logs err; a system that cannot keep the screen on turns this off.
func (k *keepAwake) failed(err error) {
	if errors.Is(err, core.ErrUnsupported) {
		k.off = true

		return
	}
	k.logf("dancefitvn: keep awake: %v", err)
}
