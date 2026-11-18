//go:build tvos

package tvkit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSlideRevealPutsTheItemInViewAtOnce: Reveal gives the offset the focus would
// (dl9 of TV06: 161, dl8: 61, an early one: 0), with no glide, and the focus that
// follows scrolls from there.
func TestSlideRevealPutsTheItemInViewAtOnce(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		row  int
		want float32
	}{{0, 0}, {7, 0}, {8, 61}, {9, 161}} {
		sl, l, _, tt := listRig(t)
		sl.Reveal(listPad+float32(tc.row)*listPitch, listRow)
		tt.Elapse(1)
		assert.Equal(t, tc.want, sl.Offset(), "row %d", tc.row)
		assert.False(t, sl.Gliding(), "row %d: no glide", tc.row)
		assert.Equal(t, tc.want, sl.Shown())
		f := l.rows[tc.row].Node().WindowFrame()
		assert.InDelta(t, 110+headH+listPad+float32(tc.row)*listPitch-tc.want, f.Y, .01, "row %d is where the offset puts it", tc.row)
	}
	sl, l, _, tt := listRig(t)
	sl.Reveal(listPad+9*listPitch, listRow)
	tt.Elapse(1)
	tt.View().Focus(l.rows[0])
	assert.Zero(t, sl.Offset(), "a focus after it scrolls as always")
}
