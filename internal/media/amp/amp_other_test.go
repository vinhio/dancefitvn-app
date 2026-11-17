// Adapted from jplayer-mobile/internal/media/amp/amp_other_test.go at 49de3c2.

//go:build !android || !cgo

package amp_test

import (
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/amp"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
)

// TestAMPNeedsAndroid checks that off Android every constructor reports
// core.ErrUnsupported and nothing else is done.
func TestAMPNeedsAndroid(t *testing.T) {
	t.Parallel()

	cfg := media.Config{Dispatch: func(fn func()) { fn() }, Clock: clock.NewFake(time.Time{})}
	tests := []struct {
		name string
		call func() error
	}{
		{"New", func() error { _, err := amp.New(cfg, nil); return err }},
		{"NewSession", func() error { _, err := amp.NewSession(cfg); return err }},
		{"ConvertYUV420", func() error {
			_, err := amp.ConvertYUV420(nil, 0, 0, 0, 0, 0, nil, 0, nil, nil, 0, 0, false)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, tt.call(), core.ErrUnsupported)
		})
	}
	_, ok := amp.StatsOf(nil)
	assert.False(t, ok)
	_, ok = amp.SessionStatsOf(nil)
	assert.False(t, ok)
	assert.Empty(t, amp.NoisyActions)
	_, ok = amp.Position(nil)
	assert.False(t, ok)
	_, _, ok = amp.PlaybackParams(nil)
	assert.False(t, ok)
}
