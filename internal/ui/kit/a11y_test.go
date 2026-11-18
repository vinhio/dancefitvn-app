// Pattern from jplayer-mobile/internal/ui/kit/a11y_test.go at 49de3c2 (each widget on
// both phones, as a screen reader reads it, and what its actions do) and from
// jplayer-tv/internal/ui/kit/hush_test.go at 781bee5 (a hushed subtree).

package kit_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// hushed is a widget with a Hush that holds a button, as the screen under a sheet
// does.
type hushed struct {
	ui.Base
	kit.Hush
}

// plate is a widget with Decor holding a text, as a poster holds its title.
type plate struct {
	ui.Base
	kit.Decor
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// nodeCalled returns the element of the tree called label, failing the test when
// there is none.
func nodeCalled(tb testing.TB, tt *ui.Tester, label string) platform.SemanticNode {
	tb.Helper()
	for _, n := range tt.Semantics().Nodes {
		if n.Label == label {
			return n
		}
	}
	require.Failf(tb, "no element", "%q is not in the tree", label)

	return platform.SemanticNode{}
}

// noNodeCalled tells whether no element of the tree is called label.
func noNodeCalled(tt *ui.Tester, label string) bool {
	for _, n := range tt.Semantics().Nodes {
		if n.Label == label {
			return false
		}
	}

	return true
}

// TestHushLeavesTheSubtreeOutOfAReading checks that a widget with a Hush describes
// nothing of its subtree while Silent says so, and everything once it does not,
// and that what is hushed takes no action.
func TestHushLeavesTheSubtreeOutOfAReading(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		pressed := 0
		b := kit.NewButton(kit.ButtonP, "Dưới", icon.NameUnknown)
		b.OnPress = func() { pressed++ }
		h := &hushed{}
		h.Add(b)
		silent := false
		h.Silent = func() bool { return silent }
		tt := mountStage(t, ph, placeAt(h, 0, 0, 200))
		assert.Equal(t, "Dưới", nodeCalled(t, tt, "Dưới").Label)
		assert.True(t, tt.PerformAction("Dưới", platform.ActPress))
		assert.Equal(t, 1, pressed)

		silent = true
		assert.True(t, noNodeCalled(tt, "Dưới"), "a hushed button is not read")
		assert.False(t, tt.PerformAction("Dưới", platform.ActPress), "nor pressed")
		assert.Equal(t, 1, pressed)
		silent = false
		assert.Equal(t, "Dưới", nodeCalled(t, tt, "Dưới").Label, "read again")
	})
}

// TestDecorLeavesTheSubtreeOutOfAReading checks decoration: a widget with Decor and
// a text inside is no element and has none; a Badge is one text, its caption, and
// the medal's own texts are not read.
func TestDecorLeavesTheSubtreeOutOfAReading(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p := &plate{}
		p.Add(&kit.Label{Text: "DANCEFIT"})
		badge := kit.NewBadge("HDV DanceFIT", tokens.Dark.Primary, "DANCEFIT")
		tt := mountStage(t, ph, placeAt(p, 10, 100, 200), placeAt(badge, 10, 200, 111))
		assert.True(t, noNodeCalled(tt, "DANCEFIT"), "a poster's title is painted, not read")
		n := nodeCalled(t, tt, "HDV DanceFIT")
		assert.Equal(t, platform.RoleText, n.Role)
		assert.True(t, noNodeCalled(tt, "HDV"), "the medal's mark is not read")
		texts := 0
		for _, nd := range tt.Semantics().Nodes[1:] {
			texts++
			assert.Equal(t, platform.RoleText, nd.Role)
		}
		assert.Equal(t, 1, texts, "one element: the badge")
	})
}

// TestPositionBarsReadAsSliders checks the scrub and the wave: a slider named by its
// label from 0 to the length, at the position, spoken as "elapsed / total"; an increment
// and a decrement ask OnStep for +10 s and -10 s; nothing else is taken.
func TestPositionBarsReadAsSliders(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var steps []float64
		step := func(d float64) { steps = append(steps, d) }
		reading := func() (float64, float64) { return 72, 220 }
		scrub := kit.NewScrub("Vị trí phát")
		scrub.Reading, scrub.OnStep = reading, step
		wave := kit.NewWave("Vị trí phát nghe")
		wave.Reading, wave.OnStep = reading, step
		bare := kit.NewScrub("Chưa có bài")
		tt := mountStage(t, ph, placeAt(scrub, 20, 100, 353), placeAt(wave, 20, 200, 353), placeAt(bare, 20, 300, 353))
		for _, label := range []string{"Vị trí phát", "Vị trí phát nghe"} {
			n := nodeCalled(t, tt, label)
			assert.Equal(t, platform.RoleSlider, n.Role)
			assert.InDelta(t, 0, n.Min, 0)
			assert.InDelta(t, 220, n.Max, 0)
			assert.InDelta(t, 72, n.Now, 0)
			assert.Equal(t, "01:12 / 03:40", n.Value, "elapsed / total")
			assert.True(t, n.Actions.Has(platform.ActIncrement|platform.ActDecrement))
			assert.True(t, tt.PerformAction(label, platform.ActIncrement))
			assert.True(t, tt.PerformAction(label, platform.ActDecrement))
			assert.False(t, tt.PerformAction(label, platform.ActScrollForward))
		}
		assert.Equal(t, []float64{10, -10, 10, -10}, steps)
		n := nodeCalled(t, tt, "Chưa có bài")
		assert.Equal(t, platform.RoleSlider, n.Role)
		assert.InDelta(t, 0, n.Max, 0)
		assert.False(t, tt.PerformAction("Chưa có bài", platform.ActIncrement), "no OnStep: nothing to step")
	})
}

// TestSideButtonSpeaksItsValueAndState checks the buttons of the video's column: a
// label other than the text with the text as the value, and the state of a toggle.
func TestSideButtonSpeaksItsValueAndState(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		speed := kit.NewSideButton(icon.Speed, "0.75x")
		speed.Label, speed.Value = "Tốc độ phát", "0.75x"
		mirror := kit.NewSideButton(icon.Mirror, "Gương")
		mirror.Toggle = true
		tt := mountStage(t, ph, placeAt(speed, 300, 300, 0), placeAt(mirror, 300, 400, 0))
		n := nodeCalled(t, tt, "Tốc độ phát")
		assert.Equal(t, platform.RoleButton, n.Role)
		assert.Equal(t, "0.75x", n.Value)
		assert.Equal(t, platform.ToggledUnknown, n.Toggle, "the speed is no toggle")
		assert.Equal(t, platform.ToggledOff, nodeCalled(t, tt, "Gương").Toggle)
		mirror.SetOn(true)
		assert.Equal(t, platform.ToggledOn, nodeCalled(t, tt, "Gương").Toggle)
	})
}

// TestIconButtonSpeaksItsStateAndValue checks the toggle and the value of an icon
// button (the loop of screen 08).
func TestIconButtonSpeaksItsStateAndValue(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		loop := kit.NewIconButton(icon.Rep, "Lặp A–B")
		loop.State = platform.ToggledOff
		plain := kit.NewIconButton(icon.More, "Tuỳ chọn")
		tt := mountStage(t, ph, placeAt(loop, 20, 100, 0), placeAt(plain, 100, 100, 0))
		n := nodeCalled(t, tt, "Lặp A–B")
		assert.Equal(t, platform.ToggledOff, n.Toggle)
		assert.Empty(t, n.Value)
		loop.State, loop.Value = platform.ToggledOn, "01:12–01:40"
		n = nodeCalled(t, tt, "Lặp A–B")
		assert.Equal(t, platform.ToggledOn, n.Toggle)
		assert.Equal(t, "01:12–01:40", n.Value)
		assert.Equal(t, platform.ToggledUnknown, nodeCalled(t, tt, "Tuỳ chọn").Toggle, "a plain button says no state")
	})
}

// TestChosenOnesAreSelected checks that the chosen tab, segment and radio are
// FlagSelected, and the others not; none of them is a toggle: the iOS bridge
// would read a toggle as a "toggle button" with 0 or 1 for a value (P8.3d dump).
func TestChosenOnesAreSelected(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		bar := kit.NewTabBar(tabItems(), 2, nil)
		seg := kit.NewSegmented("0.5x", "1x", "1.5x")
		seg.Select(1)
		a, b := kit.NewRadio("Chỉ bài này", "x", "3 MB"), kit.NewRadio("Cả khoá", "y", "9 MB")
		b.On = true
		tt := mountStage(t, ph, placeAt(seg, 20, 100, 353), placeAt(a, 20, 200, 353), placeAt(b, 20, 300, 353), placeAt(&barAtBottom{bar: bar}, 0, 0, 0))
		selected := map[string]bool{}
		for _, n := range tt.Semantics().Nodes {
			switch n.Role {
			case platform.RoleTab, platform.RoleRadio:
				on := n.Flags&platform.FlagSelected != 0
				assert.Equalf(t, platform.ToggledUnknown, n.Toggle, "%q: no toggle", n.Label)
				selected[n.Label] = on
			default:
			}
		}
		assert.Equal(t, map[string]bool{
			"Trang chủ": false, "Khoá học": false, "Tải xuống": true, "Tài khoản": false,
			"0.5x": false, "1x": true, "1.5x": false, "Chỉ bài này": false, "Cả khoá": true,
		}, selected)
	})
}

// TestBeatsAreOneElement checks the eight numbers of the beats are not read one
// by one: the bar is one text named by its label.
func TestBeatsAreOneElement(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		beats := kit.NewBeats()
		beats.Label = "Thanh đếm 8 nhịp"
		tt := mountStage(t, ph, placeAt(beats, 20, 683, 353))
		sem := tt.Semantics()
		require.Len(t, sem.Nodes, 2, "the window and the bar")
		assert.Equal(t, "Thanh đếm 8 nhịp", sem.Nodes[1].Label)
		assert.Equal(t, platform.RoleText, sem.Nodes[1].Role)
	})
}

// TestCodeBoxesAreOneField checks the six boxes and the dash are not read: the
// code is one text field named by the row's label, valued by the code typed.
func TestCodeBoxesAreOneField(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		row := kit.NewCodeBoxes("Mã trên TV")
		row.SetValue("7K4M")
		tt := mountStage(t, ph, placeAt(row, 20, 372, ph.Size.W-40))
		sem := tt.Semantics()
		require.Len(t, sem.Nodes, 2, "the window and the field: %v", sem.Nodes)
		n := sem.Nodes[1]
		assert.Equal(t, platform.RoleTextField, n.Role)
		assert.Equal(t, "Mã trên TV", n.Label)
		assert.Equal(t, "7K4M", n.Value)
	})
}

// TestProgressIsNamed checks a bar with a label reads it, with its range and a
// value: the percent, or the reading its owner made. The role is one the iOS
// bridge gives a trait (P8.3d: RoleProgress has none).
func TestProgressIsNamed(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p := &kit.Progress{Label: "Bộ nhớ máy"}
		p.SetValue(.71)
		tt := mountStage(t, ph, placeAt(p, 20, 100, 325))
		n := nodeCalled(t, tt, "Bộ nhớ máy")
		assert.Equal(t, platform.RoleText, n.Role)
		assert.InDelta(t, .71, n.Now, 1e-6)
		assert.InDelta(t, 0, n.Min, 0)
		assert.InDelta(t, 1, n.Max, 0)
		assert.Equal(t, "71%", n.Value, "the percent")
		p.SetSpoken("01:12 / 03:40")
		tt.Elapse(kittest.Frame)
		assert.Equal(t, "01:12 / 03:40", nodeCalled(t, tt, "Bộ nhớ máy").Value, "the reading of the owner")
		assert.Positive(t, tt.SemanticsChanges(platform.ChangeValue), "SetSpoken tells the screen reader")
	})
}

// TestPercentReadsWholePercent checks the text of a share: rounded, clamped, and
// allocating nothing.
func TestPercentReadsWholePercent(t *testing.T) {
	t.Parallel()

	for share, want := range map[float32]string{-1: "0%", 0: "0%", .004: "0%", .005: "1%", .424: "42%", .425: "43%", 1: "100%", 7: "100%"} {
		assert.Equal(t, want, kit.Percent(share), "%v", share)
	}
}

// TestTitlesAreHeadings checks the title of a page (.h1) and the title of the
// top bar are headings of level 1, the title of a section (.sec .d) of level 2.
func TestTitlesAreHeadings(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		bar := kit.NewTopBar("Thông tin tài khoản", "Quay lại", nil)
		tt := mountStage(t, ph, placeAt(kit.H1("Tài khoản"), 20, 70, 353), placeAt(kit.SecTitle("Mới cập nhật"), 20, 200, 353), placeAt(bar, 20, 300, 353))
		for label, level := range map[string]int32{"Tài khoản": 1, "Mới cập nhật": 2, "Thông tin tài khoản": 1} {
			n := nodeCalled(t, tt, label)
			assert.Equal(t, platform.RoleHeading, n.Role, label)
			assert.Equal(t, level, n.Level, label)
		}
	})
}

// TestToastIsAStatusThatHidesNothing checks the bubble is a status named by its
// message, one element with nothing inside it read twice, after what lies under it.
func TestToastIsAStatusThatHidesNothing(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		toast := kit.NewToast()
		under := kit.NewButton(kit.ButtonP, "Dưới", icon.NameUnknown)
		tt := kittest.Mount(t, ph, newOverlays(newStage(placeAt(under, 100, 100, 200)), toast))
		kittest.PinMotion(tt)
		tt.Semantics()
		toast.Show("Mã không đúng")
		tt.Elapse(300 * time.Millisecond)
		sem := tt.Semantics()
		require.Len(t, sem.Nodes, 3, "the window, the button and the status: %v", sem.Nodes)
		assert.Equal(t, "Dưới", sem.Nodes[1].Label)
		assert.Equal(t, platform.RoleStatus, sem.Nodes[2].Role)
		assert.Equal(t, "Mã không đúng", sem.Nodes[2].Label)
		assert.Equal(t, []string{"Mã không đúng"}, tt.Announcements())
		assert.True(t, tt.PerformAction("Dưới", platform.ActPress), "the screen under it is pressed")
		tt.Elapse(2300 * time.Millisecond)
		assert.True(t, noNodeCalled(tt, "Mã không đúng"), "gone with the toast")
	})
}

// TestLessonRowHoldsItsButtons checks a row with a download button is a list item
// with the button an element under it, named by the state of the file (download,
// cancel, downloaded), that a screen reader presses; a locked row is a button.
func TestLessonRowHoldsItsButtons(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var log []string
		row := &kit.LessonRow{No: 5, Title: "Hãy Hát Lên", Meta: "Trending · 03:30", DlLabel: "Tải xuống",
			CancelLabel: "Huỷ tải", DoneLabel: "Đã tải", PlayLabel: "Phát"}
		row.OnDl = func() { log = append(log, "dl") }
		row.OnPress = func() { log = append(log, "row") }
		tt := mountStage(t, ph, placeAt(row, 20, 200, 353))
		sem := tt.Semantics()
		item := nodeCalled(t, tt, "Hãy Hát Lên")
		assert.Equal(t, platform.RoleListItem, item.Role)
		assert.True(t, item.Actions.Has(platform.ActPress))
		child := childNode(t, sem, item, "Tải xuống")
		assert.Equal(t, platform.RoleButton, child.Role)
		assert.True(t, tt.PerformAction("Tải xuống", platform.ActPress))
		assert.True(t, tt.PerformAction("Hãy Hát Lên", platform.ActPress))
		assert.Equal(t, []string{"dl", "row"}, log)

		row.Download = kit.DlIng
		row.Relayout()
		tt.Elapse(kittest.Frame)
		assert.True(t, noNodeCalled(tt, "Tải xuống"))
		nodeCalled(t, tt, "Huỷ tải")
		row.Download = kit.DlDone
		row.Relayout()
		tt.Elapse(kittest.Frame)
		nodeCalled(t, tt, "Đã tải")

		row.Mode = kit.LessonModeLocked
		row.Relayout()
		tt.Elapse(kittest.Frame)
		assert.Equal(t, platform.RoleButton, nodeCalled(t, tt, "Hãy Hát Lên").Role, "no button of its own: a button")
		assert.True(t, noNodeCalled(tt, "Đã tải"))
	})
}

// TestDescribeButtonsSkipsWhatCannotBePressed checks the helper of rows: a nil or a
// hidden button is no element and takes no press; a disabled one is an element
// marked disabled that takes none.
func TestDescribeButtonsSkipsWhatCannotBePressed(t *testing.T) {
	t.Parallel()

	pressed := 0
	mk := func() *ui.Pressable {
		p := &ui.Pressable{}
		p.OnPress = func() { pressed++ }

		return p
	}
	shown, hidden, off := mk(), mk(), mk()
	hidden.Hidden, off.Disabled = true, true
	assert.False(t, kit.PressButton(platform.ActPress, 0, shown), "key 0 is the widget's own")
	assert.False(t, kit.PressButton(platform.ActPress, 4, shown, hidden, off), "past the buttons")
	assert.False(t, kit.PressButton(platform.ActIncrement, 1, shown), "only a press")
	assert.False(t, kit.PressButton(platform.ActPress, 2, shown, hidden, off), "hidden")
	assert.False(t, kit.PressButton(platform.ActPress, 3, shown, hidden, off), "disabled")
	assert.False(t, kit.PressButton(platform.ActPress, 1, nil), "nil")
	assert.True(t, kit.PressButton(platform.ActPress, 1, shown, hidden, off))
	assert.Equal(t, 1, pressed)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// childNode returns the element called label under parent.
func childNode(t testing.TB, sem *platform.Semantics, parent platform.SemanticNode, label string) platform.SemanticNode {
	t.Helper()
	for _, n := range sem.Nodes {
		if n.Label == label && n.Parent >= 0 && sem.Nodes[n.Parent].ID == parent.ID {
			return n
		}
	}
	require.Failf(t, "no child", "%q is not under %q", label, parent.Label)

	return platform.SemanticNode{}
}
