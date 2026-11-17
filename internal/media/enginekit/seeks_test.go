// Adapted from jplayer-mobile/internal/media/enginekit/seeks_test.go at 49de3c2.

package enginekit_test

import (
	"dancefitvn-app/internal/media/enginekit"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSeekCount checks the seeks under way of a player that lives across
// loads (F5 of M9.R): a completion lost by a load does not pin the next
// load's position to a seek target, and one told late does not hide the
// next seek.
func TestSeekCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		do   func(c *enginekit.SeekCount)
		want bool
	}{
		{name: "none", do: func(*enginekit.SeekCount) {}, want: false},
		{name: "one asked", do: func(c *enginekit.SeekCount) { c.Ask() }, want: true},
		{name: "one asked and done", do: func(c *enginekit.SeekCount) { c.Ask(); c.Done() }, want: false},
		{name: "two asked, one done", do: func(c *enginekit.SeekCount) { c.Ask(); c.Ask(); c.Done() }, want: true},
		{
			name: "a completion lost, then a new load",
			do: func(c *enginekit.SeekCount) {
				c.Ask()
				c.Rebase()
			},
			want: false,
		},
		{
			name: "a completion lost, a new load seeks and is done",
			do: func(c *enginekit.SeekCount) {
				c.Ask()
				c.Rebase()
				c.Ask()
				c.Done()
			},
			want: false,
		},
		{
			name: "an old completion told late does not end the new seek",
			do: func(c *enginekit.SeekCount) {
				c.Ask()
				c.Rebase()
				c.Done() // the old load's, late
				c.Ask()
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var c enginekit.SeekCount
			tt.do(&c)
			assert.Equal(t, tt.want, c.Pending())
		})
	}
}
