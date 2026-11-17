//go:build tvos

package screens_test

import (
	"context"
	"errors"
	"testing"

	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/screens"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errorScreen returns the error screen showing.
func (r *rig) errorScreen() *screens.Error {
	r.t.Helper()
	e, ok := r.sh.Screen().(*screens.Error)
	require.True(r.t, ok, "the screen showing is TV11")

	return e
}

// offlineServices are the services of TV11's state: the network is gone.
func offlineServices(t testing.TB) *tvtest.Services {
	t.Helper()
	svc := tvtest.NewPlayerServices(t)
	svc.OnlineNow = false

	return svc
}

// TestErrorBoxesMatchPrototype compares every element of TV11.layout.json: the
// Wi-Fi, the title, the text and the two buttons (`e1` focused).
func TestErrorBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	r := openFast(t, offlineServices(t), env.RouteError, true)
	r.settle()
	require.Equal(t, "e1", r.key())
	r.tt.Elapse(tvkit.Transition)
	r.boxes(t, "TV11")
}

// TestErrorText: the strings are the prototype's; the hotline comes from the
// support's config.
func TestErrorText(t *testing.T) {
	t.Parallel()
	svc := offlineServices(t)
	release := svc.Hold() // the read of the hotline is under way, not raced by the frames
	r := openFast(t, svc, env.RouteError, true)
	assert.Equal(t, "Mã lỗi: NET-503", r.errorScreen().CodeText(), "before the hotline is read")
	release()
	r.settle()
	lay := r.layout()
	want := map[string]string{
		"div.d":         "KHÔNG THỂ KẾT NỐI",
		"p.muted":       "Apple TV đang mất kết nối Internet. Kiểm tra Wi‑Fi rồi thử lại.",
		"e1":            "Thử lại",
		"e2":            "Quay lại",
		"tv.error.code": "Mã lỗi: NET-503 · Hỗ trợ: 036 800 8268",
	}
	for key, txt := range want {
		assert.Equal(t, txt, find(t, lay, key).Text, key)
	}
}

// TestErrorWithoutTheConfigShowsTheCodeAlone: with the support unreachable the line
// is the code alone.
func TestErrorWithoutTheConfigShowsTheCodeAlone(t *testing.T) {
	t.Parallel()
	svc := offlineServices(t)
	svc.ReadErr = errors.New("no network")
	r := openFast(t, svc, env.RouteError, false)
	r.settle()
	assert.Equal(t, "Mã lỗi: NET-503", r.errorScreen().CodeText())
	assert.Empty(t, r.tt.Leaks())
	_ = context.Canceled
}

// TestErrorRetry: `e1` with the network still gone says so in a toast and stays;
// with the network back it goes back to the screen under it. `e2` goes back, and
// Menu does too.
func TestErrorRetry(t *testing.T) {
	t.Parallel()
	svc := offlineServices(t)
	r := openFast(t, svc, env.RouteHome, false)
	r.settle()
	r.sh.Go(nav.Route{Screen: env.RouteError}, false)
	r.settle()
	require.Equal(t, "e1", r.key())

	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, env.RouteError, r.sh.Stack().Screen())
	assert.Equal(t, "Vẫn chưa có mạng", r.sh.ToastHost().Message())

	svc.OnlineNow = true
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())

	r.sh.Go(nav.Route{Screen: env.RouteError}, false)
	r.settle()
	require.True(t, r.press(keys.KeyRight))
	require.Equal(t, "e2", r.key())
	require.True(t, r.press(keys.KeyEnter))
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())

	r.sh.Go(nav.Route{Screen: env.RouteError}, false)
	r.settle()
	require.True(t, r.press(keys.KeyEscape))
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
	assert.Empty(t, r.tt.Leaks())
}

// TestErrorIsShownWhenAPlaybackIsAskedOffline: a lesson played with the network gone
// shows TV11 over the screen, and the player is not started (index.html:739).
func TestErrorIsShownWhenAPlaybackIsAskedOffline(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewPlayerServices(t)
	r := openFast(t, svc, env.RouteHome, false)
	r.settle()
	svc.OnlineNow = false
	require.Equal(t, "h1", r.key())
	require.True(t, r.press(keys.KeyEnter))
	r.settle()
	assert.Equal(t, env.RouteError, r.sh.Stack().Screen())
	assert.Zero(t, r.playback().Calls["PlayVideo"])
}
