// Adapted from jplayer-mobile/internal/media/fake/unreachable_test.go at 49de3c2.

package fake_test

import (
	"context"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadUnreachable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		url  string
		err  error
	}{
		{"unreachable prefix", fake.UnreachablePrefix + "x.mp3", media.ErrUnreachable},
		{"plain path", "/data/x.mp3", nil},
		{"link", "https://x.test/a.mp3", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, fake.Options{})
			_, err := r.eng.Load(context.Background(), media.Source{URL: tt.url}, media.LoadOptions{Auto: true})
			if tt.err == nil {
				require.NoError(t, err)

				return
			}
			require.ErrorIs(t, err, tt.err)
			assert.ErrorIs(t, err, core.ErrNotFound)
			assert.NotErrorIs(t, err, media.ErrMissingFile)
		})
	}
}
