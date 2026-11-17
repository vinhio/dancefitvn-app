// Adapted from jplayer-mobile/e2e/engine/readtime_test.go at 49de3c2.

//go:build (ios && !tvos) || (android && cgo)

package engine

import (
	"dancefitvn-app/internal/media"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestReadTime checks media.TimeReader on the sample MP3: the position
// read when asked is the start of a load at once (the seek to it under
// way), the target of a seek at once, the time played, and in the
// background, where the tick tells nothing, the time played there;
// another load's token reads nothing.
func TestReadTime(t *testing.T) { //nolint:paralleltest // one app a process
	setStatus("read time")
	h := newHarness(t)
	rt, ok := h.eng.(media.TimeReader)
	require.True(t, ok, "the engine reads its time when asked")
	read := func(tok uint64) (float64, bool) {
		var (
			v  float64
			ok bool
		)
		h.onUI(t, func() { v, ok = rt.ReadTime(tok) })

		return v, ok
	}
	// The paused state and the read in the same turn of the UI thread: as
	// the player reads it at the state, before the seek can finish.
	var atPaused float64
	var pausedOK bool
	var unsub func()
	h.onUI(t, func() {
		unsub = h.eng.Events().Subscribe(func(ev media.Event) {
			if ev.Kind == media.EventState && ev.State == media.StatePaused && !pausedOK {
				atPaused, pausedOK = rt.ReadTime(ev.Token)
			}
		})
	})
	tok := h.loadSrc(t, media.Source{URL: mediaURL(mp3Sample), AudioOnly: true}, media.LoadOptions{Start: 12})
	h.await(t, state(media.StatePaused), loadWithin)
	h.onUI(t, unsub)
	require.True(t, pausedOK, "read at the paused state")
	assert.InDelta(t, 12, atPaused, 0.05, "the start, the seek to it under way")
	_, other := read(tok + 1)
	assert.False(t, other, "another load's token")

	h.onUI(t, func() { require.NoError(t, h.eng.Play()) })
	h.await(t, state(media.StatePlaying), loadWithin)
	wait(1500 * time.Millisecond)
	got, ok := read(tok)
	require.True(t, ok)
	assert.InDelta(t, 13.5, got, 0.4, "1.5 s played")

	var atSeek float64
	h.onUI(t, func() {
		require.NoError(t, h.eng.Seek(20))
		atSeek, _ = rt.ReadTime(tok)
	})
	assert.InDelta(t, 20, atSeek, 0.05, "the seek's target at once")

	// The background: no tick, the media plays on.
	setBackground(t, h, true)
	t.Cleanup(func() { setBackground(t, h, false) })
	h.drain()
	wait(3 * time.Second)
	ticks := 0
	for len(h.events) > 0 {
		if ev := <-h.events; ev.Kind == media.EventTime && ev.Time > 20.5 {
			ticks++
		}
	}
	got, ok = read(tok)
	require.True(t, ok)
	assert.Zero(t, ticks, "no time told in the background")
	assert.InDelta(t, 23, got, 0.5, "3 s played in the background")
	h.onUI(t, func() { require.NoError(t, h.eng.Pause()) })
	h.await(t, state(media.StatePaused), loadWithin)
	paused, _ := read(tok)
	wait(time.Second)
	again, _ := read(tok)
	assert.InDelta(t, paused, again, 0.01, "paused: it stays")
	t.Logf("avf-measure: read time: at paused %.3f, seek %.3f, background %.3f, paused %.3f", atPaused, atSeek, got, paused)

	h.onUI(t, func() { require.NoError(t, h.eng.Stop()) })
	_, ok = read(tok)
	assert.False(t, ok, "stopped")
}
