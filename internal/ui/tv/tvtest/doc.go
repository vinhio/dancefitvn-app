//go:build tvos

// Package tvtest is the rig of the TV screens' tests: a Tester over a
// 1920x1080 view made as the app makes it (theme, directional focus,
// transparent background, the safe area of the Apple TV), and fakes of the
// services (Services, Pairing) that tests drive by hand. It is imported by
// tests only.
package tvtest
