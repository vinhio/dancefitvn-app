//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/stretchr/testify/assert"
)

// TestToastStateMatchesPrototype checks M-toast (Set 35's page with the toast "Đã tải xong bài 03",
// the finished download's) over the real app: the bubble is `div.toast` [16,708,361,44] within
// 1 DIP on the iPhone's size, 16 from the sides and 100 above the bottom on the 412 wide
// phone (one layout, PLAN D2), and it is gone 2200 ms after it began.
func TestToastStateMatchesPrototype(t *testing.T) {
	t.Parallel()

	r := atDetail(t, kittest.Phones[0], true, "set-35")
	r.Shell.Env().Act.Toast("Đã tải xong bài 03")
	r.Tester.Elapse(300 * time.Millisecond) // the 200 ms fade in
	lay := r.Dump("M-toast")
	checkAgainstProto(t, "M-toast", lay, 1, map[string]bool{"svg.i#14": true})
	bubble := element(t, lay, "div.toast")
	assert.Equal(t, "Đã tải xong bài 03", bubble.Text)
	r.Tester.Elapse(2000 * time.Millisecond)
	assert.False(t, r.Tester.Shows("Đã tải xong bài 03"), "gone after 2200 ms")

	rb := atDetail(t, kittest.Phones[1], true, "set-35")
	rb.Shell.Env().Act.Toast("Đã tải xong bài 03")
	rb.Tester.Elapse(300 * time.Millisecond)
	e := element(t, rb.Dump("M-toast"), "div.toast")
	assert.InDelta(t, 16, e.X, .01)
	assert.InDelta(t, 412-32, e.W, .01)
	assert.InDelta(t, 892-100-44, e.Y, .01)
	assert.InDelta(t, 44, e.H, .01)
}
