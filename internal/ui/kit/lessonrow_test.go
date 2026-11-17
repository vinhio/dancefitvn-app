package kit_test

import (
	"fmt"
	"testing"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// m05Rows builds the ten rows of M05 (Set 35): lessons 1, 2, 4 and 6 downloaded,
// 3 the current one (downloaded), 7 downloading, the others without a file.
func m05Rows() ([]*kit.LessonRow, *kit.LessonList) {
	titles := []string{"DanceFIT Warm Up", "Made You Look", "Stink and Dutty", "Em Yêu Người Ta", "Hãy Hát Lên", "Nadi Nadi", "Viva La Vida", "We So Fly", "WET", "One Love"}
	list := &kit.LessonList{}
	var rows []*kit.LessonRow
	for i, title := range titles {
		n := i + 1
		r := &kit.LessonRow{No: n, Title: title, Meta: "Tay Vai · 03:20", Download: kit.DlNone,
			DlLabel: fmt.Sprintf("Tải bài %02d", n), PlayLabel: fmt.Sprintf("Phát bài %02d", n)}
		r.Name = fmt.Sprintf("playVideo:set35-%d", n)
		r.DlName = fmt.Sprintf("dlTap:set35-%d", n)
		r.PlayName = fmt.Sprintf("playAudio:set35-%d", n)
		switch n {
		case 1, 2, 4, 6:
			r.Download = kit.DlDone
		case 3:
			r.Download, r.State, r.Meta = kit.DlDone, kit.LessonCurrent, "Bụng · đang xem 01:12 / 03:20"
		case 7:
			r.Download, r.Progress = kit.DlIng, .64
		}
		rows = append(rows, r)
		list.Add(r)
	}

	return rows, list
}

// protoKey returns the key of the nth element named base in a layout.json:
// `base`, `base#2`...
func protoKey(base string, n int) string {
	if n <= 1 {
		return base
	}

	return fmt.Sprintf("%s#%d", base, n)
}

// TestLessonRowBoxesMatchPrototype compares the rows of M05 with its layout.json:
// each 58 tall, the current one 8 outside its column, the number, the text, the
// download button and its icon.
func TestLessonRowBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		rows, list := m05Rows()
		mountStageReal(t, ph, placeAt(list, 20, 435, ph.Size.W-40))
		for _, n := range []int{1, 2, 3, 4, 5, 7, 10} {
			r := rows[n-1]
			key := fmt.Sprintf("playVideo:set35-%d", n)
			if n == 3 {
				key += "#2" // the first is the card above
			}
			want := onPhone(ph, protoBox(t, "M05", key), anchorStretch)
			rectNear(t, want, frameOf(r), 1, key)
			rectNear(t, protoBox(t, "M05", protoKey("span.no", n)), frameOf(part(t, r, "span.no")), 1, "span.no")
			tt := onPhone(ph, protoBox(t, "M05", protoKey("span.tt", n)), anchorLeft)
			tt.W += ph.Size.W - 393
			rectNear(t, tt, frameOf(part(t, r, "span.tt")), 1, "span.tt")
			b := onPhone(ph, protoBox(t, "M05", protoKey("b", n)), anchorLeft)
			b.W += ph.Size.W - 393
			rectNear(t, b, frameOf(part(t, r, "b")), 1, "title")
			dlKey := fmt.Sprintf("dlTap:set35-%d", n)
			rectNear(t, onPhone(ph, protoBox(t, "M05", dlKey), anchorRight), frameOf(part(t, r, dlKey)), 1, dlKey)
			ik := protoKey("svg.i", n+3) // svg.i, #2 and #3 are the back button, the other round button and "Tải hết"
			rectNear(t, onPhone(ph, protoBox(t, "M05", ik), anchorRight), frameOf(part(t, part(t, r, dlKey), "svg.i")), 1, "dl icon of "+key)
		}
	})
}

// TestLessonRowOfflineBoxesMatchPrototype compares the rows of M13: the play disc
// of a downloaded lesson, the cloud of the others, the narrower text.
func TestLessonRowOfflineBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var rows []*kit.LessonRow
		list := &kit.LessonList{}
		for n := 1; n <= 7; n++ {
			r := &kit.LessonRow{No: n, Title: "Bài", Meta: "Tay Vai · Audio đã tải", Mode: kit.LessonOffline, Download: kit.DlNone}
			r.Name, r.PlayName = fmt.Sprintf("offTap:set35-%d", n), fmt.Sprintf("playAudio:set35-%d", n)
			switch n {
			case 1, 2, 3, 4, 6:
				r.Download = kit.DlDone
			default:
				r.Dim = true
			}
			if n == 3 {
				r.State = kit.LessonCurrent
			}
			rows = append(rows, r)
			list.Add(r)
		}
		mountStageReal(t, ph, placeAt(list, 20, 218, ph.Size.W-40))
		for _, n := range []int{1, 2, 3, 5} {
			r := rows[n-1]
			key := fmt.Sprintf("offTap:set35-%d", n)
			rectNear(t, onPhone(ph, protoBox(t, "M13", key), anchorStretch), frameOf(r), 1, key)
			tt := onPhone(ph, protoBox(t, "M13", protoKey("span.tt", n)), anchorLeft)
			tt.W += ph.Size.W - 393
			rectNear(t, tt, frameOf(part(t, r, "span.tt")), 1, "span.tt")
			if r.Download == kit.DlDone {
				pk := fmt.Sprintf("playAudio:set35-%d", n)
				rectNear(t, onPhone(ph, protoBox(t, "M13", pk), anchorRight), frameOf(part(t, r, pk)), 1, pk)
			}
		}
		// lesson 5: the cloud (svg.i#3 [351,465.5,22,22]), the row dim
		cloud := onPhone(ph, protoBox(t, "M13", "svg.i#3"), anchorRight)
		var got geom.Rect
		for ch := range rows[4].Children() {
			if ic, ok := ch.(*kit.Icon); ok && !ic.Hidden {
				got = frameOf(ic)
			}
		}
		rectNear(t, cloud, got, 1, "cloud")
	})
}

// TestLessonRowLooksLikeThePrototype checks the colors: the current row's tint and
// filled number, the number of a watched row, the dim row, the played disc.
func TestLessonRowLooksLikeThePrototype(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		rows, list := m05Rows()
		rows[4].State = kit.LessonWatched
		tt := mountStage(t, ph, placeAt(list, 20, 435, ph.Size.W-40))
		img := tt.Snapshot()
		cur := frameOf(rows[2])
		pixelNear(t, img, int(cur.X)+40, int(cur.Y)+3, over(look.C.LessonCur, look.T.Bg), 2, "current row: rgba(255,208,40,.12)")
		pixelNear(t, img, int(cur.X)+1, int(cur.Y)+1, look.T.Bg, 12, "rounded 12")
		pixelNear(t, img, int(cur.X)+100, int(cur.Max().Y)-1, over(look.C.LessonCur, look.T.Bg), 2, "no line under the current row")
		no := frameOf(part(t, rows[2], "span.no"))
		pixelNear(t, img, int(no.X)+6, int(no.Y)+18, look.T.Primary, 2, "its number is filled --pr")
		n1 := frameOf(part(t, rows[0], "span.no"))
		pixelNear(t, img, int(n1.X)+18, int(n1.Y), look.T.Border, 40, "an outline of --bd")
		pixelNear(t, img, int(n1.X)+6, int(n1.Y)+18, look.T.Bg, 0, "and no fill")
		r1 := frameOf(rows[0])
		pixelNear(t, img, int(r1.X)+100, int(r1.Max().Y)-1, look.T.Border, 0, "1 px --bd under a plain row")

		ink := func(r *kit.LessonRow) theme.Ink {
			var l *kit.Label
			for ch := range part(t, r, "span.no").Node().Children() {
				l = ch.(*kit.Label)
			}
			return l.Ink
		}
		assert.Equal(t, theme.InkOnPrimary, ink(rows[2]))
		assert.Equal(t, theme.InkText3, ink(rows[4]), "watched: --tx3")
		assert.Equal(t, theme.InkText, ink(rows[0]))
		assert.Equal(t, 0, int(rows[0].Bleed()))
		assert.Equal(t, 8, int(rows[2].Bleed()))

		// the download icon: --ok done, --tx3 none, --pr under way
		dl := func(n int) theme.Ink {
			return part(t, part(t, rows[n-1], fmt.Sprintf("dlTap:set35-%d", n)), "svg.i").(*kit.Icon).Ink
		}
		assert.Equal(t, theme.InkSuccess, dl(1))
		assert.Equal(t, theme.InkText3, dl(5))
		assert.Equal(t, theme.InkPrimary, dl(7))
		assert.InDelta(t, 12, float64(frameOf(part(t, part(t, rows[6], "dlTap:set35-7"), "svg.i")).W), 0, "the cross is 12")
	})
}

// TestLessonRowDownloadRing checks the progress ring: a --sf2 track and a --pr
// arc from 12 o'clock, clockwise, as long as the progress.
func TestLessonRowDownloadRing(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := &kit.LessonRow{No: 7, Title: "Viva La Vida", Meta: "Czech Pop · 03:20", Download: kit.DlIng, Progress: .64, DlName: "dlTap:set35-7"}
		tt := mountStage(t, ph, placeAt(r, 20, 100, ph.Size.W-40))
		dl := frameOf(part(t, r, "dlTap:set35-7"))
		// the ring: radius 15 of 36 in the 28 box inset 3 -> 11.67 from the centre; stroke 2.33
		cx, cy := dl.X+17, dl.Y+17
		at := func(dx, dy float32) (int, int) { return int(cx + dx), int(cy + dy) }
		const rad = 11.67
		sf2 := look.T.Surface2
		x, y := at(0, -rad)
		pixelNear(t, tt.Snapshot(), x, y, look.T.Primary, 12, "12 o'clock is in the arc")
		x, y = at(rad, 0)
		pixelNear(t, tt.Snapshot(), x, y, look.T.Primary, 12, "3 o'clock (25 %) is in the arc at 64 %")
		x, y = at(0, rad-.5)
		pixelNear(t, tt.Snapshot(), x, y, look.T.Primary, 12, "6 o'clock (50 %) too")
		x, y = at(-rad, 0)
		pixelNear(t, tt.Snapshot(), x, y, sf2, 4, "9 o'clock (75 %) is not: the track")
		x, y = at(-rad*.7, -rad*.7)
		pixelNear(t, tt.Snapshot(), x, y, sf2, 4, "10:30 is the track")

		r.SetProgress(.9)
		tt.Elapse(kittest.Frame)
		x, y = at(-rad, 0)
		pixelNear(t, tt.Snapshot(), x, y, look.T.Primary, 12, "at 90 % the 9 o'clock is in the arc")
		x, y = at(-rad*.4, -rad*.917)
		pixelNear(t, tt.Snapshot(), x, y, sf2, 6, "11 o'clock (93 %) is past the arc's end at 90 %")
		r.SetProgress(0)
		tt.Elapse(kittest.Frame)
		x, y = at(0, -rad)
		pixelNear(t, tt.Snapshot(), x, y, sf2, 4, "no arc at 0")
		r.SetProgress(1)
		tt.Elapse(kittest.Frame)
		x, y = at(-rad*.7, -rad*.7)
		pixelNear(t, tt.Snapshot(), x, y, look.T.Primary, 12, "a full ring at 100 %")
	})
}

// TestLessonRowDownloadIconFollowsTheState checks that the button's icon is remade
// when Download changes after the row was laid out: none -> under way -> done -> none,
// with the layout of the row alone (no change of its constraints) to bring it about.
func TestLessonRowDownloadIconFollowsTheState(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := &kit.LessonRow{No: 4, Title: "Em Yêu Người Ta", Meta: "Bachata · 04:10", DlName: "dlTap:set35-4"}
		tt := mountStage(t, ph, placeAt(r, 20, 100, ph.Size.W-40))
		ic := part(t, part(t, r, "dlTap:set35-4"), "svg.i").(*kit.Icon)
		require.Equal(t, icon.Dl, ic.Glyph)
		require.Equal(t, theme.InkText3, ic.Ink)
		for _, tc := range []struct {
			st    kit.LessonDownload
			glyph icon.Name
			ink   theme.Ink
			size  float32
		}{
			{kit.DlIng, icon.X, theme.InkPrimary, 12},
			{kit.DlDone, icon.Dl, theme.InkSuccess, 20},
			{kit.DlIng, icon.X, theme.InkPrimary, 12},
			{kit.DlNone, icon.Dl, theme.InkText3, 20},
		} {
			r.Download = tc.st
			r.Relayout() // what the screen does; the button is not told
			tt.Elapse(kittest.Frame)
			assert.Equal(t, tc.glyph, ic.Glyph, "state %v", tc.st)
			assert.Equal(t, tc.ink, ic.Ink, "state %v", tc.st)
			assert.InDelta(t, tc.size, float64(frameOf(ic).W), 0.6, "state %v", tc.st)
		}
	})
}

// TestLessonRowPlayedDiscAndDim checks the offline play disc and the dim row.
func TestLessonRowPlayedDiscAndDim(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		played := &kit.LessonRow{No: 1, Title: "Bài", Meta: "Audio đã tải", Mode: kit.LessonOffline, Download: kit.DlDone, PlayName: "playAudio:set35-1"}
		dim := &kit.LessonRow{No: 5, Title: "Bài", Meta: "Cần có mạng", Mode: kit.LessonOffline, Download: kit.DlNone, Dim: true}
		tt := mountStage(t, ph, placeAt(played, 20, 100, ph.Size.W-40), placeAt(dim, 20, 200, ph.Size.W-40))
		img := tt.Snapshot()
		d := frameOf(part(t, played, "playAudio:set35-1"))
		pixelNear(t, img, int(d.X)+4, int(d.Y)+16, look.T.Primary, 2, "the disc is --pr")
		pixelNear(t, img, int(d.X)+16, int(d.Y)+16, look.T.OnPrimary, 4, "the triangle is --on")
		pixelNear(t, img, int(d.X), int(d.Y), look.T.Bg, 4, "round")
		f := frameOf(dim)
		line := look.T.Border
		line = color.RGBA(line.R, line.G, line.B, .38)
		pixelNear(t, img, int(f.X)+100, int(f.Max().Y)-1, over(line, look.T.Bg), 2, ".38 opacity")
		pixelNear(t, img, int(f.X), int(f.Y)+1, look.T.Bg, 0, "")
	})
}

// TestLessonRowTaps checks the row, the download button and the disc each take
// their own press, and the labels they have.
func TestLessonRowTaps(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var log []string
		r := &kit.LessonRow{No: 3, Title: "Stink and Dutty", Meta: "Bụng · 03:20", Download: kit.DlNone, DlLabel: "Tải bài 03", PlayLabel: "Phát bài 03"}
		r.OnPress = func() { log = append(log, "row") }
		r.OnDl = func() { log = append(log, "dl") }
		r.OnPlay = func() { log = append(log, "play") }
		tt := mountStage(t, ph, placeAt(r, 20, 100, ph.Size.W-40))
		require.NoError(t, tt.Click("Tải bài 03"))
		require.NoError(t, tt.Click("Stink and Dutty"))
		assert.Equal(t, []string{"dl", "row"}, log)

		r.Mode, r.Download = kit.LessonOffline, kit.DlDone
		r.Relayout()
		tt.Elapse(kittest.Frame)
		require.NoError(t, tt.Click("Phát bài 03"))
		assert.Equal(t, []string{"dl", "row", "play"}, log)
		tt.ClickAt(geom.Pt(frameOf(r).X+100, frameOf(r).Y+20))
		assert.Equal(t, []string{"dl", "row", "play", "row"}, log)

		r.Disabled = true
		tt.ClickAt(geom.Pt(frameOf(r).X+100, frameOf(r).Y+20))
		assert.Len(t, log, 4)

		sem := tt.Semantics()
		found := false
		for _, n := range sem.Nodes {
			if n.Label == "Stink and Dutty" {
				found = true
				assert.Equal(t, "Bụng · 03:20", n.Value)
				assert.Equal(t, platform.RoleListItem, n.Role, "a row with the play disc holds a button of its own")
			}
		}
		assert.True(t, found)
		assert.True(t, hasNode(sem, "Phát bài 03", platform.RoleButton), "the disc is an element under the row")
	})
}

// TestLessonRowButtonsSayWhichLesson checks the download button and the play disc
// are valued by the lesson they are for (P8.3d): "Tải xuống" alone, repeated down
// the list, did not say which lesson it downloads.
func TestLessonRowButtonsSayWhichLesson(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := &kit.LessonRow{
			No: 3, Title: "Stink and Dutty", Download: kit.DlNone, Lesson: "Bài 03 · Stink and Dutty",
			DlLabel: "Tải xuống", PlayLabel: "Phát", CancelLabel: "Huỷ tải", DoneLabel: "Đã tải",
		}
		tt := mountStage(t, ph, placeAt(r, 20, 100, ph.Size.W-40))
		value := func(label string) string {
			for _, n := range tt.Semantics().Nodes {
				if n.Label == label && n.Role == platform.RoleButton {
					return n.Value
				}
			}

			return "(none)"
		}
		assert.Equal(t, "Bài 03 · Stink and Dutty", value("Tải xuống"))
		r.Download = kit.DlIng
		r.Relayout()
		tt.Elapse(kittest.Frame)
		assert.Equal(t, "Bài 03 · Stink and Dutty", value("Huỷ tải"))
		r.Download = kit.DlDone
		r.Relayout()
		tt.Elapse(kittest.Frame)
		assert.Equal(t, "Bài 03 · Stink and Dutty", value("Đã tải"))
		r.Mode = kit.LessonOffline
		r.Relayout()
		tt.Elapse(kittest.Frame)
		assert.Equal(t, "Bài 03 · Stink and Dutty", value("Phát"), "the disc too")
	})
}

// TestLessonListBleed checks the list gives the current row its margin: 8 out on
// each side, the others at the column's width.
func TestLessonListBleed(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		rows, list := m05Rows()
		tt := mountStage(t, ph, placeAt(list, 20, 435, ph.Size.W-40))
		w := ph.Size.W - 40
		assert.InDelta(t, 20, float64(frameOf(rows[0]).X), 0)
		assert.InDelta(t, float64(w), float64(frameOf(rows[0]).W), 0)
		assert.InDelta(t, 12, float64(frameOf(rows[2]).X), 0)
		assert.InDelta(t, float64(w+16), float64(frameOf(rows[2]).W), 0)
		assert.InDelta(t, 435+58*10, float64(frameOf(list).Max().Y), 0)
		rows[2].State = kit.LessonNormal
		list.Relayout()
		tt.Elapse(kittest.Frame)
		assert.InDelta(t, float64(w), float64(frameOf(rows[2]).W), 0, "no longer current")
	})
}

// TestLessonRowsDoNotAllocate checks a steady frame of ten rows, and a frame of a
// download ring that moves, allocate nothing.
func TestLessonRowsDoNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		rows, list := m05Rows()
		tt := mountStage(t, ph, placeAt(list, 20, 20, ph.Size.W-40))
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a steady frame of ten rows", ph.Name)
		p := float32(.1)
		assert.Zero(t, testing.AllocsPerRun(50, func() {
			p += .01
			if p > 1 {
				p = .1
			}
			rows[6].SetProgress(p)
			tt.Repaint()
		}), "%s: a frame of the ring moving", ph.Name)
	}
}

// BenchmarkLessonRowsRepaint is a steady repaint of ten rows.
func BenchmarkLessonRowsRepaint(b *testing.B) {
	_, list := m05Rows()
	benchRepaint(b, placeAt(list, 20, 20, 353))
}

// TestLessonRowLockedMode checks LessonModeLocked (the rows of MS.locked,
// index.html:410): the number and the text on the left, Trailing at the right
// edge, centred, none of the download button, the triangle, the play disc or the
// cloud; at least 58 tall; the title gives way to the trailing part.
func TestLessonRowLockedMode(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		lock := kit.NewIcon(icon.Lock, 18, theme.InkText)
		r := &kit.LessonRow{
			Mode: kit.LessonModeLocked, No: 7, Title: "Bài 07", Meta: "Mở khi bạn tham gia khoá học", Trailing: lock, Dim: true,
			DlName: "dlTap:x", PlayName: "playAudio:x",
		}
		r.Name = "toast"
		width := ph.Size.W - 40
		mountStageReal(t, ph, placeAt(r, 20, 100, width))
		f := frameOf(r)
		assert.InDelta(t, 58, f.H, 0.01, "min-height 58")
		assert.InDelta(t, width, f.W, 0.01)
		lf := frameOf(lock)
		assert.InDelta(t, f.X+f.W, lf.X+lf.W, 0.01, "the lock is at the end of the row")
		assert.InDelta(t, f.Y+(f.H-1-lf.H)/2, lf.Y, 0.51, "centred above the line")
		no := frameOf(part(t, r, "span.no"))
		assert.InDelta(t, f.X, no.X, 0.01)
		for _, name := range []string{"dlTap:x", "playAudio:x"} {
			assert.Nil(t, partOrNil(r, name), name+" is not part of a locked row")
		}
	})
}

// partOrNil is part without the failure.
func partOrNil(root ui.Widget, name string) ui.Widget {
	var find func(w ui.Widget) ui.Widget
	find = func(w ui.Widget) ui.Widget {
		for c := range w.Node().Children() {
			if c.Node().Name == name && !c.Node().Hidden {
				return c
			}
			if f := find(c); f != nil {
				return f
			}
		}

		return nil
	}

	return find(root)
}

// hasNode tells whether the reading has an element of the role called label.
func hasNode(sem *platform.Semantics, label string, role platform.SemanticRole) bool {
	for _, n := range sem.Nodes {
		if n.Label == label && n.Role == role {
			return true
		}
	}

	return false
}

// TestLessonRowWithoutAudioHasNoButton checks DlUnavailable (a lesson with no audio
// file): no download button in the tree (no tap, nothing for a screen reader), its
// room stays so the triangle and the text keep their boxes, and a file coming back
// (DlNone) brings it again.
func TestLessonRowWithoutAudioHasNoButton(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		presses := 0
		r := &kit.LessonRow{No: 5, Title: "Hãy Hát Lên", Meta: "Tay Vai · 03:30", DlName: "dlTap:set35-5", OnDl: func() { presses++ }}
		r.Name = "playVideo:set35-5"
		tt := mountStageReal(t, ph, placeAt(r, 20, 100, ph.Size.W-40))
		dl := part(t, r, "dlTap:set35-5")
		wantDl := frameOf(dl)
		wantRow := frameOf(r)
		wantText := frameOf(part(t, r, "span.tt")) // the text, which the room of the button bounds

		r.Download = kit.DlUnavailable
		r.Relayout()
		tt.Elapse(kittest.Frame)
		assert.True(t, dl.Node().Hidden, "the button is out of the tree")
		assert.Equal(t, wantRow, frameOf(r))
		assert.Equal(t, wantText, frameOf(part(t, r, "span.tt")), "the text keeps its width")
		assert.Equal(t, wantDl.X, frameOf(dl).X, "its room stays: the triangle does not move")
		tt.ClickAt(wantDl.Center())
		assert.Zero(t, presses, "a tap where it was does nothing")

		r.Download = kit.DlNone
		r.Relayout()
		tt.Elapse(kittest.Frame)
		assert.False(t, dl.Node().Hidden)
		tt.ClickAt(wantDl.Center())
		assert.Equal(t, 1, presses)
	})
}
