//go:build tvos

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// playerAt opens TV07 on lesson no of the course (paused), with the motion reduced.
func playerAt(t testing.TB, no int) *rig {
	t.Helper()
	svc := tvtest.NewPlayerServices(t)
	svc.PB = tvtest.NewLessonPlayback(svc.CourseData, no)
	r := openFast(t, svc, env.RouteHome, false)
	r.settle()
	r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false)
	r.settle()

	return r
}

// TestPlayerListHasEveryLessonInOrder: the column lists all ten lessons of the
// course, top to bottom in course order, the ones after the lesson playing keeping
// the prototype's keys `pn0`.., and the heading is "BÀI HỌC" (K46).
func TestPlayerListHasEveryLessonInOrder(t *testing.T) {
	t.Parallel()
	r := playerAt(t, 3)
	rows := r.player().Rows()
	require.Len(t, rows, 10)
	want := []string{"pb0", "pb1", "pc", "pn0", "pn1", "pn2", "pn3", "pn4", "pn5", "pn6"}
	titles := []string{"DanceFIT Warm Up 13", "Made You Look", "Stink and Dutty", "Em Yêu Người Ta", "Hãy Hát Lên", "Nadi Nadi",
		"Viva La Vida", "We So Fly", "WET", "One Love"}
	lay := r.layout()
	lastY := float64(-1)
	for i, row := range rows {
		assert.Equal(t, want[i], row.Key)
		assert.Equal(t, titles[i], row.Title)
		e := find(t, lay, row.Key)
		assert.Greater(t, e.Y, lastY, "%s is below the one before", row.Key)
		lastY = e.Y
		assert.InDelta(t, 1340, e.X, boxTol)
		assert.InDelta(t, 490, e.W, boxTol)
	}
	assert.Equal(t, "BÀI HỌC", find(t, lay, "tv.player.next").Text)
	assert.Equal(t, "p1", r.key(), "the focus starts on the buttons as before")
}

// TestPlayerListMarksTheLessonPlaying: only the row of the lesson that plays is
// marked; a screen reader reads it as being watched.
func TestPlayerListMarksTheLessonPlaying(t *testing.T) {
	t.Parallel()
	r := playerAt(t, 3)
	for _, row := range r.player().Rows() {
		assert.Equal(t, row.Key == "pc", row.Current, row.Key)
	}
	sem := r.tt.Semantics()
	var got string
	for _, n := range sem.Nodes {
		if n.Label == "03 · Stink and Dutty" {
			got = n.Value
		}
	}
	assert.Equal(t, "Bụng · 03:40, đang xem", got)
	for _, n := range sem.Nodes {
		if n.Label == "04 · Em Yêu Người Ta" {
			assert.Equal(t, "Bachata · 04:10", n.Value, "the others are read as they were")
			assert.Equal(t, platform.RoleButton, n.Role)
		}
	}
}

// TestPlayerListShowsTheLessonPlaying: the list is scrolled so that the lesson that
// plays is in view: at the top for an early one, moved up for a late one.
func TestPlayerListShowsTheLessonPlaying(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		no     int
		scroll bool
	}{{1, false}, {3, false}, {6, false}, {9, true}, {10, true}} {
		r := playerAt(t, tc.no)
		p := r.player()
		var cur string
		for _, row := range p.Rows() {
			if row.Current {
				cur = row.Key
			}
		}
		require.NotEmpty(t, cur, "lesson %d", tc.no)
		e := find(t, r.layout(), cur)
		assert.GreaterOrEqual(t, e.Y, 140., "lesson %d: below the heading", tc.no)
		assert.LessOrEqual(t, e.Y+e.H, 1080., "lesson %d: on the screen", tc.no)
		assert.Equal(t, tc.scroll, p.Slide().Offset() > 0, "lesson %d: scrolled %v", tc.no, p.Slide().Offset())
		assert.Equal(t, "p1", r.key())
	}
}

// TestPlayerListFocusScrollsAndReachesEveryRow: Down from the rows reaches the last
// lesson, scrolling the list so that the focused row is in view, and Up goes back to
// the first.
func TestPlayerListFocusScrollsAndReachesEveryRow(t *testing.T) {
	t.Parallel()
	r := playerAt(t, 1)
	r.focusKey("pc")
	for i := range 9 {
		require.True(t, r.press(keys.KeyDown), "row %d", i)
	}
	assert.Equal(t, "pn8", r.key())
	e := find(t, r.layout(), "pn8")
	assert.LessOrEqual(t, e.Y+e.H, 1080.)
	assert.Greater(t, r.player().Slide().Offset(), float32(0))
	for range 9 {
		require.True(t, r.press(keys.KeyUp))
	}
	assert.Equal(t, "pc", r.key())
	assert.Zero(t, r.player().Slide().Offset(), "back at the top")
}

// TestPlayerListRowPlaysItsLesson: a row of a lesson before the one playing plays it
// too, and the playing one's row plays it again from where it was; the screen takes
// its own place.
func TestPlayerListRowPlaysItsLesson(t *testing.T) {
	t.Parallel()
	r := playerAt(t, 3)
	pb := r.playback()
	depth := r.sh.Stack().Depth()
	r.focusKey("pb0")
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	assert.Equal(t, 1, pb.Calls["PlayVideo"])
	assert.Equal(t, "l_3501", pb.LessonV.ID)
	assert.Equal(t, depth, r.sh.Stack().Depth(), "replaced, not pushed")
	assert.Equal(t, "p1", r.key())
	assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
	row := r.player().Rows()[0]
	assert.True(t, row.Current, "the list follows the lesson")
	assert.Equal(t, "pc", row.Key)
}
