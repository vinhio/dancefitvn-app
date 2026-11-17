// Adapted from jplayer-mobile/internal/media/enginekit/machine_test.go at 49de3c2.

package enginekit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	allStates = []MPState{MPUnknown, MPIdle, MPInitialized, MPPreparing, MPPrepared, MPStarted, MPPaused, MPStopped, MPCompleted, MPError, MPEnd}
	allCalls  = []MPCall{
		CallUnknown, CallSetDataSource, CallPrepareAsync, CallStart, CallPause, CallStop, CallSeekTo,
		CallSetPlaybackParams, CallSetVolume, CallGetCurrentPosition, CallGetDuration, CallSetSurface,
		CallReset, CallRelease, CallGetTrackInfo,
	}
)

func (s MPState) String() string {
	return [...]string{"Unknown", "Idle", "Initialized", "Preparing", "Prepared", "Started", "Paused", "Stopped", "Completed", "Error", "End"}[s]
}

func (c MPCall) String() string {
	return [...]string{
		"Unknown", "setDataSource", "prepareAsync", "start", "pause", "stop", "seekTo", "setPlaybackParams",
		"setVolume", "getCurrentPosition", "getDuration", "setSurface", "reset", "release", "getTrackInfo",
	}[c]
}

// valid is the "Valid and invalid states" table of the MediaPlayer
// documentation: the states each call is valid in. Preparing is not in it;
// there only reset and release are allowed (the safe reading).
var valid = map[MPCall][]MPState{
	CallSetDataSource:      {MPIdle},
	CallPrepareAsync:       {MPInitialized, MPStopped},
	CallStart:              {MPPrepared, MPStarted, MPPaused, MPCompleted},
	CallPause:              {MPStarted, MPPaused, MPCompleted},
	CallStop:               {MPPrepared, MPStarted, MPPaused, MPStopped, MPCompleted},
	CallSeekTo:             {MPPrepared, MPStarted, MPPaused, MPCompleted},
	CallSetPlaybackParams:  {MPInitialized, MPPrepared, MPStarted, MPPaused, MPCompleted},
	CallSetVolume:          {MPIdle, MPInitialized, MPStopped, MPPrepared, MPStarted, MPPaused, MPCompleted},
	CallGetCurrentPosition: {MPIdle, MPInitialized, MPPrepared, MPStarted, MPPaused, MPStopped, MPCompleted},
	CallGetDuration:        {MPPrepared, MPStarted, MPPaused, MPStopped, MPCompleted},
	CallGetTrackInfo:       {MPPrepared, MPStarted, MPPaused, MPStopped, MPCompleted},
	CallSetSurface:         {MPIdle, MPInitialized, MPStopped, MPPrepared, MPStarted, MPPaused, MPCompleted},
	CallReset:              {MPIdle, MPInitialized, MPPreparing, MPPrepared, MPStarted, MPPaused, MPStopped, MPCompleted, MPError},
	CallRelease:            {MPIdle, MPInitialized, MPPreparing, MPPrepared, MPStarted, MPPaused, MPStopped, MPCompleted, MPError, MPEnd},
}

func TestMachineAllows(t *testing.T) {
	t.Parallel()

	for _, c := range allCalls {
		for _, s := range allStates {
			want := false
			for _, v := range valid[c] {
				want = want || v == s
			}
			t.Run(fmt.Sprintf("%v in %v", c, s), func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, want, s.Allows(c))
			})
		}
	}
}

func TestMachineAllowsFindings(t *testing.T) {
	t.Parallel()

	// The cases M0.5 hit on the emulator.
	assert.False(t, MPPreparing.Allows(CallGetDuration), "getDuration while prepareAsync runs throws the player into Error")
	assert.False(t, MPPreparing.Allows(CallSetVolume), "setVolume while preparing is -38")
	assert.False(t, MPStopped.Allows(CallStart), "start after stop is -38")
	assert.True(t, MPStopped.Allows(CallPrepareAsync), "but a stopped player can be prepared again")
	assert.False(t, MPPrepared.Allows(CallPause), "pause needs Started")
	for _, c := range allCalls {
		if c != CallReset && c != CallRelease {
			assert.False(t, MPError.Allows(c), "from Error only reset: %v", c)
		}
	}
	assert.False(t, MPEnd.Allows(CallReset))
}

func TestMachineTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		from MPState
		call MPCall
		want MPState
	}{
		{MPIdle, CallSetDataSource, MPInitialized},
		{MPInitialized, CallPrepareAsync, MPPreparing},
		{MPStopped, CallPrepareAsync, MPPreparing},
		{MPPrepared, CallStart, MPStarted},
		{MPPaused, CallStart, MPStarted},
		{MPCompleted, CallStart, MPStarted},
		{MPStarted, CallStart, MPStarted},
		{MPStarted, CallPause, MPPaused},
		{MPPaused, CallPause, MPPaused},
		{MPCompleted, CallPause, MPCompleted},
		{MPStarted, CallStop, MPStopped},
		{MPPrepared, CallStop, MPStopped},
		{MPPaused, CallSeekTo, MPPaused},
		{MPStarted, CallSeekTo, MPStarted},
		{MPCompleted, CallSeekTo, MPCompleted},
		{MPInitialized, CallSetPlaybackParams, MPInitialized},
		{MPPrepared, CallSetPlaybackParams, MPStarted},
		{MPPaused, CallSetPlaybackParams, MPStarted}, // a speed over 0 is start()
		{MPStarted, CallSetPlaybackParams, MPStarted},
		{MPCompleted, CallSetPlaybackParams, MPStarted},
		{MPPrepared, CallSetVolume, MPPrepared},
		{MPStarted, CallGetCurrentPosition, MPStarted},
		{MPPaused, CallGetDuration, MPPaused},
		{MPStarted, CallSetSurface, MPStarted},
		{MPPaused, CallGetTrackInfo, MPPaused},
		{MPPreparing, CallGetTrackInfo, MPError},
		{MPStarted, CallReset, MPIdle},
		{MPError, CallReset, MPIdle},
		{MPPreparing, CallReset, MPIdle},
		{MPStarted, CallRelease, MPEnd},
		{MPError, CallRelease, MPEnd},
		// A call not allowed throws the player into Error (End stays).
		{MPStopped, CallStart, MPError},
		{MPPreparing, CallGetDuration, MPError},
		{MPIdle, CallStart, MPError},
		{MPStopped, CallSetPlaybackParams, MPError},
		{MPError, CallStart, MPError},
		{MPEnd, CallStart, MPEnd},
		{MPEnd, CallReset, MPEnd},
		{MPUnknown, CallStart, MPError},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v %v", tt.from, tt.call), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.from.After(tt.call))
		})
	}
}

func TestMachineEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from MPState
		ev   MPEvent
		want MPState
	}{
		{"prepared", MPPreparing, EvPrepared, MPPrepared},
		{"late prepared after reset", MPIdle, EvPrepared, MPIdle},
		{"prepared while started is stale", MPStarted, EvPrepared, MPStarted},
		{"completed", MPStarted, EvCompleted, MPCompleted},
		{"completed while paused is stale", MPPaused, EvCompleted, MPPaused},
		{"error while preparing", MPPreparing, EvError, MPError},
		{"error while started", MPStarted, EvError, MPError},
		{"error after release", MPEnd, EvError, MPEnd},
		{"seek complete changes nothing", MPPaused, EvSeekComplete, MPPaused},
		{"seek complete on a completed player", MPCompleted, EvSeekComplete, MPCompleted},
		{"unknown event", MPStarted, EvUnknown, MPStarted},
		{"unknown state", MPUnknown, EvPrepared, MPUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.from.On(tt.ev))
		})
	}
}

// TestMachineWalk follows a load as the engine does: the calls it makes in
// the order it makes them are each valid, and a play, a pause and a second
// load do not leave the player in Error.
func TestMachineWalk(t *testing.T) {
	t.Parallel()

	s := MPIdle
	step := func(c MPCall) {
		t.Helper()
		assert.True(t, s.Allows(c), "%v in %v", c, s)
		s = s.After(c)
		assert.NotEqual(t, MPError, s)
	}
	step(CallSetSurface)
	step(CallSetVolume)
	step(CallSetDataSource)
	step(CallSetPlaybackParams) // the rate before the load is ready: kept, no start
	assert.Equal(t, MPInitialized, s)
	step(CallPrepareAsync)
	assert.False(t, s.Allows(CallSetVolume), "kept in Pending until prepared")
	s = s.On(EvPrepared)
	step(CallGetDuration)
	step(CallSeekTo)
	step(CallStart)
	step(CallGetCurrentPosition)
	step(CallPause)
	step(CallSetPlaybackParams) // resumes
	assert.Equal(t, MPStarted, s)
	s = s.On(EvCompleted)
	assert.Equal(t, MPCompleted, s)
	step(CallSeekTo)
	step(CallStart)
	step(CallStop)
	step(CallPrepareAsync)
	s = s.On(EvPrepared)
	step(CallStart)
	step(CallReset)
	assert.Equal(t, MPIdle, s)
	step(CallRelease)
	assert.Equal(t, MPEnd, s)
}

func TestPendingZeroValue(t *testing.T) {
	t.Parallel()

	var p Pending
	assert.False(t, p.Play)
	assert.False(t, p.HasSeek)
	assert.False(t, p.RateSet)
	assert.False(t, p.VolumeSet)
}
