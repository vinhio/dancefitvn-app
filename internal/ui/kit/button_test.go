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
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestButtonBoxesMatchPrototype lays each kind of button out at the box the
// prototype gives it (local/shots/prototype-kit/mobile, 393 wide: the content
// column is 353) with the phones' real text engines, and compares the box and
// the icon with layout.json.
func TestButtonBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		state string
		key   string // of the button
		iconK string // of its icon, "" for none
		b     func() *kit.Button
	}{
		// M02 `button.btn.p` ĐĂNG NHẬP.
		{"p", "M02", "button.btn.p", "", func() *kit.Button { return kit.NewButton(kit.ButtonP, "Đăng nhập", icon.NameUnknown) }},
		// M02 `bioLogin`: .btn.g with the shield (20).
		{"g", "M02", "bioLogin", "svg.i#5", func() *kit.Button {
			b := kit.NewButton(kit.ButtonG, "Đăng nhập bằng Face ID / Vân tay", icon.Shield)
			b.Name = "bioLogin"

			return b
		}},
		// M14 `delConfirm`: .btn.r with the trash (20).
		{"r", "M14", "delConfirm", "", func() *kit.Button {
			b := kit.NewButton(kit.ButtonR, "Xoá khỏi máy", icon.Trash)
			b.Name = "delConfirm"

			return b
		}},
		// M14 `closeSheet#2`: .btn.s of 48.
		{"s", "M14", "closeSheet#2", "", func() *kit.Button {
			b := kit.NewButton(kit.ButtonS, "Huỷ", icon.NameUnknown)
			b.Name, b.Height = "closeSheet", 48

			return b
		}},
		// M15-logout `sheet:logout`: .btn.t in --dg.
		{"t", "M15-logout", "sheet:logout", "", func() *kit.Button {
			b := kit.NewButton(kit.ButtonT, "Đăng xuất", icon.NameUnknown)
			b.Name, b.Ink = "sheet:logout", theme.InkDanger

			return b
		}},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				want := protoBox(t, tc.state, tc.key)
				b := tc.b()
				mountStageReal(t, ph, placeAt(b, want.X, want.Y, want.W))
				rectNear(t, want, frameOf(b), .5, tc.key)
				if tc.iconK != "" {
					iw := protoBox(t, tc.state, tc.iconK)
					ir := b.IconRect()
					rectNear(t, geom.XYWH(iw.X-want.X, iw.Y-want.Y, iw.W, iw.H), ir, 1, tc.iconK)
				}
			})
		}
	})
}

// TestButtonKindsPaint checks the pixels of every kind at rest on both phones.
func TestButtonKindsPaint(t *testing.T) {
	t.Parallel()
	look := palette()
	tests := []struct {
		name string
		kind kit.ButtonKind
		fill color.Color // what the middle of the button's left edge is, 4 DIPs in
	}{
		{"p", kit.ButtonP, look.T.Primary},
		{"g", kit.ButtonG, look.T.Bg},
		{"s", kit.ButtonS, look.T.Surface2},
		{"r", kit.ButtonR, look.T.Danger},
		{"t", kit.ButtonT, look.T.Bg},
		{"unknown reads as p", kit.ButtonKindUnknown, look.T.Primary},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				b := kit.NewButton(tc.kind, "Xong", icon.NameUnknown)
				tt := mountStage(t, ph, placeAt(b, 20, 100, 353))
				img := tt.Snapshot()
				pixelNear(t, img, 24, 127, tc.fill, 2, tc.name+" fill")
				if tc.kind == kit.ButtonG {
					// the 1.2 px outline: the first row is the full border color
					pixelNear(t, img, 100, 100, look.T.Border, 3, "outline top")
				}
				// outside: --bg, the corner is rounded
				pixelNear(t, img, 21, 101, look.T.Bg, 2, "corner is rounded")
			})
		}
	})
}

// TestButtonPress covers the tap, the press scale and the disabled state on both
// phones: the scale is .98 at once while the finger is down and 1 as it lifts,
// with no time in between (.btn:active has no transition).
func TestButtonPress(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("tap", func(t *testing.T) {
			presses := 0
			b := kit.NewButton(kit.ButtonP, "Đăng nhập", icon.NameUnknown)
			b.OnPress = func() { presses++ }
			tt := mountStage(t, ph, placeAt(b, 20, 100, 353))
			require.NoError(t, tt.Click("Đăng nhập"))
			assert.Equal(t, 1, presses)
			assert.Equal(t, "button.btn.p", b.Name)
			assert.InDelta(t, 1, b.Scale(), 0)
		})
		t.Run("scale is instant", func(t *testing.T) {
			b := kit.NewButton(kit.ButtonP, "Đăng nhập", icon.NameUnknown)
			tt := mountStage(t, ph, placeAt(b, 20, 100, 353))
			c := frameOf(b).Center()
			img := tt.Snapshot()
			pixelNear(t, img, 21, 127, look.T.Primary, 2, "x=21 is inside at rest")
			tt.Press(c)
			assert.InDelta(t, theme.PressScale, b.Scale(), 1e-6, "scaled on the frame of the press")
			img = tt.Snapshot()
			// 353 * .98 = 345.94: the left edge moves in by 3.5 DIPs
			pixelNear(t, img, 21, 127, look.T.Bg, 2, "x=21 is outside while pressed")
			pixelNear(t, img, 25, 127, look.T.Primary, 2, "x=25 is inside while pressed")
			tt.Elapse(time.Second)
			assert.InDelta(t, theme.PressScale, b.Scale(), 1e-6, "held: no time moves it")
			tt.Release(c)
			assert.InDelta(t, 1, b.Scale(), 0, "back to 1 on the frame of the release")
		})
		t.Run("dragged away does not press", func(t *testing.T) {
			presses := 0
			b := kit.NewButton(kit.ButtonP, "Đăng nhập", icon.NameUnknown)
			b.OnPress = func() { presses++ }
			tt := mountStage(t, ph, placeAt(b, 20, 100, 353))
			tt.Press(frameOf(b).Center())
			tt.Release(geom.Pt(200, 400))
			assert.Zero(t, presses)
			assert.InDelta(t, 1, b.Scale(), 0)
		})
		t.Run("disabled", func(t *testing.T) {
			presses := 0
			b := kit.NewButton(kit.ButtonP, "Kết nối TV", icon.Tv)
			b.OnPress = func() { presses++ }
			b.SetDisabled(true)
			tt := mountStage(t, ph, placeAt(b, 20, 100, 353))
			require.NoError(t, tt.Click("Kết nối TV"))
			assert.Zero(t, presses, "a disabled button takes no press")
			assert.InDelta(t, 1, b.Scale(), 0)
			pixelNear(t, tt.Snapshot(), 24, 127, over(look.T.Primary.WithAlpha(.45), look.T.Bg), 2, "drawn at .45")
			b.SetDisabled(false)
			tt.Elapse(kittest.Frame)
			require.NoError(t, tt.Click("Kết nối TV"))
			assert.Equal(t, 1, presses)
			pixelNear(t, tt.Snapshot(), 24, 127, look.T.Primary, 2, "back to opaque")
		})
		t.Run("keyboard", func(t *testing.T) {
			presses := 0
			b := kit.NewButton(kit.ButtonS, "Huỷ", icon.NameUnknown)
			b.OnPress = func() { presses++ }
			tt := mountStage(t, ph, placeAt(b, 20, 100, 353))
			tt.Key(keys.KeyTab, 0)
			tt.Key(keys.KeyEnter, 0)
			assert.Equal(t, 1, presses)
		})
		t.Run("semantics", func(t *testing.T) {
			b := kit.NewButton(kit.ButtonP, "Đăng nhập", icon.NameUnknown)
			tt := mountStage(t, ph, placeAt(b, 20, 100, 353))
			found := false
			for _, n := range tt.Semantics().Nodes {
				if n.Role == platform.RoleButton && n.Label == "Đăng nhập" {
					found = true
				}
			}
			assert.True(t, found, "the button is a button named by its text")
		})
	})
}

// TestButtonWrapAndText checks a button that wraps its content, SetText and the
// name of each kind.
func TestButtonWrapAndText(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		b := &kit.Button{Kind: kit.ButtonG, Text: "A", Wrap: true}
		mountStage(t, ph, placeAt(b, 0, 0, 0))
		w1 := frameOf(b).W
		b.SetText("Một chữ dài hơn nhiều")
		tt := mountStage(t, ph, placeAt(b, 0, 0, 0))
		tt.Elapse(kittest.Frame)
		assert.Greater(t, frameOf(b).W, w1)
		names := map[kit.ButtonKind]string{
			kit.ButtonP: "button.btn.p", kit.ButtonG: "button.btn.g", kit.ButtonS: "button.btn.s",
			kit.ButtonR: "button.btn.r", kit.ButtonT: "button.btn.t",
		}
		for k, want := range names {
			nb := kit.NewButton(k, "x", icon.NameUnknown)
			mountStage(t, ph, placeAt(nb, 0, 0, 100))
			assert.Equal(t, want, nb.Name)
		}
	})
}

// TestButtonPaintDoesNotAllocate checks that painting a button at rest, pressed
// and disabled, and pressing it, allocates nothing.
func TestButtonPaintDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		rest := kit.NewButton(kit.ButtonP, "Đăng nhập", icon.Check)
		pressed := kit.NewButton(kit.ButtonG, "Đăng nhập bằng Face ID", icon.Shield)
		off := kit.NewButton(kit.ButtonP, "Kết nối TV", icon.Tv)
		off.SetDisabled(true)
		tt := mountStage(t, ph, placeAt(rest, 20, 100, 353), placeAt(pressed, 20, 200, 353), placeAt(off, 20, 300, 353))
		tt.Press(frameOf(pressed).Center())
		tt.Elapse(kittest.Frame)
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a repaint with a pressed and a disabled button", ph.Name)
		presses := 0
		rest.OnPress = func() { presses++ }
		assert.Zero(t, testing.AllocsPerRun(50, rest.Press), "%s: a press", ph.Name)
		assert.Positive(t, presses)
	}
}

// BenchmarkButtonRepaint is a steady repaint of a button.
func BenchmarkButtonRepaint(b *testing.B) {
	btn := kit.NewButton(kit.ButtonP, "Đăng nhập", icon.Check)
	tt := mountStage(b, kittest.Phones[0], placeAt(btn, 20, 100, 353))
	tt.Repaint()
	b.ReportAllocs()
	for b.Loop() {
		tt.Repaint()
	}
}

// TestButtonUnderline checks the opt-in underline of an `<a class="btn">` (05B's
// enroll button): one row of the text's color, as wide as the label, that the
// same button without it does not draw.
func TestButtonUnderline(t *testing.T) {
	t.Parallel()
	look := palette()
	rowsOf := func(t *testing.T, ph kittest.Phone, underline bool) (rows []int, width int) {
		t.Helper()
		b := kit.NewButton(kit.ButtonP, "ĐĂNG KÝ TRÊN WEBSITE", icon.Globe)
		b.Underline = underline
		tt := mountStage(t, ph, placeAt(b, 20, 100, 353))
		img := tt.Snapshot()
		for y := 100; y < 154; y++ {
			n := 0
			for x := 20; x < 373; x++ {
				got := pixelAt(img, x, y)
				d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
				if d(got.R, look.T.OnPrimary.R) < 40 && d(got.G, look.T.OnPrimary.G) < 40 && d(got.B, look.T.OnPrimary.B) < 40 {
					n++
				}
			}
			if n > 120 {
				rows = append(rows, y)
				width = max(width, n)
			}
		}

		return rows, width
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		without, _ := rowsOf(t, ph, false)
		with, width := rowsOf(t, ph, true)
		require.NotEmpty(t, without)
		require.Len(t, with, len(without)+1, "one DIP thick: one row more than the text's")
		assert.Equal(t, without[len(without)-1]+2, with[len(with)-1], "the rule is one row below the text's last, under the baseline")
		assert.Greater(t, width, 150, "as wide as the text")
	})
}
