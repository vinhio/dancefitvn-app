package app

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// okTransport answers 200 to everything.
type okTransport struct{}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Do answers 200.
func (okTransport) Do(context.Context, *api.Request) (*api.Response, error) {
	return &api.Response{Status: http.StatusOK, Header: http.Header{}}, nil
}

// TestDevTransport checks the count of answers, the dead network (a
// *url.Error, which the client takes for one) and a held route, released or
// left by its context.
func TestDevTransport(t *testing.T) {
	t.Parallel()

	tr := newDevTransport(okTransport{})
	idle, served := tr.idle()
	assert.True(t, idle)
	assert.Zero(t, served)
	_, err := tr.Do(t.Context(), &api.Request{RouteID: "app.config"})
	require.NoError(t, err)
	_, served = tr.idle()
	assert.Equal(t, 1, served)

	tr.setOffline(true)
	_, err = tr.Do(t.Context(), &api.Request{RouteID: "app.config", Path: "/app/config"})
	var ue *url.Error
	require.ErrorAs(t, err, &ue)
	require.ErrorIs(t, err, errNoNetwork)
	tr.setOffline(false)

	release := tr.hold("me.home")
	done := make(chan error, 1)
	go func() {
		_, err := tr.Do(context.Background(), &api.Request{RouteID: "me.home"})
		done <- err
	}()
	idle, _ = tr.idle()
	assert.True(t, idle, "a held request is no work under way")
	release()
	release()
	require.NoError(t, <-done)

	tr.hold("me.home")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = tr.Do(ctx, &api.Request{RouteID: "me.home"})
	require.True(t, errors.Is(err, context.Canceled))
}

// TestDevAPIBase checks DFV_API: an http base on this machine, https
// anywhere, nothing else.
func TestDevAPIBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  string
		set  bool
		bad  bool
	}{
		{"unset", "", false, false},
		{"blank", "  ", false, false},
		{"loopback", "http://127.0.0.1:8787/api/v1", true, false},
		{"localhost", "http://localhost:8787/api/v1", true, false},
		{"ipv6 loopback", "http://[::1]:8787/api/v1", true, false},
		{"https", "https://example.test/api/v1", true, false},
		{"lan http", "http://192.168.1.9:8787/api/v1", false, true},
		{"not a url", "127.0.0.1:8787", false, true},
		{"ftp", "ftp://127.0.0.1/api/v1", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			base, set, err := devAPIBase(func(k string) string {
				require.Equal(t, apiEnv, k)

				return tt.env
			})
			if tt.bad {
				require.ErrorIs(t, err, core.ErrInvalid)
				assert.NotContains(t, err.Error(), tt.env, "the URL is not logged")
				assert.False(t, set)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.set, set)
			if set {
				assert.Equal(t, tt.env, base)
			}
		})
	}
}
