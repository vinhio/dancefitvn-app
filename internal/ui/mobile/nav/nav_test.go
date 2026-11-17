package nav_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// step is one thing a test does to the Nav.
type step struct {
	do    string // "go", "root", "replace", "back"
	to    string
	ok    bool // what Back returns
	cur   string
	stack []string
}

// TestGoBackStack runs go(), go(root) and back() as the prototype does
// (index.html:543-551, docs/plan/03 section 1).
func TestGoBackStack(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		steps []step
	}{
		{"push and pop", []step{
			{do: "go", to: "detail", cur: "detail", stack: []string{"home"}},
			{do: "go", to: "video", cur: "video", stack: []string{"home", "detail"}},
			{do: "back", ok: true, cur: "detail", stack: []string{"home"}},
			{do: "back", ok: true, cur: "home", stack: []string{}},
			{do: "back", ok: false, cur: "home", stack: []string{}},
		}},
		{"a root clears the stack", []step{
			{do: "go", to: "detail", cur: "detail", stack: []string{"home"}},
			{do: "go", to: "video", cur: "video", stack: []string{"home", "detail"}},
			{do: "root", to: "downloads", cur: "downloads", stack: []string{}},
			{do: "back", ok: true, cur: "home", stack: []string{}},
			{do: "back", ok: false, cur: "home", stack: []string{}},
		}},
		{"courses root and back is Home", []step{
			{do: "root", to: "courses", cur: "courses", stack: []string{}},
			{do: "back", ok: true, cur: "home", stack: []string{}},
			{do: "back", ok: false, cur: "home", stack: []string{}},
		}},
		{"toDetail: stack is courses", []step{
			{do: "root", to: "courses", cur: "courses", stack: []string{}},
			{do: "go", to: "detail", cur: "detail", stack: []string{"courses"}},
			{do: "back", ok: true, cur: "courses", stack: []string{}},
			{do: "back", ok: true, cur: "home", stack: []string{}},
		}},
		{"replace keeps the stack", []step{
			{do: "go", to: "detail", cur: "detail", stack: []string{"home"}},
			{do: "go", to: "video", cur: "video", stack: []string{"home", "detail"}},
			{do: "replace", to: "video", cur: "video", stack: []string{"home", "detail"}},
			{do: "back", ok: true, cur: "detail", stack: []string{"home"}},
		}},
		{"the same screen is pushed again", []step{
			{do: "go", to: "home", cur: "home", stack: []string{"home"}},
			{do: "back", ok: true, cur: "home", stack: []string{}},
		}},
		{"a screen with no tab bar and nothing under it keeps Back for the system", []step{
			{do: "root", to: "login", cur: "login", stack: []string{}},
			{do: "back", ok: false, cur: "login", stack: []string{}},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
				r := newRig(t, ph)
				for i, s := range tc.steps {
					switch s.do {
					case "go":
						r.n.Go(route(s.to), false)
					case "root":
						r.n.Go(route(s.to), true)
					case "replace":
						r.n.Replace(route(s.to))
					case "back":
						assert.Equal(t, s.ok, r.n.Back(), "step %d", i)
					default:
						t.Fatalf("step %d: unknown action %q", i, s.do)
					}
					r.tt.Elapse(kittest.Frame)
					assert.Equal(t, s.cur, r.n.Current().Screen, "step %d", i)
					assert.Equal(t, s.stack, r.stack(), "step %d", i)
					assert.Equal(t, len(s.stack), r.n.Depth(), "step %d", i)
				}
			})
		})
	}
}

// TestRouteParamsComeBack checks the stack gives back a screen with the params it
// was opened with.
func TestRouteParamsComeBack(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		r.n.Go(nav.Route{Screen: "detail", Params: "set35"}, false)
		r.n.Go(nav.Route{Screen: "detail", Params: "set36"}, false)
		assert.Equal(t, "set36", r.n.Current().Params)
		require.True(t, r.n.Back())
		assert.Equal(t, "set35", r.n.Current().Params)
	})
}

// TestTabsAndTabBar checks the tab bar shows on the root screens only, follows the
// screen's tab, and a tap on a tab (the current one too) is a root.
func TestTabsAndTabBar(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		bar := r.n.TabBar()
		assert.False(t, bar.Hidden, "Home has the bar")
		assert.Equal(t, 0, bar.Current)
		bf := bar.WindowFrame()
		assert.InDelta(t, float64(ph.Size.H), float64(bf.Max().Y), .01, "the bar is at the bottom")
		assert.InDelta(t, 84, float64(bf.H), .01, "84 on both phones")

		require.NoError(t, r.tt.Click("Khoá học"))
		assert.Equal(t, "courses", r.n.Current().Screen)
		assert.Equal(t, 1, bar.Current)
		require.NoError(t, r.tt.Click("Tải xuống"))
		assert.Equal(t, 2, bar.Current)

		r.n.Go(route("detail"), false)
		assert.True(t, bar.Hidden, "a screen without tabs hides the bar")
		assert.Equal(t, -1, bar.Current)
		r.tt.Elapse(kittest.Frame)
		assert.False(t, r.tt.Shows("Khoá học"), "no tab label on the detail")

		require.True(t, r.n.Back())
		assert.False(t, bar.Hidden)
		assert.Equal(t, 2, bar.Current)
		r.tt.Elapse(kittest.Frame)
		assert.InDelta(t, 84, float64(bar.WindowFrame().H), .01, "the bar shown again has its frame back")
		assert.True(t, r.tt.Shows("Khoá học"), "and is painted")

		// the current tab again: a root again, a fresh screen at the top
		r.last["downloads"].Scroll().ScrollTo(geom.Pt(0, 300), false)
		built := len(r.built)
		require.NoError(t, r.tt.Click("Tải xuống"))
		assert.Len(t, r.built, built+1)
		r.tt.Elapse(kittest.Frame)
		assert.Zero(t, r.last["downloads"].Scroll().Offset().Y)
		assert.Empty(t, r.stack())
	})
}

// TestTabBarFollowsTheSafeArea checks the bar is 50 + max(34, safe bottom) tall:
// 84 on the iPhone and with Android's gesture bar, 98 with its three-button bar
// (measured, local/p2/safearea.md).
func TestTabBarFollowsTheSafeArea(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		bottom float32
		want   float64
	}{
		{"iPhone 16: 34", 34, 84},
		{"Android gesture bar: 24", 24, 84},
		{"Android three buttons: 48", 48, 98},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
				r := newRig(t, ph)
				ph.Safe.Bottom = tc.bottom
				r.tt.SetSafeArea(ph.Safe)
				r.tt.Elapse(kittest.Frame)
				assert.InDelta(t, tc.want, float64(r.n.TabBar().WindowFrame().H), .01)
				assert.InDelta(t, float64(ph.Size.H), float64(r.n.TabBar().WindowFrame().Max().Y), .01)
			})
		})
	}
}

// TestBackChain checks the Back key: the sheet first, then the stack, then Home
// from a root, then the system.
func TestBackChain(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		r.n.Go(route("courses"), true)
		r.n.Go(route("detail"), false)
		r.n.Sheet().Open(&block{h: 100}, kit.SheetOptions{})
		r.tt.Elapse(400 * time.Millisecond)
		require.True(t, r.n.Sheet().IsOpen())

		// 1: the sheet closes, the screen stays
		assert.True(t, r.tt.KeyDown(keys.KeyBack, 0), "the press is taken")
		assert.True(t, r.tt.KeyUp(keys.KeyBack, 0), "and so is the release")
		assert.False(t, r.n.Sheet().IsOpen())
		assert.Equal(t, "detail", r.n.Current().Screen)
		assert.Equal(t, 1, r.n.Depth())

		// 2: the stack pops
		assert.True(t, r.tt.KeyDown(keys.KeyBack, 0))
		assert.True(t, r.tt.KeyUp(keys.KeyBack, 0))
		assert.Equal(t, "courses", r.n.Current().Screen)
		assert.Zero(t, r.n.Depth())

		// 3: Courses is a root: Home
		assert.True(t, r.tt.KeyDown(keys.KeyBack, 0))
		assert.True(t, r.tt.KeyUp(keys.KeyBack, 0))
		assert.Equal(t, "home", r.n.Current().Screen)

		// 4: Home, nothing to close: the system's
		assert.False(t, r.tt.KeyDown(keys.KeyBack, 0), "the press is left to the system")
		assert.False(t, r.tt.KeyUp(keys.KeyBack, 0), "and the release")
		assert.Equal(t, "home", r.n.Current().Screen)
		assert.False(t, r.n.CanBack())
	})
}

// TestBackKeyHeldIsTakenOnce checks a held Back key pops one screen, and its
// repeats and release are taken with it.
func TestBackKeyHeldIsTakenOnce(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		r.n.Go(route("detail"), false)
		r.n.Go(route("video"), false)
		assert.True(t, r.tt.KeyDown(keys.KeyBack, 0))
		assert.True(t, r.tt.KeyRepeat(keys.KeyBack, 0))
		assert.True(t, r.tt.KeyRepeat(keys.KeyBack, 0))
		assert.True(t, r.tt.KeyUp(keys.KeyBack, 0))
		assert.Equal(t, "detail", r.n.Current().Screen, "one pop")
		assert.False(t, r.tt.KeyUp(keys.KeyBack, 0), "a release with no press is not taken")
		// other keys are not Back
		assert.False(t, r.tt.Key(keys.KeyEscape, 0))
		assert.Equal(t, "detail", r.n.Current().Screen)
	})
}

// TestAddBack checks a registered layer is asked first, last registered first, and
// stops being asked once removed.
func TestAddBack(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		r.n.Go(route("detail"), false)
		var order []string
		openA, openB := true, true
		removeA := r.n.AddBack(func() bool {
			order = append(order, "a")
			held := openA
			openA = false

			return held
		})
		removeB := r.n.AddBack(func() bool {
			order = append(order, "b")
			held := openB
			openB = false

			return held
		})
		assert.True(t, r.n.Back())
		assert.True(t, r.n.Back())
		assert.Equal(t, []string{"b", "b", "a"}, order, "b first; once it has nothing, a")
		assert.Equal(t, "detail", r.n.Current().Screen, "the stack is under the layers")
		assert.True(t, r.n.Back(), "both closed: the stack pops")
		assert.Equal(t, "home", r.n.Current().Screen)
		removeA()
		removeB()
		removeA() // twice is fine
		assert.False(t, r.n.CanBack())
		assert.False(t, r.n.Back())
		assert.Len(t, order, 5, "removed steps are not asked")
	})
}

// TestGoClosesTheSheetAndKeepsTheToast checks go() closes the sheet (and tells its
// OnClose) and a toast outlives the screen change.
func TestGoClosesTheSheetAndKeepsTheToast(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		closed := 0
		r.n.Sheet().Open(&block{h: 100}, kit.SheetOptions{OnClose: func() { closed++ }})
		r.n.Toast("Đã tải xong bài 03")
		r.tt.Elapse(400 * time.Millisecond)
		r.n.Go(route("detail"), false)
		assert.False(t, r.n.Sheet().IsOpen())
		assert.Equal(t, 1, closed)
		assert.True(t, r.n.Toasts().Visible(), "the toast stays")
		assert.Equal(t, "Đã tải xong bài 03", r.n.Toasts().Message())
		r.tt.Elapse(1700 * time.Millisecond) // 2100 ms since it showed
		assert.True(t, r.n.Toasts().Visible())
		r.tt.Elapse(200 * time.Millisecond)
		assert.False(t, r.n.Toasts().Visible(), "gone after 2200 ms")

		// Replace closes it too
		r.n.Sheet().Open(&block{h: 100}, kit.SheetOptions{OnClose: func() { closed++ }})
		r.n.Replace(route("video"))
		assert.False(t, r.n.Sheet().IsOpen())
		assert.Equal(t, 2, closed)
	})
}

// TestScrollKeptOnRefreshAndResetOnGoAndBack checks renderM keeps the scroll of the
// same screen and go() / back() start at the top (index.html:536-551).
func TestScrollKeptOnRefreshAndResetOnGoAndBack(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		home := r.last["home"]
		home.Scroll().ScrollTo(geom.Pt(0, 400), false)
		assert.InDelta(t, 400, float64(home.Scroll().Offset().Y), .01)

		r.n.Refresh()
		r.tt.Elapse(kittest.Frame)
		assert.NotSame(t, home, r.last["home"], "built again")
		assert.Same(t, r.last["home"], r.n.Screen())
		assert.InDelta(t, 400, float64(r.last["home"].Scroll().Offset().Y), .01, "the scroll is kept")
		r.n.Refresh()
		r.tt.Elapse(kittest.Frame)
		assert.InDelta(t, 400, float64(r.last["home"].Scroll().Offset().Y), .01, "and again")

		r.n.Go(route("detail"), false)
		r.tt.Elapse(kittest.Frame)
		assert.Zero(t, r.last["detail"].Scroll().Offset().Y, "go() starts at the top")
		require.True(t, r.n.Back())
		r.tt.Elapse(kittest.Frame)
		assert.Zero(t, r.last["home"].Scroll().Offset().Y, "back() does not restore the scroll (F-7)")

		// a Refresh that follows a Go does not bring the old offset back
		r.last["home"].Scroll().ScrollTo(geom.Pt(0, 200), false)
		r.n.Go(route("detail"), false)
		r.n.Refresh()
		r.tt.Elapse(kittest.Frame)
		assert.Zero(t, r.last["detail"].Scroll().Offset().Y)
	})
}

// TestRefreshKeepsStackAndSheet checks Refresh rebuilds only the screen.
func TestRefreshKeepsStackAndSheet(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		r.n.Go(route("detail"), false)
		r.n.Sheet().Open(&block{h: 100}, kit.SheetOptions{})
		before := len(r.built)
		r.n.Refresh()
		assert.Len(t, r.built, before+1)
		assert.True(t, r.n.Sheet().IsOpen())
		assert.Equal(t, []string{"home"}, r.stack())
		assert.Zero(t, r.left["detail"], "Refresh is not leaving")
	})
}

// TestLeaverIsToldOnGoReplaceAndBack checks the screen is told when it is left by
// go, replace or back, and not by refresh.
func TestLeaverIsToldOnGoReplaceAndBack(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		r.n.Go(route("video"), false)
		r.n.Refresh()
		assert.Zero(t, r.left["video"])
		r.n.Replace(route("video"))
		assert.Equal(t, 1, r.left["video"])
		r.n.Go(route("detail"), false)
		assert.Equal(t, 2, r.left["video"])
		assert.Zero(t, r.left["detail"])
		require.True(t, r.n.Back())
		assert.Equal(t, 1, r.left["detail"])
		r.n.Go(route("detail"), false)
		r.n.Go(route("courses"), true)
		assert.Equal(t, 2, r.left["detail"], "a root leaves the screen too")
	})
}

// TestOverlayOrder checks the screen is under the tab bar, the sheet and the toast,
// in the order of the prototype's z-index.
func TestOverlayOrder(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		var kids []ui.Widget
		for c := range r.n.Children() {
			kids = append(kids, c)
		}
		require.Len(t, kids, 4)
		assert.Same(t, r.n.TabBar(), kids[1])
		assert.Same(t, r.n.Sheet(), kids[2])
		assert.Same(t, r.n.Toasts(), kids[3])
	})
}

// TestToastOverSheetAndTabBar checks a toast shows over an open sheet and the tab
// bar, takes the touches on its bubble only, and a new one replaces and restarts it.
func TestToastOverSheetAndTabBar(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		r.n.Sheet().Open(&block{h: 100}, kit.SheetOptions{})
		r.n.Toast("Đã huỷ tải")
		r.tt.Elapse(400 * time.Millisecond)
		assert.True(t, r.tt.Shows("Đã huỷ tải"))

		// a touch on the bubble does not close the sheet behind it
		bubble := toastBubble(t, r.n.Toasts()).Node().WindowFrame()
		r.tt.ClickAt(bubble.Center())
		assert.True(t, r.n.Sheet().IsOpen())
		// elsewhere on the dim it does
		r.tt.ClickAt(geom.Pt(ph.Size.W/2, 100))
		assert.False(t, r.n.Sheet().IsOpen())
		assert.True(t, r.n.Toasts().Visible(), "the toast is not the sheet's")

		// replaced and restarted: 2000 ms after the first, a second one
		r.tt.Elapse(1700 * time.Millisecond)
		r.n.Toast("Đã xoá file âm thanh")
		assert.False(t, r.tt.Shows("Đã huỷ tải"))
		r.tt.Elapse(2100 * time.Millisecond)
		assert.True(t, r.n.Toasts().Visible(), "2100 ms into the second")
		assert.Equal(t, "Đã xoá file âm thanh", r.n.Toasts().Message())
		r.tt.Elapse(200 * time.Millisecond)
		assert.False(t, r.n.Toasts().Visible())
	})
}

// toastBubble finds the toast's bubble.
func toastBubble(t testing.TB, root ui.Widget) ui.Widget {
	t.Helper()
	for c := range root.Node().Children() {
		if c.Node().Name == "div.toast" {
			return c
		}
	}
	t.Fatal("no bubble")

	return nil
}

// TestSheetThroughTheNav checks the sheet host the Nav gives: a tap on the dim
// closes, the pull-down closes at 81 DIPs and not at 79, and the keyboard lifts the
// bottom.
func TestSheetThroughTheNav(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		btn := kit.NewButton(kit.ButtonP, "Xong", icon.NameUnknown)
		open := func() {
			r.n.Sheet().Open(btn, kit.SheetOptions{})
			r.tt.Elapse(400 * time.Millisecond)
		}
		open()
		panel := partNamed(t, r.n.Sheet(), "div.sheet").Node().WindowFrame()
		r.tt.ClickAt(geom.Pt(ph.Size.W/2, panel.Y/2))
		assert.False(t, r.n.Sheet().IsOpen(), "dim tap")

		for _, tc := range []struct {
			dy    float32
			close bool
		}{{79, false}, {81, true}} {
			open()
			panel = partNamed(t, r.n.Sheet(), "div.sheet").Node().WindowFrame()
			from := geom.Pt(panel.W/2, panel.Y+20)
			r.tt.TouchDown(from)
			for k := 1; k <= 10; k++ {
				r.tt.Elapse(50 * time.Millisecond)
				r.tt.TouchMove(geom.Pt(from.X, from.Y+tc.dy*float32(k)/10))
			}
			r.tt.TouchUp(geom.Pt(from.X, from.Y+tc.dy))
			assert.Equal(t, tc.close, !r.n.Sheet().IsOpen(), "pull %v", tc.dy)
			r.n.Sheet().Close()
		}

		open()
		bottom0 := btn.WindowFrame().Max().Y
		assert.InDelta(t, float64(ph.Size.H-40), float64(bottom0), .01)
		r.tt.SetKeyboard(geom.Insets{Bottom: 300})
		r.tt.Elapse(kittest.Frame)
		assert.InDelta(t, float64(ph.Size.H-300), float64(btn.WindowFrame().Max().Y), .01, "the keyboard lifts it")
	})
}

// partNamed finds a descendant by its Name.
func partNamed(t testing.TB, root ui.Widget, name string) ui.Widget {
	t.Helper()
	var find func(w ui.Widget) ui.Widget
	find = func(w ui.Widget) ui.Widget {
		for c := range w.Node().Children() {
			if c.Node().Name == name {
				return c
			}
			if f := find(c); f != nil {
				return f
			}
		}

		return nil
	}
	w := find(root)
	if w == nil {
		t.Fatalf("no widget named %q", name)
	}

	return w
}

// TestScreenChangeIsToldToScreenReaders checks a screen change is a semantic
// change of the whole screen, and Refresh is too (the content is new).
func TestScreenChangeIsToldToScreenReaders(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		_ = r.tt.Semantics() // a screen reader reads the tree
		before := r.tt.SemanticsChanges(platform.ChangeScreen)
		r.n.Go(route("detail"), false)
		r.tt.Elapse(kittest.Frame)
		assert.Greater(t, r.tt.SemanticsChanges(platform.ChangeScreen), before)
	})
}

// TestSheetHushesTheScreenAndTheTabBar checks that while a sheet is open nothing
// of the screen under it, nor of the tab bar, is read or taken from a screen
// reader (its dim takes every touch, so nothing there can be reached), and that the
// toast, a status over the screen, hides nothing.
func TestSheetHushesTheScreenAndTheTabBar(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		taps := 0
		r := newRigWith(t, ph, func(id string) ui.Widget {
			b := kit.NewButton(kit.ButtonP, "Dưới "+id, icon.NameUnknown)
			b.OnPress = func() { taps++ }

			return b
		})
		labels := func() map[string]bool {
			out := map[string]bool{}
			for _, n := range r.tt.Semantics().Nodes {
				out[n.Label] = true
			}

			return out
		}
		got := labels()
		require.True(t, got["Dưới home"], "the screen is read")
		require.True(t, got["Khoá học"], "the tab bar is read")

		sheet := kit.NewButton(kit.ButtonP, "Trong sheet", icon.NameUnknown)
		r.n.Sheet().Open(sheet, kit.SheetOptions{})
		r.tt.Elapse(400 * time.Millisecond)
		got = labels()
		assert.True(t, got["Trong sheet"])
		assert.False(t, got["Dưới home"], "nothing of the screen under the sheet")
		assert.False(t, got["Khoá học"], "nor the tab bar")
		assert.False(t, r.tt.PerformAction("Dưới home", platform.ActPress))
		assert.False(t, r.tt.PerformAction("Khoá học", platform.ActPress))
		assert.Zero(t, taps)

		r.n.Toast("Đã lưu")
		r.tt.Elapse(300 * time.Millisecond)
		got = labels()
		assert.True(t, got["Đã lưu"], "the toast is a status over the sheet")
		assert.True(t, got["Trong sheet"], "and hides nothing of it")

		r.n.Sheet().Close()
		r.tt.Elapse(kittest.Frame)
		got = labels()
		assert.True(t, got["Dưới home"], "the screen is read again")
		assert.True(t, got["Khoá học"])
		assert.True(t, r.tt.PerformAction("Dưới home", platform.ActPress))
		assert.Equal(t, 1, taps)
	})
}
