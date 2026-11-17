// Package tvengine holds the device tests of the Apple TV media engine
// (internal/media/avftv, docs/plan/04-media-engine.md §1–§2): test files
// only, run inside a real gDesk app on the Apple TV simulator
// (scripts/tvos-avf-test.sh e2e), the picture on the native plane under a
// transparent view, playing the mock's media served on 127.0.0.1 by
// internal/api/mock. On the Mac the tests skip.
package tvengine
