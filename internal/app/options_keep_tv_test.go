//go:build tvos

package app

import (
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	tvenv "dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tvPlayLesson plays lesson i of the demo course on the player screen and
// waits until it plays.
func tvPlayLesson(t *testing.T, r *tvRig, i int) tvenv.Playback {
	t.Helper()
	ctx := t.Context()
	h, err := r.w.Home(ctx)
	require.NoError(t, err)
	require.NotNil(t, h.Continue)
	c, err := r.w.Course(ctx, h.Continue.Course.ID)
	require.NoError(t, err)
	list := tvenv.PlayerLessons(c)
	require.Greater(t, len(list), i)
	pb := r.w.Playback()
	pb.PlayVideo(list[i], list)
	r.sh.Go(nav.Route{Screen: tvenv.RoutePlayer}, false)
	r.drive("plays", func() bool {
		if pb.State() == player.StateLoading {
			r.media.Advance(time.Millisecond)
		}

		return pb.State() == player.StatePlaying && pb.Lesson().ID == list[i].ID
	})

	return pb
}

// TestTVVideoOptionsAreRemembered (K41): a first open starts at 1x with the
// mirror and the beat bar off; what the options panel sets (SetRate,
// SetMirror, SetBeat) stays for another lesson and for a new launch on the
// same storage; the A-B loop is not kept.
func TestTVVideoOptionsAreRemembered(t *testing.T) {
	t.Parallel()
	secrets := &memSecrets{vals: map[string][]byte{}}
	dir := t.TempDir()
	first := newTVRig(t, secrets, dir)
	first.mount("")
	first.drive("TV01", func() bool { return first.route() == tvenv.RouteWelcome })
	first.adoptMock(t.Context())

	r := newTVRig(t, secrets, dir)
	r.mount("")
	r.home()
	r.drive("prefs open", func() bool { return r.w.prefStore.Load() != nil && r.w.pb.opts.applied })
	pb := tvPlayLesson(t, r, 0)
	assert.InDelta(t, 1, pb.Rate(), 0, "first open: 1x")
	assert.False(t, pb.Mirror(), "first open: mirror off")
	assert.False(t, pb.Beat(), "first open: beat bar off")

	pb.SetRate(1.5)
	pb.SetMirror(true)
	pb.SetBeat(true)
	pb.SetLoop(true)
	r.drive("saved", func() bool {
		p := r.w.prefStore.Load().Get()

		return p.Speed == 1.5 && p.Mirror && p.Beat
	})

	pb = tvPlayLesson(t, r, 1)
	assert.InDelta(t, 1.5, pb.Rate(), 0, "kept for another lesson")
	assert.True(t, pb.Mirror())
	assert.True(t, pb.Beat())

	r2 := newTVRig(t, secrets, dir)
	r2.mount("")
	r2.home()
	r2.drive("prefs open", func() bool { return r2.w.pb.opts.applied })
	pb2 := tvPlayLesson(t, r2, 0)
	assert.InDelta(t, 1.5, pb2.Rate(), 0, "kept across a launch")
	assert.True(t, pb2.Mirror())
	assert.True(t, pb2.Beat())
	assert.False(t, pb2.Loop(), "the A-B loop is not kept")
}
