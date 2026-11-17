package kit_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// optionsContent is the content of SH.options (index.html:505-512): the header,
// the speed control, three rows with switches and one with a value, and the
// button.
func optionsContent() *vstack {
	seg := kit.NewSegmented("0.5x", "0.75x", "1x", "1.25x", "1.5x")
	seg.Select(1)
	rows := []vitem{}
	for i, r := range [][2]string{{"Lật gương", "Tập theo như đứng trước gương"}, {"Lặp đoạn A–B", "01:12 → 01:40 · lặp liên tục"}, {"Thanh đếm 8 nhịp", "Hiện nhịp 1–8 trên video"}} {
		row := kit.NewRow(icon.Mirror, r[0], r[1])
		row.Trailing = kit.NewSwitch(i != 1)
		rows = append(rows, vitem{w: row})
	}
	quality := kit.NewRow(icon.Hd, "Chất lượng video", "")
	quality.Value, quality.Last = "Tự động ›", true
	rows = append(rows, vitem{w: quality})
	items := []vitem{
		{w: kit.NewSheetHeader("Tuỳ chọn phát", "Đóng", nil)},
		{w: kit.Eyebrow("Tốc độ phát"), before: kit.SheetHeaderGap},
		{w: seg, before: 10},
	}
	rows[0].before = 10
	items = append(items, rows...)
	items = append(items, vitem{w: kit.NewButton(kit.ButtonP, "Xong", icon.NameUnknown), before: 18})

	return newVStack(items...)
}

// downloadContent is the content of SH.download (:513-519), the last paragraph's
// inline line a block of its 17 DIPs.
func downloadContent() *vstack {
	r1 := kit.NewRadio("Chỉ bài này", "03 · Stink and Dutty · 03:20", "3,1 MB")
	r2 := kit.NewRadio("Cả khoá Set 35", "5 file còn thiếu · đã có 5", "15,6 MB")
	wifi := kit.NewRow(icon.Globe, "Chỉ tải qua Wi‑Fi", "")
	wifi.Trailing, wifi.Last = kit.NewSwitch(true), true

	return newVStack(
		vitem{w: kit.NewSheetHeader("Tải file âm thanh", "Đóng", nil)},
		vitem{w: r1, before: kit.SheetHeaderGap},
		vitem{w: r2, before: kit.RadioGap},
		vitem{w: wifi, before: kit.RadioGap},
		vitem{w: &block{h: 17}, before: 4},
		vitem{w: kit.NewButton(kit.ButtonP, "Tải xuống · 3,1 MB", icon.Dl), before: 16},
	)
}

// confirmContent is SH.del (:520-522): the icon disc, the title, two lines of
// text, the red button and the cancel; SH.logout (:530) has the title first.
func confirmContent(disc bool) *vstack {
	d := kit.Display("Xoá file âm thanh?", 24)
	d.Align = ui.TextAlignCenter
	items := []vitem{}
	if disc {
		items = append(items, vitem{w: &block{h: 64}, before: 10}, vitem{w: d, before: 20})
	} else {
		d.Text = "Đăng xuất?"
		items = append(items, vitem{w: d, before: 10})
	}
	p := kit.Muted("03 · Stink and Dutty · 3,1 MB sẽ bị xoá khỏi máy.\nBạn vẫn xem video bài này khi có mạng.", 14)
	p.LineHeight, p.Align, p.Wrap = 1.5, ui.TextAlignCenter, true
	if !disc {
		p.Text = "File âm thanh đã tải vẫn được giữ trên máy."
		p.LineHeight = 0
	}
	cancel := kit.NewButton(kit.ButtonS, "Huỷ", icon.NameUnknown)
	cancel.Height = 48
	items = append(items, vitem{w: p, before: 12}, vitem{w: kit.NewButton(kit.ButtonR, "Xoá khỏi máy", icon.NameUnknown), before: 20},
		vitem{w: cancel, before: 10})

	return newVStack(items...)
}

// TestSheetBoxesMatchPrototype compares the dim, the panel, the grab and the
// header of four sheets with the layout.json of their states: the panel is at
// the bottom, as tall as its content (M07 477, M09 406, M14 377, logout 268).
func TestSheetBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		state   string
		content func() *vstack
		header  bool
	}{
		{"M07", optionsContent, true},
		{"M09", downloadContent, true},
		{"M14", func() *vstack { return confirmContent(true) }, false},
		{"M15-logout", func() *vstack { return confirmContent(false) }, false},
	} {
		t.Run(tc.state, func(t *testing.T) {
			t.Parallel()
			kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
				s := kit.NewSheet()
				tt := kittest.MountReal(t, ph, newOverlays(s))
				kittest.PinMotion(tt)
				s.Open(tc.content(), kit.SheetOptions{})
				tt.Elapse(400 * time.Millisecond)

				want := protoBox(t, tc.state, "div.sheet")
				rectNear(t, fromBottom(ph, onPhone(ph, want, anchorStretch)), frameOf(part(t, s, "div.sheet")), 1, tc.state+" div.sheet")
				rectNear(t, geom.XYWH(0, 0, ph.Size.W, ph.Size.H), frameOf(part(t, s, "closeSheet")), 0, tc.state+" dim")
				grab := protoBox(t, tc.state, "div.grab")
				rectNear(t, fromBottom(ph, onPhone(ph, grab, anchorCenter)), frameOf(part(t, s, "div.grab")), 1, tc.state+" div.grab")
				if tc.header {
					rectNear(t, fromBottom(ph, onPhone(ph, protoBox(t, tc.state, "div.sh"), anchorStretch)), frameOf(part(t, s, "div.sh")), 1, tc.state+" div.sh")
					rectNear(t, fromBottom(ph, onPhone(ph, protoBox(t, tc.state, "span.d"), anchorLeft)), frameOf(part(t, s, "span.d")), 1.5, tc.state+" span.d")
				}
			})
		})
	}
}

// TestSheetHeaderCloseButton compares the 32 round close button with the box of
// closeSheet#2 and closes the sheet with its press.
func TestSheetHeaderCloseButton(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewSheet()
		closed := 0
		hdr := kit.NewSheetHeader("Tuỳ chọn phát", "Đóng", nil)
		hdr.Close.OnPress = s.Close
		tt := kittest.MountReal(t, ph, newOverlays(s))
		kittest.PinMotion(tt)
		s.Open(newVStack(vitem{w: hdr}), kit.SheetOptions{OnClose: func() { closed++ }})
		tt.Elapse(400 * time.Millisecond)
		// M07 closeSheet#2 [341,408,32,32] is 33 under the sheet's top (375)
		want := onPhone(ph, protoBox(t, "M07", "closeSheet#2"), anchorRight)
		want.Y = frameOf(part(t, s, "div.sheet")).Y + 33
		rectNear(t, want, frameOf(part(t, hdr, "closeSheet")), 1, "closeSheet#2")
		require.NoError(t, tt.Click("Đóng"))
		assert.Equal(t, 1, closed)
		assert.True(t, s.Hidden)
	})
}

// TestSheetOpensWithAnimationAndClosesAtOnce covers Q-P2-3: the dim fades in
// over 200 ms and the panel slides up over 250 ms; closing has no animation.
func TestSheetOpensWithAnimationAndClosesAtOnce(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewSheet()
		tt := kittest.Mount(t, ph, newOverlays(s))
		kittest.PinMotion(tt)
		assert.True(t, s.Hidden, "no sheet, no host in the way")
		assert.False(t, s.IsOpen())
		assert.Nil(t, s.Top())

		s.Open(newVStack(vitem{w: kit.NewButton(kit.ButtonP, "Xong", icon.NameUnknown)}), kit.SheetOptions{})
		assert.False(t, s.Hidden)
		assert.True(t, s.IsOpen())
		panel := part(t, s, "div.sheet")
		tt.Elapse(1)
		top0 := frameOf(panel).Y
		assert.InDelta(t, float64(ph.Size.H), float64(top0), 4, "it starts below the screen (translateY(100%))")
		tt.Elapse(100 * time.Millisecond)
		mid := frameOf(panel).Y
		assert.Greater(t, float64(top0), float64(mid))
		assert.Greater(t, float64(mid), float64(ph.Size.H-frameOf(panel).H))
		pixelNear(t, tt.Snapshot(), 5, 5, over(dimOf(look, .55*.5), look.T.Bg), 12, "dim half way (ease: .2 s)")
		tt.Elapse(100 * time.Millisecond)
		pixelNear(t, tt.Snapshot(), 5, 5, over(look.T.Scrim, look.T.Bg), 2, "dim done at 200 ms")
		assert.InDelta(t, float64(ph.Size.H-frameOf(panel).H), float64(frameOf(panel).Y), 12, "panel still sliding at 200 ms of 250")
		tt.Elapse(60 * time.Millisecond)
		assert.InDelta(t, float64(ph.Size.H-frameOf(panel).H), float64(frameOf(panel).Y), .01, "panel at rest at 260 ms")

		s.Close()
		assert.True(t, s.Hidden, "closing is at once")
		tt.Elapse(kittest.Frame)
		pixelNear(t, tt.Snapshot(), 5, 5, look.T.Bg, 0, "no dim a frame later")
		assert.False(t, s.IsOpen())
		assert.Nil(t, s.Content())
	})
}

// dimOf returns the scrim at the alpha a.
func dimOf(look *theme.Look, a float32) (c color.Color) {
	c = look.T.Scrim
	c.A = uint8(a*255 + .5)

	return c
}

// TestSheetContentChangeDoesNotReplayTheAnimation is K17: opening again while
// open swaps the content in place, at rest.
func TestSheetContentChangeDoesNotReplayTheAnimation(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewSheet()
		tt := kittest.Mount(t, ph, newOverlays(s))
		kittest.PinMotion(tt)
		firstClosed := 0
		s.Open(newVStack(vitem{w: &block{h: 100}}), kit.SheetOptions{OnClose: func() { firstClosed++ }})
		tt.Elapse(400 * time.Millisecond)
		panel := part(t, s, "div.sheet")
		h1 := frameOf(panel).H
		secondClosed := 0
		h2 := s.Open(newVStack(vitem{w: &block{h: 200}}), kit.SheetOptions{OnClose: func() { secondClosed++ }})
		tt.Elapse(kittest.Frame)
		assert.InDelta(t, float64(ph.Size.H-frameOf(panel).H), float64(frameOf(panel).Y), .01, "no slide")
		assert.Greater(t, float64(frameOf(panel).H), float64(h1), "the new content is taller")
		assert.Equal(t, 1, firstClosed, "the replaced sheet is told")
		assert.Zero(t, secondClosed)
		h2.Close()
		assert.Equal(t, 1, secondClosed)
		h2.Close()
		assert.Equal(t, 1, secondClosed, "once")
		assert.True(t, h2.Closed())
	})
}

// TestSheetClosesByDimAndBack covers the dim's tap and Back.
func TestSheetClosesByDimAndBack(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewSheet()
		taps := 0
		under := kit.NewButton(kit.ButtonP, "Dưới", icon.NameUnknown)
		under.OnPress = func() { taps++ }
		closed := 0
		root := newOverlays(newStage(placeAt(under, 100, 100, 200)), s)
		tt := kittest.Mount(t, ph, root)
		kittest.PinMotion(tt)
		assert.False(t, s.Back(), "Back with no sheet is not taken")

		open := func() {
			s.Open(newVStack(vitem{w: &block{h: 100}}), kit.SheetOptions{OnClose: func() { closed++ }})
			tt.Elapse(400 * time.Millisecond)
		}
		open()
		panel := frameOf(part(t, s, "div.sheet"))
		tt.ClickAt(panel.Center())
		assert.True(t, s.IsOpen(), "a tap on the panel keeps it")
		tt.ClickAt(geom.Pt(ph.Size.W/2, panel.Y/2))
		assert.False(t, s.IsOpen(), "a tap on the dim closes")
		assert.Equal(t, 1, closed)
		assert.Zero(t, taps, "the dim took the tap")

		open()
		// press on the dim, release on the panel: no close
		tt.Press(geom.Pt(ph.Size.W/2, panel.Y/2))
		tt.Release(panel.Center())
		assert.True(t, s.IsOpen())
		assert.True(t, s.Back())
		assert.False(t, s.IsOpen())
		assert.Equal(t, 2, closed)

		// with the host hidden a tap reaches the screen
		tt.ClickAt(frameOf(under).Center())
		assert.Equal(t, 1, taps)
	})
}

// pullSheet presses on the sheet's grab, pulls dy down in steps and lifts.
func pullSheet(tt *ui.Tester, panel geom.Rect, dy float32, every time.Duration) {
	from := geom.Pt(panel.W/2, panel.Y+20)
	tt.TouchDown(from)
	for k := 1; k <= 10; k++ {
		tt.Elapse(every)
		tt.TouchMove(geom.Pt(from.X, from.Y+dy*float32(k)/10))
	}
	tt.TouchUp(geom.Pt(from.X, from.Y+dy))
}

// TestSheetPullDown covers the drag that dismisses: 80 DIPs or 0.6 DIP/ms.
func TestSheetPullDown(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range []struct {
			name  string
			dy    float32
			every time.Duration
			close bool
		}{
			{"79 slow stays", 79, 50 * time.Millisecond, false},
			{"81 slow closes", 81, 50 * time.Millisecond, true},
			{"40 at .7 DIP/ms is a fling", 40, 6 * time.Millisecond, false}, // 40 DIPs in 60 ms: .67
			{"70 at .7 DIP/ms is a fling", 70, 10 * time.Millisecond, true},
			{"50 at .5 DIP/ms is not", 50, 10 * time.Millisecond, false},
		} {
			s := kit.NewSheet()
			closed := 0
			tt := kittest.Mount(t, ph, newOverlays(s))
			kittest.PinMotion(tt)
			s.Open(newVStack(vitem{w: kit.NewButton(kit.ButtonP, "Xong", icon.NameUnknown)}), kit.SheetOptions{OnClose: func() { closed++ }})
			tt.Elapse(400 * time.Millisecond)
			panel := frameOf(part(t, s, "div.sheet"))
			rest := panel.Y
			pullSheet(tt, panel, tc.dy, tc.every)
			if tc.name == "40 at .7 DIP/ms is a fling" {
				// 40 DIPs in 60 ms is .67 DIP/ms: a fling too
				tc.close = true
			}
			assert.Equal(t, tc.close, !s.IsOpen(), tc.name)
			assert.Equal(t, b2i(tc.close), closed, tc.name)
			if !tc.close {
				tt.Elapse(400 * time.Millisecond)
				assert.InDelta(t, float64(rest), float64(frameOf(part(t, s, "div.sheet")).Y), .01, tc.name+": it slides back")
			}
		}
	})
}

// b2i is 1 for true.
func b2i(b bool) int {
	if b {
		return 1
	}

	return 0
}

// TestSheetFollowsTheFinger checks the panel moves with the finger while pulled
// and does not go above its place.
func TestSheetFollowsTheFinger(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewSheet()
		tt := kittest.Mount(t, ph, newOverlays(s))
		kittest.PinMotion(tt)
		s.Open(newVStack(vitem{w: &block{h: 100}}), kit.SheetOptions{})
		tt.Elapse(400 * time.Millisecond)
		panel := part(t, s, "div.sheet")
		rest := frameOf(panel).Y
		from := geom.Pt(ph.Size.W/2, rest+20)
		tt.TouchDown(from)
		tt.Elapse(20 * time.Millisecond)
		tt.TouchMove(geom.Pt(from.X, from.Y+30))
		tt.Elapse(20 * time.Millisecond)
		tt.TouchMove(geom.Pt(from.X, from.Y+50))
		tt.Elapse(kittest.Frame)
		// the offset counts from the press, so the move that locked the axis (30) is included
		assert.InDelta(t, float64(rest+50), float64(frameOf(panel).Y), 1)
		tt.TouchMove(geom.Pt(from.X, from.Y-40))
		tt.Elapse(kittest.Frame)
		assert.InDelta(t, float64(rest), float64(frameOf(panel).Y), .01, "a pull up holds the sheet")
		tt.TouchUp(geom.Pt(from.X, from.Y-40))
		assert.True(t, s.IsOpen())
	})
}

// TestSheetScrollsInsideAndPullsOnlyAtTop covers a sheet taller than 92 % of the
// screen: its content scrolls, and the pull-down is the scroll's at the top.
func TestSheetScrollsInsideAndPullsOnlyAtTop(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewSheet()
		tt := kittest.Mount(t, ph, newOverlays(s))
		kittest.PinMotion(tt)
		s.Open(newVStack(vitem{w: &block{h: 2000}}), kit.SheetOptions{})
		tt.Elapse(400 * time.Millisecond)
		panel := part(t, s, "div.sheet")
		f := frameOf(panel)
		assert.InDelta(t, float64(ph.Size.H)*.92, float64(f.H), .01, "max-height: 92%")
		assert.InDelta(t, float64(ph.Size.H)-float64(f.H), float64(f.Y), .01)

		grab := part(t, s, "div.grab")
		top := f.Y + 14 // where the grab is while the content is at its top
		assert.InDelta(t, float64(top), float64(frameOf(grab).Y), .01)

		// a swipe up moves the content; the sheet stays
		tt.Swipe(geom.Pt(ph.Size.W/2, f.Y+f.H-100), geom.Pt(ph.Size.W/2, f.Y+f.H-300), 200*time.Millisecond)
		tt.Elapse(2 * time.Second)
		assert.True(t, s.IsOpen())
		scrolled := frameOf(grab).Y
		require.Less(t, float64(scrolled), float64(top), "the content scrolled")

		// a pull down of 90 scrolls the content back, it does not close the sheet
		pullSheet(tt, frameOf(panel), 90, 50*time.Millisecond)
		assert.True(t, s.IsOpen(), "the scroll takes the drag while it can go that way")
		assert.Greater(t, float64(frameOf(grab).Y), float64(scrolled))

		// at the top the same pull closes it
		tt.Scroll(geom.Pt(ph.Size.W/2, f.Y+100), 0, -10000)
		tt.Elapse(2 * time.Second)
		require.InDelta(t, float64(top), float64(frameOf(grab).Y), .01)
		pullSheet(tt, frameOf(panel), 90, 50*time.Millisecond)
		assert.False(t, s.IsOpen())
	})
}

// TestSheetKeyboardRaisesTheBottom covers Q-P2-7: with the on-screen keyboard up,
// the content ends above it.
func TestSheetKeyboardRaisesTheBottom(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewSheet()
		btn := kit.NewButton(kit.ButtonP, "Lưu mật khẩu", icon.NameUnknown)
		tt := kittest.Mount(t, ph, newOverlays(s))
		kittest.PinMotion(tt)
		s.Open(newVStack(vitem{w: btn}), kit.SheetOptions{})
		tt.Elapse(400 * time.Millisecond)
		panel := part(t, s, "div.sheet")
		h0, bottom0 := frameOf(panel).H, frameOf(btn).Max().Y
		assert.InDelta(t, float64(ph.Size.H-40), float64(bottom0), .01, "40 under the button without the keyboard")

		tt.SetKeyboard(geom.Insets{Bottom: 300})
		tt.Elapse(kittest.Frame)
		assert.Greater(t, float64(frameOf(panel).H), float64(h0))
		assert.InDelta(t, float64(ph.Size.H-300), float64(frameOf(btn).Max().Y), .01, "the button ends at the keyboard")
		tt.SetKeyboard(geom.Insets{})
		tt.Elapse(kittest.Frame)
		assert.InDelta(t, float64(ph.Size.H-40), float64(frameOf(btn).Max().Y), .01)
	})
}

// TestSheetDoesNotAllocate checks a steady frame of an open sheet and a move of
// the finger dragging it allocate nothing.
func TestSheetDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		s := kit.NewSheet()
		tt := kittest.Mount(t, ph, newOverlays(s))
		kittest.PinMotion(tt)
		s.Open(optionsContent(), kit.SheetOptions{})
		tt.Elapse(400 * time.Millisecond)
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a steady frame", ph.Name)

		panel := frameOf(part(t, s, "div.sheet"))
		from := geom.Pt(panel.W/2, panel.Y+20)
		tt.TouchDown(from)
		tt.Elapse(20 * time.Millisecond)
		tt.TouchMove(geom.Pt(from.X, from.Y+20))
		y, dir := from.Y+20, float32(1)
		assert.Zero(t, testing.AllocsPerRun(50, func() {
			if y > from.Y+70 || y < from.Y+10 {
				dir = -dir
			}
			y += 3 * dir
			tt.Elapse(kittest.Frame)
			tt.TouchMove(geom.Pt(from.X, y))
		}), "%s: a move of the finger dragging the sheet", ph.Name)
		tt.TouchUp(geom.Pt(from.X, from.Y))
	}
}

// BenchmarkSheetRepaint is a steady repaint of an open sheet.
func BenchmarkSheetRepaint(b *testing.B) {
	s := kit.NewSheet()
	tt := kittest.Mount(b, kittest.Phones[0], newOverlays(s))
	kittest.PinMotion(tt)
	s.Open(optionsContent(), kit.SheetOptions{})
	tt.Elapse(400 * time.Millisecond)
	tt.Repaint()
	b.ReportAllocs()
	for b.Loop() {
		tt.Repaint()
	}
}
