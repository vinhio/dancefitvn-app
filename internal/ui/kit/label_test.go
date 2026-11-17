package kit_test

import (
	"image"
	"strings"
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mountLabel puts l top-left in a view of the phone's size, in the app's
// theme with the phone's real text engine, and returns the tester.
func mountLabel(t *testing.T, ph kittest.Phone, l *kit.Label) *ui.Tester {
	t.Helper()

	return mountAt(t, ph, l, 0)
}

// mountAt is mountLabel in a box width DIPs wide (0: as wide as it needs),
// which gives the label that much room at most.
func mountAt(t *testing.T, ph kittest.Phone, w ui.Widget, width float32) *ui.Tester {
	t.Helper()
	root := &ui.Box{}
	if width > 0 {
		inner := &ui.Box{Width: width}
		inner.Add(w)
		root.Add(inner)
	} else {
		root.Add(w)
	}

	return kittest.MountReal(t, ph, root)
}

// mountStretched puts l in a column width DIPs wide that stretches it to the
// whole width, so that Align has room to place the line in.
func mountStretched(t *testing.T, ph kittest.Phone, l *kit.Label, width float32) *ui.Tester {
	t.Helper()
	col := ui.Column(0, l)
	col.Align = ui.CrossStretch

	return mountAt(t, ph, col, width)
}

// TestLabelBoxesMatchPrototype lays the prototype's texts out on both phones
// with their real engines and compares the boxes with the layout.json of the
// prototype (local/shots/prototype-1x/mobile). Heights are exact (they are
// Chrome's rounded line boxes); widths hold to widthTol, the difference between
// the engines' advances (Chrome measures with its own copy of the font).
func TestLabelBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	const widthTol = 1.5
	tests := []struct {
		name string
		l    kit.Label
		w, h float32 // the prototype's box
	}{
		// M01 `div.eb` Dễ · Vui · Khoẻ: .eb with letter-spacing 3px inline.
		{"M01 div.eb", kit.Label{Text: "Dễ · Vui · Khoẻ", Size: 11, Weight: 700, Spacing: 3, Upper: true, Ink: theme.InkText2}, 128, 14},
		// M01 `div.d.acc` DANCEFIT: .d 72/800, letter-spacing 2px, line-height 1.
		{"M01 div.d.acc", kit.Label{Text: "DANCEFIT", Role: theme.RoleDisplay, Size: 72, Weight: 800, Spacing: 2, LineHeight: 1, Upper: true}, 271.5, 72},
		// M03 `span.d` KHOÁ HỌC CỦA TÔI: .sec .d 18/700, 1px, line-height 1.
		{"M03 span.d", kit.Label{Text: "Khoá học của tôi", Role: theme.RoleDisplay, Size: 18, Weight: 700, Spacing: 1, LineHeight: 1, Upper: true}, 133.2, 18},
		{"M03 span.d#2", kit.Label{Text: "Mới cập nhật", Role: theme.RoleDisplay, Size: 18, Weight: 700, Spacing: 1, LineHeight: 1, Upper: true}, 102.1, 18},
		// M14 `div.h1`: 34px h1 box is the line-height 1 of the rule.
		{"M14 div.h1", kit.Label{Text: "Tải xuống", Role: theme.RoleDisplay, Size: 34, Weight: 700, Spacing: 1, LineHeight: 1, Upper: true}, 0, 34},
		// M04 `span.chip.c-th` THƯỜNG: 11/700 text of a 66.8 chip with padding 9.
		{"M04 chip text", kit.Label{Text: "Thường", Size: 11, Weight: 700, Upper: true}, 66.8 - 18, 14},
		// M03 `span.chip.y` TIẾP TỤC XEM: 11/800, 1px, in a 108.4 chip.
		{"M03 chip.y text", kit.Label{Text: "Tiếp tục xem", Size: 11, Weight: 800, Spacing: 1, Upper: true}, 108.4 - 18, 14},
	}
	for _, ph := range kittest.Phones {
		for _, tc := range tests {
			t.Run(ph.Name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				l := tc.l
				mountLabel(t, ph, &l)
				f := l.Frame()
				assert.InDelta(t, tc.h, f.H, 0, "height")
				if tc.w > 0 {
					assert.InDelta(t, tc.w, f.W, widthTol, "width")
				}
			})
		}
	}
}

// TestLabelLineBoxes checks the line box of the roles: `line-height: normal`
// is the sum of the rounded ascent and descent (Inter 11: 11 + 3 = 14, the .eb
// of the prototype; 16: 16 + 4), and a set line-height is its multiple of the
// size, whatever the font.
func TestLabelLineBoxes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		l    kit.Label
		h    float32
		base float32 // the baseline from the top of the box
	}{
		{"eb 11 normal", kit.Label{Text: "Ag", Size: 11, Weight: 700}, 14, 11},
		{"body default 16 normal", kit.Label{Text: "Ag"}, 20, 16},
		{"body 15 normal", kit.Label{Text: "Ag", Size: 15, Weight: 600}, 19, 15},
		{"display 28 lh 1", kit.Label{Text: "AG", Role: theme.RoleDisplay, Size: 28, Weight: 700, LineHeight: 1}, 28, 0},
		{"display 20 lh .95", kit.Label{Text: "AG", Role: theme.RoleDisplay, Size: 20, Weight: 800, LineHeight: .95}, 19, 0},
		{"mono 12 normal", kit.Label{Text: "0:42", Role: theme.RoleMono, Size: 12, Weight: 500}, 0, 0},
	}
	for _, ph := range kittest.Phones {
		for _, tc := range tests {
			t.Run(ph.Name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				l := tc.l
				mountLabel(t, ph, &l)
				if tc.h > 0 {
					assert.InDelta(t, tc.h, l.Frame().H, 0)
				}
				if tc.base > 0 {
					assert.InDelta(t, tc.base, l.Baseline(), 0)
				}
				assert.Equal(t, 1, l.Lines())
				// The baseline is a whole number of pixels, as Chrome's is.
				assert.Equal(t, float32(int(l.Baseline())), l.Baseline())
			})
		}
	}
}

// TestLabelSpacingAdvance checks that letter-spacing adds spacing after every
// character, the last included, and counts a letter with combining marks once,
// in NFC and in NFD.
func TestLabelSpacingAdvance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		s     string
		count int // characters (clusters)
	}{
		{"ascii", "DANCEFIT", 8},
		{"nfc", "KHOÁ HỌC CỦA TÔI", 16},
		{"nfd", "KHOA\u0301 HO\u0323C CU\u0309A TO\u0302I", 16},
		{"one letter, nfc", "Ổ", 1},
		{"one letter, nfd", "O\u0302\u0309", 1},
	}
	const sp = 5
	for _, ph := range kittest.Phones {
		for _, tc := range tests {
			t.Run(ph.Name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				plain := &kit.Label{Text: tc.s, Role: theme.RoleDisplay, Size: 20, Weight: 700}
				mountLabel(t, ph, plain)
				spaced := &kit.Label{Text: tc.s, Role: theme.RoleDisplay, Size: 20, Weight: 700, Spacing: sp}
				mountLabel(t, ph, spaced)
				assert.InDelta(t, float32(tc.count*sp), spaced.Frame().W-plain.Frame().W, 0.01)
			})
		}
	}
}

// TestSpaceCopiesAndSpaces checks the exported Space on a shaped layout: a
// copy that shares nothing, glyph i of a line moved by i spacings, the line
// as much wider as it has clusters, and the same layout back for 0.
func TestSpaceCopiesAndSpaces(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		sh := kittest.Shaper(t, ph)
		face := theme.NewFaces(ph.Naming).Face(theme.RoleDisplay, 20, 700)
		raw, err := sh.Layout("HỌC", face, 1, 0)
		require.NoError(t, err)
		keep := raw.Glyphs[0].X
		x2 := raw.Glyphs[len(raw.Glyphs)-1].X

		assert.Same(t, raw, kit.Space(raw, 0))
		got := kit.Space(raw, 3)
		require.NotSame(t, raw, got)
		assert.InDelta(t, raw.Size.W+9, got.Size.W, 0.01, "3 clusters")
		assert.InDelta(t, raw.Lines[0].Width+9, got.Lines[0].Width, 0.01)
		assert.InDelta(t, x2+6, got.Glyphs[len(got.Glyphs)-1].X, 0.01, "the third cluster moves two spacings")
		assert.InDelta(t, keep, raw.Glyphs[0].X, 0, "the shaper's layout is left alone")
		assert.InDelta(t, x2, raw.Glyphs[len(raw.Glyphs)-1].X, 0, "the shaper's layout is left alone")
		assert.Nil(t, kit.Space(nil, 3))
	})
}

// TestLabelUpper checks text-transform: uppercase: the layout is of the upper
// text, the accessible label is the text as given, and a text change is
// upper-cased again.
func TestLabelUpper(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		l := &kit.Label{Text: "Dễ · Vui · Khoẻ", Size: 11, Weight: 700, Upper: true}
		tt := mountLabel(t, ph, l)
		assert.Equal(t, "DỄ · VUI · KHOẺ", l.TextLayout().Text)
		assert.Equal(t, "Dễ · Vui · Khoẻ", l.AccessibleLabel())
		l.SetText("Xem tất cả")
		tt.RunUntilIdle()
		assert.Equal(t, "XEM TẤT CẢ", l.TextLayout().Text)
		assert.True(t, tt.Shows("XEM TẤT CẢ") || tt.Shows("Xem tất cả"))
	})
}

// TestLabelEllipsis checks text-overflow: ellipsis: with and without
// letter-spacing the line ends with "…" within the room it gets, and a text
// that fits is left as it is.
func TestLabelEllipsis(t *testing.T) {
	t.Parallel()

	const room = 120
	tests := []struct {
		name    string
		s       string
		spacing float32
		cut     bool
	}{
		{"cut", "Một tựa đề rất dài của một bài học", 0, true},
		{"cut with spacing", "Một tựa đề rất dài của một bài học", 1.4, true},
		{"fits", "Bài 1", 0, false},
		{"fits with spacing", "Bài 1", 1.4, false},
	}
	for _, ph := range kittest.Phones {
		for _, tc := range tests {
			t.Run(ph.Name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				l := &kit.Label{Text: tc.s, Size: 15, Weight: 600, Spacing: tc.spacing, Ellipsis: true}
				mountAt(t, ph, l, room)
				assert.Equal(t, 1, l.Lines())
				assert.LessOrEqual(t, l.Frame().W, float32(room)+0.01)
				assert.Equal(t, tc.cut, strings.HasSuffix(l.TextLayout().Text, "…"), l.TextLayout().Text)
				if tc.cut {
					assert.Greater(t, l.Frame().W, float32(room)-20, "the line fills the room")
				}
			})
		}
	}
}

// TestLabelWraps checks wrapping and the `\n` that stands for the prototype's
// <br>: each line is a box as tall as one line, MaxLines clamps with an
// ellipsis, and the lines have their own baselines.
func TestLabelWraps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		l     kit.Label
		width float32
		lines int
		one   float32 // the box of one line: Inter 15 is 15 + 4, 14 is 14 + 3
	}{
		{"break", kit.Label{Text: "Đăng nhập\nhuấn luyện viên", Size: 15}, 0, 2, 19},
		{"break spaced", kit.Label{Text: "Đăng nhập\nhuấn luyện viên", Size: 15, Spacing: 1}, 0, 2, 19},
		{"wrap", kit.Label{Text: "Một đoạn văn đủ dài để phải xuống dòng nhiều lần trong khung", Size: 14, Wrap: true}, 150, 0, 17},
		{"wrap spaced", kit.Label{Text: "Một đoạn văn đủ dài để phải xuống dòng nhiều lần trong khung", Size: 14, Wrap: true, Spacing: 1}, 150, 0, 17},
		{"clamped", kit.Label{Text: "Một đoạn văn đủ dài để phải xuống dòng nhiều lần trong khung", Size: 14, MaxLines: 2, Ellipsis: true}, 150, 2, 17},
		{"clamped spaced", kit.Label{Text: "Một đoạn văn đủ dài để phải xuống dòng nhiều lần trong khung", Size: 14, MaxLines: 2, Ellipsis: true, Spacing: 1}, 150, 2, 17},
	}
	for _, ph := range kittest.Phones {
		for _, tc := range tests {
			t.Run(ph.Name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				l := tc.l
				mountAt(t, ph, &l, tc.width)
				if tc.lines > 0 {
					assert.Equal(t, tc.lines, l.Lines())
				} else {
					assert.Greater(t, l.Lines(), 2)
				}
				one := l.Frame().H / float32(l.Lines())
				assert.InDelta(t, tc.one, one, 0, "rounded ascent + descent")
				lines := l.TextLayout().Lines
				for i := 1; i < len(lines); i++ {
					assert.InDelta(t, one, lines[i].Baseline-lines[i-1].Baseline, 0.01)
				}
				if tc.width > 0 {
					assert.LessOrEqual(t, l.Frame().W, tc.width+0.01)
				}
				if strings.Contains(tc.name, "clamped") {
					assert.True(t, strings.HasSuffix(l.TextLayout().Text, "…"), l.TextLayout().Text)
				}
			})
		}
	}
}

// TestLabelNoBreakSpaceGluesWords checks that the no-break space is a space to the eye
// that a letter-spaced text (whose breaks are found in the label, not by the engine)
// does not break at: "BÀI 02" joined by one stays on a line, and a name
// that fills the room before it wraps ahead of the pair.
func TestLabelNoBreakSpaceGluesWords(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		const room = 200
		for _, sp := range []string{" ", "\u00a0"} {
			l := &kit.Label{Text: "NNNNN NNNNN NNNNN NNNNN BÀI" + sp + "02", Size: 20, Wrap: true, Spacing: 1}
			mountAt(t, ph, l, room)
			ly := l.TextLayout()
			last := ly.Text[ly.Lines[len(ly.Lines)-1].TextStart:]
			if sp == " " {
				assert.Contains(t, ly.Text, "02")

				continue
			}
			assert.Contains(t, last, "BÀI\u00a002", "the pair stays together on the last line")
			assert.Greater(t, l.Lines(), 1)
		}
	})
}

// TestLabelPaintsInkAndAlign checks the color and the alignment against the
// pixels: a line of the label's Ink at the left of the box, a centered line
// centered in it, an end-aligned one against its right edge, and an explicit
// Color over the Ink.
func TestLabelPaintsInkAndAlign(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		const w = 300
		build := func(a ui.TextAlign, col color.Color) *ui.Tester {
			l := &kit.Label{Text: "HELLO", Role: theme.RoleDisplay, Size: 40, Weight: 800, LineHeight: 1, Align: a, Ink: theme.InkPrimary, Color: col}

			return mountStretched(t, ph, l, w)
		}
		for _, tc := range []struct {
			name string
			a    ui.TextAlign
			ok   func(left, right int) bool
		}{
			{"start", ui.TextAlignStart, func(l, _ int) bool { return l >= 0 && l < 4 }},
			{"center", ui.TextAlignCenter, func(l, r int) bool { return l > 40 && abs(l-(w-r)) <= 2 }},
			{"end", ui.TextAlignEnd, func(l, r int) bool { return l > 100 && w-r < 6 }},
		} {
			left, right := inkSpan(build(tc.a, color.Color{}).Snapshot(), w, 40)
			assert.True(t, tc.ok(left, right), "%s: ink spans %d..%d of %d", tc.name, left, right, w)
		}
		look, ok := theme.New(ph.Naming).Ext[*theme.Look]()
		require.True(t, ok)
		primary := theme.InkPrimary.Of(look)
		img := build(ui.TextAlignStart, color.Color{}).Snapshot()
		assert.True(t, hasPixelNear(img, primary), "the Ink is the color")
		green := color.RGB(0x3d, 0xd6, 0x8c)
		img = build(ui.TextAlignStart, green).Snapshot()
		assert.True(t, hasPixelNear(img, green), "an explicit Color is drawn")
		assert.False(t, hasPixelNear(img, primary), "and not the Ink")
	})
}

// TestLabelDescribe checks the semantics of a label: text, or a heading.
func TestLabelDescribe(t *testing.T) {
	t.Parallel()

	var n platform.SemanticNode
	(&kit.Label{Text: "Tải xuống"}).Describe(nil, &n)
	assert.Equal(t, platform.RoleText, n.Role)
	assert.Equal(t, "Tải xuống", n.Label)
	n = platform.SemanticNode{}
	(&kit.Label{Text: "Tải xuống", Heading: 1}).Describe(nil, &n)
	assert.Equal(t, platform.RoleHeading, n.Role)
	assert.Equal(t, int32(1), n.Level)
}

// TestLabelSetTextSameChangesNothing checks that setting the text it has does
// not lay the label out again, and a new text does.
func TestLabelSetTextSameChangesNothing(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		l := &kit.Label{Text: "A"}
		tt := kittest.Mount(t, ph, l)
		before := l.TextLayout()
		l.SetText("A")
		tt.RunUntilIdle()
		assert.Same(t, before, l.TextLayout())
		l.SetText("B")
		tt.RunUntilIdle()
		assert.NotSame(t, before, l.TextLayout())
		assert.Equal(t, "B", l.TextLayout().Text)
	})
}

// TestLabelPaintDoesNotAllocate checks that painting a label (plain, spaced,
// upper, ellipsized, wrapped) in a steady frame allocates nothing.
func TestLabelPaintDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	labels := []*kit.Label{
		{Text: "Xin chào, Thảo Vy", Size: 15, Weight: 600},
		{Text: "Thứ bảy, 14/11 · Chào buổi sáng", Size: 11, Weight: 700, Spacing: 1.4, Upper: true, Ink: theme.InkText2},
		{Text: "Khoá học của tôi", Role: theme.RoleDisplay, Size: 18, Weight: 700, Spacing: 1, LineHeight: 1, Upper: true},
		{Text: "Một tựa đề rất dài của một bài học", Size: 15, Weight: 600, Spacing: 1, Ellipsis: true},
		{Text: "Một đoạn văn đủ dài để phải xuống dòng nhiều lần\ntrong khung", Size: 14, Wrap: true, Spacing: 1},
	}
	for _, ph := range kittest.Phones {
		col := ui.Column(4)
		for _, l := range labels {
			col.Add(l)
		}
		tt := mountAt(t, ph, col, 200)
		tt.Elapse(kittest.Frame)
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a repaint of the labels", ph.Name)
	}
}

// TestLabelSetSameTextDoesNotAllocate checks that SetText with the text the
// label has costs nothing.
func TestLabelSetSameTextDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	l := &kit.Label{Text: "Khoá học của tôi", Size: 18, Spacing: 1, Upper: true}
	kittest.Mount(t, kittest.Phones[0], l)
	assert.Zero(t, testing.AllocsPerRun(100, func() { l.SetText("Khoá học của tôi") }))
}

// BenchmarkLabelRepaint is a steady repaint of a spaced label.
func BenchmarkLabelRepaint(b *testing.B) {
	l := &kit.Label{Text: "Thứ bảy, 14/11 · Chào buổi sáng", Size: 11, Weight: 700, Spacing: 1.4, Upper: true}
	tt := kittest.Mount(b, kittest.Phones[0], l)
	tt.Repaint()
	b.ReportAllocs()
	for b.Loop() {
		tt.Repaint()
	}
}

// inkSpan returns the first x and the end of the last x of the pixels in the
// top h rows of the snapshot that differ clearly from the background, within
// the first w columns.
func inkSpan(img *image.RGBA, w, h int) (left, right int) {
	left, right = -1, -1
	bg := img.Pix[(h+20)*img.Stride:]
	for x := range w {
		for y := range h {
			i := y*img.Stride + 4*x
			if absDiff(img.Pix[i], bg[0])+absDiff(img.Pix[i+1], bg[1])+absDiff(img.Pix[i+2], bg[2]) > 120 {
				if left < 0 {
					left = x
				}
				right = x + 1

				break
			}
		}
	}

	return left, right
}

// hasPixelNear reports whether the image has a pixel near c: within 40 per
// channel, which a thin antialiased stroke reaches at its core at scale 1.
func hasPixelNear(img *image.RGBA, c color.Color) bool {
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if absDiff(img.Pix[i], c.R) <= 40 && absDiff(img.Pix[i+1], c.G) <= 40 && absDiff(img.Pix[i+2], c.B) <= 40 {
			return true
		}
	}

	return false
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}

	return int(b - a)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}

	return v
}

// TestBareTextFallsOnInter checks the theme's default face: a ui.Text that
// names no family draws in Inter 16 / 400 on both engines, as wide as the same
// text in a Label (F-14: with Family "" Core Text would draw SF).
func TestBareTextFallsOnInter(t *testing.T) {
	t.Parallel()

	const s = "Hamburgefonstiv Đăng nhập"
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		bare := &ui.Text{Value: s}
		mountAt(t, ph, bare, 0)
		named := &kit.Label{Text: s}
		mountAt(t, ph, named, 0)
		assert.InDelta(t, named.Frame().W, bare.Frame().W, 0.01)
	})
}
