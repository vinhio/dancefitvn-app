//go:build tvos

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// toastLoad is player.err.load.
const toastLoad = "Không phát được bài này. Thử lại sau"

// TestPlayerFailedLoadShowsPlayAndRetries checks TV07 when the lesson does not load
// (the stream answers 404, the playlist is gone): the toast says so, `p1` is "Phát"
// again (the video is not playing), and a press asks the player once more.
func TestPlayerFailedLoadShowsPlayAndRetries(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	r := inPlayer(t, svc, true)
	pb := r.playback()
	require.Equal(t, "p1", r.key())

	require.True(t, r.press(keys.KeyEnter))
	pb.Publish(player.ChangeState)
	pb.StateV = player.StateLoading
	r.settle()
	require.Equal(t, "Tạm dừng", find(t, r.layout(), "p1").Text, "a load under way shows pause, as playing does")

	pb.Fail(&api.Error{Status: 404, Code: "not_found"})
	r.settle()
	assert.Equal(t, "Phát", find(t, r.layout(), "p1").Text)
	assert.Equal(t, toastLoad, r.sh.ToastHost().Message())
	assert.True(t, r.sh.ToastHost().Visible())
	assert.Equal(t, "p1", r.key(), "the focus stays where it was")

	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 2, pb.Calls["Toggle"], "the press asks the player, which fetches the stream again")
	assert.Equal(t, player.StateLoading, pb.StateV)
	pb.Publish(player.ChangeState)
	r.settle()
	assert.Equal(t, "Tạm dừng", find(t, r.layout(), "p1").Text)
	assert.Empty(t, r.tt.Leaks())
}

// TestMusicFailedLoadShowsPlayAndRetries is the same on TV09: the disc shows PLAY
// again, the toast speaks, a press on the disc retries.
func TestMusicFailedLoadShowsPlayAndRetries(t *testing.T) {
	t.Parallel()
	svc := audioState(t)
	r := openFast(t, svc, env.RouteMusic, true)
	r.settle()
	pb := svc.PB.(*tvtest.Playback)
	pb.Start()
	pb.Publish(player.ChangeState)
	r.settle()
	require.Equal(t, icon.Pause, r.music().Toggle().Glyph())

	pb.Fail(&api.Error{Status: 404, Code: "not_found"})
	r.settle()
	assert.Equal(t, icon.Play, r.music().Toggle().Glyph())
	assert.Equal(t, toastLoad, r.sh.ToastHost().Message())

	for r.key() != "mu1" {
		require.True(t, r.press(keys.KeyRight))
	}
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, 1, pb.Calls["Toggle"])
	assert.Equal(t, player.StateLoading, pb.StateV)
}
