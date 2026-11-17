//go:build !tvos

package screens_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/stretchr/testify/assert"
)

// TestInfoNameRowKeepsToTheColumnOnBothPhones checks the pair "Tên" / "Họ" of 15B
// (K29, fixed by the owner 2026-11-17): the prototype's two `flex:1` fields keep
// an `<input>` of 171 DIPs at least and the second runs off the screen (M15B
// div.inp 20,252,201,54 and 233,252,201,54); here each box is half of the column
// less the 12 gap, so that "Họ" ends at the right gutter. The row is the same
// on both phones (D2) and the text typed, short or long, never moves or widens a
// box: the text scrolls inside it.
func TestInfoNameRowKeepsToTheColumnOnBothPhones(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("Nguyễn Thị Thảo Vy ", 6)
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, in := atInfo(t, ph, true)
		col := float64(ph.Size.W) - 40
		half := (col - 12) / 2
		first, last := element(t, r.Dump("a"), "div.inp"), element(t, r.Dump("a"), "div.inp#2")
		assert.InDelta(t, 20, first.X, .5, "Tên at the page's gutter")
		assert.InDelta(t, 20+half+12, last.X, .5, "Họ after the 12 gap")
		assert.InDelta(t, half, first.W, .5)
		assert.InDelta(t, half, last.W, .5)
		assert.LessOrEqual(t, last.X+last.W, float64(ph.Size.W)-20+.5, "Họ ends at the right gutter, not past the screen")
		assert.InDelta(t, 54, first.H, .5)
		assert.InDelta(t, first.Y, last.Y, .01, "side by side")
		email := element(t, r.Dump("a"), "div.inp#3")
		assert.InDelta(t, col, email.W, .5, "the rest of the form spans the gutters")

		for name, v := range map[string][2]string{"short": {"Lan", "Tran"}, "long": {long, long}, "empty": {"", ""}} {
			in.First().SetText(v[0])
			in.Last().SetText(v[1])
			r.Tester.Elapse(kittest.Frame)
			f2, l2 := element(t, r.Dump(name), "div.inp"), element(t, r.Dump(name), "div.inp#2")
			assert.Equal(t, first, f2, name+": Tên's box")
			assert.Equal(t, last, l2, name+": Họ's box")
			assert.Equal(t, email, element(t, r.Dump(name), "div.inp#3"), name+": the email's box")
		}
	})
}
