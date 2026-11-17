// Adapted from jplayer-mobile/internal/ui/style/ease.go at 49de3c2.

package theme

import (
	"time"

	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The prototype's motion, each with the selector and line that sets it
// (reference/prototype/index.html).
const (
	// SheetIn is `.sheet{animation:su .25s ease-out}` (:125, :129).
	SheetIn = 250 * time.Millisecond
	// FadeIn is `.dim-bg{animation:fi .2s}` and the toast's (:124, :130, :131):
	// CSS `ease`.
	FadeIn = 200 * time.Millisecond
	// SwitchDur is `.sw{transition:.2s}` and its knob's (:118-119): CSS `ease`.
	SwitchDur = 200 * time.Millisecond
	// ToastTime is how long a toast stays, `setTimeout(..., 2200)` (:552).
	ToastTime = 2200 * time.Millisecond
)

// PressScale is `.btn:active{transform:scale(.98)}` (:63): no transition, so
// the scale applies at once.
const PressScale float32 = .98

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// CSSEase is the CSS keyword `ease`, cubic-bezier(.25, .1, .25, 1): the timing
// function of every animation and transition the prototype gives no other
// (`animation:fi .2s`, `transition:.2s`). ui.EaseOut is CSS `ease-out`
// (cubic-bezier(0, 0, .58, 1)), which `.sheet` names. Built once: ui.Bezier
// allocates its table.
var CSSEase = ui.Bezier(.25, .1, .25, 1)
