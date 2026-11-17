//go:build !tvos

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/require"
)

// atAudio signs in, boots, opens the detail of set-35 (so that the course is
// read) and plays the audio of lesson 3 on the Fake's FakePlayback: screen 08
// with the state of the prototype's M08 (paused at 72 s of 220, speed .75, beat on,
// loop off, the file on the device). real asks for the phone's own text engine.
func atAudio(tb testing.TB, ph kittest.Phone, real bool) (*shelltest.Rig, *shelltest.FakePlayback, *screens.Audio) {
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
	r.Shell.PlayAudio("set-35", "l_3503", 3)
	r.Settle()
	require.Equal(tb, env.RouteAudio, r.Route())
	pb := f.FakePlayback()
	require.NotNil(tb, pb)
	pb.Tick(72)
	pb.Pause()
	pb.ClearCalls()
	a, ok := r.Shell.Nav().Screen().(*screens.Audio)
	require.True(tb, ok, "the screen showing is Audio")
	r.Tester.Elapse(kittest.Frame)

	return r, pb, a
}

// tapKey taps the middle of the element of the dump named key, as a finger does.
func tapKey(tb testing.TB, r *shelltest.Rig, key string) {
	tb.Helper()
	e, ok := r.Dump("tap").Find(key)
	require.Truef(tb, ok, "no element %q", key)
	r.Tester.ClickAt(geom.Pt(float32(e.X+e.W/2), float32(e.Y+e.H/2)))
	r.Settle()
}

// element returns the element key of the dump.
func element(tb testing.TB, lay layoutdump.Layout, key string) layoutdump.Element {
	tb.Helper()
	e, ok := lay.Find(key)
	require.Truef(tb, ok, "no element %q", key)

	return e
}
