//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// second is the countdown's step.
const second = time.Second

// doneSeconds is the 5 of the countdown.
const doneSeconds = 5

// TestDoneBoxesMatchPrototype lays screen 10 out with the iPhone's real text engine
// and compares every element of M10 with the prototype's within 1 DIP, the
// countdown pinned at 5 as the shot state has it.
func TestDoneBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r, _, d := atDoneAt(t, kittest.Phones[0], true)
	d.Hold()
	checkAgainstProto(t, "M10", r.Dump("M10"), 1, nil)
	assert.True(t, r.Tester.Shows("TIẾP THEO · TỰ PHÁT SAU 5 GIÂY"))
}

// TestDoneOnTheWiderPhone checks the 412 wide phone against the iPhone's: the y of
// every element is the same (the page hangs from the top), the width of the cards
// and the buttons grows with the window and what is centered follows the middle.
func TestDoneOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	rI, _, dI := atDoneAt(t, kittest.Phones[0], true)
	rA, _, dA := atDoneAt(t, kittest.Phones[1], true)
	dI.Hold()
	dA.Hold()
	a, b := rI.Dump("M10"), rA.Dump("M10")
	require.Len(t, b.Elements, len(a.Elements))
	for i, ea := range a.Elements {
		eb := b.Elements[i]
		require.Equal(t, ea.Key, eb.Key)
		assert.InDelta(t, ea.Y, eb.Y, .6, ea.Key+": y")
	}
	for _, k := range []string{"div.card", "div.card#2", "playVideo:set35-4", "toDetail:set35", "div.d"} {
		assert.InDelta(t, 372, element(t, b, k).W, .6, k+": spans the margins")
	}
	assert.InDelta(t, element(t, a, "svg.i#2").X+9.5, element(t, b, "svg.i#2").X, .6, "the check is in the middle")
}

// TestDoneShowsTheProgress checks the numbers of a lesson that played to its end:
// it counts as watched, so the card reads 3/10, on both phones, and the text under
// it.
func TestDoneShowsTheProgress(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, d := atDone(t, ph, false)
		for _, s := range []string{"Xong bài 03!", "Stink and Dutty · Bụng", "DanceFIT Set 35", "3/10", "Đã xem 3/10 bài",
			"Còn 7 bài trong Set 35", "04", "Em Yêu Người Ta", "Bachata · 04:10", "Xem bài 04", "Về khoá học",
			"TIẾP THEO · TỰ PHÁT SAU 5 GIÂY"} {
			assert.True(t, r.Tester.Shows(s), s)
		}
		assert.Equal(t, 5, d.Seconds())
		assert.True(t, d.Counting())
		assert.False(t, r.Tester.Shows("Bạn đã xem hết khoá học"))
	})
}

// TestDoneCountdownPlaysTheNext checks 5 to 0 once a second, then the next lesson:
// 06 takes the place of 10 with lesson 4 playing from its start.
func TestDoneCountdownPlaysTheNext(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, d := atDone(t, ph, false)
		pb.ClearCalls()
		r.Tester.Elapse(900 * time.Millisecond)
		assert.Equal(t, 5, d.Seconds(), "5 until a whole second has passed")
		r.Tester.Elapse(100 * time.Millisecond)
		for _, n := range []string{"4", "3", "2", "1"} {
			assert.True(t, r.Tester.Shows("TIẾP THEO · TỰ PHÁT SAU "+n+" GIÂY"), n)
			assert.Equal(t, env.RouteDone, r.Route())
			r.Tester.Elapse(second)
		}
		r.Settle()
		assert.Equal(t, env.RouteVideo, r.Route(), "at 0 the next lesson plays")
		assert.Equal(t, 4, pb.Lesson().No)
		assert.True(t, pb.Playing())
		assert.Equal(t, []string{"AutoAdvance"}, pb.Calls())
		assert.Zero(t, pb.Count("Leave"), "10 never tells the player Leave")
		assert.True(t, r.Tester.Shows("Bài 04 / 10"))
	})
}

// TestDoneAutoNextOff checks that with "Tự phát bài tiếp theo" off the seconds count
// to 0 as the prototype's do (`cdM--`) and nothing plays: the screen stays.
func TestDoneAutoNextOff(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, d := atDone(t, ph, false)
		r.Fake.FakePrefs().SetAutoNext(false)
		pb.ClearCalls()
		r.Tester.Elapse(6 * second)
		r.Settle()
		assert.Equal(t, env.RouteDone, r.Route())
		assert.Equal(t, 3, pb.Lesson().No)
		assert.Equal(t, 0, d.Seconds())
		assert.False(t, d.Counting(), "the countdown ended")
		assert.Equal(t, []string{"AutoAdvance"}, pb.Calls(), "asked once, answered no")
		r.Tester.Elapse(10 * second)
		assert.Equal(t, 1, pb.Count("AutoAdvance"))
	})
}

// TestDoneLastLesson checks a course on its last lesson: the line "Bạn đã xem hết
// khoá học" in place of the card and the button, no countdown, nothing plays.
func TestDoneLastLesson(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r2, pb2, d2 := atDone(t, ph, false, 10)
		assert.True(t, r2.Tester.Shows("Xong bài 10!"))
		assert.True(t, r2.Tester.Shows("Bạn đã xem hết khoá học"))
		assert.False(t, r2.Tester.Shows("Xem bài 11"))
		assert.False(t, r2.Tester.Shows("TIẾP THEO · TỰ PHÁT SAU 5 GIÂY"))
		assert.True(t, r2.Tester.Shows("Về khoá học"))
		assert.False(t, d2.Counting())
		pb2.ClearCalls()
		r2.Tester.Elapse(10 * second)
		r2.Settle()
		assert.Equal(t, env.RouteDone, r2.Route())
		assert.Empty(t, pb2.Calls())
	})
}

// TestDoneActions checks the buttons: "Xem bài 04" plays lesson 4 in place of 10,
// "Về khoá học" shows the detail with [courses] under it, Back shows the screen under
// and ends the countdown.
func TestDoneActions(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atDone(t, ph, false)
		pb.ClearCalls()
		tapKey(t, r, "playVideo:set35-4")
		assert.Equal(t, env.RouteVideo, r.Route())
		assert.Equal(t, 4, pb.Lesson().No)
		assert.Equal(t, []string{"PlayVideo:l_3504"}, pb.Calls())
		r.Tester.Elapse(10 * second)
		assert.Equal(t, env.RouteVideo, r.Route(), "10 is gone, its countdown too")
		assert.Zero(t, pb.Count("AutoAdvance"))

		r, pb, _ = atDone(t, ph, false)
		pb.ClearCalls()
		tapKey(t, r, "toDetail:set35")
		assert.Equal(t, "detail", r.Route())
		r.Tester.Elapse(10 * second)
		assert.Equal(t, "detail", r.Route())
		assert.Zero(t, pb.Count("AutoAdvance"), "leaving 10 stops the countdown")
		require.NoError(t, r.Tester.Click("Quay lại"))
		assert.Equal(t, "courses", r.Route(), "the stack was [courses, detail]")

		r, pb, _ = atDone(t, ph, false)
		pb.ClearCalls()
		r.Tester.Elapse(2 * second)
		require.NoError(t, r.Tester.Click("Quay lại"))
		assert.Equal(t, "detail", r.Route(), "Back shows what 06 stood on")
		r.Tester.Elapse(10 * second)
		assert.Zero(t, pb.Count("AutoAdvance"), "Back cancels the countdown")
		assert.Zero(t, pb.Count("Leave"))
	})
}

// TestDoneCountdownDoesNotAllocate is the gate of the countdown: a tick of the second
// and the paint of its text allocate nothing, and neither does a paint with
// nothing new.
func TestDoneCountdownDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r, _, d := atDone(t, ph, false)
		r.Fake.FakePrefs().SetAutoNext(false) // 0 asks, nothing plays: the screen stays
		step := func() {
			r.Tester.Elapse(second)
		}
		for range doneSeconds { // every second's text is shaped once
			step()
		}
		require.Zero(t, d.Seconds())
		d.Restart()
		require.Equal(t, 5, d.Seconds())
		n := 0
		for d.Seconds() > 1 && n < 10 {
			n++
			assert.Zero(t, testing.AllocsPerRun(1, step), ph.Name+": a tick of the countdown and its paint")
		}
		steady := func() { r.Tester.Repaint() }
		steady()
		assert.Zero(t, testing.AllocsPerRun(100, steady), ph.Name+": a paint with nothing new")
	}
}
