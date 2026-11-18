//go:build tvos

package tvkit_test

import (
	"fmt"
	"image"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// rowW is the width the settings card gives the row: 600 less the 30 of padding on
// each side (index.html:710).
const rowW float32 = 540

// rowStage holds a MedalRow at (30, 500) as wide as the card's row, as tall as it
// asks.
type rowStage struct {
	ui.Base
	row *tvkit.MedalRow
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Layout puts the row where the settings card does.
func (s *rowStage) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(s.row, ui.Constraints{Min: geom.Sz(rowW, 0), Max: geom.Sz(rowW, ui.Unbounded)})
	c.Place(s.row, geom.Pt(30, 500))

	return cs.Constrain(cs.Max)
}

// medalRig mounts a row of n medals named "BADGE 0".."BADGE n-1".
func medalRig(t testing.TB, n int) (*tvkit.MedalRow, *rowStage, *ui.Tester) {
	t.Helper()
	medals := make([]*tvkit.Medal, n)
	for i := range medals {
		medals[i] = tvkit.NewMedal(fmt.Sprintf("BADGE %d", i), color.RGB(0x7a, 0xc9, 0x43), "SUB")
	}
	s := &rowStage{row: tvkit.NewMedalRow(medals)}
	s.Add(s.row)
	tt := mount(t, s)
	tt.Elapse(tvtest.Frame)

	return s.row, s, tt
}

// labels returns what a screen reader reads of the tree.
func labels(tt *ui.Tester) map[string]bool {
	out := map[string]bool{}
	for _, n := range tt.Semantics().Nodes {
		out[n.Label] = true
	}

	return out
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestMedalRowPages: as many medals as fit the prototype's three are one page, the
// prototype's row; more are shared among as few pages as hold them, evenly.
func TestMedalRowPages(t *testing.T) {
	t.Parallel()
	for n, want := range map[int][]int{
		1: {1}, 2: {2}, 3: {3}, 4: {2, 2}, 5: {3, 2}, 6: {3, 3}, 7: {3, 2, 2}, 8: {3, 3, 2}, 10: {3, 3, 2, 2}, 12: {3, 3, 3, 3}, 20: {3, 3, 3, 3, 3, 3, 2},
	} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			row, _, _ := medalRig(t, n)
			require.Equal(t, len(want), row.Pages())
			got := make([]int, row.Pages())
			for i := range n {
				got[row.PageOf(i)]++
			}
			assert.Equal(t, want, got)
		})
	}
}

// TestMedalRowThreeOrFewerIsThePrototype: three medals stand as `space-around` puts
// them in the 540 (140 wide, 40 of space about each), with no dots, and never turn.
func TestMedalRowThreeOrFewerIsThePrototype(t *testing.T) {
	t.Parallel()
	row, s, tt := medalRig(t, 3)
	medals := make([]geom.Rect, 3)
	for i := range medals {
		medals[i] = row.Medal(i).Node().Frame()
		assert.InDelta(t, tvkit.MedalSize, medals[i].W, 1.5, "140 wide")
	}
	assert.InDelta(t, 20, medals[0].X, 1)
	assert.InDelta(t, 200, medals[1].X-(medals[1].W-medals[0].W)/2, 2)
	before := s.row.Node().Frame().H
	assert.Less(t, before, float32(240), "no line of dots under the medals")
	tt.Elapse(5 * tvkit.MedalPageTime)
	assert.Equal(t, 1, row.Pages())
	assert.Zero(t, row.Page())
	assert.False(t, row.Fading())
}

// untilTurn runs frames until the row shows another page and returns the time
// of the frame that showed it.
func untilTurn(t *testing.T, tt *ui.Tester, row *tvkit.MedalRow) time.Duration {
	t.Helper()
	was := row.Page()
	for range int(2 * tvkit.MedalPageTime / tvtest.Frame) {
		tt.Elapse(tvtest.Frame)
		if row.Page() != was {
			return tt.View().Context().Now()
		}
	}
	t.Fatal("the page never turned")

	return 0
}

// TestMedalRowTurnsThroughEveryBadge: with more than three medals a page stays
// MedalPageTime, the next one fades in over MedalFade, and round and round, so
// that every badge is on a page showing at some time.
func TestMedalRowTurnsThroughEveryBadge(t *testing.T) {
	t.Parallel()
	for _, n := range []int{4, 8, 12, 20} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			row, _, tt := medalRig(t, n)
			shown := map[int]bool{}
			show := func() {
				for i := range n {
					if row.PageOf(i) == row.Page() {
						shown[i] = true
					}
				}
			}
			show()
			at := tt.View().Context().Now()
			for turn := 1; turn <= row.Pages(); turn++ {
				now := untilTurn(t, tt, row)
				assert.InDelta(t, tvkit.MedalPageTime.Seconds(), (now - at).Seconds(), tvtest.Frame.Seconds()+.001, "a page stays its time")
				at = now
				assert.Equal(t, turn%row.Pages(), row.Page(), "turn %d", turn)
				assert.True(t, row.Fading(), "the page fades in")
				show()
				tt.Elapse(tvkit.MedalFade + tvtest.Frame)
				assert.False(t, row.Fading())
			}
			assert.Len(t, shown, n, "every badge was on the page showing")
			assert.Zero(t, row.Page(), "round to the first")
		})
	}
}

// TestMedalRowPagesAreAsTallAsTheTallest: the medals of every page are laid at the
// prototype's size, in the row's room, whatever page shows, so that the row never
// changes height and the dots sit under all of them.
func TestMedalRowPagesAreAsTallAsTheTallest(t *testing.T) {
	t.Parallel()
	row, s, tt := medalRig(t, 8)
	h := s.row.Node().Frame().H
	rf := s.row.Node().Frame()
	for i := range 8 {
		f := row.Medal(i).Node().Frame()
		assert.InDelta(t, tvkit.MedalSize, row.Medal(i).Size(), .01, "the prototype's size")
		assert.GreaterOrEqual(t, f.X, float32(0), "medal %d inside the row", i)
		assert.LessOrEqual(t, f.X+f.W, rowW+.5, "medal %d inside the row", i)
		assert.LessOrEqual(t, f.Y+f.H, h-10, "medal %d over the dots", i)
	}
	tt.Elapse(tvkit.MedalPageTime + tvkit.MedalFade + 2*tvtest.Frame)
	assert.Equal(t, rf, s.row.Node().Frame())
}

// TestMedalRowCrossFades: halfway through the fade the picture is neither the
// page that was nor the one coming; at its end it is the new page.
func TestMedalRowCrossFades(t *testing.T) {
	t.Parallel()
	row, _, tt := medalRig(t, 6)
	// the tester draws into one picture: a copy is what was seen then
	snap := func() *image.RGBA {
		img := tt.Snapshot()

		return &image.RGBA{Pix: append([]uint8(nil), img.Pix...), Stride: img.Stride, Rect: img.Rect}
	}
	same := func(a, b *image.RGBA) bool { return string(a.Pix) == string(b.Pix) }
	first := snap()
	tt.Elapse(tvkit.MedalPageTime)
	require.Equal(t, 1, row.Page())
	require.True(t, row.Fading())
	tt.Elapse(tvkit.MedalFade / 2)
	mid := snap()
	tt.Elapse(tvkit.MedalFade)
	last := snap()
	assert.False(t, same(first, mid), "the first page is going")
	assert.False(t, same(last, mid), "the second page is coming")
	assert.False(t, same(first, last), "the second page differs from the first")
	tt.Elapse(tvkit.MedalPageTime)
	require.Zero(t, row.Page())
	assert.True(t, same(first, snap()), "the pages come round to the first")
}

// TestMedalRowReducedMotionTurnsWithoutFading: with motion reduced the pages still
// turn, at once.
func TestMedalRowReducedMotionTurnsWithoutFading(t *testing.T) {
	t.Parallel()
	row, _, tt := medalRig(t, 8)
	tt.SetPreferences(platform.Preferences{ReduceMotion: true})
	tt.Elapse(tvkit.MedalPageTime)
	assert.Equal(t, 1, row.Page())
	assert.False(t, row.Fading(), "no fade")
	tt.Elapse(tvkit.MedalPageTime)
	assert.Equal(t, 2, row.Page())
}

// TestMedalRowStopsWhenLeft: a row taken off the screen turns no more.
func TestMedalRowStopsWhenLeft(t *testing.T) {
	t.Parallel()
	row, s, tt := medalRig(t, 8)
	s.Remove(row)
	tt.Elapse(3 * tvkit.MedalPageTime)
	assert.Zero(t, row.Page())
	s.Add(row)
	tt.Elapse(tvkit.MedalPageTime + tvtest.Frame)
	assert.Equal(t, 1, row.Page(), "back on the screen it turns again")
}

// TestMedalRowReadsEveryBadge: a screen reader finds every badge by its caption
// whichever page shows.
func TestMedalRowReadsEveryBadge(t *testing.T) {
	t.Parallel()
	row, _, tt := medalRig(t, 8)
	for range row.Pages() + 1 {
		got := labels(tt)
		for i := range 8 {
			assert.True(t, got[fmt.Sprintf("BADGE %d", i)], "badge %d at page %d", i, row.Page())
		}
		tt.Elapse(tvkit.MedalPageTime + tvkit.MedalFade + 2*tvtest.Frame)
	}
}

// TestMedalRowDoesNotAllocate: the frames of a page showing and of a page fading
// in allocate nothing.
func TestMedalRowDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	row, _, tt := medalRig(t, 8)
	tt.Elapse(time.Second)
	step := func() { tt.Elapse(tvtest.Frame) }
	step()
	assert.Zero(t, testing.AllocsPerRun(20, step), "a page showing")
	for !row.Fading() { // on to the turn
		step()
	}
	step()
	step()
	assert.Zero(t, testing.AllocsPerRun(10, step), "a page fading in")
	require.True(t, row.Fading(), "the runs were within the fade")
}
