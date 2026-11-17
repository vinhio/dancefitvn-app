// Adapted from jplayer-mobile/e2e/engine/doc.go at 49de3c2.

// Package engine holds the device tests of the phones' media engines
// (internal/media/avf on the iPhone, internal/media/amp on Android,
// docs/plan/04-media-engine.md §2, §3): test files only, run inside a real
// gDesk app on the iOS simulator (e2e/devtest-ios.sh) or an Android device
// (e2e/devtest-android.sh), playing the mock's media served on 127.0.0.1
// by internal/api/mock. On the Mac the tests skip. See e2e/README.md.
package engine
