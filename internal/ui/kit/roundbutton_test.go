package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoundButtonBoxesMatchPrototype compares the back button, the dark one and
// the close button of a sheet with their layout.json boxes, icons included.
func TestRoundButtonBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		// M09 `back` [20,54,40,40] with `svg.i` [29,63,22,22]; `button.rb.dk`
		// [333,54,40,40]; `closeSheet#2` [341,479,32,32] with `svg.i#14` [348,486,18,18].
		back := kit.NewRoundButton(icon.Cl, true, "Quay lại")
		back.Name = "back"
		dk := kit.NewRoundButton(icon.More, true, "Thêm")
		closeB := kit.NewRoundButton(icon.X, false, "Đóng")
		closeB.Name, closeB.Size, closeB.IconSize, closeB.Bg = "closeSheet", 32, 18, look.T.Surface2
		tt := mountStage(t, ph, placeAt(back, 20, 54, 0), placeAt(dk, 333, 54, 0), placeAt(closeB, 341, 479, 0))
		rectNear(t, protoBox(t, "M09", "back"), frameOf(back), 0, "back")
		rectNear(t, protoBox(t, "M09", "svg.i"), frameOf(part(t, back, "svg.i")), .01, "back icon")
		rectNear(t, protoBox(t, "M09", "button.rb.dk"), frameOf(dk), 0, "button.rb.dk")
		assert.Equal(t, "button.rb.dk", dk.Name)
		rectNear(t, protoBox(t, "M09", "closeSheet#2"), frameOf(closeB), 0, "closeSheet#2")
		rectNear(t, protoBox(t, "M09", "svg.i#14"), frameOf(part(t, closeB, "svg.i")), .01, "close icon")

		img := tt.Snapshot()
		pixelNear(t, img, 20+20, 54+20, over(look.C.RoundDark, look.T.Bg), 3, "back is .dk too (M09)")
		pixelNear(t, img, 333+20, 54+4, over(look.C.RoundDark, look.T.Bg), 3, "dark disc")
		pixelNear(t, img, 341+16, 479+3, look.T.Surface2, 3, "close disc in --sf2")
		pixelNear(t, img, 341, 479, look.T.Bg, 2, "disc corner")
	})
}

// TestRoundButtonPress covers the tap, disabled and the plain disc's color.
func TestRoundButtonPress(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		presses := 0
		b := kit.NewRoundButton(icon.Cl, false, "Quay lại")
		b.OnPress = func() { presses++ }
		tt := mountStage(t, ph, placeAt(b, 20, 54, 0))
		assert.Equal(t, "button.rb", b.Name)
		pixelNear(t, tt.Snapshot(), 20+20, 54+3, look.T.Surface, 3, ".rb is --sf")
		require.NoError(t, tt.Click("Quay lại"))
		assert.Equal(t, 1, presses)
		b.Disabled = true
		tt.ClickAt(frameOf(b).Center())
		assert.Equal(t, 1, presses)
	})
}

// TestIconButtonMatchesPrototype compares the eye of M02 (22 x 26) and checks
// that pressing it toggles the mask of its field.
func TestIconButtonMatchesPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		frame := protoBox(t, "M02", "div.inp#2")
		eye := kit.NewIconButton(icon.Eye, "Hiện mật khẩu")
		eye.Name = "pwEye"
		in := kit.NewInput(icon.Lock, "")
		in.Mask, in.Trailing = '•', eye
		eye.OnPress = func() {
			if in.Mask == 0 {
				in.SetMask('•')
			} else {
				in.SetMask(0)
			}
		}
		tt := mountStage(t, ph, placeAt(in, frame.X, frame.Y, frame.W))
		rectNear(t, protoBox(t, "M02", "pwEye"), frameOf(eye), 0, "pwEye")
		rectNear(t, protoBox(t, "M02", "svg.i#3"), frameOf(part(t, eye, "svg.i")), .01, "eye icon")
		tt.ClickAt(frameOf(in.Field()).Center())
		tt.Type("matkhau")
		assert.Equal(t, "•••••••", in.Field().TextLayout().Text)
		require.NoError(t, tt.Click("Hiện mật khẩu"))
		assert.Equal(t, "matkhau", in.Field().TextLayout().Text, "the eye shows the text")
		require.NoError(t, tt.Click("Hiện mật khẩu"))
		assert.Equal(t, "•••••••", in.Field().TextLayout().Text)
	})
}

// TestTextPresets compares the text atoms with the boxes of the prototype: the
// `.eb` line is 14 high, `.d` and `.h1` are as high as their size.
func TestTextPresets(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		eb := kit.Eyebrow("Thứ bảy, 14/11 · Chào buổi sáng")
		d := kit.Display("DanceFIT Set 35", 36)
		h1 := kit.H1("Tải xuống")
		sec := kit.SecTitle("Khoá học của tôi")
		m13, m12 := kit.Muted("Cùng nhảy cùng hát", 13), kit.Muted("2/10 đã xem", 12)
		mountStageReal(t, ph, placeAt(eb, 20, 100, 0), placeAt(d, 20, 140, 0), placeAt(h1, 20, 200, 0), placeAt(sec, 20, 250, 0),
			placeAt(m13, 20, 300, 0), placeAt(m12, 20, 340, 0))
		assert.InDelta(t, 14, frameOf(eb).H, 0)
		assert.InDelta(t, 36, frameOf(d).H, 0)
		assert.InDelta(t, 34, frameOf(h1).H, 0)
		assert.InDelta(t, 18, frameOf(sec).H, 0)
		assert.InDelta(t, 133.2, frameOf(sec).W, 1.5, "M03 span.d KHOÁ HỌC CỦA TÔI")
		assert.Equal(t, "div.eb", eb.Name)
		assert.Equal(t, "div.d", d.Name)
		assert.Equal(t, "div.h1", h1.Name)
		assert.Equal(t, "span.d", sec.Name)
		assert.InDelta(t, 16, frameOf(m13).H, 0, "M09 div.muted 13px")
		assert.InDelta(t, 15, frameOf(m12).H, 0, "M09 div.muted#2 12px")
	})
}
