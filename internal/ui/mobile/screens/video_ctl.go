//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/np/taps.go and vui.go at 49de3c2 (the
// layer that fades with `ui.Animated[float32]`, the hide timer made once, a press
// that is a click only inside the slop, a press on the spot of a hidden button
// taken by the area under it, a pause that shows the layer, a Loading that
// waits). The rules are the prototype's, not jPlayer's: 3000 ms, no double tap,
// no ripple (docs/plan/03 section 5a, index.html:554-557, :585-587, :606, :785).

package screens

import (
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the auto-hide of screen 06.
const (
	// ctlHideAfter is `CTL_HIDE_MS=3000` (index.html:554): the controls hide this
	// long after the last touch while the video plays.
	ctlHideAfter = 3000 * time.Millisecond
	// ctlFade is `.vui{transition:opacity .25s}` (index.html:146): not scaled by
	// the motion setting of the page, as the prototype has none.
	ctlFade = 250 * time.Millisecond
	// tapSlop is how far a finger may wander and still make a click: the 8 DIPs of
	// G.SLOP (jPlayer Mobile gestures.js:8), the browser's own click slop being of
	// the same order.
	tapSlop float32 = 8
)

// autoHide is the state of S.m.ctlHid and S.m.ctlT (index.html:554-557) with the
// fade of the layer they drive. The controls are shown unless the video plays,
// no sheet is open and the user (or the 3 s) hid them: a pause, a sheet or a load
// shows them and nothing counts then.
//
// The owner calls Reset on each action of the screen but the tap on the stage
// and the scrub, StageTap for the stage, State for each change of state of the
// player, Sheet when a sheet opens or closes, and reads Shown and Opacity.
type autoHide struct {
	// w is the widget that owns the fade: it repaints while the layer fades.
	w ui.Widget
	// pb says what plays.
	pb func() player.State
	// video tells whether the media is a video (a sound has nothing to hide).
	video func() bool

	ctx   *ui.Context // the view's, from Mounted: the timers need it
	hid   bool        // S.m.ctlHid: hidden by a tap or by the 3 s
	sheet bool        // S.m.sheet: a sheet the screen opened itself is open
	// ext tells whether any sheet is open, the navigator's (F-14): the download
	// sheet of the side button is opened by the shell, not by the screen; nil:
	// only the screen's own sheet counts.
	ext   func() bool
	wasUp bool // a sheet was up at the last paint
	told  bool // Shown at the last time a screen reader was told (tell)
	fade  ui.Animated[float32]
	timer ui.Timer
	fn    func()
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newAutoHide returns the controller of w. The layer starts shown, with no fade.
func newAutoHide(w ui.Widget, state func() player.State, video func() bool) *autoHide {
	a := &autoHide{w: w, pb: state, video: video}
	a.fn = a.fire
	a.fade.Duration, a.fade.Ease = ctlFade, theme.CSSEase
	a.fade.Set(1)
	a.told = true

	return a
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// mount keeps the view's context for the timer; unmount stops it.
func (a *autoHide) mount(c *ui.Context) { a.ctx = c }

func (a *autoHide) unmount() {
	a.timer.Stop()
	a.ctx = nil
}

// Shown tells whether the controls are shown now: `!ctlHidden()` (index.html:555).
func (a *autoHide) Shown() bool {
	return !(a.hid && a.pb() == player.StatePlaying && a.video() && !a.sheetUp())
}

// Opacity is the layer's opacity at this paint: it follows Shown with the
// transition of 0.25 s. The first call puts it at the target at once.
func (a *autoHide) Opacity() float32 {
	if up := a.sheetUp(); a.wasUp != up {
		a.wasUp = up
		switch {
		case up:
			a.timer.Stop() // a sheet the shell opened: nothing counts while it is open
		case !a.timer.Pending(): // it closed: `closeSheet` starts the 3 s again
			a.hid = false
			a.arm()
		default: // the screen's own sheet closed and started them already
		}
		a.tell()
	}
	target := float32(0)
	if a.Shown() {
		target = 1
	}

	return a.fade.Track(a.w, target)
}

// Reset is `ctlShow()` (index.html:556): the controls show and the 3 s start
// again, if the video plays.
func (a *autoHide) Reset() {
	a.hid = false
	a.arm()
	a.w.Node().Repaint()
	a.tell()
}

// StageTap is `stageTap` (index.html:785): a tap on the picture shows the
// controls when the video does not play; while it plays the tap toggles them and
// the 3 s start again.
func (a *autoHide) StageTap() {
	if a.pb() != player.StatePlaying {
		a.Reset()

		return
	}
	a.hid = !a.hid
	a.arm()
	a.w.Node().Repaint()
	a.tell()
}

// State follows the player's state. Anything but Playing shows the controls and
// leaves nothing to count (the pause of the prototype, a load, an end, an error);
// Playing starts the 3 s: the video that took longer to load than the wait, or
// that resumed from the lock screen or a headset, would otherwise leave the
// controls over the picture for good (Q-P4-5, jPlayer VIDEO.H).
func (a *autoHide) State(s player.State) {
	if s != player.StatePlaying {
		a.hid = false
	}
	a.arm()
	a.w.Node().Repaint()
	a.tell()
}

// Sheet tells that a sheet opened or closed: open shows the controls and stops
// the count; closed starts it again (`closeSheet` is a data-act, index.html:587).
func (a *autoHide) Sheet(open bool) {
	a.sheet = open
	if open {
		a.hid = false
		a.arm()
	} else {
		a.start(false) // the navigator may still call the closing sheet open: it is closing
	}
	a.w.Node().Repaint()
	a.tell()
}

// sheetUp tells whether a sheet is open: the screen's own, or any the navigator
// holds (F-14).
func (a *autoHide) sheetUp() bool { return a.sheet || (a.ext != nil && a.ext()) }

// tell tells a screen reader that the controls came or went, when Shown changed
// since it was last told: they are out of its reading from that moment, not after
// the fade (kit.Hush). It does nothing before the screen is mounted or while no
// screen reader reads, and allocates nothing.
func (a *autoHide) tell() {
	if s := a.Shown(); s != a.told {
		a.told = s
		if a.ctx != nil {
			a.ctx.SemanticsChanged(platform.ChangeLayout)
		}
	}
}

// Pending tells whether the 3 s are running.
func (a *autoHide) Pending() bool { return a.timer.Pending() }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// arm starts the 3 s again, replacing the count running, when the controls are
// the kind that hide: a video that plays or loads, with no sheet open. The timer
// needs the view's context: unmounted, there is none to wait on.
func (a *autoHide) arm() { a.start(a.sheetUp()) }

// start is arm with the answer to "is a sheet open" given: Sheet(false) says no
// while the navigator still counts the sheet that is closing.
func (a *autoHide) start(sheetOpen bool) {
	a.timer.Stop()
	if a.ctx == nil || sheetOpen || !a.video() {
		return
	}
	if s := a.pb(); s != player.StatePlaying && s != player.StateLoading {
		return
	}
	a.timer = a.ctx.AfterFunc(ctlHideAfter, a.fn)
}

// fire is the 3 s: `ctlHid=true` when the video plays (index.html:785); still
// loading it waits another 3 s, one timer at a time, instead of giving up.
func (a *autoHide) fire() {
	switch a.pb() {
	case player.StatePlaying:
		if a.video() && !a.sheetUp() {
			a.hid = true
			a.w.Node().Repaint()
			a.tell()
		}
	case player.StateLoading:
		a.arm()
	case player.StateUnknown, player.StateIdle, player.StatePaused, player.StateEnded, player.StateError:
	default:
	}
}

// tapArea is the press of a stage that may become a click: a press down, where
// it began, and whether it is still a candidate (the finger has not wandered).
type tapArea struct {
	armed bool
	down  geom.Point
}

// begin starts a press at pos.
func (t *tapArea) begin(pos geom.Point) { t.armed, t.down = true, pos }

// handle follows a press; click is called for a release that has not wandered
// and is inside the widget (the `click` event of the prototype).
func (t *tapArea) handle(e *ui.PointerEvent, click func()) {
	switch e.Kind {
	case ui.PointerDown:
		if isPrimary(e) {
			t.begin(e.Pos)
			e.Handle()
		}
	case ui.PointerMove:
		if t.armed && dist(e.Pos, t.down) > tapSlop {
			t.armed = false
		}
		if t.armed {
			e.Handle()
		}
	case ui.PointerUp:
		was := t.armed
		t.armed = false
		e.Handle()
		if was && e.Inside && dist(e.Pos, t.down) <= tapSlop {
			click()
		}
	case ui.PointerCancel:
		t.armed = false
	case ui.PointerUnknown, ui.PointerEnter, ui.PointerLeave, ui.PointerScroll:
	default:
	}
}

// isPrimary tells whether the press is of the primary button or a touch.
func isPrimary(e *ui.PointerEvent) bool {
	return e.Button == platform.ButtonPrimary || e.Button == platform.ButtonNone
}

// dist is how far apart two points are, as the larger of the two distances.
func dist(a, b geom.Point) float32 {
	dx, dy := a.X-b.X, a.Y-b.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}

	return max(dx, dy)
}
