// Adapted from jplayer-tv/internal/media/avf/state_test.go at 781bee5.

package avftv

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"dancefitvn-app/internal/media"
)

func TestStateOf(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                      string
		r                         reading
		loaded, playAsked, paused bool
		want                      media.PlayState
	}{
		{"nothing loaded", reading{control: controlPlaying}, false, true, false, media.StateUnknown},
		{"nothing loaded, failed item", reading{item: itemFailed}, false, true, false, media.StateUnknown},
		{"loaded, not asked to play", reading{item: itemReady}, true, false, false, media.StateUnknown},
		{"failure shows before play", reading{item: itemFailed}, true, false, false, media.StateError},
		{"player failed", reading{player: playerFailed, control: controlPlaying}, true, true, false, media.StateError},
		{"failed to play to the end", reading{item: itemReady, control: controlPlaying, failedToEnd: true}, true, true, false, media.StateError},
		{"failure wins over pause", reading{item: itemFailed}, true, true, true, media.StateError},
		{"waiting", reading{item: itemUnknown, control: controlWaiting}, true, true, false, media.StateLoading},
		{"playing", reading{item: itemReady, control: controlPlaying}, true, true, false, media.StatePlaying},
		{"paused by the engine", reading{item: itemReady, control: controlPaused}, true, true, true, media.StatePaused},
		{"paused but the reading still plays", reading{item: itemReady, control: controlPlaying}, true, true, true, media.StateUnknown},
		{"paused by someone else", reading{item: itemReady, control: controlPaused}, true, true, false, media.StateUnknown},
		{"unknown control", reading{item: itemReady, control: 7}, true, true, false, media.StateUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, c.want, stateOf(c.r, c.loaded, c.playAsked, c.paused))
		})
	}
}

func TestMachineEmitsOnlyChanges(t *testing.T) {
	t.Parallel()

	var m machine
	m.load()
	m.play()
	steps := []struct {
		r    reading
		want media.PlayState
		emit bool
	}{
		{reading{control: controlWaiting}, media.StateLoading, true},
		{reading{control: controlWaiting}, media.StateLoading, false},
		{reading{item: itemReady, control: controlPlaying}, media.StatePlaying, true},
		{reading{item: itemReady, control: controlPlaying}, media.StatePlaying, false},
		{reading{item: itemReady, control: controlWaiting}, media.StateLoading, true},
		{reading{item: itemReady, control: controlPlaying}, media.StatePlaying, true},
		{reading{item: itemFailed}, media.StateError, true},
		{reading{item: itemFailed}, media.StateError, false},
	}
	for i, s := range steps {
		got, emit := m.step(s.r)
		assert.Equal(t, s.emit, emit, "step %d", i)
		if emit {
			assert.Equal(t, s.want, got, "step %d", i)
		}
	}
}

func TestMachineFailedToEndIsSticky(t *testing.T) {
	t.Parallel()

	var m machine
	m.load()
	m.play()
	got, emit := m.step(reading{item: itemReady, control: controlPlaying, failedToEnd: true})
	assert.True(t, emit)
	assert.Equal(t, media.StateError, got)
	_, emit = m.step(reading{item: itemReady, control: controlPlaying})
	assert.False(t, emit, "the failure stays until Load or Stop")

	m.load()
	m.play()
	got, emit = m.step(reading{item: itemReady, control: controlPlaying})
	assert.True(t, emit)
	assert.Equal(t, media.StatePlaying, got, "Load clears the failure")
}

func TestMachineSilentAfterStop(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		r    reading
	}{
		{"waiting", reading{control: controlWaiting}},
		{"playing", reading{item: itemReady, control: controlPlaying}},
		{"paused", reading{item: itemReady, control: controlPaused}},
		{"failed", reading{item: itemFailed}},
		{"failed to end", reading{failedToEnd: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var m machine
			m.load()
			m.play()
			m.stop()
			_, emit := m.step(c.r)
			assert.False(t, emit)
		})
	}
}

func TestMachinePauseWhileLoadingStaysPaused(t *testing.T) {
	t.Parallel()

	var m machine
	m.load()
	m.play()
	got, emit := m.step(reading{control: controlWaiting})
	assert.True(t, emit)
	assert.Equal(t, media.StateLoading, got)
	m.pause()
	got, emit = m.step(reading{control: controlPaused})
	assert.True(t, emit)
	assert.Equal(t, media.StatePaused, got)
	// The item gets ready meanwhile: still paused, nothing new.
	_, emit = m.step(reading{item: itemReady, control: controlPaused})
	assert.False(t, emit)
	// Play again: loading, then playing.
	m.play()
	got, emit = m.step(reading{item: itemReady, control: controlWaiting})
	assert.True(t, emit)
	assert.Equal(t, media.StateLoading, got)
}

func TestMachineSilentBeforePlay(t *testing.T) {
	t.Parallel()

	var m machine
	m.load()
	_, emit := m.step(reading{item: itemReady, control: controlPaused})
	assert.False(t, emit)
}

func TestMachineStepDoesNotAllocate(t *testing.T) {
	var m machine
	m.load()
	m.play()
	r := reading{item: itemReady, control: controlPlaying}
	m.step(r)
	assert.Zero(t, testing.AllocsPerRun(100, func() { m.step(r) }))
}

func BenchmarkMachineStep(b *testing.B) {
	var m machine
	m.load()
	m.play()
	r := reading{item: itemReady, control: controlPlaying}
	b.ReportAllocs()
	for b.Loop() {
		m.step(r)
	}
}
