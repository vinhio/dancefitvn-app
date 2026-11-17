// Adapted from jplayer-tv/internal/media/avf/engine_other_test.go at 781bee5.

//go:build !(ios && tvos)

package avftv_test

import (
	"context"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avftv"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUnsupportedOffTVOS(t *testing.T) {
	t.Parallel()

	e, err := avftv.New(avftv.Config{
		Clock:    clock.NewFake(time.Unix(0, 0)),
		Post:     func(func()) bool { return true },
		Underlay: func(uintptr) (avftv.Plane, error) { return nil, core.ErrUnsupported },
	})
	require.ErrorIs(t, err, core.ErrUnsupported)
	assert.Nil(t, e)
}

func TestZeroEngineOffTVOS(t *testing.T) {
	t.Parallel()

	var z avftv.Engine
	var _ media.Engine = &z
	_, err := z.Load(context.Background(), media.Source{URL: "http://127.0.0.1/x.m3u8"}, media.LoadOptions{})
	require.ErrorIs(t, err, core.ErrUnsupported)
	for _, call := range []func() error{z.Play, z.Pause, func() error { return z.Seek(1) }, func() error { return z.SetRate(1.5) }} {
		require.ErrorIs(t, call(), core.ErrUnsupported)
	}
	z.Place(geom.XYWH(0, 0, 10, 10))
	z.SetMirror(true)
	assert.False(t, z.Mirrored())
	assert.Nil(t, z.Video())
	assert.NotNil(t, z.Events())
	assert.Equal(t, avftv.Stats{}, z.Stats())
	assert.False(t, avftv.Tick(&z))
	_, _, ok := avftv.PlayerRate(&z)
	assert.False(t, ok)
	_, ok = avftv.LayerScaleX(&z)
	assert.False(t, ok)
	_, ok = avftv.LayerReadyForDisplay(&z)
	assert.False(t, ok)
	var _ media.Backgrounder = &z
	z.SetBackground(true)
	z.SetBackground(false)
	require.NoError(t, z.Stop())
	require.NoError(t, z.Close())
}
