//go:build tvos

package tvkit_test

import (
	"image"
	"image/color"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// toastText is the message of the TV-toast state of the prototype.
const toastText = "Đã đăng nhập bằng thaovy.hlv@gmail.com"

// toastRig mounts a Toast over the whole stage.
func toastRig(t testing.TB) (*tvkit.Toast, *stage) {
	t.Helper()
	toast := tvkit.NewToast()
	root := &stage{}
	root.putSized(toast, 0, 0, tvtest.Width, tvtest.Height)

	return toast, root
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestToastBoxMatchesPrototype: the bubble of the TV-toast capture stands at
// x 633..1286, y 942..1020 (measured on local/shots/prototype-1x/tv/TV-toast.png:
// the layout.json has no `div.ttoast`): centered, 60 above the bottom, 78 tall
// (Inter 28/700 line 34 and 22 of padding twice).
func TestToastBoxMatchesPrototype(t *testing.T) {
	t.Parallel()
	toast, root := toastRig(t)
	tt := mount(t, root)
	toast.Show(toastText)
	tt.Elapse(tvkit.ToastFade + tvtest.Frame)

	b := toast.Bubble()
	near(t, geom.XYWH(633, 942, 654, 78), b, boxTol, "bubble")
	assert.InDelta(t, tvtest.Height-60, b.Y+b.H, .01, "60 above the bottom")
	assert.InDelta(t, tvtest.Width/2, b.X+b.W/2, .01, "centered")
	assert.InDelta(t, 78, b.H, .01)
	lay := layoutdump.Dump(tt.View().Context(), tt.View().Root(), "T", "tv", geom.Sz(tvtest.Width, tvtest.Height), nil)
	e, ok := lay.Find("div.ttoast")
	require.True(t, ok)
	s, txt := e.Style, e.Text
	assert.Equal(t, toastText, txt)
	assert.Equal(t, "rgb(244, 244, 242)", s.Background)
	assert.Equal(t, "rgb(20, 21, 24)", s.Color)
	assert.Equal(t, "700", s.FontWeight)
	assert.Equal(t, "28px", s.FontSize)
	assert.Equal(t, "24px", s.Radius)
}

// TestToastTimingAndFade: 2400 ms on screen, a fade in over 200 ms, no fade
// out, a message replacing the one showing restarts the 2400 ms with no new
// fade (K17), Hide takes it away at once.
func TestToastTimingAndFade(t *testing.T) {
	t.Parallel()
	toast, root := toastRig(t)
	tt := mount(t, root)
	assert.False(t, toast.Visible())
	assert.Empty(t, toast.Message())

	toast.Show(toastText)
	assert.True(t, toast.Visible())
	tt.Elapse(tvtest.Frame)
	f := toast.Bubble()
	x, y := int(f.X)+10, int(f.Y+f.H/2)
	assert.Less(t, int(pixelAt(tt, x, y).R), 0xf4, "fading in: still dark at the first frame")
	tt.Elapse(tvkit.ToastFade)
	assert.Equal(t, uint8(0xf4), pixelAt(tt, x, y).R, "fully in at 200 ms")

	tt.Elapse(2000 * time.Millisecond) // 1 frame + 200 + 2000 ms
	assert.True(t, toast.Visible())
	toast.Show("b")
	tt.Elapse(tvtest.Frame)
	f = toast.Bubble()
	assert.Equal(t, uint8(0xf4), pixelAt(tt, int(f.X)+10, int(f.Y+f.H/2)).R, "no new fade for a message that replaces one")
	tt.Elapse(2399*time.Millisecond - tvtest.Frame)
	assert.True(t, toast.Visible(), "2399 ms after the second message")
	tt.Elapse(time.Millisecond)
	assert.False(t, toast.Visible(), "2400 ms: gone, with no fade out")
	assert.Zero(t, pixelAt(tt, x, y).A)

	toast.Show("c")
	toast.Hide()
	assert.False(t, toast.Visible())
	tt.Elapse(5 * time.Second)
	assert.False(t, toast.Visible(), "the timer of a hidden toast did nothing")
	require.Empty(t, tt.Leaks())
}

// TestToastTickDoesNotAllocate: the frames of a toast on screen, fully faded
// in, and a message replacing the showing one, allocate nothing.
func TestToastTickDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	toast, root := toastRig(t)
	tt := mount(t, root)
	toast.Show(toastText)
	tt.Elapse(time.Second)
	msgs := [2]string{"a", "b"}
	i := 0
	step := func() {
		toast.Show(msgs[i%2]) // replaces: no fade, the timer restarts
		i++
		tt.Elapse(40 * time.Millisecond)
	}
	for range 4 {
		step()
	}
	frame := testing.AllocsPerRun(50, func() { tt.Elapse(16 * time.Millisecond) })
	assert.InDelta(t, 0, frame, 0, "a steady frame")
	assert.InDelta(t, 0, testing.AllocsPerRun(50, func() {
		toast.Hide()
		toast.Show(toastText)
		tt.Elapse(40 * time.Millisecond)
	}), 0, "hiding and opening")
	_ = step
}

// pixelAt returns the pixel of the tester's picture.
func pixelAt(tt interface{ Snapshot() *image.RGBA }, x, y int) color.RGBA {
	return tt.Snapshot().RGBAAt(x, y)
}
