//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/button.go at 781bee5.

package focus

import (
	"time"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Dir is a direction of focus movement: the arrows of tvKey (index.html:761-766).
type Dir int

// The directions. DirUnknown is the zero value.
const (
	DirUnknown Dir = iota
	DirLeft
	DirRight
	DirUp
	DirDown
)

// Button is a button of the Siri Remote, as the keydown listener of the
// prototype maps the keyboard to it (index.html:767). Home and Siri are not
// buttons here: the system owns them.
type Button int

// The buttons. ButtonUnknown is the zero value.
const (
	ButtonUnknown Button = iota
	ButtonLeft
	ButtonRight
	ButtonUp
	ButtonDown
	ButtonSelect
	ButtonMenu
	ButtonPlayPause
)

// Timings of the prototype.
const (
	// HoldDuration is how long Select must be held to count as a hold
	// (PLAN D8, 03 section 7.1, K27: the prototype acts on keydown and has no
	// hold; a hold fires once at 550 ms, a short press on release).
	HoldDuration = 550 * time.Millisecond
	// PressedPulse is how long an item shows .pressed after Select (the
	// prototype's TV has no pressed look, so tvkit items ignore it). The item
	// owns the animation.
	PressedPulse = 140 * time.Millisecond
)

// Item is a focusable control: an element with [data-f].
type Item interface {
	ui.Widget
	// FocusKey is the stable key that finds the item again after its screen
	// is rebuilt: data-fk.
	FocusKey() string
	// FocusDefault tells that Initial prefers the item when the screen has no
	// remembered key: [data-default].
	FocusDefault() bool
}

// Activator is an Item that does something on Select. Long is true for a hold.
type Activator interface {
	// Activate runs the item's action. For a hold it returns false when the
	// item has nothing to run for a hold (no _hold, no data-hold), and the
	// Manager then activates it as a short press.
	Activate(long bool) bool
}

// Presser is an Item that shows the press: SetPressing is .pressing while
// Select is down, Pulse is .pressed for PressedPulse.
type Presser interface {
	SetPressing(on bool)
	Pulse()
}

// ChromeKind tells which fixed bar a Chrome is.
type ChromeKind int

// The kinds of chrome. ChromeUnknown is the zero value.
const (
	ChromeUnknown ChromeKind = iota
	ChromeSidebar
	ChromePlaybar
)

// Chrome is the navigation bar (ChromeSidebar names it here, the `.tnav` of
// every root screen): items inside it are left out of Initial's default pick,
// as renderTV does (`!fid.startsWith('nav')`, index.html:723), unless the root
// asked for is the bar itself or a key names the item (the settings screen
// takes `nav0`).
type Chrome interface {
	FocusChrome() ChromeKind
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// ButtonOf maps a key to the remote button it stands for: the arrows; Enter;
// Escape and Backspace (Menu); Space, P and MediaPlayPause (Play/Pause)
// (tvKey and its listener, index.html:761-767). Other keys map to
// ButtonUnknown.
func ButtonOf(k keys.Key) Button {
	switch k {
	case keys.KeyLeft:
		return ButtonLeft
	case keys.KeyRight:
		return ButtonRight
	case keys.KeyUp:
		return ButtonUp
	case keys.KeyDown:
		return ButtonDown
	case keys.KeyEnter:
		return ButtonSelect
	case keys.KeyEscape, keys.KeyBackspace:
		return ButtonMenu
	case keys.KeySpace, keys.KeyP, keys.KeyMediaPlayPause:
		return ButtonPlayPause
	default:
		return ButtonUnknown
	}
}

// Dir returns the direction of an arrow button, DirUnknown for the others.
func (b Button) Dir() Dir {
	switch b {
	case ButtonLeft:
		return DirLeft
	case ButtonRight:
		return DirRight
	case ButtonUp:
		return DirUp
	case ButtonDown:
		return DirDown
	default:
		return DirUnknown
	}
}

// String returns the prototype's name of the direction.
func (d Dir) String() string {
	switch d {
	case DirLeft:
		return "left"
	case DirRight:
		return "right"
	case DirUp:
		return "up"
	case DirDown:
		return "down"
	default:
		return "unknown"
	}
}

// String returns the prototype's name of the button (the keys of tvKey).
func (b Button) String() string {
	switch b {
	case ButtonLeft:
		return "ArrowLeft"
	case ButtonRight:
		return "ArrowRight"
	case ButtonUp:
		return "ArrowUp"
	case ButtonDown:
		return "ArrowDown"
	case ButtonSelect:
		return "select"
	case ButtonMenu:
		return "menu"
	case ButtonPlayPause:
		return "playpause"
	default:
		return "unknown"
	}
}
