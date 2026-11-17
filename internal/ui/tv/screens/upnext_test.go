//go:build tvos

package screens_test

import (
	"testing"
	"time"

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

// upNextState is TV10's state: lesson 3 of set-35 has just played to its end.
func upNextState(t testing.TB) *tvtest.Services {
	t.Helper()
	svc := tvtest.NewPlayerServices(t)
	pb := svc.PB.(*tvtest.Playback)
	pb.PosV, pb.PlayingV = pb.DurV, false

	return svc
}

// upNext returns the up-next screen showing.
func (r *rig) upNext() *screens.UpNext {
	r.t.Helper()
	u, ok := r.sh.Screen().(*screens.UpNext)
	require.True(r.t, ok, "the screen showing is TV10")

	return u
}

// inUpNext opens TV10 over the home with the course read, as a lesson that ended in
// the player gives way to it.
func inUpNext(t testing.TB, svc *tvtest.Services, real bool) *rig {
	t.Helper()
	r := openFast(t, svc, env.RouteHome, real)
	r.settle()
	r.sh.Env().RememberCourse(svc.CourseData)
	r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false)
	r.settle()
	r.sh.Replace(nav.Route{Screen: env.RouteUpNext})
	r.settle()

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestUpNextBoxesMatchPrototype compares every element of TV10.layout.json: the
// chip, the cheer, the two buttons (`u1` focused), the card of the next lesson and
// its lines.
func TestUpNextBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	svc := upNextState(t)
	svc.PB.(*tvtest.Playback).PosV = 0 // the capture counts two watched lessons, not the one that ended
	r := inUpNext(t, svc, true)
	require.Equal(t, "u1", r.key())
	r.tt.Elapse(tvkit.Transition)
	// the cheer's width is 3.2 px under the capture's: Chrome shapes "TUYỆT VỜI" with
	// its own metrics for the stacked accents; the block and the line under it share it
	r.boxesLoose(t, "TV10", map[string]float32{"div.d": 3.5, "p.muted": 3.5})
}

// TestUpNextText: the strings are the prototype's, filled from the lesson that
// ended, its course and the next lesson.
func TestUpNextText(t *testing.T) {
	t.Parallel()
	r := inUpNext(t, upNextState(t), true)
	lay := r.layout()
	want := map[string]string{
		"span.chip.okc":   "Xong bài 03 · Stink and Dutty",
		"div.d":           "TUYỆT VỜI, THẢO VY!",
		"p.muted":         "DanceFIT Set 35 · đã xem 3/10 bài",
		"u1":              "Xem ngay",
		"u2":              "Về khoá học",
		"div.eb":          "TIẾP THEO · TỰ PHÁT SAU 5 GIÂY",
		"tv.upnext.next":  "Bài 04 · Em Yêu Người Ta",
		"p.muted#2":       "Bachata · 04:10 · video + file âm thanh",
		"tv.upnext.count": "5",
	}
	for key, txt := range want {
		assert.Equal(t, txt, find(t, lay, key).Text, key)
	}
}

// TestUpNextCountsDownToTheNextLesson: the disc counts 5 to 0 once a second; at 0
// with "Tự phát bài tiếp theo" on the next lesson plays from its start, TV07 takes
// the screen's place and `p1` has the focus.
func TestUpNextCountsDownToTheNextLesson(t *testing.T) {
	t.Parallel()
	svc := upNextState(t)
	r := inUpNext(t, svc, false)
	pb := r.playback()
	u := r.upNext()
	depth := r.sh.Stack().Depth()
	assert.Equal(t, 5, u.Seconds())
	assert.True(t, u.Counting())
	for _, want := range []int{4, 3, 2, 1} {
		r.tt.Elapse(time.Second)
		assert.Equal(t, want, u.Seconds())
		assert.Equal(t, env.RouteUpNext, r.sh.Stack().Screen())
	}
	assert.Zero(t, pb.Calls["PlayVideo"])
	r.tt.Elapse(time.Second)
	r.settle()
	assert.Equal(t, 1, pb.Calls["PlayVideo"])
	assert.Equal(t, "l_3504", pb.LessonV.ID)
	assert.Zero(t, pb.LessonV.ResumeSec, "from the start (S.pt.pos=0)")
	assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
	assert.Equal(t, depth, r.sh.Stack().Depth(), "replaced, not pushed")
	assert.Equal(t, "p1", r.key())
	r.player()
	assert.Empty(t, r.tt.Leaks())
}

// TestUpNextWithAutoNextOffCountsToZeroAndStays: with the preference off the count
// still reaches 0 and nothing plays (Q-P4.5-1).
func TestUpNextWithAutoNextOffCountsToZeroAndStays(t *testing.T) {
	t.Parallel()
	svc := upNextState(t)
	svc.PrefsV.SetAutoNext(false)
	r := inUpNext(t, svc, false)
	u := r.upNext()
	for range 5 {
		r.tt.Elapse(time.Second)
	}
	r.settle()
	assert.Zero(t, u.Seconds())
	assert.False(t, u.Counting(), "the count stopped at 0")
	assert.Zero(t, r.playback().Calls["PlayVideo"])
	assert.Equal(t, env.RouteUpNext, r.sh.Stack().Screen())
	for range 10 {
		r.tt.Elapse(time.Second)
	}
	assert.Equal(t, env.RouteUpNext, r.sh.Stack().Screen())
	assert.Equal(t, "0", find(t, r.layout(), "tv.upnext.count").Text)
}

// TestUpNextActions: `u1` plays the next lesson at once (TV07, `p1`), `u2` opens
// the course (TV06), Menu goes back and leaving stops the count.
func TestUpNextActions(t *testing.T) {
	t.Parallel()
	svc := upNextState(t)
	r := inUpNext(t, svc, false)
	pb := r.playback()
	require.Equal(t, "u1", r.key())
	require.True(t, r.press(keys.KeyDown))
	require.Equal(t, "u2", r.key())
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	assert.Equal(t, env.RouteDetail, r.sh.Stack().Screen())
	assert.Equal(t, "d1", r.key())
	require.True(t, r.press(keys.KeyEscape))
	assert.Equal(t, env.RouteUpNext, r.sh.Stack().Screen(), "Back returns to TV10")
	r.settle()

	require.Equal(t, "u1", r.key())
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	assert.Equal(t, 1, pb.Calls["PlayVideo"])
	assert.Equal(t, "l_3504", pb.LessonV.ID)
	assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
	assert.Equal(t, "p1", r.key())

	r.tt.Elapse(10 * time.Second) // the count of the screen that left does not run
	assert.Equal(t, 1, pb.Calls["PlayVideo"])
	assert.Empty(t, r.tt.Leaks())
}

// TestUpNextOnTheLastLesson: with no next lesson there is no card, no `u1` and no
// count, and `u2` has the focus.
func TestUpNextOnTheLastLesson(t *testing.T) {
	t.Parallel()
	svc := upNextState(t)
	n := len(svc.CourseData.Lessons)
	svc.PB = tvtest.NewLessonPlayback(svc.CourseData, n)
	r := inUpNext(t, svc, false)
	u := r.upNext()
	assert.Nil(t, u.Item("u1"))
	assert.Equal(t, "u2", r.key())
	assert.False(t, u.Counting())
	_, ok := u.NextLesson()
	assert.False(t, ok)
	lay := r.layout()
	for _, k := range []string{"div.poster", "tv.upnext.count", "div.eb"} {
		_, ok := lay.Find(k)
		assert.False(t, ok, k)
	}
	r.tt.Elapse(10 * time.Second)
	assert.Equal(t, env.RouteUpNext, r.sh.Stack().Screen())
}

// TestUpNextReadsTheCourseWhenNobodyDid: with the course not read, the screen waits
// for it with `u1`'s stand-in focused, then shows and counts.
func TestUpNextReadsTheCourseWhenNobodyDid(t *testing.T) {
	t.Parallel()
	svc := upNextState(t)
	r := openFast(t, svc, env.RouteUpNext, false)
	r.settle()
	u := r.upNext()
	assert.Equal(t, "u1", r.key())
	require.NotNil(t, u.Item("u1"))
	assert.True(t, u.Item("u1").IsFocused())
	assert.True(t, u.Counting())
	_, ok := r.sh.Env().CachedCourse("set-35")
	assert.True(t, ok)
}

// TestUpNextCountdownDoesNotAllocate: a second of the countdown (the timer, the
// disc, the frame) allocates nothing, once the digits have been shown.
func TestUpNextCountdownDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	svc := upNextState(t)
	svc.PrefsV.SetAutoNext(false) // the count reaches 0 and the screen stays
	r := inUpNext(t, svc, false)
	u := r.upNext()
	step := func() {
		r.tt.Elapse(time.Second)
		if u.Seconds() == 0 {
			u.Restart()
		}
	}
	for range 12 {
		step()
	}
	require.Equal(t, env.RouteUpNext, r.sh.Stack().Screen())
	assert.Zero(t, testing.AllocsPerRun(50, step), "a second")
	r.tt.Elapse(time.Second)
	assert.Zero(t, testing.AllocsPerRun(50, func() { r.tt.Elapse(16 * time.Millisecond) }), "a steady frame")
}

// TestUpNextKeepsTheSecondsLeftThroughTheCourse: the count is one number for the app,
// as `cdT` is: leaving for the course with 3 s left and coming back shows 3 and goes
// on, and a new end of a video starts it at 5 again.
func TestUpNextKeepsTheSecondsLeftThroughTheCourse(t *testing.T) {
	t.Parallel()
	r := inUpNext(t, upNextState(t), false)
	r.tt.Elapse(time.Second)
	r.tt.Elapse(time.Second)
	require.Equal(t, 3, r.upNext().Seconds())
	require.True(t, r.press(keys.KeyDown))
	require.Equal(t, "u2", r.key())
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	require.Equal(t, env.RouteDetail, r.sh.Stack().Screen())
	r.tt.Elapse(10 * time.Second) // nothing counts while it is away
	require.True(t, r.press(keys.KeyEscape))
	r.settle()
	u := r.upNext()
	assert.Equal(t, 3, u.Seconds())
	assert.Equal(t, "3", find(t, r.layout(), "tv.upnext.count").Text)
	assert.True(t, u.Counting())
	r.tt.Elapse(time.Second)
	assert.Equal(t, 2, u.Seconds())

	// a video that ends starts over at 5
	r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false)
	r.settle()
	r.playback().End()
	r.settle()
	assert.Equal(t, 5, r.upNext().Seconds())
}
