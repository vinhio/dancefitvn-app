//go:build tvos

// Package shell is the root of the Apple TV app: a focus.Root over a stage that
// holds the screen of the route showing and the toast (docs/plan/P7 section 1).
// It carries out the actions of docs/plan/03 section 7.3 for the screens
// (env.Actions): tvGo and tvBack with their stack, the panel, the toast, the
// Menu and Play/Pause chains, and puts the focus where the prototype's renderTV
// does after each change of screen (index.html:719-727).
//
// Every route has its screen (screens.New*); a route nobody knows is the welcome.
//
// The focus of a new screen is given by the next Layout, once the screen's boxes
// exist (settle). Go, Replace, Back, SwitchTab and the panel's open and close each
// end in Relayout, which asks the view for a frame (or finds one already pending),
// so a route change made by an event with no frame after it, such as the network
// flip that shows TV11 or the end of a video, has its focus as soon as the display
// delivers its next frame. Nothing runs between the call and that frame: tests that
// run only the frames asked for (ui.Tester.Frame) see it (handoff_test.go). Before
// the view exists (the first route, shown before the window) the mount asks for the
// first frame.
//
// A screen that reads its data shows a waitAnchor with the key of its default
// control meanwhile (K12); the focus is on it, drawn as nothing, and goes to the
// control when the data comes, unless the user moved it to the nav bar.
package shell
