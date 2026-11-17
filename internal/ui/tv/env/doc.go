//go:build tvos

// Package env is the contract between the Apple TV screens and the app: the
// interface of what the screens ask of the app (Services, Playback, Prefs,
// Pairing), the actions of docs/plan/03 section 7.3 the Shell carries out
// (Actions), and the Env that holds them with the texts and the navigation
// state. It has no widgets, and imports no store, api client or auth manager:
// internal/app adapts those to Services (tvWire), and the tests give the
// screens a fake (tvtest).
package env
