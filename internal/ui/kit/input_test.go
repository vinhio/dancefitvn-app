package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInputBoxesMatchPrototype compares inputs with the boxes of M02, M02B and
// M04: the frame, the field and the icon, 54 tall and 44 for the search.
func TestInputBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("email", func(t *testing.T) {
			// M02 `div.inp` [20,322.59,353,54] with the mail icon and `input` [67,340.09,291,19].
			frame := protoBox(t, "M02", "div.inp")
			in := kit.NewInput(icon.Mail, "")
			mountStageReal(t, ph, placeAt(in, frame.X, frame.Y, frame.W))
			rectNear(t, frame, frameOf(in), 0, "div.inp")
			rectNear(t, protoBox(t, "M02", "input"), frameOf(in.Field()), .5, "input")
			assert.Equal(t, "div.inp", in.Name)
			assert.Equal(t, "input", in.Field().Name)
		})
		t.Run("password with the eye", func(t *testing.T) {
			// M02 `div.inp#2` with the lock, `input#2` [67,432.09,259,19], `pwEye` [336,428.59,22,26].
			frame := protoBox(t, "M02", "div.inp#2")
			eye := kit.NewIconButton(icon.Eye, "Hiện mật khẩu")
			eye.Name = "pwEye"
			in := kit.NewInput(icon.Lock, "Bản thử: bất kỳ ≥ 6 ký tự")
			in.Mask, in.Trailing = '•', eye
			mountStageReal(t, ph, placeAt(in, frame.X, frame.Y, frame.W))
			rectNear(t, frame, frameOf(in), 0, "div.inp#2")
			rectNear(t, protoBox(t, "M02", "input#2"), frameOf(in.Field()), .5, "input#2")
			rectNear(t, protoBox(t, "M02", "pwEye"), frameOf(eye), .5, "pwEye")
		})
		t.Run("search", func(t *testing.T) {
			// M04 `div.inp` [20,168,353,44], `input` [67,180.5,291,19].
			frame := protoBox(t, "M04", "div.inp")
			in := kit.NewInput(icon.Search, "Tìm khoá học theo tên…")
			in.Search = true
			mountStageReal(t, ph, placeAt(in, frame.X, frame.Y, frame.W))
			rectNear(t, frame, frameOf(in), 0, "search div.inp")
			rectNear(t, protoBox(t, "M04", "input"), frameOf(in.Field()), .5, "search input")
		})
		t.Run("without icon", func(t *testing.T) {
			// M15B `div.inp` [20,252,201,54] and `input` [35,269.5,171,19].
			frame := protoBox(t, "M15B", "div.inp")
			in := kit.NewInput(icon.NameUnknown, "")
			mountStageReal(t, ph, placeAt(in, frame.X, frame.Y, frame.W))
			rectNear(t, frame, frameOf(in), 0, "div.inp")
			rectNear(t, protoBox(t, "M15B", "input"), frameOf(in.Field()), .5, "input")
		})
	})
}

// TestInputFocusAndError covers the border states: 1 px --bd at rest, 1.6 --pr
// with the focus, 1.6 --dg in error, error over focus.
func TestInputFocusAndError(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		a, b := kit.NewInput(icon.Mail, "Email"), kit.NewInput(icon.Lock, "Mật khẩu")
		tt := mountStage(t, ph, placeAt(a, 20, 100, 353), placeAt(b, 20, 200, 353))
		img := tt.Snapshot()
		assert.InDelta(t, 1, a.BorderWidth(), 0)
		pixelNear(t, img, 120, 100, look.T.Border, 3, "rest: border row 0")
		pixelNear(t, img, 120, 101, look.T.Surface, 3, "rest: border row 1 is the fill")
		pixelNear(t, img, 120, 127, look.T.Surface, 2, "rest: fill")

		tt.ClickAt(frameOf(a.Field()).Center())
		assert.True(t, a.Focused())
		assert.InDelta(t, 1.6, a.BorderWidth(), 1e-5)
		img = tt.Snapshot()
		pixelNear(t, img, 120, 100, look.T.Primary, 3, "focus: border row 0")
		pixelNear(t, img, 120, 101, look.T.Surface, 3, "focus: the ring is drawn 1 px, Chrome floors 1.6px")
		assert.InDelta(t, 1, b.BorderWidth(), 0, "the other field is not focused")

		// the focus moves: the first goes back
		tt.ClickAt(frameOf(b.Field()).Center())
		assert.False(t, a.Focused())
		assert.InDelta(t, 1, a.BorderWidth(), 0)
		assert.InDelta(t, 1.6, b.BorderWidth(), 1e-5)

		// error: --dg, wins over the focus
		b.SetError(true)
		tt.Elapse(kittest.Frame)
		assert.Equal(t, "div.inp.err", b.Name)
		assert.True(t, b.Focused())
		img = tt.Snapshot()
		pixelNear(t, img, 120, 200, look.T.Danger, 3, "error with focus: --dg")
		b.SetError(false)
		tt.Elapse(kittest.Frame)
		assert.Equal(t, "div.inp", b.Name)
		pixelNear(t, tt.Snapshot(), 120, 200, look.T.Primary, 3, "focus again")
	})
}

// TestInputTyping covers typing, OnChange, the mask, the keyboard and the
// placeholder.
func TestInputTyping(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("type", func(t *testing.T) {
			var got []string
			in := kit.NewInput(icon.Mail, "Email")
			in.OnChange = func(s string) { got = append(got, s) }
			in.Keyboard = platform.KeyboardEmail
			tt := mountStage(t, ph, placeAt(in, 20, 100, 353))
			tt.ClickAt(frameOf(in.Field()).Center())
			require.True(t, tt.Type("thao@x.vn"))
			assert.Equal(t, "thao@x.vn", in.Text())
			require.NotEmpty(t, got)
			assert.Equal(t, "thao@x.vn", got[len(got)-1])
			assert.Equal(t, platform.KeyboardEmail, tt.TextInput().Keyboard, "the on-screen keyboard is the e-mail one")
			in.SetText("a")
			assert.Equal(t, "a", in.Text())
			assert.Equal(t, "thao@x.vn", got[len(got)-1], "SetText does not tell OnChange")
		})
		t.Run("mask", func(t *testing.T) {
			in := kit.NewInput(icon.Lock, "Mật khẩu")
			in.Mask = '•'
			tt := mountStage(t, ph, placeAt(in, 20, 100, 353))
			tt.ClickAt(frameOf(in.Field()).Center())
			tt.Type("abc123")
			assert.Equal(t, "abc123", in.Text())
			assert.Equal(t, "••••••", in.Field().TextLayout().Text, "what is drawn is the mask")
			for _, n := range tt.Semantics().Nodes {
				if n.Role == platform.RoleTextField {
					assert.Empty(t, n.Value, "a password is told to no one")
				}
			}
		})
		t.Run("submit", func(t *testing.T) {
			var sub string
			in := kit.NewInput(icon.Search, "")
			in.OnSubmit = func(s string) { sub = s }
			tt := mountStage(t, ph, placeAt(in, 20, 100, 353))
			tt.ClickAt(frameOf(in.Field()).Center())
			tt.Type("zumba")
			tt.Keys("Enter")
			assert.Equal(t, "zumba", sub)
		})
		t.Run("placeholder is drawn until there is text", func(t *testing.T) {
			look := palette()
			in := kit.NewInput(icon.NameUnknown, "Email")
			tt := mountStageReal(t, ph, placeAt(in, 20, 100, 353))
			f := frameOf(in.Field())
			ink := func() bool {
				img := tt.Snapshot()
				for y := int(f.Y); y < int(f.Y+f.H); y++ {
					for x := int(f.X); x < int(f.X+40); x++ {
						if p := pixelAt(img, x, y); int(p.R) > int(look.T.Surface.R)+30 {
							return true
						}
					}
				}

				return false
			}
			assert.True(t, ink(), "the placeholder shows")
			tt.ClickAt(f.Center())
			tt.Type("ab")
			assert.True(t, ink(), "the text shows")
		})
		t.Run("semantics", func(t *testing.T) {
			in := kit.NewInput(icon.Mail, "Email")
			fl := &kit.Field{Label: "Email", Input: in}
			tt := mountStage(t, ph, placeAt(fl, 20, 100, 353))
			found := false
			for _, n := range tt.Semantics().Nodes {
				if n.Role == platform.RoleTextField && n.Label == "Email" {
					found = true
				}
			}
			assert.True(t, found, "the field is named by its label")
		})
	})
}

// TestFieldBoxesMatchPrototype compares a field with the elements of M02B: the
// label, the frame and the help line under it, and M15B's required label.
func TestFieldBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("error with help", func(t *testing.T) {
			label := protoBox(t, "M02B", "label#2")
			in := kit.NewInput(icon.Lock, "Bản thử: bất kỳ ≥ 6 ký tự")
			in.Mask, in.Error = '•', true
			fl := &kit.Field{Label: "Mật khẩu", Help: "Mật khẩu chưa đúng. Kiểm tra lại Caps Lock.", Input: in}
			mountStageReal(t, ph, placeAt(fl, label.X, label.Y, label.W))
			lf := fl.LabelFrame()
			rectNear(t, protoBox(t, "M02B", "label#2"), geom.XYWH(label.X+lf.X, label.Y+lf.Y, lf.W, lf.H), .5, "label")
			rectNear(t, protoBox(t, "M02B", "div.inp.err"), frameOf(in), 0, "div.inp.err")
			help := protoBox(t, "M02B", "div.help")
			hf := fl.HelpFrame()
			assert.InDelta(t, help.Y, label.Y+hf.Y, .5, "help y")
			assert.InDelta(t, help.H, hf.H, .5, "help h")
			// the field is as tall as its parts: 14 + 8 + 54 + 7 + 15
			assert.InDelta(t, 14+8+54+7+15, frameOf(fl).H, .5)
		})
		t.Run("required", func(t *testing.T) {
			label := protoBox(t, "M15B", "div.inp#3")
			in := kit.NewInput(icon.Mail, "")
			fl := &kit.Field{Label: "Email", Required: true, Input: in}
			mountStageReal(t, ph, placeAt(fl, label.X, label.Y-22, label.W))
			rectNear(t, label, frameOf(in), 0, "div.inp#3")
			assert.InDelta(t, 14+8+54, frameOf(fl).H, .5, "no help: label, gap, frame")
			assert.InDelta(t, float64(kit.FieldGap), 16, 0)
		})
	})
}

// TestInputPaintDoesNotAllocate checks that painting inputs (empty, typed, with
// the focus, in error, with a mask) allocates nothing.
func TestInputPaintDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		empty := kit.NewInput(icon.Mail, "Email")
		typed := kit.NewInput(icon.Lock, "")
		typed.Mask = '•'
		bad := kit.NewInput(icon.Lock, "Mật khẩu")
		bad.Error = true
		fl := &kit.Field{Label: "Mật khẩu", Required: true, Help: "Mật khẩu chưa đúng.", Input: bad}
		tt := mountStage(t, ph, placeAt(empty, 20, 100, 353), placeAt(typed, 20, 200, 353), placeAt(fl, 20, 300, 353))
		tt.ClickAt(frameOf(typed.Field()).Center())
		tt.Type("matkhau")
		tt.Elapse(kittest.Frame)
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a repaint of the inputs", ph.Name)
	}
}
