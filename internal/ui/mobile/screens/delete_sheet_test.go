//go:build !tvos

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openDeleteSheet opens sheet 14 over screen 11 for lesson 3, as the button of its
// row does, and lets it slide in.
func openDeleteSheet(t testing.TB, ph kittest.Phone, real bool) *shelltest.Rig {
	t.Helper()
	r := atDownloads(t, ph, real)
	tap(r, named(r.Shell, "askDelete:set35-3", 1))
	r.Settle()
	slideIn(r)
	require.True(t, r.Shell.Nav().Sheet().IsOpen())

	return r
}

// TestDeleteSheetBoxesMatchPrototype lays sheet 14 out over screen 11 with the
// iPhone's real text engine and compares every element of M14 with the prototype's
// within 1 DIP: the panel and its grab, the disc, the question, the body, the two
// buttons, and screen 11 under the dim.
func TestDeleteSheetBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := openDeleteSheet(t, kittest.Phones[0], true)
	d := r.Dump("M14")
	skip := map[string]bool{"span.muted": true} // the value's box, 0,2 narrower in Core Text
	checkAgainstProto(t, "M14", d, 1, skip)
	want := shelltest.ProtoLayout(t, "M14")
	w, _ := want.Find("span.muted")
	g, ok := d.Find("span.muted")
	require.True(t, ok)
	assert.InDelta(t, w.X, g.X, .5)
	assert.InDelta(t, w.W, g.W, .5)
}

// TestDeleteSheetOnTheWiderPhone checks the 412 wide phone's sheet against the
// iPhone's: the panel as wide as the window on its bottom, the parts in their
// order and heights, the buttons spanning the margins, the disc in the middle.
func TestDeleteSheetOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	a := openDeleteSheet(t, kittest.Phones[0], true).Dump("M14")
	b := openDeleteSheet(t, kittest.Phones[1], true).Dump("M14")
	pa, _ := a.Find("div.sheet")
	pb, _ := b.Find("div.sheet")
	assert.InDelta(t, 412, pb.W, .01)
	assert.InDelta(t, 892, pb.Y+pb.H, .01)
	assert.InDelta(t, pa.H, pb.H, 1.5, "as tall as on the iPhone (the body may wrap another way)")
	for _, k := range []string{"div.d", "delConfirm", "closeSheet#2"} {
		ea, _ := a.Find(k)
		eb, ok := b.Find(k)
		require.True(t, ok, k)
		assert.InDelta(t, 372, eb.W, .6, k+": spans the margins")
		assert.InDelta(t, ea.H, eb.H, .6, k+": height")
		assert.InDelta(t, ea.Y-pa.Y, eb.Y-pb.Y, 1.5, k+": place in the sheet")
	}
	da, _ := a.Find("svg.i#18")
	db, _ := b.Find("svg.i#18")
	assert.InDelta(t, da.X+9.5, db.X, .6, "the disc stays in the middle")
}

// TestDeleteSheetSaysWhatGoes checks the texts of sheet 14: the question, the file
// with its number, title and size, what stays, the two buttons.
func TestDeleteSheetSaysWhatGoes(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := openDeleteSheet(t, ph, false)
		s, ok := r.Shell.Nav().Sheet().Content().(*screens.DeleteSheet)
		require.True(t, ok)
		assert.Equal(t, env.DeleteSheetParams{CourseID: "set-35", LessonID: "l_3503", No: 3}, s.Params)
		for _, text := range []string{
			"Xoá file âm thanh?",
			"03 · Stink and Dutty · 8,8 MB sẽ bị xoá khỏi máy.\nBạn vẫn xem video bài này khi có mạng.",
			"Xoá khỏi máy", "Huỷ",
		} {
			assert.True(t, r.Tester.Shows(text), text)
		}
	})
}

// TestDeleteSheetActions checks the buttons: "Xoá khỏi máy" is `delConfirm` (the
// file goes, the sheet closes, "Đã xoá file âm thanh" shows), "Huỷ" closes the sheet
// and deletes nothing.
func TestDeleteSheetActions(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("confirm", func(t *testing.T) {
			r := openDeleteSheet(t, ph, false)
			tap(r, named(r.Shell, "delConfirm", 1))
			r.Settle()
			assert.Equal(t, []string{"l_3503"}, r.Fake.Dl().Deleted())
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.True(t, shows(r, toastDeleted))
		})
		t.Run("cancel", func(t *testing.T) {
			r := openDeleteSheet(t, ph, false)
			tap(r, named(r.Shell, "closeSheet", 2))
			r.Settle()
			assert.Empty(t, r.Fake.Dl().Deleted())
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.Equal(t, 5, downloadsOf(t, r).Files())
		})
		t.Run("dim", func(t *testing.T) {
			r := openDeleteSheet(t, ph, false)
			tap(r, named(r.Shell, "closeSheet", 1))
			r.Settle()
			assert.Empty(t, r.Fake.Dl().Deleted())
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		})
		t.Run("from the detail", func(t *testing.T) {
			r := atDetail(t, ph, false, "set-35")
			r.Shell.DlTap("set-35", "l_3503", 3) // a lesson with a file
			r.Settle()
			slideIn(r)
			assert.True(t, r.Tester.Shows("03 · Stink and Dutty · 8,8 MB sẽ bị xoá khỏi máy.\nBạn vẫn xem video bài này khi có mạng."))
			tap(r, named(r.Shell, "delConfirm", 1))
			r.Settle()
			assert.Equal(t, []string{"l_3503"}, r.Fake.Dl().Deleted())
		})
	})
}

// TestDeleteSheetSteadyFrameDoesNotAllocate checks that a frame of the open sheet
// standing still allocates nothing, on both phones.
func TestDeleteSheetSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := openDeleteSheet(t, ph, false)
		warmDownloads(r)
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
	}
}
