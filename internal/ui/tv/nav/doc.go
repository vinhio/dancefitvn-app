//go:build tvos

// Package nav is the navigation state of the Apple TV shell: the stack of
// screens of tvGo and tvBack (reference/prototype/index.html:730-731) and the
// panel flag of the player. It has no widgets; the Shell builds the screens of
// the routes and puts the focus where the prototype's renderTV does.
//
// tvGo pushes the screen showing and opens the new one, a root clears the
// stack (the four nav items, the first screen, the screen after a login or a
// logout); tvBack closes the panel first, then pops, and does nothing when the
// stack is empty (Menu is then left to the system: the Apple TV's Home).
package nav
