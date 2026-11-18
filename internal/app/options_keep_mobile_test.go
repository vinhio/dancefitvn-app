//go:build !tvos

package app

import (
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// optionsOf reads the three kept options of the player.
func optionsOf(pb env.Playback) (rate float64, mirror, beat bool) {
	return pb.Rate(), pb.Mirror(), pb.Beat()
}

// TestFlowVideoOptionsAreRemembered (K41): a first open starts at 1x with the
// mirror and the beat bar off; what the sheet changes stays for another
// lesson and for a new launch on the same storage; the A-B loop is not kept.
func TestFlowVideoOptionsAreRemembered(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		secrets := &memSecrets{vals: map[string][]byte{}}
		dir := t.TempDir()
		seedSession(t, secrets, dir, sys.BiometryNone)

		pf := newPlayFlowOn(t, ph, 220, secrets, dir, nil, wireKnobs{})
		pb := pf.startLesson3()
		rate, mirror, beat := optionsOf(pb)
		assert.InDelta(t, 1, rate, 0, "first open: 1x")
		assert.False(t, mirror, "first open: mirror off")
		assert.False(t, beat, "first open: beat bar off")
		assert.InDelta(t, 1, pf.eng().Rate(), 0, "the engine plays at 1x")
		assert.False(t, pf.w.prefStore.Load().Get().Beat)

		// the options sheet changes all three, and a loop
		v, ok := pf.sh.Nav().Screen().(*screens.Video)
		require.True(t, ok)
		pf.showControls(v)
		pf.tap("sheet:options", 1)
		pf.tt.Elapse(300 * time.Millisecond)
		require.True(t, pf.sh.Nav().Sheet().IsOpen())
		pf.tap("speed:1.5", 1)
		pf.tap("mirror", 1)
		pf.tap("beat", 1)
		pf.tap("loop", 1)
		require.NoError(t, pf.tt.Click("Xong"))
		pf.tt.Elapse(300 * time.Millisecond)
		require.True(t, pb.Loop())
		pf.drive("saved", func() bool {
			p := pf.w.prefStore.Load().Get()

			return p.Speed == 1.5 && p.Mirror && p.Beat
		})

		// another lesson of the same course: the options are kept, the loop is not
		require.True(t, pf.back())
		pf.until(env.RouteDetail)
		pf.drive("the lessons", func() bool { return widgetNth(pf.sh, "playVideo:set35-5", 1) != nil })
		pf.tapName("playVideo:set35-5")
		pf.until(env.RouteVideo)
		pf.drive("lesson 5 loaded", func() bool { return pb.Lesson().ID == "l_3505" && pb.State() == player.StatePlaying })
		rate, mirror, beat = optionsOf(pb)
		assert.InDelta(t, 1.5, rate, 0)
		assert.True(t, mirror)
		assert.True(t, beat)
		assert.InDelta(t, 1.5, pf.eng().Rate(), 0)

		// a new launch on the same storage: kept, with no loop
		pf2 := newPlayFlowOn(t, ph, 220, secrets, dir, nil, wireKnobs{})
		pb2 := pf2.startLesson3()
		pf2.drive("restored", func() bool { return pb2.Rate() == 1.5 })
		rate, mirror, beat = optionsOf(pb2)
		assert.InDelta(t, 1.5, rate, 0)
		assert.True(t, mirror)
		assert.True(t, beat)
		assert.False(t, pb2.Loop(), "the A-B loop is not kept")

		// switching them off is kept as well
		v2, ok := pf2.sh.Nav().Screen().(*screens.Video)
		require.True(t, ok)
		pf2.showControls(v2)
		pf2.tap("sheet:options", 1)
		pf2.tt.Elapse(300 * time.Millisecond)
		pf2.tap("speed:1", 1)
		pf2.tap("mirror", 1)
		pf2.tap("beat", 1)
		pf2.drive("saved off", func() bool {
			p := pf2.w.prefStore.Load().Get()

			return p.Speed == 1 && !p.Mirror && !p.Beat
		})
	})
}
