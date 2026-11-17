// Package kittest is the rig the widget tests of internal/ui share: the two
// phones every screen is tested on, and ways to mount a widget on one of
// them, with a fake text shaper for behavior or with the real text engine of
// the phone for sizes.
//
// It holds no assertions: the helpers stop a test with t.Fatal, so that this
// non-test package needs no testify.
package kittest
