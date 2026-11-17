//go:build !tvos

package screens_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chipsOf returns the boxes of the chips of a state: the tag chip (`span.chip.y`) and
// the first other chip, whatever its key.
func chipsOf(r *shelltest.Rig, id, tagKey string) (tag, other struct {
	x, w float64
	ok   bool
}) {
	for _, e := range r.Dump(id).Elements {
		if !strings.HasPrefix(e.Key, "span.chip") {
			continue
		}
		switch {
		case e.Key == tagKey:
			tag.x, tag.w, tag.ok = e.X, e.W, true
		case !other.ok:
			other.x, other.w, other.ok = e.X, e.W, true
		}
	}

	return tag, other
}

// untagged makes the player move on to a lesson with no tag, as production lessons are.
func untagged(r *shelltest.Rig, pb *shelltest.FakePlayback, kind player.Kind) {
	first := pb.Lesson()
	second := first
	second.ID, second.No, second.Tag = "l_untagged", first.No+1, ""
	pb.Set(first, kind, player.StatePaused, 10, 100)
	pb.SetList([]player.Lesson{first, second})
	pb.Next()
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
	r.Tester.Elapse(kittest.Frame)
}

// TestVideoWithoutATagHasNoEmptyChip is the empty tag chip on screen 06, on both phones:
// a lesson with no tag shows no yellow pill, and the "Video dọc 9:16" chip takes its place
// at the left of the row, with no gap before it; a tagged lesson is as it was.
func TestVideoWithoutATagHasNoEmptyChip(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		tag, other := chipsOf(r, "tagged", "span.chip.y")
		require.True(t, tag.ok, "a lesson with a tag has its chip")
		require.True(t, other.ok)
		assert.Greater(t, other.x, tag.x+tag.w, "the other chip follows the tag chip and the gap")

		untagged(r, pb, player.KindVideo)
		tag2, other2 := chipsOf(r, "untagged", "span.chip.y")
		assert.False(t, tag2.ok, "no empty tag chip")
		require.True(t, other2.ok)
		assert.InDelta(t, tag.x, other2.x, .01, "the chip takes the first place, no gap")
		assert.InDelta(t, other.w, other2.w, .01)
	})
}

// TestAudioWithoutATagHasNoEmptyChip is the same for screen 07.
func TestAudioWithoutATagHasNoEmptyChip(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atAudio(t, ph, false)
		tag, other := chipsOf(r, "tagged", "span.chip")
		require.True(t, tag.ok, "a lesson with a tag has its chip")
		require.True(t, other.ok)
		assert.Greater(t, other.x, tag.x+tag.w)

		untagged(r, pb, player.KindAudio)
		var chips []float64
		for _, e := range r.Dump("untagged").Elements {
			if strings.HasPrefix(e.Key, "span.chip") {
				chips = append(chips, e.X)
			}
		}
		require.Len(t, chips, 1, "only the file's chip: no empty tag chip")
		assert.InDelta(t, tag.x, chips[0], .01, "the file's chip takes the first place, no gap")
	})
}
