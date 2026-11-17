//go:build tvos

package screens_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
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

// home returns the Home screen showing.
func (r *rig) home() *screens.Home {
	r.t.Helper()
	h, ok := r.sh.Screen().(*screens.Home)
	require.True(r.t, ok, "the screen showing is TV04")

	return h
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestHomeBoxesMatchPrototype compares every element of TV04.layout.json with the
// screen's: the hero, the nav bar, the text block, the bar, `h1` (focused, so at
// its scaled box), `h2`, the shelf and its four tiles.
func TestHomeBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteHome, true)
	r.settle()
	require.Equal(t, "h1", r.key())
	r.tt.Elapse(tvkit.Transition)
	r.boxes(t, "TV04")
}

// TestHomeText: the strings are the prototype's, filled from the home of the API.
func TestHomeText(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteHome, true)
	r.settle()
	lay := r.layout()
	want := map[string]string{
		"span.chip.y":  "TIẾP TỤC XEM",
		"tv.home.meta": "DANCEFIT SET 35 · BÀI 03 / 10 · BỤNG",
		"div.d":        "STINK AND DUTTY",
		"p.muted":      "“Cùng nhảy cùng hát” · 03:40 · đang xem 01:12",
		"h1":           "Tiếp tục xem",
		"h2":           "10 bài học",
		"div.d#2":      "KHOÁ HỌC CỦA TÔI · 15",
		"hc3":          "DANCEFIT\nSET 33",
		"nav0":         "Trang chủ",
		"nav3":         "Cài đặt",
	}
	for key, txt := range want {
		e, ok := lay.Find(key)
		require.True(t, ok, key)
		assert.Equal(t, txt, e.Text, key)
	}
}

// TestHomeWaitsWithTheNavBarOnly: until the home is read the screen is the nav bar
// over the background (K12), the default focus is where `h1` will be, and when the
// data comes the focus is on the real `h1`.
func TestHomeWaitsWithTheNavBarOnly(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteHome, false)
	h := r.home()
	assert.False(t, h.Loaded(), "nothing is read before the read's turn")
	assert.Nil(t, h.Item("h2"), "no content yet")
	assert.Equal(t, "h1", r.key(), "the stand-in of h1 has the focus")
	lay := r.layout()
	_, ok := lay.Find("div.tnav")
	assert.True(t, ok, "the nav bar is there")
	_, ok = lay.Find("span.chip.y")
	assert.False(t, ok)

	r.settle()
	assert.True(t, h.Loaded())
	require.NotNil(t, h.Item("h1"))
	assert.Equal(t, "h1", r.key())
	assert.True(t, h.Item("h1").IsFocused(), "the real h1 has the focus, not its stand-in")
	assert.Empty(t, r.tt.Leaks())
}

// TestHomeKeepsAFocusThatWentToTheNavBar: a user who moved to the nav bar while the
// home was read stays there.
func TestHomeKeepsAFocusThatWentToTheNavBar(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteHome, true)
	require.True(t, r.press(keys.KeyUp))
	require.Contains(t, []string{"nav0", "nav1", "nav2", "nav3"}, r.key())
	k := r.key()
	r.settle()
	assert.True(t, r.home().Loaded())
	assert.Equal(t, k, r.key())
}

// TestHomeMoves: Right goes from `h1` to `h2`, Down to the first tile and along the
// shelf; Up comes back to the buttons, and the nav bar is above them.
func TestHomeMoves(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteHome, true)
	r.settle()
	require.Equal(t, "h1", r.key())
	assert.True(t, r.press(keys.KeyRight))
	assert.Equal(t, "h2", r.key())
	assert.True(t, r.press(keys.KeyLeft))
	assert.Equal(t, "h1", r.key())
	assert.True(t, r.press(keys.KeyDown))
	assert.Equal(t, "hc0", r.key())
	for _, want := range []string{"hc1", "hc2", "hc3"} {
		assert.True(t, r.press(keys.KeyRight))
		assert.Equal(t, want, r.key())
	}
	r.press(keys.KeyRight)
	assert.Equal(t, "hc3", r.key(), "nothing at the right of the last tile")
	for range 3 {
		r.press(keys.KeyLeft)
	}
	require.Equal(t, "hc0", r.key())
	r.press(keys.KeyUp)
	assert.Contains(t, []string{"h1", "h2"}, r.key())
	r.press(keys.KeyUp)
	assert.Contains(t, []string{"nav0", "nav1", "nav2", "nav3"}, r.key())
	assert.Empty(t, r.tt.Leaks())
}

// TestHomeResumePlaysTheLesson: Select on `h1` plays the lesson to continue from
// where the user was, as a video, and opens TV07 over TV04 (`playLes`, index.html:739).
func TestHomeResumePlaysTheLesson(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewSignedServices(t)
	svc.PB = &tvtest.Playback{}
	r := openFast(t, svc, env.RouteHome, false)
	r.settle()
	require.Equal(t, "h1", r.key())
	require.True(t, r.press(keys.KeyEnter))

	pb := svc.PB.(*tvtest.Playback)
	assert.Equal(t, 1, pb.Calls["PlayVideo"])
	l := pb.Lesson()
	assert.Equal(t, "l_3503", l.ID)
	assert.Equal(t, "set-35", l.CourseID)
	assert.Equal(t, 3, l.No)
	assert.Equal(t, 10, l.Count)
	assert.Equal(t, 72, l.ResumeSec)
	assert.Equal(t, player.KindVideo, pb.Kind())
	assert.True(t, pb.Playing())
	assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
	assert.Equal(t, 1, r.sh.Stack().Depth())

	assert.True(t, r.press(keys.KeyEscape), "Menu goes back")
	r.settle()
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
	assert.False(t, pb.Playing(), "and pauses")
	assert.Empty(t, r.tt.Leaks())
}

// TestHomeResumeOffline: `playLes` while offline shows TV11 (index.html:739).
func TestHomeResumeOffline(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewSignedServices(t)
	svc.PB = &tvtest.Playback{}
	r := openFast(t, svc, env.RouteHome, false)
	r.settle()
	svc.OnlineNow = false
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, env.RouteError, r.sh.Stack().Screen())
	assert.Zero(t, svc.PB.(*tvtest.Playback).Calls["PlayVideo"])
}

// TestHomeOpensCourses: `h2` and each tile open TV06 of their course over TV04, and
// Menu comes back to TV04 with `h1` focused.
func TestHomeOpensCourses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path []keys.Key
		key  string
		id   string
	}{
		{"h2", []keys.Key{keys.KeyRight}, "h2", "set-35"},
		{"hc0", []keys.Key{keys.KeyDown}, "hc0", "set-35"},
		{"hc2", []keys.Key{keys.KeyDown, keys.KeyRight, keys.KeyRight}, "hc2", "hot-extra"},
		{"hc3", []keys.Key{keys.KeyDown, keys.KeyRight, keys.KeyRight, keys.KeyRight}, "hc3", "set-33"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := openFast(t, tvtest.NewSignedServices(t), env.RouteHome, true)
			r.settle()
			for _, k := range tc.path {
				r.press(k)
			}
			require.Equal(t, tc.key, r.key())
			require.True(t, r.press(keys.KeyEnter))
			cur, _ := r.sh.Stack().Current()
			assert.Equal(t, env.RouteDetail, cur.Screen)
			assert.Equal(t, tc.id, cur.Params)
			assert.Equal(t, 1, r.sh.Stack().Depth())

			assert.True(t, r.press(keys.KeyEscape))
			r.settle()
			assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
			assert.Equal(t, "h1", r.key(), "no memory: the default focus")
		})
	}
}

// TestHomeNavSwitchesInPlace: Select on a nav item shows that tab and keeps the focus
// on the item.
func TestHomeNavSwitchesInPlace(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteHome, true)
	r.settle()
	r.press(keys.KeyUp)
	r.walkTo("nav1")
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, env.RouteCourses, r.sh.Stack().Screen())
	assert.Zero(t, r.sh.Stack().Depth())
	assert.Equal(t, "nav1", r.key())
	r.settle()
	assert.Equal(t, "nav1", r.key(), "the data does not take the focus from the nav bar")
}

// TestHomeNoContinue: with no lesson to continue the hero is the first lesson of the
// first course, `h1` reads "Xem bài 01" and the line has no "đang xem" (Q-P7-9).
func TestHomeNoContinue(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewSignedServices(t)
	svc.HomeData.Continue = nil
	svc.PB = &tvtest.Playback{}
	r := openFast(t, svc, env.RouteHome, true)
	r.settle()
	lay := r.layout()
	for key, txt := range map[string]string{
		"div.d":   "DANCEFIT WARM UP 13",
		"p.muted": "“Cùng nhảy cùng hát” · 05:20",
		"h1":      "Xem bài 01",
	} {
		e, ok := lay.Find(key)
		require.True(t, ok, key)
		assert.Equal(t, txt, e.Text, key)
	}
	require.Equal(t, "h1", r.key())
	require.True(t, r.press(keys.KeyEnter))
	pb := svc.PB.(*tvtest.Playback)
	assert.Equal(t, "l_3501", pb.Lesson().ID)
}

// TestHomeWithoutCourses: a home with nothing in it shows the nav bar and the shelf's
// heading, and the focus is on the nav bar.
func TestHomeWithoutCourses(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewServices()
	r := openFast(t, svc, env.RouteHome, false)
	r.settle()
	assert.True(t, r.home().Loaded())
	assert.Contains(t, []string{"nav0", "nav1", "nav2", "nav3"}, r.key())
	assert.Empty(t, r.tt.Leaks())
}

// TestHomeNoLessonsShowsTheShelf: a first course with no lesson gives no hero; the
// shelf shows and the focus is on its first tile.
func TestHomeNoLessonsShowsTheShelf(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewSignedServices(t)
	svc.HomeData.Continue = nil
	svc.CourseData.Lessons = nil
	r := openFast(t, svc, env.RouteHome, false)
	r.settle()
	lay := r.layout()
	_, ok := lay.Find("span.chip.y")
	assert.False(t, ok, "no hero")
	e, ok := lay.Find("hc0")
	require.True(t, ok)
	assert.InDelta(t, 840, e.Y+e.H/2-118, 2, "the shelf is where it is with a hero")
	assert.Equal(t, "hc0", r.key())
}

// TestHomeReadFailsShowsTV11: a failed read puts TV11 over the screen (K28); a
// canceled one (the screen was left) shows nothing.
func TestHomeReadFailsShowsTV11(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"failed", errors.New("boom"), env.RouteError},
		{"canceled", context.Canceled, env.RouteHome},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := tvtest.NewSignedServices(t)
			svc.ReadErr = tc.err
			r := openFast(t, svc, env.RouteHome, false)
			r.settle()
			assert.Equal(t, tc.want, r.sh.Stack().Screen())
		})
	}
}

// TestHomeLeftBeforeTheAnswer: leaving while the read is in flight drops the answer.
func TestHomeLeftBeforeTheAnswer(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteHome, false)
	r.sh.Go(nav.Route{Screen: env.RouteWelcome}, true)
	r.settle()
	assert.Equal(t, env.RouteWelcome, r.sh.Stack().Screen())
	assert.Empty(t, r.tt.Leaks())
}

// TestHomeAvatar: the nav bar's `.tav` shows the user's picture once it is read.
func TestHomeAvatar(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewSignedServices(t)
	svc.User = model.User{AvatarURL: "mock-media://avatars/u_412.jpg"}
	r := openFast(t, svc, env.RouteHome, false)
	r.settle()
	assert.Equal(t, []string{"mock-media://avatars/u_412.jpg"}, svc.AvatarCalls())
}

// TestHomeSteadyFrameDoesNotAllocate: a repaint of TV04 with the focus settled
// allocates nothing.
func TestHomeSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteHome, false)
	r.settle()
	r.tt.Elapse(time.Second)
	r.tt.Repaint()
	assert.Zero(t, testing.AllocsPerRun(20, r.tt.Repaint))
}

// TestHomeFocusMoveDoesNotAllocate: Right and Left between the two buttons, with the
// frames of the focus animation, allocate nothing.
func TestHomeFocusMoveDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteHome, false)
	r.settle()
	right := true
	step := func() {
		k := keys.KeyRight
		if !right {
			k = keys.KeyLeft
		}
		right = !right
		r.tt.Key(k, 0)
		r.tt.Elapse(40 * time.Millisecond)
	}
	for range 4 {
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(10, step))
}
