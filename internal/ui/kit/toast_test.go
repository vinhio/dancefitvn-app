package kit_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
)

// TestToastBoxesMatchPrototype compares the bubble and its icon with the
// layout.json of M-toast: 16 from the sides, 100 above the bottom, 44 tall.
func TestToastBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		toast := kit.NewToast()
		tt := kittest.MountReal(t, ph, newOverlays(toast))
		kittest.PinMotion(tt)
		assert.True(t, toast.Hidden)
		toast.Show("Đã tải xong bài 03")
		tt.Elapse(300 * time.Millisecond)

		bubble := part(t, toast, "div.toast")
		// M-toast div.toast [16,708,361,44]: 100 above the bottom, 16 in from the sides
		rectNear(t, fromBottom(ph, onPhone(ph, protoBox(t, "M-toast", "div.toast"), anchorStretch)), frameOf(bubble), 1, "div.toast")
		want := fromBottom(ph, onPhone(ph, protoBox(t, "M-toast", "svg.i#14"), anchorLeft))
		rectNear(t, want, frameOf(part(t, bubble, "svg.i")), .01, "toast icon")
		img := tt.Snapshot()
		f := frameOf(bubble)
		pixelNear(t, img, int(f.X)+8, int(f.Y)+22, look.C.ToastBg, 1, "#F4F4F2")
		pixelNear(t, img, int(f.X), int(f.Y), look.T.Bg, 8, "rounded 14: the corner is the screen's")
	})
}

// TestToastTiming covers 2200 ms on screen, no fade out, a new message
// replacing the old one and starting the time again, and the fade in.
func TestToastTiming(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		toast := kit.NewToast()
		tt := kittest.Mount(t, ph, newOverlays(toast))
		kittest.PinMotion(tt)
		assert.False(t, toast.Visible())
		assert.Empty(t, toast.Message())

		toast.Show("Đã tải xong bài 03")
		assert.True(t, toast.Visible())
		assert.Equal(t, "Đã tải xong bài 03", toast.Message())
		f := frameOf(part(t, toast, "div.toast"))
		tt.Elapse(kittest.Frame)
		f = frameOf(part(t, toast, "div.toast"))
		probe := func() (int, int) { return int(f.X) + 8, int(f.Y) + 22 }
		x, y := probe()
		assert.Less(t, int(pixelAt(tt.Snapshot(), x, y).R), int(look.C.ToastBg.R), "fading in: still dark at the first frame")
		tt.Elapse(200 * time.Millisecond)
		pixelNear(t, tt.Snapshot(), x, y, look.C.ToastBg, 1, "fully in at 200 ms")

		tt.Elapse(1799 * time.Millisecond) // 1 frame + 200 + 1799 = 2016 ms: not gone
		assert.True(t, toast.Visible())

		// a new message replaces it and starts the 2200 ms again
		tt.Elapse(100 * time.Millisecond)
		toast.Show("Đã huỷ tải")
		assert.Equal(t, "Đã huỷ tải", toast.Message())
		tt.Elapse(2199 * time.Millisecond)
		assert.True(t, toast.Visible(), "2199 ms after the second message")
		tt.Elapse(1 * time.Millisecond)
		assert.False(t, toast.Visible(), "2200 ms: gone, with no fade out")
		tt.Elapse(kittest.Frame)
		pixelNear(t, tt.Snapshot(), x, y, look.T.Bg, 0, "nothing left")

		toast.Show("Lần ba")
		toast.Hide()
		assert.False(t, toast.Visible())
		tt.Elapse(3 * time.Second)
		assert.False(t, toast.Visible())
	})
}

// TestToastAnnouncesAndLetsTouchesThrough checks the screen reader hears the
// message and only the bubble takes touches.
func TestToastAnnouncesAndLetsTouchesThrough(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		toast := kit.NewToast()
		taps := 0
		under := kit.NewButton(kit.ButtonP, "Dưới", icon.NameUnknown)
		under.OnPress = func() { taps++ }
		tt := kittest.Mount(t, ph, newOverlays(newStage(placeAt(under, 100, 100, 200), placeAt(kit.NewButton(kit.ButtonS, "Sát đáy", icon.NameUnknown), 100, ph.Size.H-120, 200)), toast))
		kittest.PinMotion(tt)
		tt.Semantics() // a screen reader is reading
		toast.Show("Mã không đúng")
		tt.Elapse(300 * time.Millisecond)
		assert.Contains(t, tt.Announcements(), "Mã không đúng")

		tt.ClickAt(frameOf(under).Center())
		assert.Equal(t, 1, taps, "a touch outside the bubble reaches the screen")
		bubble := frameOf(part(t, toast, "div.toast"))
		tt.ClickAt(bubble.Center())
		assert.True(t, toast.Visible(), "a tap on the bubble does nothing")
		assert.Equal(t, 1, taps)
	})
}

// TestToastAboveADeeperSafeArea checks the bubble stays 100 above the bar where
// the home indicator is deeper than the prototype's 34.
func TestToastAboveADeeperSafeArea(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		toast := kit.NewToast()
		tt := kittest.Mount(t, ph, newOverlays(toast))
		kittest.PinMotion(tt)
		toast.Show("x")
		tt.Elapse(300 * time.Millisecond)
		base := frameOf(part(t, toast, "div.toast")).Max().Y
		assert.InDelta(t, float64(ph.Size.H-100), float64(base), .01)
		tt.SetSafeArea(geom.Insets{Top: 59, Bottom: 50})
		tt.Elapse(kittest.Frame)
		assert.InDelta(t, float64(ph.Size.H-100-16), float64(frameOf(part(t, toast, "div.toast")).Max().Y), .01)
	})
}

// TestToastDoesNotAllocate checks a steady frame of a showing toast and a frame
// of its fade allocate nothing.
func TestToastDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		toast := kit.NewToast()
		tt := kittest.Mount(t, ph, newOverlays(toast))
		kittest.PinMotion(tt)
		toast.Show("Đã tải xong bài 03")
		tt.Elapse(300 * time.Millisecond)
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a steady frame", ph.Name)
		toast.Show("Đã huỷ tải")
		tt.Elapse(kittest.Frame)
		assert.Zero(t, testing.AllocsPerRun(5, func() { tt.Elapse(kittest.Frame / 4) }), "%s: a frame of the fade", ph.Name)
	}
}
