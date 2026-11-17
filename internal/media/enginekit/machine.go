// Adapted from jplayer-mobile/internal/media/enginekit/machine.go at 49de3c2.

package enginekit

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// MPState is a state of android.media.MediaPlayer. A call in a state that
// does not allow it throws no Java exception: the player logs the failure,
// reports error (-38, 0) and goes to the Error state (M0.5), where only
// reset and release are valid. The engine therefore asks Allows first.
type MPState uint8

// The states of a MediaPlayer (the "Valid and invalid states" table of its
// documentation, with Preparing, the state while prepareAsync runs).
const (
	MPUnknown MPState = iota
	MPIdle
	MPInitialized
	MPPreparing
	MPPrepared
	MPStarted
	MPPaused
	MPStopped
	MPCompleted
	MPError
	MPEnd
)

// MPCall is a method of MediaPlayer the engine calls.
type MPCall uint8

// The calls the machine knows.
const (
	CallUnknown MPCall = iota
	CallSetDataSource
	CallPrepareAsync
	CallStart
	CallPause
	CallStop
	CallSeekTo
	CallSetPlaybackParams
	CallSetVolume
	CallGetCurrentPosition
	CallGetDuration
	CallSetSurface
	CallReset
	CallRelease
	CallGetTrackInfo
)

// MPEvent is what a MediaPlayer's listener reports.
type MPEvent uint8

// The events of a MediaPlayer.
const (
	EvUnknown MPEvent = iota
	EvPrepared
	EvCompleted
	EvError
	EvSeekComplete
)

// Pending is what the engine wants of a MediaPlayer that is not in a state to
// take it yet: it keeps it, and applies it when Allows says so (on
// onPrepared, say), because a call too early is an error. It holds the
// intentions of the player (jPlayer calls SetRate, SetVolume and Seek right
// after Load).
type Pending struct {
	// Play starts as soon as the player is prepared.
	Play bool
	// Seek is the position to go to, in seconds, when HasSeek.
	Seek    float64
	HasSeek bool
	// Rate is the speed, when RateSet.
	Rate    float64
	RateSet bool
	// Volume is the volume, 0 to 1, when VolumeSet.
	Volume    float32
	VolumeSet bool
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Allows tells whether call c is valid in state s. Preparing allows only
// reset and release, which is safe, if a little stricter than the player.
//
// setPlaybackParams is valid from Initialized on, but not in Preparing: it
// does not change the state before the player is prepared, and after that a
// speed over 0 is the same as start() (a paused player plays) — After says so.
func (s MPState) Allows(c MPCall) bool {
	switch c {
	case CallRelease:
		return s != MPUnknown
	case CallReset:
		return s != MPUnknown && s != MPEnd
	case CallSetDataSource:
		return s == MPIdle
	case CallPrepareAsync:
		return s == MPInitialized || s == MPStopped
	case CallStart, CallSeekTo:
		return s == MPPrepared || s == MPStarted || s == MPPaused || s == MPCompleted
	case CallPause:
		return s == MPStarted || s == MPPaused || s == MPCompleted
	case CallStop:
		return s == MPPrepared || s == MPStarted || s == MPPaused || s == MPStopped || s == MPCompleted
	case CallGetDuration, CallGetTrackInfo:
		return s == MPPrepared || s == MPStarted || s == MPPaused || s == MPStopped || s == MPCompleted
	case CallGetCurrentPosition:
		return s == MPIdle || s == MPInitialized || s.Allows(CallGetDuration)
	case CallSetVolume, CallSetSurface:
		return s == MPIdle || s == MPInitialized || s == MPStopped || s == MPPrepared ||
			s == MPStarted || s == MPPaused || s == MPCompleted
	case CallSetPlaybackParams:
		return s == MPInitialized || s == MPPrepared || s == MPStarted || s == MPPaused || s == MPCompleted
	case CallUnknown:
		return false
	default:
		return false
	}
}

// After returns the state s goes to when call c is made, as the engine
// follows it. A call that s does not allow gives MPError (End stays End).
// setPlaybackParams is taken to carry a speed over 0, which the engine
// passes, so that it starts a prepared, paused or completed player.
func (s MPState) After(c MPCall) MPState {
	if !s.Allows(c) {
		if s == MPEnd {
			return MPEnd
		}

		return MPError
	}
	switch c {
	case CallSetDataSource:
		return MPInitialized
	case CallPrepareAsync:
		return MPPreparing
	case CallStart:
		return MPStarted
	case CallPause:
		if s == MPStarted {
			return MPPaused
		}

		return s
	case CallStop:
		return MPStopped
	case CallSetPlaybackParams:
		if s == MPInitialized {
			return s
		}

		return MPStarted
	case CallReset:
		return MPIdle
	case CallRelease:
		return MPEnd
	case CallSeekTo, CallSetVolume, CallGetCurrentPosition, CallGetDuration, CallSetSurface, CallGetTrackInfo, CallUnknown:
		return s
	default:
		return s
	}
}

// On returns the state s goes to when its listener reports e. An event that
// does not fit the state (a late onPrepared after reset, say) changes
// nothing; an error always leads to Error, bar End.
func (s MPState) On(e MPEvent) MPState {
	if s == MPEnd || s == MPUnknown {
		return s
	}
	switch e {
	case EvPrepared:
		if s == MPPreparing {
			return MPPrepared
		}
	case EvCompleted:
		if s == MPStarted {
			return MPCompleted
		}
	case EvError:
		return MPError
	case EvSeekComplete, EvUnknown:
	default:
	}

	return s
}
