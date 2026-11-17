//go:build tvos

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/screens"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// playerState is the state the prototype's TV07 and TV08 are captured in: lesson
// 3 at 72 of 220 seconds, paused, at .75x, mirrored, with the beat bar on.
func playerState(t testing.TB) *tvtest.Services {
	t.Helper()
	svc := tvtest.NewPlayerServices(t)
	pb := svc.PB.(*tvtest.Playback)
	pb.RateV, pb.MirrorV, pb.BeatV = .75, true, true

	return svc
}

// playback returns the fake player of the services of a rig.
func (r *rig) playback() *tvtest.Playback {
	r.t.Helper()
	pb, ok := r.svc.(*tvtest.Services).PB.(*tvtest.Playback)
	require.True(r.t, ok)

	return pb
}

// player returns the player screen showing.
func (r *rig) player() *screens.Player {
	r.t.Helper()
	p, ok := r.sh.Screen().(*screens.Player)
	require.True(r.t, ok, "the screen showing is TV07")

	return p
}

// inPlayer opens TV07 over the home, as `h1` does, and settles it.
func inPlayer(t testing.TB, svc *tvtest.Services, real bool) *rig {
	t.Helper()
	r := openFast(t, svc, env.RouteHome, real)
	r.settle()
	r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false)
	r.settle()

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestPlayerBoxesMatchPrototype compares every element of TV07.layout.json with the
// screen's: the title, the chips, the status lines, the buttons (`p1` focused, so
// at its scaled box) and the next four lessons.
func TestPlayerBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	r := openFast(t, playerState(t), env.RoutePlayer, true)
	r.settle()
	require.Equal(t, "p1", r.key())
	r.tt.Elapse(tvkit.Transition)
	r.boxes(t, "TV07")
}

// TestPlayerPanelBoxesMatchPrototype compares TV08: the player of TV07 with the
// panel open, whose first item `o0` has the focus.
func TestPlayerPanelBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	r := openFast(t, playerState(t), env.RoutePlayer, true)
	r.settle()
	require.True(t, r.press(keys.KeyRight))
	require.Equal(t, "p2", r.key())
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	require.Equal(t, "o0", r.key())
	r.tt.Elapse(tvkit.PanelFade + tvkit.Transition)
	r.boxes(t, "TV08")
}

// TestPlayerText: the strings are the prototype's, filled from the lesson, the
// options and the clock.
func TestPlayerText(t *testing.T) {
	t.Parallel()
	r := openFast(t, playerState(t), env.RoutePlayer, true)
	r.settle()
	lay := r.layout()
	want := map[string]string{
		"tv.player.eyebrow": "DANCEFIT SET 35 · BÀI 03 / 10",
		"div.d":             "STINK AND DUTTY",
		"span.chip.y":       "Bụng",
		"span.chip":         "Video dọc 9:16",
		"tv.player.pos":     "01:12",
		"tv.player.dur":     "03:40",
		"p1":                "Phát",
		"p2":                "Tuỳ chọn",
		"tv.player.next":    "TIẾP THEO",
		"pn0":               "04\nEm Yêu Người Ta\nBachata · 04:10",
		"pn3":               "07\nViva La Vida\nCzech Pop · 04:05",
	}
	for key, txt := range want {
		e, ok := lay.Find(key)
		require.True(t, ok, key)
		assert.Equal(t, txt, e.Text, key)
	}
	assert.Equal(t, "Tốc độ 0.75x", find(t, lay, "tv.player.speed").Text)
	assert.Equal(t, "Lật gương", find(t, lay, "tv.player.mirror").Text)
	_, ok := lay.Find("tv.player.loop")
	assert.False(t, ok, "the loop is off: no line")
}

// find returns the element of the key of a dump.
func find(t testing.TB, lay layoutdump.Layout, key string) layoutdump.Element {
	t.Helper()
	e, ok := lay.Find(key)
	require.True(t, ok, key)

	return e
}
