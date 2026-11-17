// Adapted from jplayer-mobile/internal/media/media.go at 49de3c2.

package media

import (
	"context"
	"dancefitvn-app/internal/clock"
	"fmt"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/event"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Kind tells audio from video.
type Kind uint8

// The kinds of media.
const (
	KindUnknown Kind = iota
	KindAudio
	KindVideo
)

// PlayState is what the engine is doing with the loaded media.
type PlayState uint8

// The states of an engine.
const (
	StateUnknown PlayState = iota
	StateLoading
	StatePlaying
	StatePaused
	StateEnded
	StateError
)

// EventKind tells which fields of an Event hold news.
type EventKind uint8

// The kinds of Event.
const (
	EventUnknown EventKind = iota
	// EventTime reports the position, Time.
	EventTime
	// EventDuration reports the length, Duration.
	EventDuration
	// EventBuffered reports how far the media is loaded, Buffered.
	EventBuffered
	// EventState reports a change of State.
	EventState
	// EventEnded reports that the media played to its end.
	EventEnded
	// EventError reports a failure, Err.
	EventError
	// EventVideoSize reports the picture size, Width and Height. The
	// engines that draw frames into a Surface (avf, amp) tell it when the
	// size is known or changes, their frames telling what is shown; the
	// Apple TV's engine (avftv), whose picture is a system layer the view
	// cannot see, tells it once a load when the first frame is on the
	// screen, so that the app hides what covers the picture until then
	// (K5's stage).
	EventVideoSize
)

// Source is what to play.
type Source struct {
	// URL is an http(s) address or the path of a file.
	URL string
	// AudioOnly says the media has no picture to show.
	AudioOnly bool
	// Live says the media has no end, so no seeking and no duration.
	Live bool
	// Analyse asks the engine to hear the sound for the visualizer (DM9):
	// an audio media played from a file (player.js:70-71). Engines without
	// an analyser ignore it.
	Analyse bool
	// BackgroundPeak is the most bits per second the engine may fetch while the
	// app is in the background; 0 is no limit. Only an adaptive (HLS) stream
	// that is not live uses it.
	BackgroundPeak int
}

// FreqBins is the frequencyBinCount of the prototype's analyser: its
// fftSize, 256, halved (player.js:281).
const FreqBins = 128

// LoadOptions tells how Load starts.
type LoadOptions struct {
	// Auto starts playing once loaded.
	Auto bool
	// Start is the position to begin at, in seconds.
	Start float64
}

// Event is what an engine reports. Events are published on the UI thread.
type Event struct {
	Kind EventKind
	// Token is the one Load returned; the engine drops events of older
	// loads, and a listener may check it as well.
	Token                    uint64
	Time, Duration, Buffered float64
	State                    PlayState
	// Err wraps one of the sentinels below.
	Err           error
	Width, Height int
}

// Errors an engine reports, each wrapping a core sentinel.
var (
	// ErrAborted is a load given up on, as the next Load does to the last.
	ErrAborted = fmt.Errorf("media: loading aborted: %w", core.ErrClosed)
	// ErrNetwork is a network failure while loading or playing.
	ErrNetwork = fmt.Errorf("media: network error: %w", core.ErrNotFound)
	// ErrDecode is media the system cannot decode.
	ErrDecode = fmt.Errorf("media: cannot decode: %w", core.ErrUnsupported)
	// ErrSource is a source that cannot be opened.
	ErrSource = fmt.Errorf("media: cannot open the source: %w", core.ErrInvalid)
	// ErrMissingFile is a file that is no longer on the device.
	ErrMissingFile = fmt.Errorf("media: file missing: %w", core.ErrNotFound)
	// ErrUnreachable is a linked file that cannot be reached now (a drive
	// out, a revoked grant, a provider offline) but may be later.
	ErrUnreachable = fmt.Errorf("media: file unreachable: %w", core.ErrNotFound)
)

// Config is what an engine is made with.
type Config struct {
	// Dispatch runs fn on the UI thread (the app passes app.Enqueue). Hot
	// callers pass a func value made once, so a call allocates nothing.
	Dispatch func(func())
	// Clock drives the engine's 250 ms time tick.
	Clock clock.Clock
}

// Engine plays one media at a time.
type Engine interface {
	// Load starts loading src and returns the token that tags its events;
	// a later Load makes the earlier one's events stale.
	Load(ctx context.Context, src Source, opts LoadOptions) (token uint64, err error)
	// Play starts or resumes playing.
	Play() error
	// Pause pauses, keeping the position.
	Pause() error
	// Stop stops and unloads.
	Stop() error
	// Seek moves to seconds.
	Seek(seconds float64) error
	// SetVolume sets the volume, 0 to 1.
	SetVolume(v float64) error
	// SetMuted mutes or unmutes.
	SetMuted(on bool) error
	// SetRate sets the playback speed, 1 being normal.
	SetRate(r float64) error
	// Events is the topic the engine publishes to, on the UI thread.
	Events() *event.Topic[Event]
	// Video returns the surface the picture of a video is drawn to.
	Video() *Surface
	// Close releases the engine.
	Close() error
}

// Backgrounder is an engine that follows the app's lifecycle (04 §6): in the
// background it stops its time tick and its frames; back in the foreground it
// tells the time and shows a frame at once. The app calls it, on the UI
// thread, when the engine implements it.
type Backgrounder interface {
	SetBackground(on bool)
}

// Rejoiner is a Backgrounder that lets the video of a live stream go for
// good in the background: it plays its sound only there, and cannot show
// its picture again (Android's MediaPlayer given its Surface back shows
// none on API 36, local/online/spike.md §5.1). Back in the foreground the
// app loads the stream again, which plays from its live edge with its
// video. UI thread.
type Rejoiner interface {
	// LiveStale reports whether the live stream loaded played without its
	// video in the background and must be loaded again to show it.
	LiveStale() bool
}

// Analyser is an engine that hears what it plays: the prototype's
// AnalyserNode on its audio element (player.js:276-283, D7). UI thread.
type Analyser interface {
	// Frequencies fills dst as getByteFrequencyData does, dt after the
	// last call (the smoothing is per 60 Hz frame); paused, loading, or
	// before the engine hears its first sound, the input is silence (the
	// prototype's analyser is real from the load on, player.js:70,
	// :302-303). It returns false, leaving dst alone, when the media
	// playing is heard by no analyser (not Source.Analyse, no audio track,
	// a tap that never gives anything): the visualizer then draws its
	// synthetic bars. It allocates nothing.
	Frequencies(dst *[FreqBins]uint8, dt time.Duration) bool
}

// TimeReader is an engine that reads its position when asked. In the
// background its time tick stops (04 §6) while the media plays on, so the
// time its events last told falls behind: the player reads the true one at
// the moments it acts on it (a change of state or speed, ‹, a skip, the
// resume position kept, Now Playing shown) rather than with a timer (F1 of
// M9). A read allocates nothing. Every engine of the app implements it.
type TimeReader interface {
	// ReadTime returns the position, in seconds, of the media of the load
	// token, read from the system now; false when token is not the current
	// load's or that load cannot tell yet (loading, failed, unloaded). A
	// seek under way tells the position it goes to. UI thread.
	ReadTime(token uint64) (float64, bool)
}

// SessionEventKind tells what happened to the audio session.
type SessionEventKind uint8

// The kinds of SessionEvent.
const (
	SessionEventUnknown SessionEventKind = iota
	// InterruptBegin is a call or another app taking the audio.
	InterruptBegin
	// InterruptEnd is the audio given back.
	InterruptEnd
	// Noisy is the headphones pulled out.
	Noisy
	// Duck asks to lower the volume for a short sound. Kept for systems that
	// ask; neither engine reports it (Android ducks by itself).
	Duck
	// Unduck ends a Duck. Kept as Duck is.
	Unduck
	// MediaReset is the system rebuilding its media services (iOS): the
	// engine's player is gone and the current media must be loaded again.
	MediaReset
)

// SessionEvent is a change of the audio session.
type SessionEvent struct {
	Kind SessionEventKind
	// ShouldResume, on InterruptEnd, says the system advises to play again.
	ShouldResume bool
}

// Session is the system's audio session.
type Session interface {
	// Activate makes the app the one playing.
	Activate() error
	// Events is the topic the session publishes to, on the UI thread.
	Events() *event.Topic[SessionEvent]
}
