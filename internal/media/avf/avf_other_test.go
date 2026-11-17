// Adapted from jplayer-mobile/internal/media/avf/avf_other_test.go at 49de3c2.

//go:build !ios || tvos || !cgo

package avf_test

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avf"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnsupportedOffIOS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func() error
	}{
		{"engine", func() error { _, err := avf.New(media.Config{}, nil); return err }},
		{"session", func() error { _, err := avf.NewSession(media.Config{}); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, tt.call(), core.ErrUnsupported)
		})
	}
}

func TestStatsOffIOS(t *testing.T) {
	t.Parallel()

	_, ok := avf.StatsOf(nil)
	assert.False(t, ok)
	_, ok = avf.SessionStatsOf(nil)
	assert.False(t, ok)
	assert.False(t, avf.Tick(nil))
	assert.False(t, avf.SetLateOutput(nil, true))
	assert.False(t, avf.SetKeepVideoInBackground(nil, true))
	_, _, ok = avf.VideoTracks(nil)
	assert.False(t, ok)
	assert.False(t, avf.SetVariantSizing(nil, true))
	assert.False(t, avf.SetNextStarved(nil, true))
	_, _, ok = avf.PlayerRate(nil)
	assert.False(t, ok)
	_, ok = avf.PitchAlgorithm(nil)
	assert.False(t, ok)
	assert.False(t, avf.SetMaxResolution(nil, 854, 480))
	_, _, ok = avf.MaxResolution(nil)
	assert.False(t, ok)
}
