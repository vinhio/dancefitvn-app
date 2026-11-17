package format

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgressText(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		frac  float64
		bytes int64
		want  string
	}{
		{"35 percent", 0.35, 8800000, "3,1 / 8,8 MB · 35%"},
		{"64 percent", 0.64, 12800000, "8,2 / 12,8 MB · 64%"},
		{"zero", 0, 8800000, "0,0 / 8,8 MB · 0%"},
		{"done", 1, 8800000, "8,8 / 8,8 MB · 100%"},
		{"negative", -1, 8800000, "0,0 / 8,8 MB · 0%"},
		{"nan", math.NaN(), 8800000, "0,0 / 8,8 MB · 0%"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, Progress(tc.frac, tc.bytes))
		})
	}
}

func TestPercent(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		p    float64
		want int
	}{
		{0, 0}, {0.004, 0}, {0.005, 1}, {0.35, 35}, {0.995, 100}, {1, 100},
		{0.0049999999999999999, 1}, {0.49999999999999994 / 100, 0}, {-1, 0}, {math.NaN(), 0},
	} {
		assert.Equal(t, tc.want, Percent(tc.p), "p %v", tc.p)
	}
}

func TestNo(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		n    int
		want string
	}{{0, "00"}, {3, "03"}, {9, "09"}, {10, "10"}, {123, "123"}, {-4, "-4"}} {
		assert.Equal(t, tc.want, No(tc.n), "n %d", tc.n)
	}
}

func TestRemainingMinutes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		dur, pos float64
		want     int
	}{
		{"mid", 220, 72, 2},
		{"almost done", 220, 219, 1},
		{"start", 250, 0, 4},
		{"past the end", 100, 120, 1},
		{"half minute rounds up", 210, 0, 4},
		{"nan", math.NaN(), 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, RemainingMinutes(tc.dur, tc.pos))
		})
	}
}

// TestJoinDropsEmptyParts joins the parts of a meta line with " · ": the plain join
// when every part is there, and no stray separator where a lesson has no tag.
func TestJoinDropsEmptyParts(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		parts []string
		want  string
	}{
		{"all", []string{"Warm-up", "04:32", "Đã xem"}, "Warm-up · 04:32 · Đã xem"},
		{"no tag", []string{"", "04:32"}, "04:32"},
		{"no tag, three", []string{"", "04:32", "Đã xem"}, "04:32 · Đã xem"},
		{"blank middle", []string{"a", " ", "b"}, "a · b"},
		{"one", []string{"a"}, "a"},
		{"none", nil, ""},
		{"all empty", []string{"", ""}, ""},
	} {
		assert.Equal(t, tc.want, Join(tc.parts...), tc.name)
	}
}

// TestDropBlankKeepsFullLines removes only the blank parts a template leaves.
func TestDropBlankKeepsFullLines(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ in, want string }{
		{"A · Bài 04 · Warm · còn 3 phút", "A · Bài 04 · Warm · còn 3 phút"},
		{"A · Bài 04 ·  · còn 3 phút", "A · Bài 04 · còn 3 phút"},
		{"A · BÀI 01 / 10 · ", "A · BÀI 01 / 10"},
		{" · 04:32", "04:32"},
		{"No separator", "No separator"},
		{"", ""},
	} {
		assert.Equal(t, tc.want, DropBlank(tc.in), tc.in)
	}
}
