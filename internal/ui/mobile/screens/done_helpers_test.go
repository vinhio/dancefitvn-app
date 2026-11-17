//go:build !tvos

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/require"
)

// atVideoOf signs in, boots, opens the detail of set-35 and plays lesson no of it
// as a video; the player is the Fake's FakePlayback.
func atVideoOf(tb testing.TB, ph kittest.Phone, real bool, no int) (*shelltest.Rig, *shelltest.FakePlayback) {
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
	ids := map[int]string{3: "l_3503", 9: "l_3509", 10: "l_3510"}
	r.Shell.PlayVideo("set-35", ids[no], no)
	r.Settle()
	require.Equal(tb, env.RouteVideo, r.Route())
	pb := f.FakePlayback()
	require.NotNil(tb, pb)
	r.Tester.Elapse(kittest.Frame)

	return r, pb
}

// atDone plays lesson no and lets it end: screen 10 on the Shell's stack, the
// countdown running from now.
func atDone(tb testing.TB, ph kittest.Phone, real bool, lesson ...int) (*shelltest.Rig, *shelltest.FakePlayback, *screens.Done) {
	tb.Helper()
	no := 3
	if len(lesson) > 0 {
		no = lesson[0]
	}
	r, pb := atVideoOf(tb, ph, real, no)
	pb.End()
	r.Settle()
	require.Equal(tb, env.RouteDone, r.Route())
	d, ok := r.Shell.Nav().Screen().(*screens.Done)
	require.True(tb, ok, "the screen showing is Done")
	r.Tester.Elapse(kittest.Frame)

	return r, pb, d
}

// atDoneAt is the state of the prototype's M10: lesson 3 is the one finished and
// two lessons are watched; the player is at 72 s, short of what makes the lesson
// watched, so that the card reads 2/10 as the capture does.
func atDoneAt(tb testing.TB, ph kittest.Phone, real bool) (*shelltest.Rig, *shelltest.FakePlayback, *screens.Done) {
	tb.Helper()
	r, pb := atVideoOf(tb, ph, real, 3)
	pb.Tick(72)
	r.Shell.Nav().Replace(nav.Route{Screen: env.RouteDone})
	r.Settle()
	require.Equal(tb, env.RouteDone, r.Route())
	d, ok := r.Shell.Nav().Screen().(*screens.Done)
	require.True(tb, ok)
	r.Tester.Elapse(kittest.Frame)

	return r, pb, d
}
