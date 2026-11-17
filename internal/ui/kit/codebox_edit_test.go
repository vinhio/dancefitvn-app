package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
)

// codeRig mounts a row of boxes on a phone with its field focused by a tap on
// the middle, and records what OnChange tells.
func codeRig(t *testing.T, ph kittest.Phone) (*kit.CodeBoxes, *ui.Tester, *[]string) {
	t.Helper()
	var told []string
	row := kit.NewCodeBoxes("Mã trên TV")
	row.OnChange = func(s string) { told = append(told, s) }
	tt := mountStage(t, ph, placeAt(row, 20, 372, ph.Size.W-40))
	tt.ClickAt(frameOf(row).Center())
	assert.True(t, row.Field().Focused())

	return row, tt, &told
}

// backspace presses Backspace as the system keyboard's delete key does and
// checks the caret stays at the end.
func backspace(t *testing.T, row *kit.CodeBoxes, tt *ui.Tester) {
	t.Helper()
	assert.True(t, tt.Key(keys.KeyBackspace, 0))
	assert.Equal(t, len(row.Value), row.Field().Caret(), "the caret is at the end of %q", row.Value)
	assert.False(t, row.Field().HasSelection())
}

// TestCodeBoxesBackspaceTakesBackTheLastCharacter is the bug report: a wrong
// character could not be deleted. A wrong character, a lower-case one, a full row
// and a paste each lose their last character to Backspace, and the code can be
// typed again.
func TestCodeBoxesBackspaceTakesBackTheLastCharacter(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range []struct{ name, typed, value string }{
			{"a wrong digit", "7", "7"},
			{"lower case", "7a", "7A"},
			{"a full row", "7K4M2P", "7K4M2P"},
			{"a seventh character", "7K4M2P9", "7K4M2P"},
			{"a paste with dash and space", " 7k4-m2p ", "7K4M2P"},
			{"only dropped characters", "-- ", ""},
		} {
			row, tt, told := codeRig(t, ph)
			tt.Type(tc.typed)
			assert.Equal(t, tc.value, row.Value, tc.name)
			assert.Equal(t, tc.value, row.Field().Text(), tc.name)
			assert.Equal(t, len(tc.value), row.Field().Caret(), "%s: the filter leaves the caret at the end", tc.name)
			if tc.value == "" {
				continue
			}
			n := len(*told)
			backspace(t, row, tt)
			assert.Equal(t, tc.value[:len(tc.value)-1], row.Value, tc.name)
			assert.Equal(t, row.Value, row.Field().Text(), tc.name)
			assert.Len(t, *told, n+1, "%s: OnChange hears the deletion", tc.name)
			assert.Equal(t, row.Value, (*told)[n])
			// the rest goes the same way, and the row takes a new code
			for range len(row.Value) {
				backspace(t, row, tt)
			}
			assert.Empty(t, row.Value, tc.name)
			assert.Equal(t, 0, row.FocusedBox())
			tt.Type("5b-9")
			assert.Equal(t, "5B9", row.Value, tc.name)
			assert.True(t, tt.Shows("B"), "the boxes show what was typed after deleting")
		}
	})
}

// TestCodeBoxesCaretStaysAtTheEnd is the root cause: the hidden field is as wide
// as the row, so a tap, a double or triple tap or a drag on a box put the caret
// inside the text or at its start, and Backspace deleted another character or
// none. Whatever the user touches or the keyboard asks, the caret stays at the
// end.
func TestCodeBoxesCaretStaysAtTheEnd(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		row, tt, _ := codeRig(t, ph)
		tt.Type("7K4M2X")
		f := frameOf(row)
		y := f.Center().Y
		for _, dx := range []float32{0, 1, 10, 30, 60, 100, 200, f.W - 1} {
			tt.ClickAt(geom.Pt(f.X+dx, y))
			assert.Equalf(t, 6, row.Field().Caret(), "a tap %v from the left", dx)
			assert.False(t, row.Field().HasSelection())
		}
		tt.Drag(geom.Pt(f.X+2, y), geom.Pt(f.X+40, y))
		assert.Equal(t, 6, row.Field().Caret(), "a drag")
		assert.False(t, row.Field().HasSelection())
		for range 3 { // double and triple clicks
			tt.ClickAt(geom.Pt(f.X+10, y))
		}
		assert.False(t, row.Field().HasSelection())
		for _, k := range []keys.Key{keys.KeyLeft, keys.KeyHome, keys.KeyUp, keys.KeyRight, keys.KeyEnd} {
			tt.Key(k, 0)
			assert.Equal(t, 6, row.Field().Caret(), "key %v", k)
		}
		tt.Key(keys.KeyLeft, keys.ModShift)
		tt.Key(keys.KeyA, keys.ModSuper|keys.ModCtrl)
		assert.False(t, row.Field().HasSelection())
		assert.Equal(t, 6, row.Field().Caret())

		// a tap on the first box and Backspace takes back the last character
		tt.ClickAt(geom.Pt(f.X+10, y))
		backspace(t, row, tt)
		assert.Equal(t, "7K4M2", row.Value)
		tt.Type("p")
		assert.Equal(t, "7K4M2P", row.Value)
	})
}

// TestCodeBoxesInputMethod drives the field as the backends do: UIKit's
// setSelectedTextRange and replaceRange:withText: and Android's compositions reach
// it as text-source calls and replacing or composing text events.
func TestCodeBoxesInputMethod(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		row, tt, told := codeRig(t, ph)
		tt.Type("ABCDEX")
		src := tt.TextSource()
		assert.Equal(t, 6, src.TextLength())

		// the keyboard moves the caret to the start (a tap, a loupe) and selects: ignored
		src.SelectText(platform.TextRange{Start: 0, End: 0})
		assert.Equal(t, platform.TextRange{Start: 6, End: 6}, src.TextSelection())
		src.SelectText(platform.TextRange{Start: 0, End: 6})
		assert.Equal(t, platform.TextRange{Start: 6, End: 6}, src.TextSelection())
		backspace(t, row, tt)
		assert.Equal(t, "ABCDE", row.Value)

		// an autocorrection replacing the last character, as UIKit names the range
		tt.Replace(platform.TextRange{Start: 4, End: 5}, "f", false, 0)
		assert.Equal(t, "ABCDF", row.Value, "the replacement is filtered and upper-cased")
		assert.Equal(t, 5, row.Field().Caret())
		backspace(t, row, tt)
		assert.Equal(t, "ABCD", row.Value)

		// a replacement with a character the code never holds
		tt.Replace(platform.TextRange{Start: 3, End: 4}, "-", false, 0)
		assert.Equal(t, "ABC", row.Value)
		assert.Equal(t, 3, row.Field().Caret())
		backspace(t, row, tt)
		assert.Equal(t, "AB", row.Value)

		// a composition (Telex, predictive text) shows in the boxes and is the code from
		// the moment it is composed, so that the button of the screen follows it; a
		// deletion in it shortens it before the committed code is touched
		n := len(*told)
		tt.Compose("k4", 2)
		assert.Equal(t, "ABK4", row.Value)
		assert.Equal(t, "AB", row.Field().Text(), "the composition is not in the field's text yet")
		assert.Equal(t, 4, row.FocusedBox(), "AB and the composition K4 fill four boxes")
		assert.True(t, tt.Shows("K"))
		assert.True(t, tt.Shows("4"))
		tt.Compose("k", 1)
		assert.Equal(t, "ABK", row.Value)
		assert.False(t, tt.Shows("4"), "a shorter composition shows shorter")
		tt.Compose("", 0)
		assert.Equal(t, "AB", row.Value, "a composition taken back takes its characters back")
		assert.Equal(t, 2, row.FocusedBox())
		assert.False(t, tt.Shows("K"))
		assert.Equal(t, []string{"ABK4", "ABK", "AB"}, (*told)[n:], "every step of a composition is told")
		tt.Compose("k4", 2)
		tt.Type("k4") // the keyboard commits it
		assert.Equal(t, "ABK4", row.Value)
		assert.Equal(t, "ABK4", row.Field().Text())
		assert.Equal(t, 4, row.Field().Caret())
		backspace(t, row, tt)
		assert.Equal(t, "ABK", row.Value)
	})
}

// TestCodeBoxesCompositionIsTheCode is the other half of the device bug: with a
// keyboard that composes every letter (Vietnamese Telex), a code of six letters was
// all composition, Value stayed empty and the screen's button stayed off. The code
// is what the boxes show, a composition's characters included, and the focus going
// away commits it.
func TestCodeBoxesCompositionIsTheCode(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		row, tt, told := codeRig(t, ph)
		// letters typed one by one, each making the composition longer, and a
		// digit and a seventh letter past the limit
		word := ""
		for _, ch := range "qz9ab" {
			word += string(ch)
			tt.Compose(word, len(word))
		}
		assert.Equal(t, "QZ9AB", row.Value)
		assert.Equal(t, "QZ9AB", (*told)[len(*told)-1])
		assert.Empty(t, row.Field().Text())
		tt.Compose(word+"cd", 7)
		assert.Equal(t, "QZ9ABC", row.Value)

		// the keyboard goes away: the composition is committed, not dropped
		tt.View().Focus(nil)
		assert.Equal(t, "QZ9ABC", row.Value)
		assert.Equal(t, "QZ9ABC", row.Field().Text())
		assert.Empty(t, row.Field().Composing())
		assert.Equal(t, 5, row.FocusedBox())

		// a tap focuses the field again; Backspace takes the last character back
		tt.ClickAt(frameOf(row).Center())
		backspace(t, row, tt)
		assert.Equal(t, "QZ9AB", row.Value)
	})
}

// TestCodeBoxesCompositionOverTheLimit checks a composition longer than the code
// shows six characters, as the field will hold.
func TestCodeBoxesCompositionOverTheLimit(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		row, tt, _ := codeRig(t, ph)
		tt.Compose("abcdefgh", 8)
		assert.Equal(t, 5, row.FocusedBox())
		assert.Equal(t, "ABCDEF", row.Value)
		tt.Type("abcdefgh")
		assert.Equal(t, "ABCDEF", row.Value)
		backspace(t, row, tt)
		assert.Equal(t, "ABCDE", row.Value)
	})
}

// inkIn tells whether the pixels of the frame last drawn inside r (in DIPs, inset
// by 6 so that the border and its ring stay out) are not all one colour: a
// character is drawn there.
func inkIn(tt *ui.Tester, ph kittest.Phone, r geom.Rect) bool {
	tt.Elapse(kittest.Frame)
	img := tt.Snapshot()
	k := float32(img.Rect.Dx()) / ph.Size.W
	r = r.Inset(geom.Uniform(6))
	first := img.RGBAAt(int(r.X*k), int(r.Y*k))
	for y := int(r.Y * k); y < int((r.Y+r.H)*k); y++ {
		for x := int(r.X * k); x < int((r.X+r.W)*k); x++ {
			if img.RGBAAt(x, y) != first {
				return true
			}
		}
	}

	return false
}

// TestCodeBoxesDeletedCharacterLeavesTheBox is the device bug seen with the first
// fix in place: Backspace moved the ring and shortened the code, yet the boxes
// kept their letters on the screen. A box whose character went had its label's
// text emptied without the label being laid out again, so that it painted the
// layout of the character it held before; the tests only read the text. The
// pixels of every box are checked after each deletion, by Backspace and by the
// corrections of an input method.
func TestCodeBoxesDeletedCharacterLeavesTheBox(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		row := kit.NewCodeBoxes("Mã trên TV")
		tt := mountStageReal(t, ph, placeAt(row, 20, 372, ph.Size.W-40))
		tt.ClickAt(frameOf(row).Center())
		boxes := codeBoxesOf(row)
		ink := func(n int) bool { return inkIn(tt, ph, frameOf(boxes[n])) }

		tt.Type("7K4M2P")
		for i := range boxes {
			assert.True(t, ink(i), "box %d shows its character", i)
		}
		for n := 5; n >= 0; n-- {
			assert.True(t, tt.Key(keys.KeyBackspace, 0))
			assert.Equal(t, n, len(row.Value))
			for i := range boxes {
				assert.Equalf(t, i < n, ink(i), "after Backspace to %d characters, box %d", n, i)
			}
		}

		// a composition that shrinks, and one an input method ends
		tt.Compose("abcdef", 6)
		assert.Equal(t, 6, countInk(boxes, ink))
		tt.Compose("ab", 2)
		assert.Equal(t, 2, countInk(boxes, ink), "a shorter composition empties the boxes it left")
		tt.Compose("", 0)
		assert.Equal(t, 0, countInk(boxes, ink), "a composition taken back empties the boxes")
	})
}

// countInk counts the boxes that show a character.
func countInk(boxes []ui.Widget, ink func(int) bool) int {
	n := 0
	for i := range boxes {
		if ink(i) {
			n++
		}
	}

	return n
}
