//go:build tvos

package screens_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// detailOf opens TV06 of a course cut to n lessons (the mock's has ten).
func detailOf(t testing.TB, n int) *rig {
	t.Helper()
	svc := tvtest.NewSignedServices(t)
	if n < len(svc.CourseData.Lessons) {
		svc.CourseData.Lessons = svc.CourseData.Lessons[:n]
	}
	r := openAtFast(t, svc, nav.Route{Screen: env.RouteDetail, Params: "set-35"})
	r.settle()

	return r
}

// TestDetailRightFromTheButtonsGoesToTheTopOfTheList checks K33: from "Quay lại" (`d2`),
// the last of the buttons, Right lands on the first lesson in sight, whatever the length
// of the course. The prototype's pick (centre to centre, `moveFocus`) lands on the row at
// the buttons' height: the last row of a course of 8 lessons or fewer, `dl7` of a longer
// one. The first button still goes to the second, and a list scrolled down is not
// scrolled back: the first row in sight is its top.
func TestDetailRightFromTheButtonsGoesToTheTopOfTheList(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 2, 3, 5, 8, 10, 12} {
		t.Run(fmt.Sprintf("%d lessons", n), func(t *testing.T) {
			t.Parallel()
			r := detailOf(t, n)
			require.Equal(t, "d1", r.key(), "d1 first (03 section 7.2)")
			require.True(t, r.press(keys.KeyRight))
			assert.Equal(t, "d2", r.key(), "Xem bài 01 -> Quay lại")
			require.True(t, r.press(keys.KeyRight))
			assert.Equal(t, "dl0", r.key(), "Quay lại -> the first lesson")
			require.True(t, r.press(keys.KeyLeft))
			assert.Equal(t, "d2", r.key(), "and back, as the prototype does")
		})
	}

	t.Run("a scrolled list", func(t *testing.T) {
		t.Parallel()
		r := detailOf(t, 10)
		r.focusKey("dl0")
		for range 9 {
			require.True(t, r.press(keys.KeyDown))
		}
		require.Equal(t, "dl9", r.key())
		r.tt.Elapse(tvtestTransition)
		clipTop := r.item("dl9").Node().WindowFrame().Y
		require.True(t, r.press(keys.KeyLeft))
		require.Equal(t, "d2", r.key())
		require.True(t, r.press(keys.KeyRight))
		got := r.key()
		assert.NotEqual(t, "dl9", got, "the first row in sight, not the one it left")
		f := r.item(got).Node().WindowFrame()
		assert.GreaterOrEqual(t, f.Max().Y, float32(110), "in sight: not above the top of the list's clip")
		assert.Less(t, f.Y, clipTop, "above the last row")
	})
}

// tvtestTransition is long enough for the list's glide (tvkit.Transition is 200 ms).
const tvtestTransition = 400 * time.Millisecond

// TestDetailVectorsOnlyDifferAtTheListKey checks TV06 against the prototype's vectors
// (internal/ui/tv/focus/testdata/screen_vectors.json): every move the prototype's pick
// makes is the app's, but Right from `d2`, which K33 sends to the top of the list. A
// second deviation would show here.
func TestDetailVectorsOnlyDifferAtTheListKey(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../focus/testdata/screen_vectors.json")
	require.NoError(t, err)
	var vf vecFile
	require.NoError(t, json.Unmarshal(raw, &vf))
	for _, sc := range vf.Screens {
		if sc.Screen != "TV06" {
			continue
		}
		r := detailOf(t, 10)
		differ := 0
		for _, c := range sc.Cases {
			if !r.pristineFrames(t) {
				r = detailOf(t, 10)
			}
			require.True(t, r.refocus(c.From), c.From)
			r.press(vecKeys[c.Dir])
			got := r.key()
			if c.From == "d2" && c.Dir == "ArrowRight" {
				assert.Equal(t, "dl0", got, "K33")
				assert.Equal(t, "dl7", c.To, "the prototype's pick")
				differ++

				continue
			}
			assert.Equalf(t, c.To, got, "%s %s", c.From, c.Dir)
		}
		assert.Equal(t, 1, differ)

		return
	}
	t.Fatal("no vectors for TV06")
}

// pristineFrames tells whether the detail has the list where the first build had it
// (a move that scrolled it asks for a new rig).
func (r *rig) pristineFrames(t testing.TB) bool {
	t.Helper()
	f := r.item("dl0")

	return f != nil && f.Node().WindowFrame().Y == 118
}
