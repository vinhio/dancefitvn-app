//go:build tvos

package tvkit_test

import (
	"image/color"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/tv/focus"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	gcolor "github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// swatch is an Item that fills its box with a solid color, for the order tests.
type swatch struct {
	tvkit.Item
	col   color.RGBA
	fill  func(p *ui.Painter, r geom.Rect, c color.RGBA)
	w, h  float32
	child ui.Widget
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout is a box of w by h.
func (s *swatch) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Sz(s.w, s.h))
}

// Paint fills the box with the color.
func (s *swatch) Paint(_ *ui.Context, p *ui.Painter) {
	p.Fill(s.Bounds(), scene.Solid(colorOf(s.col)), 0)
}

// TestButtonSizesMatchPrototype compares the unscaled width and the height of
// the buttons with the focused boxes of the prototype (1.06 times larger when
// focused) and the resting ones.
func TestButtonSizesMatchPrototype(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		id    string
		key   string
		btn   func() *tvkit.Button
		scale float32
	}{
		{"w1 icon", "TV01", "w1", func() *tvkit.Button { return tvkit.NewButton("w1", texts()["tv.welcome.login"], icon.User) }, 1.06},
		{"w2", "TV01", "w2", func() *tvkit.Button { return tvkit.NewButton("w2", texts()["tv.welcome.more"], icon.NameUnknown) }, 1},
		{"d2", "TV06", "d2", func() *tvkit.Button { return tvkit.NewButton("d2", texts()["tv.back"], icon.NameUnknown) }, 1},
		{"c2 tiny", "TV02", "c2", func() *tvkit.Button {
			b := tvkit.NewButton("c2", texts()["tv.back"], icon.NameUnknown)
			b.Size = tvkit.ButtonTiny

			return b
		}, 1},
		{"p2 small with a 30 icon", "TV07", "p2", func() *tvkit.Button {
			b := tvkit.NewButton("p2", "Tuỳ chọn", icon.Speed)
			b.Size, b.IconSize = tvkit.ButtonSmall, 30

			return b
		}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := &stage{}
			b := tc.btn()
			root.put(b, 100, 100)
			mount(t, root)
			want := tvtest.ProtoBox(t, tc.id, tc.key)
			want.W /= tc.scale
			want.H /= tc.scale
			assert.InDelta(t, want.W, b.Frame().W, boxTol, "width")
			assert.InDelta(t, want.H, b.Frame().H, boxTol, "height")
		})
	}
}

// TestItemFocusLook: focusing a Button switches its fill and ink at once and
// grows the scale to 1.06 over 150 ms (CSS `ease`); leaving shrinks it back; the
// shadow follows the same 150 ms.
func TestItemFocusLook(t *testing.T) {
	t.Parallel()
	root := &stage{}
	b := tvkit.NewButton("w2", "Tìm hiểu thêm", icon.NameUnknown)
	root.put(b, 100, 100)
	tt := mount(t, root)
	assert.Equal(t, float32(1), b.Scale())
	assert.Zero(t, b.Blend())
	s, _ := b.LayoutStyle(tt.View().Context())
	assert.Equal(t, "rgba(255, 255, 255, 0.12)", s.Background)
	assert.Equal(t, "rgb(244, 244, 242)", s.Color)
	assert.Equal(t, "44px", s.Radius)
	assert.Equal(t, "700", s.FontWeight)
	assert.Equal(t, "30px", s.FontSize)

	tt.View().Focus(b)
	tt.Elapse(tvtest.Frame)
	s, _ = b.LayoutStyle(tt.View().Context())
	assert.Equal(t, "rgb(255, 208, 40)", s.Background, "--pr at once")
	assert.Equal(t, "rgb(20, 21, 24)", s.Color, "--on at once")
	mid := b.Scale()
	assert.Greater(t, mid, float32(1))
	assert.Less(t, mid, tvkit.FocusScale)
	tt.Elapse(75 * time.Millisecond)
	assert.Greater(t, b.Scale(), mid)
	tt.Elapse(75 * time.Millisecond)
	assert.InDelta(t, 1.06, b.Scale(), .0001)
	assert.InDelta(t, 1, b.Blend(), .0001)
	// the box is the scaled one for layoutdump
	m, ok := b.PaintTransform()
	require.True(t, ok)
	got := m.Bounds(b.Bounds())
	assert.InDelta(t, b.Frame().W*1.06, got.W, .01)
	assert.InDelta(t, -b.Frame().W*.03, got.X, .01)

	tt.View().Focus(nil)
	tt.Elapse(tvkit.Transition)
	assert.InDelta(t, 1, b.Scale(), .0001)
	_, ok = b.PaintTransform()
	assert.False(t, ok)
	assert.Equal(t, tvkit.Transition, 150*time.Millisecond)
}

// TestItemPixels checks the paint of a focused Button: --pr inside, the drop
// shadow below it, nothing at rest below it.
func TestItemPixels(t *testing.T) {
	t.Parallel()
	root := &stage{}
	b := tvkit.NewButton("w2", "Tìm hiểu thêm", icon.NameUnknown)
	root.put(b, 400, 300)
	tt := mount(t, root)
	rest := tt.Snapshot()
	f := b.Frame()
	px := func(img interface{ At(int, int) color.Color }, x, y float32) color.RGBA {
		r, g, bl, a := img.At(int(x), int(y)).RGBA()

		return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), uint8(a >> 8)}
	}
	below := func(img interface{ At(int, int) color.Color }) color.RGBA {
		return px(img, f.X+f.W/2, f.Y+f.H+30)
	}
	assert.Zero(t, below(rest).A, "no shadow at rest: the transparent view shows through")

	tt.View().Focus(b)
	tt.Elapse(tvkit.Transition + tvtest.Frame)
	img := tt.Snapshot()
	inside := px(img, f.X+10, f.Y+f.H/2)
	assert.Equal(t, color.RGBA{0xff, 0xd0, 0x28, 0xff}, inside, "--pr inside")
	sh := below(img)
	assert.Greater(t, int(sh.A), 30, "the drop shadow (0 26 50 black .6) is below the button")
	assert.Zero(t, sh.R)
}

// TestRaisePaintsFocusedLast: the child with the focus is painted over its
// siblings (F-12), whatever its place in the order.
func TestRaisePaintsFocusedLast(t *testing.T) {
	t.Parallel()
	red := &swatch{col: color.RGBA{255, 0, 0, 255}, w: 100, h: 100}
	blue := &swatch{col: color.RGBA{0, 0, 255, 255}, w: 100, h: 100}
	red.Key, blue.Key = "a", "b"
	for _, s := range []*swatch{red, blue} {
		s.Name = s.Key
	}
	group := &tvkit.Raise{}
	group.Add(red, blue)
	root := &stage{}
	root.put(group, 200, 200)
	tt := mount(t, root)

	over := func() color.RGBA {
		img := tt.Snapshot()
		r, g, b, a := img.At(250, 250).RGBA()

		return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
	}
	assert.Equal(t, color.RGBA{0, 0, 255, 255}, over(), "blue is last in the order")
	tt.View().Focus(red)
	tt.Elapse(tvtest.Frame)
	assert.Equal(t, color.RGBA{255, 0, 0, 255}, over(), "red has the focus: painted last")
	tt.View().Focus(blue)
	tt.Elapse(tvtest.Frame)
	assert.Equal(t, color.RGBA{0, 0, 255, 255}, over())
	tt.View().Focus(nil)
	tt.Elapse(tvtest.Frame)
	assert.Equal(t, color.RGBA{0, 0, 255, 255}, over())
}

// TestItemActivateAndMove: OnSelect on a tap, OnHold on a hold (false without
// it, so the Manager taps), OnMove takes an arrow before the focus moves.
func TestItemActivateAndMove(t *testing.T) {
	t.Parallel()
	it := &tvkit.Item{}
	assert.False(t, it.Activate(false))
	assert.False(t, it.Activate(true))
	taps, holds := 0, 0
	it.OnSelect = func() { taps++ }
	assert.True(t, it.Activate(false))
	assert.False(t, it.Activate(true), "no hold: the Manager runs the tap")
	it.OnHold = func() { holds++ }
	assert.True(t, it.Activate(true))
	assert.Equal(t, []int{1, 1}, []int{taps, holds})
	assert.True(t, it.AcceptsFocus())
	it.Disabled = true
	assert.False(t, it.AcceptsFocus())
	it.Disabled, it.Hidden = false, true
	assert.False(t, it.AcceptsFocus())
	it.SetPressing(true)
	assert.True(t, it.Pressing())
	it.Pulse()

	// OnMove: used presses do not move the focus
	root := &stage{}
	a := tvkit.NewButton("a", "A", icon.NameUnknown)
	b := tvkit.NewButton("b", "B", icon.NameUnknown)
	root.put(a, 100, 100)
	root.put(b, 400, 100)
	var seen []focus.Dir
	a.OnMove = func(d focus.Dir) bool { seen = append(seen, d); return d == focus.DirRight }
	tt := mount(t, root)
	tt.View().Focus(a)
	assert.True(t, tt.Key(keys.KeyRight, 0))
	assert.Same(t, a, tt.View().Focused(), "OnMove took the arrow")
	assert.False(t, tt.Key(keys.KeyLeft, 0))
	assert.Equal(t, []focus.Dir{focus.DirRight, focus.DirLeft}, seen)
	_ = b
}

// colorOf converts a test color to gDesk's.
func colorOf(c color.RGBA) gcolor.Color { return gcolor.Color{R: c.R, G: c.G, B: c.B, A: c.A} }
