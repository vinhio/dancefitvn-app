// Adapted from jplayer-tv/internal/media/avf/doc.go at 781bee5.

// Package avftv is the Apple TV playback engine: a media.Engine over
// AVFoundation's AVPlayer, called through gDesk's foundation/ffi/objc,
// without cgo (Q-P0-4: a CMTime goes through an objc.Invocation).
//
// One AVPlayer lives as long as the Engine; every Load makes a new
// AVPlayerItem and hands it to replaceCurrentItemWithPlayerItem:. The
// engine learns of changes by key-value observing (the player's
// timeControlStatus and status, each item's status) and by the item's
// notifications (played to the end, failed to, stalled, new error log
// entry).
//
// Threading: every method runs on the UI thread. AVFoundation's callbacks
// come on any thread, and key-value observing fires synchronously inside
// the engine's own sends (replaceCurrentItemWithPlayerItem:, play, pause),
// so a callback only sets atomic bits and posts one prebuilt function
// through Config.Post; that function reads the player's current state on
// the UI thread and publishes what changed ("notify, then read"). A
// 250 ms tick tells the position while the media plays (allocating
// nothing), and Seek, SetRate (the player's defaultRate, which play uses,
// and its rate while it plays) and the end of the item are those of
// jplayer-mobile's engine (docs/plan/04-media-engine.md §1). The one
// exception to the UI thread is the audio session's first activation,
// which blocks for about 100 ms: New starts it on a goroutine of its own
// and takes the result back through Config.Post (warmSession), so that
// the first Load does not pay it on the UI thread (R-P4-6).
//
// Picture: the native plane B (D4): an AVPlayerLayer under the view, put
// there through Config.Underlay (gDesk's underlays), which the system
// composites where the view leaves itself transparent; Place puts it at
// the frame of the widget showing it. A mirrored picture (04 §2.3) is
// the layer's transform (transform.scale.x -1). Video returns nil: the
// view draws no frame. The first frame of a load on the screen is told
// once, as an EventVideoSize carrying the item's presentationSize: the
// engine observes the layer's readyForDisplay, which turns false at every
// load with a picture since the layer gets the player afresh (P7.5a); the
// app hides its stage over the picture on it (K5). No Place, no layer, no
// such event.
//
// Lifecycle: the engine is a media.Backgrounder. In the background its
// tick stops and a video pauses, telling Paused (tvOS stops a
// backgrounded video by itself without a change the engine could read,
// Q-P4.1-2); a sound plays on under the audio background mode.
//
// Errors never carry a URL or a localized description, which can echo the
// URL: only NSError domains and codes (see streamError).
//
// The engine exists only with the tvos build tag (ios && tvos); elsewhere
// New fails with core.ErrUnsupported, so the app compiles and its tests
// run on a Mac.
package avftv
