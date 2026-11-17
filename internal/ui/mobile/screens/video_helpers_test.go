//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"

	"github.com/stretchr/testify/require"
)

// atVideo signs in, boots, opens the detail of set-35 (so that the course is
// read) and plays lesson 3 on the Fake's FakePlayback: screen 06 with the state
// of the prototype's M06 (resume 72 s of 220, speed .75, mirror on, beat on, loop
// off, A 72 and B 100). real asks for the phone's own text engine. The video is
// playing, as `playVideo` leaves it.
func atVideo(tb testing.TB, ph kittest.Phone, real bool) (*shelltest.Rig, *shelltest.FakePlayback, *screens.Video) {
	tb.Helper()
	f, err := shelltest.New(shelltest.MockFS(tb))
	require.NoError(tb, err)
	require.NoError(tb, f.SeedSession())
	r := shelltest.MountRig(tb, ph, real, f, shell.Options{})
	r.Boot()
	require.Equal(tb, "home", r.Route())
	r.Shell.Detail("set-35")
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
	require.Equal(tb, "detail", r.Route())
	r.Shell.PlayVideo("set-35", "l_3503", 3)
	r.Settle()
	require.Equal(tb, env.RouteVideo, r.Route())
	pb := f.FakePlayback()
	require.NotNil(tb, pb)
	pb.SetAB(72, 100)
	v, ok := r.Shell.Nav().Screen().(*screens.Video)
	require.True(tb, ok, "the screen showing is Video")
	r.Tester.Elapse(kittest.Frame)

	return r, pb, v
}

// ms3s is the auto-hide's wait, 3000 ms.
const ms3s = 3000 * time.Millisecond

// stagePoint is a place on the picture that no control covers, on both phones:
// below the top band, left of the side column, above the panel.
func stagePoint() geom.Point { return geom.Pt(150, 400) }

// restart makes the 3 s of the auto-hide start now, the controls shown: the player
// pauses and plays.
func restart(r *shelltest.Rig, pb *shelltest.FakePlayback) {
	pb.SetState(player.StatePaused)
	pb.SetState(player.StatePlaying)
	r.Settle()
}
