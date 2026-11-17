// Adapted from jplayer-tv/internal/media/avf/state.go at 781bee5.

package avftv

import "dancefitvn-app/internal/media"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// The status values AVFoundation reports, as its headers number them.
const (
	itemUnknown = 0 // AVPlayerItemStatusUnknown, AVPlayerItem.h:46
	itemReady   = 1 // AVPlayerItemStatusReadyToPlay, AVPlayerItem.h:48
	itemFailed  = 2 // AVPlayerItemStatusFailed, AVPlayerItem.h:50

	playerFailed = 2 // AVPlayerStatusFailed, AVPlayer.h:37

	controlPaused  = 0 // AVPlayerTimeControlStatusPaused, AVPlayer.h:190
	controlWaiting = 1 // AVPlayerTimeControlStatusWaitingToPlayAtSpecifiedRate, AVPlayer.h:192
	controlPlaying = 2 // AVPlayerTimeControlStatusPlaying, AVPlayer.h:194
)

// reading is what the engine reads of AVFoundation on the UI thread when a
// callback asked it to look.
type reading struct {
	item        int  // the current item's status
	player      int  // the player's status
	control     int  // the player's timeControlStatus
	failedToEnd bool // AVPlayerItemFailedToPlayToEndTimeNotification came
}

// machine turns readings into the events of the prototype's media element
// (player.js:13-16): playing → Playing, waiting → Loading, pause → Paused,
// error → Error. It emits only changes. Pure Go: it never touches
// AVFoundation, so it is tested on a Mac.
type machine struct {
	loaded      bool // an item is loaded
	playAsked   bool // Play was called since the last Load
	paused      bool // Pause was called since the last Play
	failedToEnd bool // the item failed to play to its end; sticky until Load or Stop
	last        media.PlayState
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// stateOf is the state a reading shows, StateUnknown for "nothing to say":
// nothing loaded, nothing asked to play yet (the prototype's element is
// silent until play), or a pause the reading does not show yet. A failure
// shows from the moment the item is loaded. Unlike the simulated engine, a
// pause while connecting stays paused (the sim quirk is the sim's alone).
func stateOf(r reading, loaded, playAsked, paused bool) media.PlayState {
	switch {
	case !loaded:
		return media.StateUnknown
	case r.item == itemFailed || r.player == playerFailed || r.failedToEnd:
		return media.StateError
	case !playAsked:
		return media.StateUnknown
	case paused:
		if r.control == controlPaused {
			return media.StatePaused
		}

		return media.StateUnknown
	case r.control == controlPlaying:
		return media.StatePlaying
	case r.control == controlWaiting:
		return media.StateLoading
	default:
		return media.StateUnknown
	}
}

// load starts a new item: nothing emitted yet, nothing asked.
func (m *machine) load() { *m = machine{loaded: true} }

// stop forgets the item: every later reading is silent.
func (m *machine) stop() { *m = machine{} }

// play records that Play was called.
func (m *machine) play() { m.playAsked, m.paused = true, false }

// pause records that Pause was called.
func (m *machine) pause() { m.paused = true }

// step reads r and returns the state to emit and true when it differs
// from the last one emitted. It allocates nothing.
func (m *machine) step(r reading) (media.PlayState, bool) {
	if r.failedToEnd && m.loaded {
		m.failedToEnd = true
	}
	r.failedToEnd = m.failedToEnd
	s := stateOf(r, m.loaded, m.playAsked, m.paused)
	if s == media.StateUnknown || s == m.last {
		return s, false
	}
	m.last = s

	return s, true
}

// stateName names s for the log.
func stateName(s media.PlayState) string { //nolint:unused // used by tvos files (engine_tvos.go)
	switch s {
	case media.StateLoading:
		return "loading"
	case media.StatePlaying:
		return "playing"
	case media.StatePaused:
		return "paused"
	case media.StateEnded:
		return "ended"
	case media.StateError:
		return "error"
	case media.StateUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}
