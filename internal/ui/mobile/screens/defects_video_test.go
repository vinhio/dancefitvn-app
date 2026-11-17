//go:build !tvos

package screens_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVideoSideColumnClearsALongTitle gives the player a lesson whose title and
// course line wrap to several lines, so that the panel grows up past the prototype's
// `top:300px` of the side column: the column ends above the panel (the "Âm thanh"
// button never lies over the title), on both phones.
func TestVideoSideColumnClearsALongTitle(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		long := strings.Repeat("Một bài học có tên khá dài ", 2)
		l := pb.Lesson()
		l.ID, l.Title, l.CourseName, l.Tagline = "l_long", long, "Set 35", "Tên"
		first := pb.Lesson()
		pb.Set(first, player.KindVideo, player.StatePaused, 10, 100)
		pb.SetList([]player.Lesson{first, l})
		pb.Next() // announces the lesson, whose title is the long one
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		r.Tester.Elapse(kittest.Frame)

		lay := r.Dump("long")
		panelTop := float64(ph.Size.H)
		var sideBottom float64
		for _, e := range lay.Elements {
			if e.Key == "div.d" {
				panelTop = e.Y
				assert.Greater(t, e.H, float64(40), "the long title wraps")
			}
			if strings.HasPrefix(e.Key, "dlTap") || e.Key == "mirror" || e.Key == "loop" || e.Key == "speed" || e.Key == "dlTap" {
				sideBottom = max(sideBottom, e.Y+e.H)
			}
		}
		require.NotZero(t, sideBottom, "the side buttons are in the dump")
		assert.LessOrEqual(t, sideBottom, panelTop+.5, "the side column ends over the title")
	})
}

// TestVideoTopButtonsHaveALightRing checks K35: the back and "…" circles of the
// player are drawn with a light ring, so their edge is brighter than their inside
// and they stay visible over the black band of a letterboxed video, on both phones.
func TestVideoTopButtonsHaveALightRing(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, _ := atVideo(t, ph, false)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()

		shot := r.Tester.Snapshot()
		found := 0
		for _, e := range r.Dump("ring").Elements {
			if e.Key != "back" && e.Key != "sheet:options" {
				continue
			}
			found++
			y := int(e.Y + e.H/2)
			edge := shot.RGBAAt(int(e.X+.5), y)
			inside := shot.RGBAAt(int(e.X+6), y)
			assert.Greater(t, int(edge.R)+int(edge.G)+int(edge.B), int(inside.R)+int(inside.G)+int(inside.B),
				"%s: the ring is lighter than the disc", e.Key)
		}
		require.Equal(t, 2, found, "both top buttons are in the dump")
	})
}
