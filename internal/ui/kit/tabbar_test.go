package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tabItems are TABS (index.html:323) with their keys.
func tabItems() [kit.TabCount]kit.TabItem {
	return [kit.TabCount]kit.TabItem{
		{Icon: icon.Home, Label: "Trang chủ", Name: "go.home"},
		{Icon: icon.Grid, Label: "Khoá học", Name: "go.courses"},
		{Icon: icon.Dl, Label: "Tải xuống", Name: "go.downloads"},
		{Icon: icon.User, Label: "Tài khoản", Name: "go.account"},
	}
}

// mountTabs mounts a tab bar at the bottom of the phone.
func mountTabs(t testing.TB, ph kittest.Phone, real bool, bar *kit.TabBar) *ui.Tester {
	t.Helper()
	root := newOverlays(&barAtBottom{bar: bar})
	if real {
		return kittest.MountReal(t, ph, root)
	}

	return kittest.Mount(t, ph, root)
}

// TestTabBarBoxesMatchPrototype compares the bar, the four buttons, the pills and
// the icons with M03: the bar is 84 tall at the bottom, the buttons 83 under the
// line, the pill 8 under it.
func TestTabBarBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		bar := kit.NewTabBar(tabItems(), 0, nil)
		mountTabs(t, ph, true, bar)
		rectNear(t, fromBottom(ph, onPhone(ph, protoBox(t, "M03", "div.tabs"), anchorStretch)), frameOf(bar), 1, "div.tabs")
		bw := ph.Size.W / 4
		for i, key := range []string{"go.home", "go.courses#2", "go.downloads", "go.account#2"} {
			want := protoBox(t, "M03", key)
			want.X, want.W = float32(i)*bw, bw
			want.Y += ph.Size.H - 852
			btn := part(t, bar, tabItems()[i].Name)
			rectNear(t, want, frameOf(btn), 1, key)
			pk := []string{"span.pill", "span.pill#2", "span.pill#3", "span.pill#4"}[i]
			pw := protoBox(t, "M03", pk)
			pw.X = float32(i)*bw + (bw-56)/2
			pw.Y += ph.Size.H - 852
			rectNear(t, pw, frameOf(part(t, btn, "span.pill")), 1, pk)
			iw := protoBox(t, "M03", []string{"svg.i", "svg.i#2", "svg.i#3", "svg.i#4"}[i])
			iw.X = float32(i)*bw + (bw-56)/2 + 17
			iw.Y += ph.Size.H - 852
			rectNear(t, iw, frameOf(part(t, btn, "svg.i")), 1, "tab icon")
		}
	})
}

// TestTabBarLooksLikeThePrototype checks the colors of the bar, the pill of the
// current tab, and that the others have none.
func TestTabBarLooksLikeThePrototype(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		bar := kit.NewTabBar(tabItems(), 1, nil)
		tt := kittest.Mount(t, ph, newOverlays(&barAtBottom{bar: bar}))
		img := tt.Snapshot()
		f := frameOf(bar)
		pixelNear(t, img, 5, int(f.Y)+60, look.C.TabBar, 0, "#1A1B1F")
		pixelNear(t, img, 5, int(f.Y), look.T.Border, 0, "the 1 px line")
		pixelNear(t, img, 5, int(f.Y)-1, look.T.Bg, 0, "above the bar")
		bw := int(ph.Size.W / 4)
		// the pill's corner area (inside the 56 x 30 box, near its left end) is tinted on the current tab only
		pillY := int(f.Y) + 1 + 8 + 15
		pixelNear(t, img, bw+bw/2-24, pillY, over(look.C.PillOn, look.C.TabBar), 3, "pill of the current tab")
		pixelNear(t, img, bw/2-24, pillY, look.C.TabBar, 0, "no pill on the others")
		bar.SetCurrent(0)
		tt.Elapse(kittest.Frame)
		img = tt.Snapshot()
		pixelNear(t, img, bw/2-24, pillY, over(look.C.PillOn, look.C.TabBar), 3, "pill moved")
		pixelNear(t, img, bw+bw/2-24, pillY, look.C.TabBar, 0, "pill left")
	})
}

// TestTabBarHeightFollowsTheSafeArea covers Q-P2-1: 50 + max(34, inset).
func TestTabBarHeightFollowsTheSafeArea(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		inset, want float32
	}{{0, 84}, {24, 84}, {34, 84}, {40, 90}, {50, 100}} {
		assert.InDelta(t, float64(tc.want), float64(kit.TabBarHeight(tc.inset)), 0, "inset %v", tc.inset)
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		bar := kit.NewTabBar(tabItems(), 0, nil)
		tt := kittest.Mount(t, ph, newOverlays(&barAtBottom{bar: bar}))
		assert.InDelta(t, 84, float64(frameOf(bar).H), 0, "%s: 84 with its own safe area", ph.Name)
		tt.SetSafeArea(geom.Insets{Top: 59, Bottom: 50})
		tt.Elapse(kittest.Frame)
		assert.InDelta(t, 100, float64(frameOf(bar).H), 0)
		assert.InDelta(t, float64(ph.Size.H), float64(frameOf(bar).Max().Y), 0)
	})
}

// TestTabBarTap checks a tap on a tab calls OnTap with its index, the current tab
// too, and the buttons are described as tabs.
func TestTabBarTap(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var taps []int
		bar := kit.NewTabBar(tabItems(), 0, func(i int) { taps = append(taps, i) })
		tt := kittest.Mount(t, ph, newOverlays(&barAtBottom{bar: bar}))
		require.NoError(t, tt.Click("Khoá học"))
		require.NoError(t, tt.Click("Tài khoản"))
		require.NoError(t, tt.Click("Trang chủ"))
		assert.Equal(t, []int{1, 3, 0}, taps, "the current one too: the prototype goes to its root again")
		// a tap on the bar's line area does nothing
		tt.ClickAt(geom.Pt(10, frameOf(bar).Y+0.5))
		assert.Len(t, taps, 3)

		bar.SetCurrent(2)
		tt.Elapse(kittest.Frame)
		sem := tt.Semantics()
		var on, off int
		for _, n := range sem.Nodes {
			if n.Role != platform.RoleTab {
				continue
			}
			assert.Equal(t, platform.ToggledUnknown, n.Toggle, "a tab is no toggle: %q", n.Label)
			if n.Flags.Has(platform.FlagSelected) {
				on++
				assert.Equal(t, "Tải xuống", n.Label)
			} else {
				off++
			}
		}
		assert.Equal(t, 1, on)
		assert.Equal(t, 3, off)
	})
}

// TestTabBarDoesNotAllocate checks a steady frame and a press allocate nothing.
func TestTabBarDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		bar := kit.NewTabBar(tabItems(), 0, func(int) {})
		tt := kittest.Mount(t, ph, newOverlays(&barAtBottom{bar: bar}))
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a steady frame", ph.Name)
		p := frameOf(part(t, bar, "go.courses")).Center()
		assert.Zero(t, testing.AllocsPerRun(20, func() { tt.ClickAt(p) }), "%s: a tap", ph.Name)
	}
}

// BenchmarkTabBarRepaint is a steady repaint of the bar.
func BenchmarkTabBarRepaint(b *testing.B) {
	bar := kit.NewTabBar(tabItems(), 0, nil)
	tt := kittest.Mount(b, kittest.Phones[0], newOverlays(&barAtBottom{bar: bar}))
	tt.Repaint()
	b.ReportAllocs()
	for b.Loop() {
		tt.Repaint()
	}
}
