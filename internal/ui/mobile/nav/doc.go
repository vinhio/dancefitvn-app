//go:build !tvos

// Package nav is the navigation of the phone shell: the stack of screens, the
// sheet and the toast over them, the tab bar of the four root screens, and the
// two ways back (the system's Back key and the swipe from the left edge).
//
// It follows the prototype's go() and back() (index.html:540-552): going to a
// screen pushes the current one, a root (a tab) clears the stack, the screen is
// rebuilt without a transition and at the top; back pops, or goes Home when
// nothing is left, and closes the sheet first (docs/plan/03 section 1). The
// sheet and the toast are in the tree of the Nav, over the screen and the tab
// bar, in the order of the prototype's z-index (tabs 30, sheet 70/71, toast 90).
//
// Both phones behave the same (PLAN D2): Android's Back key and iOS's edge swipe
// are one Back; where the system takes the swipe from the left edge itself
// (SystemGestures().Left > 0) it sends the key and the Nav leaves the edge
// alone. Nothing here asks which system it runs on.
//
// The screen frame (the 54 DIP top, the tab bar's height, the scroll's padding)
// is in frame.go, so that it changes in one place when a device measures
// otherwise (Q-P2-1).
package nav
