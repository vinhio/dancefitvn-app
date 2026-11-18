//go:build !tvos

package screens_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spokenDamaged is a11y.fileDamaged (K49).
const spokenDamaged = "file tải về bị lỗi, đang phát trực tuyến"

// warnIcons counts the icons of the warning glyph in --dg under w.
func warnIcons(w ui.Widget) int {
	n := 0
	var walk func(ui.Widget)
	walk = func(w ui.Widget) {
		if ic, ok := w.(*kit.Icon); ok && ic.Glyph == icon.Warn && ic.Ink == theme.InkDanger {
			n++
		}
		for ch := range w.Node().Children() {
			walk(ch)
		}
	}
	walk(w)

	return n
}

// chipOkc returns the downloaded chip under w.
func chipOkc(w ui.Widget) *kit.Chip {
	var found *kit.Chip
	var walk func(ui.Widget)
	walk = func(w ui.Widget) {
		if c, ok := w.(*kit.Chip); ok && c.Kind == kit.ChipOkc {
			found = c
		}
		for ch := range w.Node().Children() {
			walk(ch)
		}
	}
	walk(w)

	return found
}

// spokenCount counts the elements of the tree whose label or value holds spokenDamaged.
func spokenCount(sem *platform.Semantics) int {
	n := 0
	for _, nd := range sem.Nodes {
		if strings.Contains(nd.Label, spokenDamaged) || strings.Contains(nd.Value, spokenDamaged) {
			n++
		}
	}

	return n
}

// TestDamagedFileMarkedInTheCourseRow is K49 on both phones: the row of a lesson whose
// file is judged unplayable has the warning in --dg in place of the download icon in
// --ok and says so; the sound files keep theirs; deleting the file or downloading it
// again clears the mark.
func TestDamagedFileMarkedInTheCourseRow(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, true, "set-35")
		dl := r.Fake.Dl()
		dlIcon := func(key string) *kit.Icon {
			b := named(r.Tester.View().Root(), "dlTap:"+key, 1)
			require.NotNil(t, b, key)

			return named(b, "svg.i", 1).(*kit.Icon)
		}
		label := func(no string) string {
			b := named(r.Tester.View().Root(), "dlTap:set35-"+no, 1)
			require.NotNil(t, b)

			return b.(ui.Labeler).AccessibleLabel()
		}
		require.Equal(t, icon.Dl, dlIcon("set35-3").Glyph, "a sound file")
		require.Equal(t, theme.InkSuccess, dlIcon("set35-3").Ink)

		dl.SetDamaged("l_3503", true)
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, icon.Warn, dlIcon("set35-3").Glyph, "the warning in place of the download icon")
		assert.Equal(t, theme.InkDanger, dlIcon("set35-3").Ink, "--dg")
		assert.Equal(t, spokenDamaged, label("3"))
		assert.Equal(t, icon.Dl, dlIcon("set35-1").Glyph, "the other files are as they were")
		assert.Equal(t, theme.InkSuccess, dlIcon("set35-1").Ink)
		assert.Equal(t, "Đã tải", label("1"))
		assert.Equal(t, 1, spokenCount(r.Tester.Semantics()), "one spoken state")

		dl.SetDamaged("l_3503", false)
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, icon.Dl, dlIcon("set35-3").Glyph, "the mark cleared")
		assert.Equal(t, "Đã tải", label("3"))

		dl.SetDamaged("l_3503", true)
		r.Tester.Elapse(kittest.Frame)
		require.NoError(t, dl.Delete("l_3503"))
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, icon.Dl, dlIcon("set35-3").Glyph, "deleted: no file, no mark")
		assert.Equal(t, theme.InkText3, dlIcon("set35-3").Ink)
		dl.SetDownload("l_3503", kit.DlDone, 1) // downloaded again
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, icon.Dl, dlIcon("set35-3").Glyph, "a file downloaded again is judged anew")
		assert.Equal(t, theme.InkSuccess, dlIcon("set35-3").Ink)
		assert.Zero(t, spokenCount(r.Tester.Semantics()))
	})
}

// TestDamagedFileMarkedInTheDownloadsRow is K49 on screen 11: the row of the file has
// the 16 warning before its size and says so; sound rows have none; the mark goes with
// the file.
func TestDamagedFileMarkedInTheDownloadsRow(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDownloads(t, ph, true, func(d *shelltest.FakeDownloads) { d.SetDamaged("l_3503", true) })
		root := func() ui.Widget { return r.Tester.View().Root() }
		bad := func() ui.Widget { return named(root(), "playAudio:set35-3", 1) }
		require.NotNil(t, bad())
		assert.Equal(t, 1, warnIcons(bad()), "the marker of the damaged file")
		assert.Equal(t, 1, warnIcons(root()), "and only its row has one")
		assert.Zero(t, warnIcons(named(root(), "playAudio:set35-1", 1)), "a sound file has none")
		assert.Equal(t, 1, spokenCount(r.Tester.Semantics()))

		dl := r.Fake.Dl()
		require.NoError(t, dl.Delete("l_3503"))
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		assert.Zero(t, warnIcons(root()), "deleted: the row and its mark are gone")
		dl.SetDownload("l_3503", kit.DlDone, 1)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		assert.Zero(t, warnIcons(root()), "downloaded again: judged anew")
		assert.Zero(t, spokenCount(r.Tester.Semantics()))

		dl.SetDamaged("l_3504", true) // a verdict arriving while the screen shows
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		assert.Equal(t, 1, warnIcons(named(root(), "playAudio:set35-4", 1)))
	})
}

// TestDamagedFileMarkedOnTheAudioScreen is K49 on screen 08: the chip of the downloaded
// file is the warning on the red of .16, says that the file is bad and the sound
// streams, and goes back when the file is deleted; a sound file keeps its green chip.
func TestDamagedFileMarkedOnTheAudioScreen(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, _ := atAudio(t, ph, true)
		root := func() ui.Widget { return r.Tester.View().Root() }
		chip := chipOkc(root())
		require.NotNil(t, chip)
		require.Equal(t, icon.Dl, chip.Icon, "a sound file")
		assert.Zero(t, spokenCount(r.Tester.Semantics()))

		r.Fake.Dl().SetDamaged("l_3503", true)
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, icon.Warn, chip.Icon, "the warning in the chip")
		assert.True(t, r.Tester.Shows("Đã tải · 8,8 MB"), "the chip keeps its text")
		assert.Equal(t, 1, spokenCount(r.Tester.Semantics()), "spoken once")
		bg := chip.Bg
		assert.Equal(t, tokens.Dark.Danger, chip.Fg, "--dg")
		assert.InDelta(t, .16, float64(bg.A)/255, .01, "on the red of .16")

		r.Fake.Dl().SetDamaged("l_3503", false)
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, icon.Dl, chip.Icon)
		assert.Zero(t, spokenCount(r.Tester.Semantics()))

		r.Fake.Dl().SetDamaged("l_3503", true)
		r.Tester.Elapse(kittest.Frame)
		require.NoError(t, r.Fake.Dl().Delete("l_3503"))
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, r.Tester.Shows("Đang phát trực tuyến"), "the file is gone: the streaming chip")
		assert.Zero(t, spokenCount(r.Tester.Semantics()))
	})
}
