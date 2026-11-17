//go:build tvos

// Pattern from dancefitvn-app/internal/ui/mobile/screens/a11y_test.go (P8.3a) and
// jplayer-tv/internal/ui/{overlay,views,watch}/semantics_test.go at 781bee5 (what a
// screen reader reads of a screen, found by the tree Tester.Semantics returns:
// labels, roles, the layer under a modal, the toast), over every capture state of
// the prototype's TV at 1920x1080.

package screens_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// a11yState is a capture state of the prototype's TV as the app reaches it in a
// test: the rig at it, rested, with a screen reader listening.
type a11yState struct {
	// id is the state's id in local/shots (TV01 to TV12, TV-toast).
	id string
	// panel tells that the panel of the player is open over the route.
	panel bool
	open  func(t *testing.T) *rig
}

// toastText is the toast of TV-toast: the greeting of the account after the pairing.
const toastText = "Đã đăng nhập bằng thaovy.hlv@gmail.com"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// a11yStates are the capture states of the TV: TV01 to TV12 and the toast.
func a11yStates() []a11yState {
	at := func(svc func(testing.TB) *tvtest.Services, route nav.Route) func(t *testing.T) *rig {
		return func(t *testing.T) *rig {
			t.Helper()
			r := openAt(t, svc(t), route, false)
			r.tt.View().Context().Theme().ReduceMotion = true
			r.settle()

			return r
		}
	}
	signed := tvtest.NewSignedServices
	plain := func(testing.TB) *tvtest.Services { return tvtest.NewServices() }

	return []a11yState{
		{id: "TV01", open: at(plain, nav.Route{Screen: env.RouteWelcome})},
		{id: "TV02", open: func(t *testing.T) *rig {
			r, _, p := onCode(t, false)
			r.tt.View().Context().Theme().ReduceMotion = true
			p.Push(mockState(), 272*time.Second)
			r.settle()

			return r
		}},
		{id: "TV04", open: at(signed, nav.Route{Screen: env.RouteHome})},
		{id: "TV05", open: at(signed, nav.Route{Screen: env.RouteCourses})},
		{id: "TV06", open: at(signed, nav.Route{Screen: env.RouteDetail, Params: "set-35"})},
		{id: "TV07", open: at(playerState, nav.Route{Screen: env.RoutePlayer})},
		{id: "TV08", panel: true, open: func(t *testing.T) *rig { return atPanel(t) }},
		{id: "TV09", open: at(audioState, nav.Route{Screen: env.RouteMusic})},
		{id: "TV10", open: func(t *testing.T) *rig {
			r := inUpNext(t, upNextState(t), false)
			r.settle()

			return r
		}},
		{id: "TV11", open: at(offlineServices, nav.Route{Screen: env.RouteError})},
		{id: "TV12", open: at(playerState, nav.Route{Screen: env.RouteSettings})},
		{id: "TV-toast", open: func(t *testing.T) *rig {
			r := at(signed, nav.Route{Screen: env.RouteHome})(t)
			r.tt.Semantics() // a screen reader is listening
			r.sh.Toast(toastText)
			r.tt.Elapse(300 * time.Millisecond)

			return r
		}},
	}
}

// atPanel is TV08: the options of the player open over TV07, `p2` pressed.
func atPanel(t *testing.T) *rig {
	t.Helper()
	r := openAt(t, playerState(t), nav.Route{Screen: env.RoutePlayer}, false)
	r.tt.View().Context().Theme().ReduceMotion = true
	r.settle()
	r.focusKey("p2")
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	require.True(t, r.sh.Stack().Panel(), "TV08: the panel is open")

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestA11yEveryStateIsLabelled checks, in every capture state, what a screen
// reader finds: every element it can press or focus has a label that is no icon
// key; every focusable item of what is reachable is an element of its own with
// its own box; the focus is an element (VoiceOver on tvOS follows it); the
// panel leaves nothing of the player under it; the screen has a heading of
// level 1.
func TestA11yEveryStateIsLabelled(t *testing.T) {
	t.Parallel()

	for _, s := range a11yStates() {
		t.Run(s.id, func(t *testing.T) {
			t.Parallel()
			r := s.open(t)
			sem := r.tt.Semantics()
			require.NotEmpty(t, sem.Nodes)
			if os.Getenv("A11Y_DUMP") != "" {
				t.Logf("\n%s\n%s", s.id, dumpTree(sem))
			}
			checkLabels(t, sem)
			checkItemsAreElements(t, r, sem)
			checkFocusIsAnElement(t, r, sem)
			if s.panel {
				checkNothingUnderThePanel(t, r, sem)
			}
			checkHasTitle(t, sem)
		})
	}
}

// TestA11yReadingOrderIsVisualOrder checks the order of the elements of TV02,
// TV04, TV06, TV07 and TV08 against the order of the screen, top to bottom and
// left to right: the labels as a screen reader walks them. The nav bar, painted
// over the page, is read first as it is the top of it.
func TestA11yReadingOrderIsVisualOrder(t *testing.T) {
	t.Parallel()

	nav := []string{"DANCEFIT", "Trang chủ", "Khoá học", "Nhạc tập", "Cài đặt"}
	golden := map[string][]string{
		"TV02": {
			"Đăng nhập Apple TV", "Dùng tài khoản huấn luyện viên đã đăng ký trên dancefitvn.com",
			"Mở app DanceFITVN trên điện thoại", "Vào Tài khoản → Kết nối Apple TV", "Nhập mã bên dưới", "Mã liên kết",
			"Mã hết hạn sau 04:32 · mã mới sẽ tự tạo",
			"Hoặc quét mã QR", "Mở dancefitvn.com/mobile/tv trên điện thoại", "Quay lại",
		},
		"TV04": append(append([]string{}, nav...),
			"TIẾP TỤC XEM", "DanceFIT Set 35 · BÀI 03 / 10 · Bụng", "Stink and Dutty",
			"“Cùng nhảy cùng hát” · 03:40 · đang xem 01:12", "Vị trí phát", "Tiếp tục xem", "10 bài học",
			"Khoá học của tôi · 15", "DanceFIT Set 35", "DanceFIT Set 34", "DanceFIT Hot Extra", "DanceFIT Set 33"),
		"TV06": {
			"KHOÁ HỌC Thường · 10 BÀI HỌC", "DanceFIT Set 35", "“Cùng nhảy cùng hát”", "Mỗi bài gồm video hướng dẫn và file âm thanh.",
			"Tiếp tục bài 03", "Quay lại", "BÀI HỌC",
			"01 · DanceFIT Warm Up 13", "02 · Made You Look", "03 · Stink and Dutty", "04 · Em Yêu Người Ta", "05 · Hãy Hát Lên",
			"06 · Nadi Nadi", "07 · Viva La Vida", "08 · We So Fly", "09 · WET", "10 · One Love",
		},
		"TV07": {
			"DanceFIT Set 35 · BÀI 03 / 10", "Stink and Dutty", "Bụng", "Video dọc 9:16", "Tốc độ 0.75x", "Lật gương",
			"Thanh đếm nhịp", "01:12", "03:40", "Vị trí phát", "Phát", "Tuỳ chọn",
			"TIẾP THEO", "04 · Em Yêu Người Ta", "05 · Hãy Hát Lên", "06 · Nadi Nadi", "07 · Viva La Vida",
		},
		"TV08": {
			"Tuỳ chọn phát", "Tốc độ", "0.5x", "0.75x", "1x", "1.25x", "Hình ảnh & lặp", "Lật gương", "Lặp đoạn A–B",
			"Thanh đếm nhịp", "Đóng",
		},
	}
	states := map[string]a11yState{}
	for _, s := range a11yStates() {
		states[s.id] = s
	}
	for id, want := range golden {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			r := states[id].open(t)
			var got []string
			for _, n := range r.tt.Semantics().Nodes[1:] {
				if n.Role == platform.RoleScroll {
					continue // an area, not a thing to read
				}
				got = append(got, n.Label)
			}
			assert.Equal(t, want, got)
		})
	}
}

// TestDetailDownFromContinueLandsOnLesson09 checks the move the P8.3d simulator run
// questioned: Down from `d1` ("Tiếp tục bài 03") goes to `dl8` ("09 · WET"), the
// one the prototype's moveFocus reaches (focus/testdata/screen_vectors.json, TV06
// d1 ArrowDown to dl8). `dl7` lies beside `d1` (its centre is 2 below `d1`'s, and
// moveFocus wants more than 4), so the nearest one under it is `dl8`; with the
// stand-in fonts of the plain Mount `d1` is 17 higher and `dl7` would win, which is
// the font, not the app. Down again goes to `dl9`.
func TestDetailDownFromContinueLandsOnLesson09(t *testing.T) {
	t.Parallel()

	r := openAtFast(t, tvtest.NewSignedServices(t), nav.Route{Screen: env.RouteDetail, Params: "set-35"}) // the layout fonts of the prototype's boxes
	r.settle()
	require.Equal(t, "d1", r.key())
	require.True(t, r.press(keys.KeyDown))
	r.settle()
	assert.Equal(t, "dl8", r.key(), "the prototype's d1 ArrowDown")
	sem := r.tt.Semantics()
	assert.Equal(t, "09 · WET", sem.Nodes[sem.Focus].Label)
	r.tt.Elapse(time.Second) // the list scrolls to show it
	require.True(t, r.press(keys.KeyDown))
	r.tt.Elapse(time.Second)
	assert.Equal(t, "dl9", r.key())
}

// TestA11yPositionBarsHaveAReading checks the bars of TV04, TV07 and TV09 say where
// they are as the clocks do ("01:12 / 03:40"), and that the reading follows the
// second while it plays: a bar with a Now and no value is silent on tvOS.
func TestA11yPositionBarsHaveAReading(t *testing.T) {
	t.Parallel()

	t.Run("TV04", func(t *testing.T) {
		t.Parallel()
		r := openAt(t, playerState(t), nav.Route{Screen: env.RouteHome}, false)
		r.tt.View().Context().Theme().ReduceMotion = true
		r.settle()
		bar, ok := findNode(r.tt.Semantics(), "Vị trí phát", platform.RoleText)
		require.True(t, ok)
		assert.Equal(t, "01:12 / 03:40", bar.Value)
	})
	t.Run("TV07", func(t *testing.T) {
		t.Parallel()
		r := openAt(t, playerState(t), nav.Route{Screen: env.RoutePlayer}, false)
		r.tt.View().Context().Theme().ReduceMotion = true
		r.settle()
		bar, ok := findNode(r.tt.Semantics(), "Vị trí phát", platform.RoleText)
		require.True(t, ok)
		assert.Equal(t, "01:12 / 03:40", bar.Value)
		told := func() int { // the clock's text lays out: the tick is told as a layout change, a bare value as a value one
			return r.tt.SemanticsChanges(platform.ChangeLayout) + r.tt.SemanticsChanges(platform.ChangeValue)
		}
		changes := told()
		pb := r.playback()
		pb.Tick(73.4)
		r.settle()
		r.tt.Elapse(time.Second) // a value change is told once the quiet time has passed
		bar, _ = findNode(r.tt.Semantics(), "Vị trí phát", platform.RoleText)
		assert.Equal(t, "01:13 / 03:40", bar.Value, "the next second")
		assert.Greater(t, told(), changes, "the new second tells the screen reader")
	})
	t.Run("TV09", func(t *testing.T) {
		t.Parallel()
		r := openAt(t, audioState(t), nav.Route{Screen: env.RouteMusic}, false)
		r.tt.View().Context().Theme().ReduceMotion = true
		r.settle()
		bar, ok := findNode(r.tt.Semantics(), "Vị trí phát", platform.RoleText)
		require.True(t, ok)
		assert.Equal(t, "01:12 / 03:40", bar.Value)
	})
}

// TestA11yPanelHushesThePlayerUnderIt checks TV08: while the panel is open nothing
// of the player is read (not its buttons, not its next lessons) and nothing under
// it takes a press; the panel is announced as a new screen; once it closes (Menu)
// the player is read again and the focus is on `p2`.
func TestA11yPanelHushesThePlayerUnderIt(t *testing.T) {
	t.Parallel()

	r := openAt(t, playerState(t), nav.Route{Screen: env.RoutePlayer}, false)
	r.tt.View().Context().Theme().ReduceMotion = true
	r.settle()
	sem := r.tt.Semantics()
	for _, l := range []string{"Phát", "Tuỳ chọn", "TIẾP THEO", "04 · Em Yêu Người Ta", "Stink and Dutty", "Vị trí phát", "Thanh đếm nhịp"} {
		require.Truef(t, hasLabel(sem, l), "%q is read before the panel opens", l)
	}
	changes := r.tt.SemanticsChanges(platform.ChangeScreen)
	r.focusKey("p2")
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	require.True(t, r.sh.Stack().Panel())
	assert.Greater(t, r.tt.SemanticsChanges(platform.ChangeScreen), changes, "the panel tells a new screen")

	sem = r.tt.Semantics()
	for _, l := range []string{"Phát", "Tuỳ chọn", "TIẾP THEO", "04 · Em Yêu Người Ta", "Stink and Dutty", "Vị trí phát", "01:12", "Tốc độ 0.75x"} {
		assert.Falsef(t, hasLabel(sem, l), "%q is read under the panel", l)
	}
	assert.False(t, r.tt.PerformAction("Phát", platform.ActPress), "nothing under the panel takes a press")
	assert.False(t, r.tt.PerformAction("04 · Em Yêu Người Ta", platform.ActPress))
	assert.True(t, hasLabel(sem, "Tuỳ chọn phát"), "the panel is read")

	changes = r.tt.SemanticsChanges(platform.ChangeScreen)
	assert.True(t, r.sh.Back(), "Menu closes the panel")
	r.settle()
	assert.False(t, r.sh.Stack().Panel())
	assert.Greater(t, r.tt.SemanticsChanges(platform.ChangeScreen), changes, "closing it tells a new screen")
	sem = r.tt.Semantics()
	assert.True(t, hasLabel(sem, "Tuỳ chọn"), "the player is read again")
	assert.False(t, hasLabel(sem, "Tuỳ chọn phát"), "the panel is gone")
	assert.Equal(t, "p2", r.key())
	assert.Equal(t, "Tuỳ chọn", sem.Nodes[sem.Focus].Label, "the focus is on the button that opened the panel")
}

// TestA11yPanelOptionsSayTheirState checks the rows of the panel: the speed in
// force is the chosen radio button, the hints are values, the three toggles are
// switches whose state is the "Bật" or "Tắt" the row shows, and a press of a
// toggle by a screen reader flips it.
func TestA11yPanelOptionsSayTheirState(t *testing.T) {
	t.Parallel()

	r := atPanel(t)
	sem := r.tt.Semantics()
	want := []struct {
		label, value string
		role         platform.SemanticRole
		on           bool
	}{
		{"0.5x", "Chậm để tách động tác", platform.RoleRadio, false},
		{"0.75x", "Đang dùng", platform.RoleRadio, true},
		{"1x", "Bình thường", platform.RoleRadio, false},
		{"1.25x", "", platform.RoleRadio, false},
		{"Lật gương", "Bật", platform.RoleSwitch, true},
		{"Lặp đoạn A–B", "Tắt", platform.RoleSwitch, false},
		{"Thanh đếm nhịp", "Bật", platform.RoleSwitch, true},
	}
	for _, w := range want {
		n, ok := findNode(sem, w.label, w.role)
		require.Truef(t, ok, "%q as role %d", w.label, w.role)
		assert.Equalf(t, w.value, n.Value, "%q", w.label)
		if w.role == platform.RoleRadio {
			// the speed in force says so in its value; none is FlagSelected (the focused row is
			// the selected one of the bridge) and none is a toggle (it would speak 0 or 1)
			assert.Equalf(t, platform.ToggledUnknown, n.Toggle, "%q is no toggle", w.label)
			assert.Falsef(t, n.Flags.Has(platform.FlagSelected), "%q is not FlagSelected", w.label)
		} else {
			assert.Equalf(t, w.on, n.Toggle == platform.ToggledOn, "%q is on", w.label)
		}
		assert.Truef(t, n.Actions.Has(platform.ActPress), "%q takes a press", w.label)
	}
	pb := r.playback()
	require.True(t, pb.LoopV == false)
	assert.True(t, r.tt.PerformAction("Lặp đoạn A–B", platform.ActPress))
	assert.True(t, pb.LoopV, "a screen reader's press flips the toggle")
	pb.Publish(player.ChangeOptions) // the player tells what changed, as the real one does
	r.settle()
	loop, _ := findNode(r.tt.Semantics(), "Lặp đoạn A–B", platform.RoleSwitch)
	assert.Equal(t, "Bật", loop.Value)
	assert.Equal(t, platform.ToggledOn, loop.Toggle)
	assert.True(t, r.tt.PerformAction("1.25x", platform.ActPress))
	assert.InDelta(t, 1.25, pb.RateV, 1e-9)
	pb.Publish(player.ChangeRate)
	r.settle()
	speed, _ := findNode(r.tt.Semantics(), "1.25x", platform.RoleRadio)
	assert.Equal(t, "Đang dùng", speed.Value, "the new speed is in force")
	slow, _ := findNode(r.tt.Semantics(), "0.75x", platform.RoleRadio)
	assert.Empty(t, slow.Value, "the old one no longer")
	assert.True(t, r.tt.PerformAction("Đóng", platform.ActPress))
	r.settle()
	assert.False(t, r.sh.Stack().Panel(), "the close button closes it")
}

// TestA11yToastIsAnnouncedAndHidesNothing checks the toast of TV-toast: announced
// once, a status in the tree after the screen, and the screen under it still read
// and pressed; it goes with the toast.
func TestA11yToastIsAnnouncedAndHidesNothing(t *testing.T) {
	t.Parallel()

	var toast a11yState
	for _, s := range a11yStates() {
		if s.id == "TV-toast" {
			toast = s
		}
	}
	r := toast.open(t)
	assert.Equal(t, []string{toastText}, r.tt.Announcements())
	sem := r.tt.Semantics()
	node, ok := findNode(sem, toastText, platform.RoleStatus)
	require.True(t, ok, "the toast is a status")
	assert.Zero(t, node.Flags&platform.FlagFocusable, "it takes no focus")
	assert.Equal(t, node.ID, sem.Nodes[len(sem.Nodes)-1].ID, "after the screen, as it lies over it")
	assert.True(t, hasLabel(sem, "Tiếp tục xem"), "the screen is still read")
	assert.True(t, hasLabel(sem, "Trang chủ"))
	assert.Equal(t, 1, countLabel(sem, toastText), "once: its text is not read a second time")

	r.sh.Toast(toastText) // the same notice again while it shows
	r.tt.Elapse(50 * time.Millisecond)
	assert.Len(t, r.tt.Announcements(), 2, "a toast is announced each time it is shown")

	r.tt.Elapse(tvkit.ToastTime + tvkit.ToastFade)
	assert.False(t, hasLabel(r.tt.Semantics(), toastText), "gone with the toast")
}

// TestA11yScreenChangesAreTold checks that going to a screen, going back, switching
// the tab and the panel each tell a new screen to a screen reader that is reading,
// which starts again from the first element.
func TestA11yScreenChangesAreTold(t *testing.T) {
	t.Parallel()

	r := openAt(t, tvtest.NewSignedServices(t), nav.Route{Screen: env.RouteHome}, false)
	r.tt.View().Context().Theme().ReduceMotion = true
	r.settle()
	r.tt.Semantics()
	n := r.tt.SemanticsChanges(platform.ChangeScreen)

	steps := []struct {
		name string
		do   func()
	}{
		{"go (tvGo to the course)", func() { r.sh.Go(nav.Route{Screen: env.RouteDetail, Params: "set-35"}, false) }},
		{"back (tvBack to the home)", func() { require.True(t, r.sh.Back()) }},
		{"a tab (Select on a nav item)", func() { r.sh.SwitchTab(3) }},
		{"replace (the player takes the up-next screen's place)", func() { r.sh.Replace(nav.Route{Screen: env.RouteCourses}) }},
	}
	for _, s := range steps {
		s.do()
		r.settle()
		r.tt.Semantics()
		next := r.tt.SemanticsChanges(platform.ChangeScreen)
		assert.Greaterf(t, next, n, "%s tells a new screen", s.name)
		n = next
	}
}

// TestA11yFocusIsTheElement checks that VoiceOver on tvOS, which follows the focus,
// finds the focused item as the element the tree names: a move of the focus is
// told (ChangeFocus) and the element after it is the new item, with its label and
// its box.
func TestA11yFocusIsTheElement(t *testing.T) {
	t.Parallel()

	r := openAt(t, tvtest.NewSignedServices(t), nav.Route{Screen: env.RouteHome}, false)
	r.tt.View().Context().Theme().ReduceMotion = true
	r.settle()
	sem := r.tt.Semantics()
	require.Equal(t, "h1", r.key())
	assert.Equal(t, "Tiếp tục xem", sem.Nodes[sem.Focus].Label)
	moved := r.tt.SemanticsChanges(platform.ChangeFocus)
	require.True(t, r.press(keys.KeyRight))
	require.Equal(t, "h2", r.key())
	assert.Greater(t, r.tt.SemanticsChanges(platform.ChangeFocus), moved, "a move of the focus is told")
	sem = r.tt.Semantics()
	assert.Equal(t, "10 bài học", sem.Nodes[sem.Focus].Label)
	require.True(t, r.press(keys.KeyDown))
	require.True(t, strings.HasPrefix(r.key(), "hc"), "a tile: %s", r.key())
	sem = r.tt.Semantics()
	focused := sem.Nodes[sem.Focus]
	assert.True(t, strings.HasPrefix(focused.Label, "DanceFIT "), "a poster tile is named by the course: %q", focused.Label)
	assert.Equal(t, "10 bài học", focused.Value)
	assert.True(t, focused.Flags.Has(platform.FlagFocusable))
	require.True(t, r.press(keys.KeyUp))
	require.True(t, r.press(keys.KeyUp))
	require.True(t, strings.HasPrefix(r.key(), "nav"), "the nav bar: %s", r.key())
	sem = r.tt.Semantics()
	tab := sem.Nodes[sem.Focus]
	assert.Equal(t, platform.RoleTab, tab.Role)
	assert.Equal(t, "Trang chủ", tab.Label)
	assert.True(t, tab.Flags.Has(platform.FlagSelected), "the tab of the screen showing is selected")
	other, ok := findNode(sem, "Khoá học", platform.RoleTab)
	require.True(t, ok)
	assert.False(t, other.Flags.Has(platform.FlagSelected))
}

// TestA11yPressesFromAScreenReader checks that a screen reader's press does what
// Select does on the items that hold an action of their own: a poster tile opens
// its course, a lesson row plays it, a settings row changes its value, the nav bar
// switches the screen.
func TestA11yPressesFromAScreenReader(t *testing.T) {
	t.Parallel()

	t.Run("tile", func(t *testing.T) {
		t.Parallel()
		r := openAt(t, tvtest.NewSignedServices(t), nav.Route{Screen: env.RouteHome}, false)
		r.tt.View().Context().Theme().ReduceMotion = true
		r.settle()
		r.tt.Semantics()
		require.True(t, r.tt.PerformAction("DanceFIT Set 34", platform.ActPress))
		r.settle()
		assert.Equal(t, env.RouteDetail, r.sh.Stack().Screen())
	})
	t.Run("lesson", func(t *testing.T) {
		t.Parallel()
		r := openAt(t, tvtest.NewSignedServices(t), nav.Route{Screen: env.RouteDetail, Params: "set-35"}, false)
		r.tt.View().Context().Theme().ReduceMotion = true
		r.settle()
		r.tt.Semantics()
		require.True(t, r.tt.PerformAction("04 · Em Yêu Người Ta", platform.ActPress))
		r.settle()
		assert.Equal(t, env.RoutePlayer, r.sh.Stack().Screen())
	})
	t.Run("settings", func(t *testing.T) {
		t.Parallel()
		r := openAt(t, playerState(t), nav.Route{Screen: env.RouteSettings}, false)
		r.tt.View().Context().Theme().ReduceMotion = true
		r.settle()
		sem := r.tt.Semantics()
		auto, ok := findNode(sem, "Tự phát bài tiếp theo", platform.RoleButton)
		require.True(t, ok)
		assert.Equal(t, "Bật", auto.Value)
		require.True(t, r.tt.PerformAction("Tự phát bài tiếp theo", platform.ActPress))
		auto, _ = findNode(r.tt.Semantics(), "Tự phát bài tiếp theo", platform.RoleButton)
		assert.Equal(t, "Tắt", auto.Value, "the row says its new value")
	})
	t.Run("nav", func(t *testing.T) {
		t.Parallel()
		r := openAt(t, tvtest.NewSignedServices(t), nav.Route{Screen: env.RouteHome}, false)
		r.tt.View().Context().Theme().ReduceMotion = true
		r.settle()
		r.tt.Semantics()
		require.True(t, r.tt.PerformAction("Khoá học", platform.ActPress))
		r.settle()
		assert.Equal(t, env.RouteCourses, r.sh.Stack().Screen())
		tab, ok := findNode(r.tt.Semantics(), "Khoá học", platform.RoleTab)
		require.True(t, ok)
		assert.True(t, tab.Flags.Has(platform.FlagSelected))
	})
}

// TestA11yCoursesFiltersAreRadios checks TV05: the four filters are radio buttons
// named by their text and count, the one in force chosen, and a screen reader's
// press changes the filter; the tiles are named by the course.
func TestA11yCoursesFiltersAreRadios(t *testing.T) {
	t.Parallel()

	r := openAt(t, tvtest.NewSignedServices(t), nav.Route{Screen: env.RouteCourses}, false)
	r.tt.View().Context().Theme().ReduceMotion = true
	r.settle()
	sem := r.tt.Semantics()
	var chips []platform.SemanticNode
	for _, n := range sem.Nodes {
		if n.Role == platform.RoleRadio {
			chips = append(chips, n)
		}
	}
	require.Len(t, chips, 4)
	assert.True(t, strings.HasPrefix(chips[0].Label, "Tất cả · "), chips[0].Label)
	assert.True(t, chips[0].Flags.Has(platform.FlagSelected), "the first filter is in force")
	assert.Equal(t, platform.ToggledUnknown, chips[0].Toggle, "a filter is a selected button, not a toggle")
	for _, c := range chips[1:] {
		assert.Falsef(t, c.Flags.Has(platform.FlagSelected), "%q", c.Label)
		assert.Contains(t, c.Label, " · ", "the count is part of the name")
	}
	assert.True(t, hasLabel(sem, "DanceFIT Set 35"), "a tile is named by the course")
	require.True(t, r.tt.PerformAction(chips[1].Label, platform.ActPress))
	r.settle()
	sem = r.tt.Semantics()
	chosen, ok := findNode(sem, chips[1].Label, platform.RoleRadio)
	require.True(t, ok)
	assert.True(t, chosen.Flags.Has(platform.FlagSelected), "the press chose the filter")
	first, _ := findNode(sem, chips[0].Label, platform.RoleRadio)
	assert.False(t, first.Flags.Has(platform.FlagSelected))
}

// TestA11yPairingCodeIsReadAsACode checks TV02: the code is one element named
// "Mã liên kết" whose value spells it one character at a time, the dash left
// out; the QR symbol and the numbers of the steps are not read.
func TestA11yPairingCodeIsReadAsACode(t *testing.T) {
	t.Parallel()

	r, _, p := onCode(t, false)
	r.tt.View().Context().Theme().ReduceMotion = true
	sem := r.tt.Semantics()
	code, ok := findNode(sem, "Mã liên kết", platform.RoleText)
	require.True(t, ok)
	assert.Empty(t, code.Value, "no code before the first state")
	p.Push(mockState(), mockExpires)
	r.settle()
	sem = r.tt.Semantics()
	code, ok = findNode(sem, "Mã liên kết", platform.RoleText)
	require.True(t, ok)
	assert.Equal(t, "7 K 4 M 2 P", code.Value)
	for _, n := range sem.Nodes {
		assert.NotContainsf(t, []string{"1", "2", "3", "7K4-M2P", "7K4M2P"}, n.Label, "%q is read as a word or a bare number", n.Label)
	}
	qr := r.layout()
	box, ok := qr.Find("tv.code.qr")
	require.True(t, ok)
	for i, n := range sem.Nodes {
		if i == 0 {
			continue
		}
		inside := float64(n.Bounds.X) >= box.X && float64(n.Bounds.Y) >= box.Y &&
			float64(n.Bounds.X+n.Bounds.W) <= box.X+box.W && float64(n.Bounds.Y+n.Bounds.H) <= box.Y+box.H
		assert.Falsef(t, inside, "%q lies in the QR box: the symbol is not an element", n.Label)
	}
}

// TestA11yMusicDiscNamesWhatItDoes checks TV09: the disc is named "Phát" while it
// shows PLAY and "Tạm dừng" while it shows PAUSE, the chevrons of the two text
// buttons are not read, the wave is a progress bar named by the position.
func TestA11yMusicDiscNamesWhatItDoes(t *testing.T) {
	t.Parallel()

	r := openAt(t, audioState(t), nav.Route{Screen: env.RouteMusic}, false)
	r.tt.View().Context().Theme().ReduceMotion = true
	r.settle()
	sem := r.tt.Semantics()
	assert.True(t, hasLabel(sem, "Phát"))
	assert.True(t, hasLabel(sem, "Bài trước"), "without the chevron")
	assert.True(t, hasLabel(sem, "Bài sau"))
	wave, ok := findNode(sem, "Vị trí phát", platform.RoleText)
	require.True(t, ok)
	assert.InDelta(t, 72.0/220.0, wave.Now, .01)
	assert.Equal(t, "01:12 / 03:40", wave.Value, "the position as the clocks show it")
	require.True(t, r.tt.PerformAction("Phát", platform.ActPress))
	r.playback().Publish(player.ChangeState) // the player tells it plays
	r.settle()
	sem = r.tt.Semantics()
	assert.True(t, hasLabel(sem, "Tạm dừng"), "playing now: the disc pauses")
	assert.False(t, hasLabel(sem, "Phát"))
}

// TestA11yUpNextCountdownIsNotRead checks TV10: the digit of the disc is not an
// element (the line next to it says "tự phát sau 5 giây"), and it does not speak as
// it counts down.
func TestA11yUpNextCountdownIsNotRead(t *testing.T) {
	t.Parallel()

	r := inUpNext(t, upNextState(t), false)
	r.tt.View().Context().Theme().ReduceMotion = true
	r.settle()
	sem := r.tt.Semantics()
	for _, d := range []string{"5", "4", "3", "2", "1", "0"} {
		assert.Falsef(t, hasLabel(sem, d), "the digit %s is read", d)
	}
	assert.True(t, hasLabel(sem, "Tiếp theo · tự phát sau 5 giây"))
}

// TestA11yMedalsAreOneElementEach checks TV12: a badge is its caption, one text,
// not the three it paints ("HDV", "DANCEFIT", "HDV DanceFIT").
func TestA11yMedalsAreOneElementEach(t *testing.T) {
	t.Parallel()

	r := openAt(t, playerState(t), nav.Route{Screen: env.RouteSettings}, false)
	r.tt.View().Context().Theme().ReduceMotion = true
	r.settle()
	sem := r.tt.Semantics()
	for _, l := range []string{"HDV DanceFIT", "HDV StepX", "HDV BodyX"} {
		assert.Equal(t, 1, countLabel(sem, l), l)
	}
	for _, l := range []string{"HDV", "STEPX", "BODYX"} {
		assert.Falsef(t, hasLabel(sem, l), "the mark %q is read", l)
	}
	assert.Equal(t, 1, countLabel(sem, "DANCEFIT"), "the brand of the nav bar, not also the mark of a medal")
}

// TestA11yWaitingStateHasALabelledAnchor checks the waiting state of a screen
// (K12: the nav bar and the stand-in that holds the focus): the stand-in is a
// status that says "Đang tải", not a button with no name that takes a press.
func TestA11yWaitingStateHasALabelledAnchor(t *testing.T) {
	t.Parallel()

	r := openFast(t, tvtest.NewSignedServices(t), env.RouteHome, false)
	require.False(t, r.home().Loaded(), "nothing is read before the read's turn: the screen is still waiting")
	sem := r.tt.Semantics()
	anchor, ok := findNode(sem, "Đang tải", platform.RoleStatus)
	require.True(t, ok, "the stand-in: %s", dumpTree(sem))
	assert.False(t, anchor.Actions.Has(platform.ActPress), "it takes no press")
	assert.True(t, anchor.Flags.Has(platform.FlagFocusable), "it holds the focus")
	assert.True(t, hasLabel(sem, "Trang chủ"), "the nav bar is there")
	checkLabels(t, sem)
}

// TestA11yEveryBodyIsReadOnce checks that no text is read twice in any state: the
// element of a button holds its text, and a container's description is not added
// to its children's.
func TestA11yEveryBodyIsReadOnce(t *testing.T) {
	t.Parallel()

	for _, s := range a11yStates() {
		t.Run(s.id, func(t *testing.T) {
			t.Parallel()
			sem := s.open(t).tt.Semantics()
			seen := map[string]platform.SemanticNode{}
			for i, n := range sem.Nodes {
				if i == 0 || n.Label == "" {
					continue
				}
				if p, dup := seen[n.Label]; dup {
					// the nav bar's brand and the page title of the screens that repeat a word are not duplicates
					// of one place: two elements with one name must differ in role or place
					assert.Falsef(t, p.Role == n.Role && p.Bounds == n.Bounds, "%q is read twice at %v", n.Label, n.Bounds)
				}
				seen[n.Label] = n
			}
		})
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// dumpTree writes the elements of a reading, one a line.
func dumpTree(sem *platform.Semantics) string {
	var b strings.Builder
	for i, n := range sem.Nodes {
		fmt.Fprintf(&b, "%3d p%-3d role=%d L%d %q val=%q act=%b fl=%b tg=%d [%.0f,%.0f %.0fx%.0f]\n",
			i, n.Parent, n.Role, n.Level, n.Label, n.Value, n.Actions, n.Flags, n.Toggle,
			n.Bounds.X, n.Bounds.Y, n.Bounds.W, n.Bounds.H)
	}

	return b.String()
}

// hasLabel tells whether an element of the reading is called label.
func hasLabel(sem *platform.Semantics, label string) bool { return countLabel(sem, label) > 0 }

// countLabel counts the elements of the reading called label.
func countLabel(sem *platform.Semantics, label string) int {
	n := 0
	for _, e := range sem.Nodes {
		if e.Label == label {
			n++
		}
	}

	return n
}

// findNode returns the first element called label of that role.
func findNode(sem *platform.Semantics, label string, role platform.SemanticRole) (platform.SemanticNode, bool) {
	for _, n := range sem.Nodes {
		if n.Label == label && n.Role == role {
			return n, true
		}
	}

	return platform.SemanticNode{}, false
}

// iconKeys are the keys of the icons, which a label must not be.
func iconKeys() map[string]bool {
	m := map[string]bool{}
	for _, n := range icon.Names() {
		m[n.String()] = true
	}

	return m
}

// checkLabels checks the first assertion: each element that takes a press or the
// focus has a label, and no label is the key of an icon or of a control (`p1`,
// `mu1`: what an unnamed control was called).
func checkLabels(t *testing.T, sem *platform.Semantics) {
	t.Helper()
	keys := iconKeys()
	for i, n := range sem.Nodes {
		if i == 0 {
			continue
		}
		if n.Actions.Has(platform.ActPress) || n.Flags.Has(platform.FlagFocusable) {
			assert.NotEmptyf(t, n.Label, "element %d (role %d at %v) takes a press or the focus and has no label", i, n.Role, n.Bounds)
		}
		assert.Falsef(t, keys[n.Label], "element %d is labelled with the icon key %q", i, n.Label)
		assert.Falsef(t, isFocusKey(n.Label), "element %d is labelled with the key of a control: %q", i, n.Label)
		if n.Max > n.Min && n.Role == platform.RoleText { // a bar (kit.DescribeBar)
			assert.NotEmptyf(t, n.Label, "progress bar %d", i)
			assert.NotEmptyf(t, n.Value, "bar %q has a reading: the bridge gives it no trait, the value is all it says", n.Label)
			assert.GreaterOrEqualf(t, n.Now, n.Min, "bar %q is in its range", n.Label)
			assert.LessOrEqualf(t, n.Now, n.Max, "bar %q is in its range", n.Label)
		}
	}
}

// isFocusKey tells whether s looks like a `data-fid`: letters then digits ("mu1", "hc0").
func isFocusKey(s string) bool {
	if len(s) < 2 || len(s) > 4 {
		return false
	}
	i := 0
	for i < len(s) && s[i] >= 'a' && s[i] <= 'z' {
		i++
	}
	if i == 0 || i == len(s) {
		return false
	}
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

// checkHasTitle checks that the reading has a heading of level 1: the title of the
// screen, or of the panel over it.
func checkHasTitle(t *testing.T, sem *platform.Semantics) {
	t.Helper()
	for _, n := range sem.Nodes {
		if n.Role == platform.RoleHeading && n.Level == 1 {
			return
		}
	}
	t.Errorf("no heading of level 1:\n%s", dumpTree(sem))
}

// checkFocusIsAnElement checks that the focused item is an element of the reading
// and the one the tree says holds the focus: VoiceOver on tvOS lands on it.
func checkFocusIsAnElement(t *testing.T, r *rig, sem *platform.Semantics) {
	t.Helper()
	cur := r.sh.Manager().Current()
	if cur == nil {
		return
	}
	require.GreaterOrEqual(t, sem.Focus, 1, "the focus %q is no element", cur.FocusKey())
	n := sem.Nodes[sem.Focus]
	assert.True(t, n.Flags.Has(platform.FlagFocusable), "the focused element is focusable")
	w, ok := cur.(ui.Widget)
	require.True(t, ok)
	assert.Truef(t, sameBox(n.Bounds, w.Node().WindowFrame()), "the focused element %q at %v is not the item %q at %v",
		n.Label, n.Bounds, cur.FocusKey(), w.Node().WindowFrame())
	assert.NotEmpty(t, n.Label)
}

// checkNothingUnderThePanel checks the second assertion: with the panel open every
// element but the window lies in the panel.
func checkNothingUnderThePanel(t *testing.T, r *rig, sem *platform.Semantics) {
	t.Helper()
	require.True(t, r.sh.Stack().Panel())
	panel, ok := r.layout().Find("div.tpanel")
	require.True(t, ok, "the panel")
	frame := geom.XYWH(float32(panel.X)-1, float32(panel.Y)-1, float32(panel.W)+2, float32(panel.H)+2)
	for i, n := range sem.Nodes {
		if i == 0 {
			continue
		}
		assert.Truef(t, frame.Contains(n.Bounds.Min()) && frame.Contains(n.Bounds.Max()),
			"element %d %q at %v is not in the panel %v: the player under it is read", i, n.Label, n.Bounds, frame)
	}
}

// checkItemsAreElements checks that each focusable item of what is reachable is an
// element with a press and the item's own box: a button inside another is no
// element unless the other says so.
func checkItemsAreElements(t *testing.T, r *rig, sem *platform.Semantics) {
	t.Helper()
	var visit func(w ui.Widget)
	visit = func(w ui.Widget) {
		b := w.Node()
		if b.Hidden {
			return
		}
		if k, ok := w.(interface{ FocusKey() string }); ok && b.AcceptsFocus() && k.FocusKey() != "" {
			f := b.WindowFrame()
			if f.W > 0 && f.H > 0 {
				found := false
				for _, n := range sem.Nodes {
					if n.Flags.Has(platform.FlagFocusable) && sameBox(n.Bounds, f) {
						found = true

						break
					}
				}
				assert.Truef(t, found, "%T %q at %v takes the focus and no element of the reading is it", w, k.FocusKey(), f)
			}
		}
		for c := range b.Children() {
			visit(c)
		}
	}
	visit(r.tt.View().Root())
}

// sameBox tells whether two boxes are the same, within a DIP and a half.
func sameBox(a, b geom.Rect) bool {
	const tol = 1.5
	d := func(x, y float32) bool {
		if x < y {
			x, y = y, x
		}

		return x-y <= tol
	}

	return d(a.X, b.X) && d(a.Y, b.Y) && d(a.W, b.W) && d(a.H, b.H)
}
