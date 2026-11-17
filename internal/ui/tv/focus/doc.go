//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/doc.go at 781bee5.

// Package focus is the Apple TV app's focus layer over gDesk: the semantics of
// the prototype's focus engine (reference/prototype/index.html, moveFocus,
// renderTV, tvKey) that are not geometry. A Manager picks the initial item,
// heals focus by key when the tree is rebuilt (the prototype has no per-screen
// focus memory, PLAN D8 and Q-P7-2), runs the Pre hook and the Menu and
// Play/Pause handler chains, and turns Select into a press (on release) or a
// 550 ms hold. A Root maps the Siri Remote's keys to Buttons.
//
// The geometry (which item is next in a direction, scrolling the focused item
// into view, the candidate list of a scope) is gDesk's; the Manager reaches it
// through the small Engine interface, so this package does not depend on how
// gDesk implements it. internal/ui/focus/view.go adapts a *ui.View.
//
// Everything runs on the UI thread. A key press, a focus move and the
// one-second tick allocate nothing.
package focus
