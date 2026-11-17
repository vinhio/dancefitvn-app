// Adapted from jplayer-mobile/internal/media/avf/doc.go at 49de3c2.

// Package avf is the iPhone playback engine on AVFoundation
// (docs/plan/04-media-engine.md §2): New makes the media.Engine (one
// AVPlayer, an item per Load, key-value observing and notifications for
// its state and its end, a 250 ms time tick through objc.Invocation),
// NewSession the media.Session (AVAudioSession, category playback). Every
// item keeps its pitch at any speed (audioTimePitchAlgorithm TimeDomain,
// 04 §2.2), and Play plays at the speed chosen (the player's defaultRate).
// A video's frames are pulled by the UI (media.Surface.Pull) through one
// small cgo shim, the only cgo of the package (CLAUDE.md §5); when frames
// start coming (playing, or settling after a seek or a pause) the engine
// marks the surface Active and notifies it, so a view starts its pulls
// with nothing else on screen to repaint it. A mirrored picture is the
// view's business (04 §2.3): the engine knows nothing of it. In the
// background (SetBackground) the item's video tracks are disabled, so the
// player decodes the sound only. The session drops an interruption that
// only tells the app was suspended, and follows a reset of the media
// services: it sets its category again and publishes media.MediaReset,
// and the engine makes a new AVPlayer at the Load that follows. It builds
// only for iOS (not tvOS: internal/media/avftv) with cgo; elsewhere every
// constructor reports core.ErrUnsupported.
package avf
