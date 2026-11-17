package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTopBarBoxesMatchPrototype compares the row, the back button and its icon
// with M15B, and checks the title is in the middle of the row.
func TestTopBarBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		top := kit.NewTopBar("Thông tin tài khoản", "Quay lại", nil)
		kittest.MountReal(t, ph, newStage(placeAt(top, 20, 54, ph.Size.W-40)))
		rectNear(t, onPhone(ph, protoBox(t, "M15B", "div.top"), anchorStretch), frameOf(top), 1, "div.top")
		rectNear(t, protoBox(t, "M15B", "back"), frameOf(part(t, top, "back")), 0, "back")
		rectNear(t, protoBox(t, "M15B", "svg.i"), frameOf(part(t, top, "svg.i")), .01, "back icon")
		var title geom.Rect // the title is unnamed: the capture does not record it
		for ch := range top.Children() {
			if _, ok := ch.(*kit.Label); ok {
				title = frameOf(ch)
			}
		}
		bar := frameOf(top)
		assert.InDelta(t, float64(bar.Center().X), float64(title.Center().X), .6, "the title is in the middle (space-between with a 40 spacer)")
		assert.InDelta(t, float64(bar.Center().Y), float64(title.Center().Y), .6)
		assert.Equal(t, "div.top", top.Name)
	})
}

// TestTopBarBack checks the back button calls OnBack and the title can change.
func TestTopBarBack(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		backs := 0
		top := kit.NewTopBar("Kết nối Apple TV", "Quay lại", func() { backs++ })
		tt := mountStage(t, ph, placeAt(top, 20, 54, ph.Size.W-40))
		require.NoError(t, tt.Click("Quay lại"))
		assert.Equal(t, 1, backs)
		top.SetTitle("Đổi mật khẩu")
		tt.Elapse(kittest.Frame)
		assert.True(t, tt.Shows("Đổi mật khẩu"))
		assert.False(t, tt.Shows("Kết nối Apple TV"))
	})
}

// TestOfflineBarBoxesMatchPrototype compares the strip and its icon with M13 and
// checks its tint.
func TestOfflineBarBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		bar := kit.NewOfflineBar("Không có mạng · Đang dùng chế độ ngoại tuyến")
		tt := mountStageReal(t, ph, placeAt(bar, 0, 54, ph.Size.W))
		rectNear(t, onPhone(ph, protoBox(t, "M13", "div.offbar"), anchorStretch), frameOf(bar), 0, "div.offbar")
		rectNear(t, protoBox(t, "M13", "svg.i"), frameOf(part(t, bar, "svg.i")), .01, "wifi icon")
		f := frameOf(bar)
		pixelNear(t, tt.Snapshot(), 3, int(f.Y)+3, over(look.C.OfflineBg, look.T.Bg), 1, "rgba(255,176,32,.16)")
		pixelNear(t, tt.Snapshot(), 3, int(f.Y)-2, look.T.Bg, 0, "above the strip")
		// the text starts 8 after the 18 icon: 20 + 18 + 8
		assert.True(t, tt.Shows("Không có mạng · Đang dùng chế độ ngoại tuyến"))
	})
}

// TestBarsDoNotAllocate checks a steady frame of the top bar and the offline bar.
func TestBarsDoNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		tt := mountStage(t, ph, placeAt(kit.NewTopBar("Kết nối Apple TV", "Quay lại", nil), 20, 54, ph.Size.W-40),
			placeAt(kit.NewOfflineBar("Không có mạng · Đang dùng chế độ ngoại tuyến"), 0, 120, ph.Size.W))
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a steady frame", ph.Name)
	}
}
