// Adapted from jplayer-mobile/e2e/engine/avf_ios_test.go at 49de3c2 (TestAVFSession, sessionKeysOf, post).

//go:build ios && !tvos

package engine

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avf"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/ffi/cf"
	"github.com/gdeskdev/desk/foundation/ffi/dl"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// sessionKeys are the audio session's notification names and keys.
type sessionKeys struct {
	interruption, routeChange, mediaReset objc.ID
	typeKey, optionKey                    cf.Ref
	reasonKey                             cf.Ref // of a route change
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestAVFSession checks the audio session (category playback, 04 §2.1):
// Activate once while active, an interruption making it inactive, and the
// notifications it maps (interruption began, ended with and without
// ShouldResume, the headphones out), posted by hand.
func TestAVFSession(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	got := make(chan media.SessionEvent, 16)
	var unsub func()
	h.onUI(t, func() { unsub = h.sess.Events().Subscribe(func(ev media.SessionEvent) { got <- ev }) })
	defer h.onUI(t, func() { unsub() })
	activations := func() int {
		var n int
		h.onUI(t, func() {
			st, ok := avf.SessionStatsOf(h.sess)
			require.True(t, ok)
			n = st.Activations
		})

		return n
	}
	h.onUI(t, func() { require.NoError(t, h.sess.Activate()) })
	a0 := activations()
	h.onUI(t, func() {
		require.NoError(t, h.sess.Activate())
		require.NoError(t, h.sess.Activate())
	})
	assert.Equal(t, a0, activations(), "no setActive while active")

	names := sessionKeysOf(t)
	for _, c := range []struct {
		name string
		note objc.ID
		info []cf.Ref
		want media.SessionEvent
	}{
		{"began", names.interruption, []cf.Ref{names.typeKey, cf.Int(1)}, media.SessionEvent{Kind: media.InterruptBegin}},
		{"ended, resume", names.interruption, []cf.Ref{names.typeKey, cf.Int(0), names.optionKey, cf.Int(1)}, media.SessionEvent{Kind: media.InterruptEnd, ShouldResume: true}},
		{"ended", names.interruption, []cf.Ref{names.typeKey, cf.Int(0)}, media.SessionEvent{Kind: media.InterruptEnd}},
		{"headphones out", names.routeChange, []cf.Ref{names.reasonKey, cf.Int(2)}, media.SessionEvent{Kind: media.Noisy}},
	} {
		post(t, h, c.note, c.info)
		select {
		case ev := <-got:
			assert.Equal(t, c.want, ev, c.name)
		case <-time.After(2 * time.Second):
			assert.Fail(t, "no session event", c.name)
		}
	}
	post(t, h, names.routeChange, []cf.Ref{names.reasonKey, cf.Int(1)}) // a new device: nothing
	select {
	case ev := <-got:
		assert.Fail(t, "an event for a new device", "%+v", ev)
	case <-time.After(500 * time.Millisecond):
	}
	h.onUI(t, func() { require.NoError(t, h.sess.Activate()) })
	assert.Equal(t, a0+1, activations(), "active again after the interruption")
}

// TestAVFMediaReset posts the media services' reset: the session publishes
// MediaReset, and the engine's next Load makes a new player that plays.
func TestAVFMediaReset(t *testing.T) { //nolint:paralleltest // one app a process
	h := newHarness(t)
	got := make(chan media.SessionEvent, 16)
	var unsub func()
	h.onUI(t, func() { unsub = h.sess.Events().Subscribe(func(ev media.SessionEvent) { got <- ev }) })
	defer h.onUI(t, func() { unsub() })
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	post(t, h, sessionKeysOf(t).mediaReset, nil)
	select {
	case ev := <-got:
		assert.Equal(t, media.MediaReset, ev.Kind)
	case <-time.After(2 * time.Second):
		require.Fail(t, "no MediaReset")
	}
	h.load(t, mediaURL(hlsMaster), media.LoadOptions{Auto: true})
	h.await(t, state(media.StatePlaying), playingWithin)
	h.onUI(t, func() {
		st, ok := avf.StatsOf(h.eng)
		require.True(t, ok)
		assert.Equal(t, 2, st.Players, "a new player after the reset")
	})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// sessionKeysOf reads them from AVFAudio.
func sessionKeysOf(t *testing.T) sessionKeys {
	t.Helper()

	lib, err := dl.Open("/System/Library/Frameworks/AVFAudio.framework/AVFAudio")
	require.NoError(t, err)
	c := func(name string) cf.Ref {
		r, err := cf.Constant(lib.Handle(), name)
		require.NoError(t, err)

		return r
	}

	return sessionKeys{
		interruption: objc.ID(c("AVAudioSessionInterruptionNotification")),
		routeChange:  objc.ID(c("AVAudioSessionRouteChangeNotification")),
		typeKey:      c("AVAudioSessionInterruptionTypeKey"),
		optionKey:    c("AVAudioSessionInterruptionOptionKey"),
		reasonKey:    c("AVAudioSessionRouteChangeReasonKey"),
		mediaReset:   objc.ID(c("AVAudioSessionMediaServicesWereResetNotification")),
	}
}

// post posts the notification name with the user info of the key and
// value pairs (released after), from a stand-in object, on the UI thread:
// the session observes every sender; AVFoundation's own observers, which
// watch the shared session, do not hear it.
func post(t *testing.T, h *harness, name objc.ID, pairs []cf.Ref) {
	t.Helper()

	h.onUI(t, func() {
		objc.WithPool(func() {
			var info cf.Ref
			if len(pairs) > 0 {
				info = cf.Dictionary(pairs...)
			}
			sender := objc.New(objc.GetClass("NSObject"))
			center := objc.Call(objc.ID(objc.GetClass("NSNotificationCenter")), objc.Sel("defaultCenter"))
			objc.Call(center, objc.Sel("postNotificationName:object:userInfo:"), uintptr(name), uintptr(sender), uintptr(info))
			objc.Release(sender)
			if info != 0 {
				cf.Release(info)
			}
		})
	})
	for i := 1; i < len(pairs); i += 2 {
		cf.Release(pairs[i])
	}
}
