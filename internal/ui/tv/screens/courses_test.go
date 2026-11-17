//go:build tvos

package screens_test

import (
	"errors"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/tv/env"
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

// courses returns the Courses screen showing.
func (r *rig) courses() *screens.Courses {
	r.t.Helper()
	c, ok := r.sh.Screen().(*screens.Courses)
	require.True(r.t, ok, "the screen showing is TV05")

	return c
}

// walkTo presses Right and then Left, at most four times each, until the focus is
// on key; it fails the test when it never is.
func (r *rig) walkTo(key string) {
	r.t.Helper()
	for _, k := range []keys.Key{keys.KeyRight, keys.KeyLeft} {
		for range 4 {
			if r.key() == key {
				return
			}
			r.press(k)
		}
	}
	require.Equal(r.t, key, r.key())
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestCoursesBoxesMatchPrototype compares every element of TV05.layout.json with
// the screen's: the heading, the nav bar, the filters (`cf0` focused, so at its
// scaled box), and the fifteen cells, those below the fold too. The filters after
// the first two sit up to 3.2 px off: the digits of "15" and "12" are 1.6 px
// narrower in Core Text's Inter than in Chrome's, and the chips stack the error.
func TestCoursesBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteCourses, true)
	r.settle()
	require.Equal(t, "cf0", r.key())
	r.tt.Elapse(tvkit.Transition)
	r.boxesLoose(t, "TV05", map[string]float32{"cf2": 3.5, "cf3": 3.5})
}

// TestCoursesText: the strings are the prototype's, filled from the courses of the
// API: the filters with their counts, the name with its quote, the category line
// in the color of its category.
func TestCoursesText(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteCourses, true)
	r.settle()
	lay := r.layout()
	want := map[string]string{
		"div.d":                 "KHOÁ HỌC CỦA TÔI",
		"cf0":                   "Tất cả · 15",
		"cf1":                   "Thường · 12",
		"cf2":                   "Đặc biệt · 2",
		"cf3":                   "Giới thiệu · 1",
		"tv.courses.caption":    "DanceFIT Set 35 · “Cùng nhảy cùng hát”",
		"tv.courses.meta":       "THƯỜNG · 10 BÀI HỌC",
		"tv.courses.caption#3":  "DanceFIT Hot Extra · “Hot Extra”",
		"tv.courses.meta#3":     "ĐẶC BIỆT · 10 BÀI HỌC",
		"tv.courses.caption#7":  "DanceFIT Set 30",
		"tv.courses.meta#15":    "GIỚI THIỆU · 3 BÀI HỌC",
		"tv.courses.caption#15": "Giới thiệu DanceFIT · “Làm quen với DanceFIT”",
		"tv.courses.caption#14": "DanceFIT Hot Extra 2",
		"nav1":                  "Khoá học",
		"cg13":                  "DANCEFIT\nHOT EXTRA 2",
		"cg14":                  "DANCEFIT\nGIỚI THIỆU",
	}
	for key, txt := range want {
		e, ok := lay.Find(key)
		require.True(t, ok, key)
		assert.Equal(t, txt, e.Text, key)
	}
	colors := map[string]string{
		"tv.courses.meta":    "rgb(255, 107, 110)", // #FF6B6E
		"tv.courses.meta#3":  "rgb(77, 163, 255)",  // --info
		"tv.courses.meta#15": "rgb(61, 214, 140)",  // --ok
	}
	for key, col := range colors {
		e, _ := lay.Find(key)
		assert.Equal(t, col, e.Style.Color, key)
	}
}

// TestCoursesWaitsWithTheNavBarOnly: until the courses are read the screen is the
// nav bar over the background (K12) with the focus where `cf0` will be, and when
// they come it is on the real `cf0`.
func TestCoursesWaitsWithTheNavBarOnly(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteCourses, false)
	c := r.courses()
	assert.Nil(t, c.Chip(0))
	assert.Zero(t, c.Cells())
	assert.Equal(t, "cf0", r.key())
	r.settle()
	require.NotNil(t, c.Chip(0))
	assert.Equal(t, 15, c.Cells())
	assert.Equal(t, "cf0", r.key())
	assert.True(t, c.Chip(0).IsFocused(), "the real chip has the focus, not its stand-in")
	assert.Empty(t, r.tt.Leaks())
}

// TestCoursesGridScrollOffsets: Down from `cf0` goes into the grid, and each row
// down moves the grid to the offset `scrollTV` computes: 0, 100, 536, 972, 1408
// (index.html:729); Up goes back, the offset following.
func TestCoursesGridScrollOffsets(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteCourses, true)
	r.settle()
	sl := r.courses().Slide()
	require.NotNil(t, sl)
	require.Equal(t, float32(0), sl.Offset())

	steps := []struct {
		key    string
		offset float32
	}{
		{"cg0", 0}, {"cg3", 100}, {"cg6", 536}, {"cg9", 972}, {"cg12", 1408},
	}
	for _, st := range steps {
		require.True(t, r.press(keys.KeyDown))
		assert.Equal(t, st.key, r.key())
		assert.Equal(t, st.offset, sl.Offset(), st.key)
	}
	r.press(keys.KeyDown)
	assert.Equal(t, "cg12", r.key(), "nothing under the last row")
	for i := len(steps) - 2; i >= 0; i-- {
		require.True(t, r.press(keys.KeyUp))
		assert.Equal(t, steps[i].key, r.key())
		assert.Equal(t, steps[i].offset, sl.Offset(), steps[i].key)
	}
	assert.Empty(t, r.tt.Leaks())
}

// TestCoursesGridGlides: the grid is painted gliding to its offset (the 250 ms of
// the glide is pinned by the tvkit Slide tests, each frame being costly here).
func TestCoursesGridGlides(t *testing.T) {
	t.Parallel()
	r := open(t, tvtest.NewSignedServices(t), env.RouteCourses, false)
	r.settle()
	sl := r.courses().Slide()
	r.tt.Key(keys.KeyDown, 0)
	r.tt.Elapse(tvtest.Frame)
	r.tt.Key(keys.KeyDown, 0)
	r.tt.Elapse(tvtest.Frame)
	require.Positive(t, sl.Offset(), "the second row is under the fold")
	assert.True(t, sl.Gliding())
	assert.Less(t, sl.Shown(), sl.Offset(), "the picture is behind the offset the focus picks by")
}

// TestCoursesFilterSelect: Select on a filter keeps the focus on the chip, makes it
// the selected one and shows its category's courses from the top.
func TestCoursesFilterSelect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		chip  string
		cells int
		first string // the course under `cg0`
	}{
		{"cf1", 12, "set-35"},
		{"cf2", 2, "hot-extra"},
		{"cf3", 1, "intro"},
	}
	for _, tc := range tests {
		t.Run(tc.chip, func(t *testing.T) {
			t.Parallel()
			r := openFast(t, tvtest.NewSignedServices(t), env.RouteCourses, true)
			r.settle()
			c := r.courses()
			// scroll the grid one row, then come back to the chips
			r.press(keys.KeyDown)
			r.press(keys.KeyDown)
			require.Equal(t, float32(100), c.Slide().Offset())
			r.press(keys.KeyUp)
			r.press(keys.KeyUp)
			require.Equal(t, "cf", r.key()[:2])
			r.walkTo(tc.chip)
			require.Equal(t, tc.chip, r.key())

			require.True(t, r.press(keys.KeyEnter))
			assert.Equal(t, tc.chip, r.key(), "the focus stays on the chip")
			assert.Equal(t, env.RouteCourses, r.sh.Stack().Screen())
			assert.Equal(t, tc.cells, c.Cells())
			assert.Equal(t, float32(0), c.Slide().Offset(), "the grid starts from the top")
			assert.Equal(t, float32(0), c.Slide().Shown())
			for i := range 4 {
				assert.Equal(t, "cf"+string(rune('0'+i)) == tc.chip, c.Chip(i).Selected, "cf%d", i)
			}

			r.press(keys.KeyDown)
			assert.Equal(t, "cg0", r.key())
			require.True(t, r.press(keys.KeyEnter))
			cur, _ := r.sh.Stack().Current()
			assert.Equal(t, env.RouteDetail, cur.Screen)
			assert.Equal(t, tc.first, cur.Params)
		})
	}
}

// TestCoursesFilterChipsAreLaidOutAgain: the selected chip has no border, so it is
// 4 narrower than it is not: selecting another swaps the widths and the chips move.
func TestCoursesFilterChipsAreLaidOutAgain(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteCourses, true)
	r.settle()
	before := r.layout()
	cf0, _ := before.Find("cf0")
	r.press(keys.KeyRight)
	require.Equal(t, "cf1", r.key())
	require.True(t, r.press(keys.KeyEnter))
	r.tt.Elapse(tvkit.Transition)
	after := r.layout()
	n0, _ := after.Find("cf0")
	assert.InDelta(t, cf0.W/1.06+4, n0.W, 0.5, "cf0 is not selected now: it has the border")
}

// TestCoursesOpenACourse: Select on a tile opens TV06 of its course over TV05, and
// Menu comes back to TV05 with `cf0` focused (no memory).
func TestCoursesOpenACourse(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteCourses, true)
	r.settle()
	r.press(keys.KeyDown)
	r.press(keys.KeyRight)
	r.press(keys.KeyRight)
	require.Equal(t, "cg2", r.key())
	require.True(t, r.press(keys.KeyEnter))
	cur, _ := r.sh.Stack().Current()
	assert.Equal(t, env.RouteDetail, cur.Screen)
	assert.Equal(t, "hot-extra", cur.Params)

	assert.True(t, r.press(keys.KeyEscape))
	r.settle()
	assert.Equal(t, env.RouteCourses, r.sh.Stack().Screen())
	assert.Equal(t, "cf0", r.key())
	assert.Empty(t, r.tt.Leaks())
}

// TestCoursesMenuIsLeftToTheSystem: TV05 is a root, Menu is not used there.
func TestCoursesMenuIsLeftToTheSystem(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteCourses, false)
	r.settle()
	assert.False(t, r.press(keys.KeyEscape))
	assert.Equal(t, env.RouteCourses, r.sh.Stack().Screen())
}

// TestCoursesNavSwitchesInPlace: Select on "Trang chủ" shows TV04 and keeps the focus
// on the nav item.
func TestCoursesNavSwitchesInPlace(t *testing.T) {
	t.Parallel()
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteCourses, true)
	r.settle()
	require.True(t, r.press(keys.KeyUp))
	r.walkTo("nav0")
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
	assert.Equal(t, "nav0", r.key())
	r.settle()
	assert.Equal(t, "nav0", r.key())
}

// TestCoursesReadFailsShowsTV11: a failed read puts TV11 over the screen (K28).
func TestCoursesReadFailsShowsTV11(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewSignedServices(t)
	svc.ReadErr = errors.New("boom")
	r := openFast(t, svc, env.RouteCourses, false)
	r.settle()
	assert.Equal(t, env.RouteError, r.sh.Stack().Screen())
	assert.Equal(t, "e1", r.key())
	// "Thử lại" with the network back goes to TV05, which reads again
	svc.ReadErr = nil
	assert.True(t, r.sh.Back())
	r.settle()
	assert.Equal(t, env.RouteCourses, r.sh.Stack().Screen())
	assert.Equal(t, 15, r.courses().Cells())
}

// TestCoursesSteadyFrameDoesNotAllocate: a repaint of TV05 with the focus settled
// allocates nothing.
func TestCoursesSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteCourses, false)
	r.settle()
	r.tt.Elapse(time.Second)
	r.tt.Repaint()
	assert.Zero(t, testing.AllocsPerRun(20, r.tt.Repaint))
}

// TestCoursesGridFocusDoesNotAllocate: Down and Up between the first two rows of the
// grid, with the frames of the focus animation and of the grid's glide after each,
// allocate nothing.
func TestCoursesGridFocusDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := openFast(t, tvtest.NewSignedServices(t), env.RouteCourses, false)
	r.settle()
	r.press(keys.KeyDown)
	down := true
	step := func() {
		k := keys.KeyDown
		if !down {
			k = keys.KeyUp
		}
		down = !down
		r.tt.Key(k, 0)
		r.tt.Elapse(tvkit.SlideTime + 20*time.Millisecond)
	}
	for range 4 {
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(10, step))
}
