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

// openDetail mounts TV06 of set-35 as a root, with the motion reduced.
func openDetail(t testing.TB, svc env.Services, real bool) *rig {
	t.Helper()
	r := openAt(t, svc, nav.Route{Screen: env.RouteDetail, Params: "set-35"}, real)
	r.tt.View().Context().Theme().ReduceMotion = true

	return r
}

// detail returns the Detail screen showing.
func (r *rig) detail() *screens.Detail {
	r.t.Helper()
	d, ok := r.sh.Screen().(*screens.Detail)
	require.True(r.t, ok, "the screen showing is TV06")

	return d
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestDetailBoxesMatchPrototype compares every element of TV06.layout.json with the
// screen's: the poster, the block, `d1` (focused, so at its scaled box), `d2`, and
// the ten rows of the list with their numbers and titles, those below the fold too.
func TestDetailBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	r := openDetail(t, tvtest.NewSignedServices(t), true)
	r.settle()
	require.Equal(t, "d1", r.key())
	r.tt.Elapse(tvkit.Transition)
	r.boxes(t, "TV06")
}

// TestDetailText: the strings are the prototype's, filled from the course of the API.
func TestDetailText(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewSignedServices(t)
	svc.MarkWatched("l_3504")
	r := openDetail(t, svc, true)
	r.settle()
	lay := r.layout()
	want := map[string]string{
		"tv.detail.eyebrow": "KHOÁ HỌC THƯỜNG · 10 BÀI HỌC",
		"div.d":             "DANCEFIT SET 35",
		"p.muted":           "“Cùng nhảy cùng hát”",
		"tv.detail.note":    "Mỗi bài gồm video hướng dẫn và file âm thanh.",
		"d1":                "Tiếp tục bài 03",
		"d2":                "Quay lại",
		"tv.detail.head":    "BÀI HỌC",
		"dl0":               "01\nDanceFIT Warm Up 13\nWarm Up · 05:20 · Đã xem",
		"dl1":               "02\nMade You Look\nTay Vai · 03:05 · Đã xem",
		"dl2":               "03\nStink and Dutty\nBụng · 03:40",
		"dl3":               "04\nEm Yêu Người Ta\nBachata · 04:10 · Đã xem", // watched in this run
		"dl9":               "10\nOne Love\nPrecooldown · 04:40",
	}
	for key, txt := range want {
		e, ok := lay.Find(key)
		require.True(t, ok, key)
		assert.Equal(t, txt, e.Text, key)
	}
}

// TestDetailWaitsWithTheBackgroundOnly: until the course is read the screen is the
// background (K12) with the focus where `d1` will be, and when it comes the focus is
// on the real `d1`.
func TestDetailWaitsWithTheBackgroundOnly(t *testing.T) {
	t.Parallel()
	r := openDetail(t, tvtest.NewSignedServices(t), false)
	d := r.detail()
	assert.False(t, d.Loaded())
	assert.Nil(t, d.Item("d2"))
	assert.Equal(t, "d1", r.key())
	r.settle()
	assert.True(t, d.Loaded())
	assert.Equal(t, "d1", r.key())
	assert.True(t, d.Item("d1").IsFocused(), "the real d1 has the focus, not its stand-in")
	assert.Empty(t, r.tt.Leaks())
}

// TestDetailListScrollsToTheLastRow: from `d1` Right and Right again reach the list,
// Down goes along it, and at `dl9` the list has slid 161 over the heading (index.html:
// 729, F-10).
func TestDetailListScrollsToTheLastRow(t *testing.T) {
	t.Parallel()
	r := openDetail(t, tvtest.NewSignedServices(t), true)
	r.settle()
	sl := r.detail().Slide()
	require.NotNil(t, sl)
	r.press(keys.KeyRight)
	require.Equal(t, "d2", r.key())
	r.press(keys.KeyRight)
	require.Contains(t, r.key(), "dl", "the list is at the right of d2")
	offsets := map[string]float32{}
	for range 12 {
		offsets[r.key()] = sl.Offset()
		if r.key() == "dl9" {
			break
		}
		r.press(keys.KeyDown)
	}
	assert.Equal(t, "dl9", r.key())
	assert.Equal(t, float32(161), sl.Offset())
	assert.Equal(t, float32(61), offsets["dl8"])
	for _, k := range []string{"dl0", "dl3", "dl7"} {
		assert.Zero(t, offsets[k], "%s is above the fold: the list does not move", k)
	}
	r.press(keys.KeyDown)
	assert.Equal(t, "dl9", r.key())
	assert.Empty(t, r.tt.Leaks())
}

// TestDetailPlayButton: `d1` plays the lesson to go on with (the player's of this
// course, else the API's resume lesson, else the first) and says which.
func TestDetailPlayButton(t *testing.T) {
	t.Parallel()
	other := player.Lesson{ID: "l_3405", CourseID: "set-34", No: 5}
	mine := player.Lesson{ID: "l_3505", CourseID: "set-35", No: 5}
	tests := []struct {
		name   string
		resume bool
		cur    *player.Lesson
		label  string
		want   string
	}{
		{"resume lesson of the API", true, nil, "Tiếp tục bài 03", "l_3503"},
		{"the player's lesson of this course", true, &mine, "Tiếp tục bài 05", "l_3505"},
		{"the player's lesson of another course", true, &other, "Tiếp tục bài 03", "l_3503"},
		{"nothing to go on with", false, nil, "Xem bài 01", "l_3501"},
		{"another course's lesson, no resume", false, &other, "Xem bài 01", "l_3501"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := tvtest.NewSignedServices(t)
			pb := &tvtest.Playback{}
			if tc.cur != nil {
				pb.LessonV = *tc.cur
			}
			svc.PB = pb
			if !tc.resume {
				svc.CourseData.Resume = nil
			}
			r := openDetail(t, svc, false)
			r.settle()
			lay := r.layout()
			e, ok := lay.Find("d1")
			require.True(t, ok)
			assert.Equal(t, tc.label, e.Text)
			require.True(t, r.press(keys.KeyEnter))
			assert.Equal(t, 1, pb.Calls["PlayVideo"])
			assert.Equal(t, tc.want, pb.Lesson().ID)
			assert.Equal(t, player.KindVideo, pb.Kind())
			assert.True(t, pb.Playing())
			assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
		})
	}
}

// TestDetailRowPlaysItsLesson: Select on a row plays that lesson, handing the player
// the whole course to walk; offline it shows TV11 instead.
func TestDetailRowPlaysItsLesson(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewSignedServices(t)
	pb := &tvtest.Playback{}
	svc.PB = pb
	r := openDetail(t, svc, true)
	r.settle()
	r.press(keys.KeyRight)
	r.press(keys.KeyRight)
	require.Equal(t, "dl0", r.key(), "Right from d2 lands on the top of the list (K33)")
	for range 12 { // go down to the fourth
		if r.key() == "dl3" {
			break
		}
		r.press(keys.KeyDown)
	}
	require.Equal(t, "dl3", r.key())
	require.True(t, r.press(keys.KeyEnter))
	l := pb.Lesson()
	assert.Equal(t, "l_3504", l.ID)
	assert.Equal(t, 4, l.No)
	assert.Equal(t, 10, l.Count)
	assert.Equal(t, "Em Yêu Người Ta", l.Title)
	assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())

	assert.True(t, r.press(keys.KeyEscape))
	r.settle()
	assert.Equal(t, env.RouteDetail, r.sh.Stack().Screen())
	assert.False(t, pb.Playing())
	svc.OnlineNow = false
	pb.PlayingV = false
	assert.Equal(t, "d1", r.key())
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, env.RouteError, r.sh.Stack().Screen())
	assert.Equal(t, 1, pb.Calls["PlayVideo"], "nothing played offline")
}

// TestDetailBack: `d2` and Menu go back to the screen that opened TV06.
func TestDetailBack(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		back func(r *rig)
	}{
		{"d2", func(r *rig) {
			r.press(keys.KeyRight)
			require.Equal(t, "d2", r.key())
			require.True(t, r.press(keys.KeyEnter))
		}},
		{"Menu", func(r *rig) { require.True(t, r.press(keys.KeyEscape)) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := openFast(t, tvtest.NewSignedServices(t), env.RouteCourses, false)
			r.settle()
			r.press(keys.KeyDown)
			require.Equal(t, "cg0", r.key())
			require.True(t, r.press(keys.KeyEnter))
			r.settle()
			require.Equal(t, env.RouteDetail, r.sh.Stack().Screen())
			require.Equal(t, "d1", r.key())

			tc.back(r)
			r.settle()
			assert.Equal(t, env.RouteCourses, r.sh.Stack().Screen())
			assert.Equal(t, "cf0", r.key())
			assert.Empty(t, r.tt.Leaks())
		})
	}
}

// TestDetailReadsTheCourseOfTheRoute: the course id of the route is the one read.
func TestDetailReadsTheCourseOfTheRoute(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewSignedServices(t)
	r := openFast(t, svc, env.RouteHome, false)
	r.settle()
	r.press(keys.KeyDown)
	r.press(keys.KeyRight)
	r.press(keys.KeyRight)
	require.Equal(t, "hc2", r.key())
	svc.CourseData = model.Course{ID: "hot-extra", Name: "DanceFIT Hot Extra", Category: model.CategorySpecial, CategoryLabel: "Đặc biệt", LessonCount: 1,
		Lessons: []model.Lesson{{ID: "l_h1", No: 1, Title: "Hot", Tag: "Latin", DurationSec: 60}}}
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	d := r.detail()
	assert.Equal(t, "hot-extra", d.Course().ID)
	lay := r.layout()
	e, _ := lay.Find("tv.detail.eyebrow")
	assert.Equal(t, "KHOÁ HỌC ĐẶC BIỆT · 1 BÀI HỌC", e.Text)
	_, hasQuote := lay.Find("p.muted")
	assert.False(t, hasQuote, "no tagline, no quote line")
	e, _ = lay.Find("d1")
	assert.Equal(t, "Xem bài 01", e.Text)
}

// TestDetailReadFailsShowsTV11: a failed read puts TV11 over the screen (K28), and a
// canceled one (the screen was left) shows nothing.
func TestDetailReadFailsShowsTV11(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"failed", errors.New("boom"), env.RouteError},
		{"canceled", context.Canceled, env.RouteDetail},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc := tvtest.NewSignedServices(t)
			svc.ReadErr = tc.err
			r := openDetail(t, svc, false)
			r.settle()
			assert.Equal(t, tc.want, r.sh.Stack().Screen())
		})
	}
}

// TestDetailSteadyFrameDoesNotAllocate: a repaint of TV06 with the focus settled
// allocates nothing.
func TestDetailSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := openDetail(t, tvtest.NewSignedServices(t), false)
	r.settle()
	r.tt.Elapse(time.Second)
	r.tt.Repaint()
	assert.Zero(t, testing.AllocsPerRun(20, r.tt.Repaint))
}

// TestDetailListFocusDoesNotAllocate: Down and Up along the list, with its slide,
// allocate nothing.
func TestDetailListFocusDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := openDetail(t, tvtest.NewSignedServices(t), false)
	r.settle()
	r.press(keys.KeyRight)
	r.press(keys.KeyRight)
	for range 12 {
		if r.key() == "dl8" {
			break
		}
		r.press(keys.KeyDown)
	}
	require.Equal(t, "dl8", r.key())
	down := true
	step := func() {
		k := keys.KeyDown
		if !down {
			k = keys.KeyUp
		}
		down = !down
		r.tt.Key(k, 0)
		r.tt.Elapse(tvtest.Frame)
	}
	for range 4 {
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(10, step))
}
