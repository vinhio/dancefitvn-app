// Adapted from jplayer-mobile/internal/media/amp/engine_other.go at 49de3c2.

//go:build !android || !cgo

package amp

import (
	"dancefitvn-app/internal/media"
	"fmt"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// NoisyActions are the broadcasts the session takes for headphones
// pulled out; off Android there is none.
var NoisyActions []string

// Stats are an engine's counters, for the device tests and measures
// (StatsOf).
type Stats struct {
	// LiveGlobalRefs, LiveProxies and LiveReaders count the global
	// references, listener proxies and AImageReaders of the package not
	// yet released (every engine and session of the process).
	LiveGlobalRefs, LiveProxies, LiveReaders int
	// Frames counts the frames the readers handed over; CopiedFrames those
	// converted into the Surface, DroppedFrames those let go unconverted
	// (no view watching, or the app in the background), BadFrames those
	// that could not be read.
	Frames, CopiedFrames, DroppedFrames, BadFrames int64
	// ReaderCallbacks and ReaderFailures count the current reader's
	// callbacks and failed acquires.
	ReaderCallbacks, ReaderFailures int64
	// Releasing counts the loads the opener has still to open or the
	// releaser to free.
	Releasing int
	// WrongState counts the players' errors of a call in a wrong state
	// (what -38); Refused the calls the state machine kept from them.
	WrongState, Refused int
	// ConvertP50 and ConvertP95 are the times of the last conversions.
	ConvertP50, ConvertP95 time.Duration
	// Loads and Unloads are the times the last loads and unloads took on
	// the UI thread; Opens those the opener took to make their players.
	Loads, Unloads, Opens []time.Duration
	// OffMainEvents counts the players' events that came on another
	// thread than Java's main one: 0, as no player is made on a thread
	// with a Looper.
	OffMainEvents int64
	// NativeHeap is Debug.getNativeHeapAllocatedSize; RSS the resident
	// set of the process, in bytes.
	NativeHeap, RSS int64
	// WifiLock tells that the engine's Wi-Fi lock is held, as the system
	// answers (WifiLock.isHeld): while a link plays or prepares to.
	WifiLock bool
	// HandoffLock tells that the CPU is kept awake from a media's end to
	// the next's start (PowerManager.WakeLock.isHeld).
	HandoffLock bool
	// Detaches and Attaches count the Surfaces the background took from
	// the players of videos with sound and the foreground gave back;
	// Detached tells that the current player has none now.
	Detaches, Attaches int
	Detached           bool
	// Reseeks counts the seeks the engine made again after a Surface
	// given back, as the player went to a key frame on its own (Samsung's
	// NuPlayer).
	Reseeks int
	// HoldsImages tells that the video's Surface holds images.
	HoldsImages bool
}

// SessionStats are a session's counters (see the Android build).
type SessionStats struct {
	Requests int
	Held     bool
}

// LinkVerdict is what the app's check of a linked document tells (see
// the Android build).
type LinkVerdict uint8

// The verdicts of a check of a linked document.
const (
	LinkVerdictUnknown LinkVerdict = iota
	LinkFine
	LinkGone
	LinkAway
)

// errUnsupported is what every call reports off Android.
var errUnsupported = fmt.Errorf("amp: android.media.MediaPlayer needs Android with cgo: %w", core.ErrUnsupported)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// New reports core.ErrUnsupported: the engine needs Android.
func New(media.Config, media.Session) (media.Engine, error) { return nil, errUnsupported }

// NewSession reports core.ErrUnsupported: the session needs Android.
func NewSession(media.Config) (media.Session, error) { return nil, errUnsupported }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetLinkCheck has no engine to tell off Android.
func SetLinkCheck(func(url string) LinkVerdict) {}

// StatsOf has no engine to read off Android.
func StatsOf(media.Engine) (Stats, bool) { return Stats{}, false }

// SessionStatsOf has no session to read off Android.
func SessionStatsOf(media.Session) (SessionStats, bool) { return SessionStats{}, false }

// ConvertYUV420 reports core.ErrUnsupported: the conversion is the
// engine's C, on Android.
func ConvertYUV420(_ []byte, _, _, _, _, _ int, _ []byte, _ int, _, _ []byte, _, _ int, _ bool) (time.Duration, error) {
	return 0, errUnsupported
}

// PlaybackParams has no engine to read off Android. For device tests and
// the spike only.
func PlaybackParams(media.Engine) (speed, pitch float32, ok bool) { return 0, 0, false }

// Position has no engine to read off Android. For device tests only
// (testhooks_android.go).
func Position(media.Engine) (float64, bool) { return 0, false }
