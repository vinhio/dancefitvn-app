//go:build !tvos

package env_test

import (
	"testing"

	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/stretchr/testify/assert"
)

// TestFillDropsTheSeparatorOfAnEmptyValue is F3: a lesson with no tag leaves no stray
// " · " in the continue card ("Bài 04 ·  · còn 3 phút"); with the tag the text is the
// template filled, byte for byte.
func TestFillDropsTheSeparatorOfAnEmptyValue(t *testing.T) {
	t.Parallel()

	const meta = "{course} · Bài {no} · {tag} · còn {min} phút"
	assert.Equal(t, "Set 35 · Bài 04 · Warm-up · còn 3 phút", env.Fill(meta, "course", "Set 35", "no", "04", "tag", "Warm-up", "min", "3"))
	assert.Equal(t, "Set 35 · Bài 04 · còn 3 phút", env.Fill(meta, "course", "Set 35", "no", "04", "tag", "", "min", "3"))
	assert.Equal(t, "Warm-up · đang xem 01:02 / 04:32", env.Fill("{tag} · đang xem {pos} / {dur}", "tag", "Warm-up", "pos", "01:02", "dur", "04:32"))
	assert.Equal(t, "đang xem 01:02 / 04:32", env.Fill("{tag} · đang xem {pos} / {dur}", "tag", "", "pos", "01:02", "dur", "04:32"))
}
