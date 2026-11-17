// Package gesture holds the touch gestures of the app as logic that draws
// nothing: the widgets that show them (kit.Sheet, the navigation's edge swipe)
// own the pointer and the looks, and ask these types what the finger did.
//
// Every coordinate is a window coordinate (the event's Pos plus the widget's
// WindowFrame), so that a widget sliding under the finger does not distort the
// distance the finger travelled, and every time is Context.Now. Nothing here
// allocates after the first use: the types are plain structs a widget embeds.
//
// gDesk's finger-velocity tracker is unexported (framework/ui/velocity.go), so
// Track mirrors it. The pieces:
//
//   - Track: the axis lock after 8 DIPs and the velocity of the finger.
//   - Drag: a sheet pulled down: dismissed past 80 DIPs or faster than
//     0.6 DIP/ms (docs/plan/03 section 1).
//   - Edge: a swipe back from the left edge: a 26 DIP strip, committed past
//     80 DIPs or faster than 0.6 DIP/ms (Q-P2-4).
//   - Exempt: the widgets a sheet's drag leaves alone (text fields).
package gesture
