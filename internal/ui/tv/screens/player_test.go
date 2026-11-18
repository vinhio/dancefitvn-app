//go:build tvos

package screens_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/screens"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/geom"
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

// listShift is how far K46 pushes the rows that follow the lesson playing down from
// where the prototype puts them: lesson 3 has two lessons before it and its own
// row, three rows of 110 and 12 between them.
const listShift float32 = 3 * (110 + 12)

// listRows is what K46 changes of the prototype's TV07: the rows of the lessons
// (`pn*` and their texts, which the layout names by order) and the heading of the
// list, whose text is another and so is its width.
func listRows(key string) bool {
	return strings.HasPrefix(key, "pn") || strings.HasPrefix(key, "span.tt") || key == "b" ||
		strings.HasPrefix(key, "b#") || key == "tv.player.next"
}

// listBoxes checks the lessons that follow the one playing: the prototype's
// `pn0`..`pn3` boxes pushed down by listShift (K46), the column and the heading
// where they were.
func listBoxes(t *testing.T, r *rig, id string) {
	t.Helper()
	lay := r.layout()
	for _, key := range []string{"pn0", "pn1", "pn2", "pn3"} {
		e := find(t, lay, key)
		want := tvtest.ProtoBox(t, id, key)
		got := geom.XYWH(float32(e.X), float32(e.Y), float32(e.W), float32(e.H))
		near(t, geom.XYWH(want.X, want.Y+listShift, want.W, want.H), got, boxTol, id+" "+key)
	}
	head := find(t, lay, "tv.player.next")
	assert.InDelta(t, 1340, head.X, boxTol, "the heading is where the prototype puts it")
}

// TestPlayerBoxesMatchPrototype compares every element of TV07.layout.json with the
// screen's: the title, the chips, the status lines, the buttons (`p1` focused, so
// at its scaled box); the lessons, now all of the course (K46), are checked by
// listBoxes.
func TestPlayerBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	r := openFast(t, playerState(t), env.RoutePlayer, true)
	r.settle()
	require.Equal(t, "p1", r.key())
	r.tt.Elapse(tvkit.Transition)
	r.boxesSkip(t, "TV07", nil, listRows)
	listBoxes(t, r, "TV07")
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
	r.boxesSkip(t, "TV08", nil, listRows)
	listBoxes(t, r, "TV08")
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
		"tv.player.next":    "BÀI HỌC", // K46: the heading of the list of all the lessons
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
