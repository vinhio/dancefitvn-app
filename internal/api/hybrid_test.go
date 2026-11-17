package api

import (
	"context"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
)

func TestHybridRouting(t *testing.T) {
	t.Parallel()

	live := always(200, `{"from":"live"}`)
	mock := always(200, `{"from":"mock"}`)
	h := NewHybrid(live, mock, []string{"me.courses", "app.config"})

	for _, route := range Routes() {
		t.Run(route.ID, func(t *testing.T) {
			t.Parallel()

			resp, err := h.Do(context.Background(), &Request{RouteID: route.ID, Method: route.Method, Path: route.Path})
			assert.NoError(t, err)
			want := `{"from":"mock"}`
			if route.ID == "me.courses" || route.ID == "app.config" {
				want = `{"from":"live"}`
			}
			assert.Equal(t, want, string(resp.Body))
		})
	}
}

func TestHybridMissingTransport(t *testing.T) {
	t.Parallel()

	h := NewHybrid(nil, nil, []string{"a"})
	for _, id := range []string{"a", "b"} {
		_, err := h.Do(context.Background(), &Request{RouteID: id})
		assert.ErrorIs(t, err, core.ErrInvalid)
	}
}

func TestHybridUnderClient(t *testing.T) {
	t.Parallel()

	live := always(200, `{}`)
	mock := always(200, `{}`)
	c := newTestClient(t, NewHybrid(live, mock, []string{"app.config"}))
	assert.NoError(t, ignore(c.Config(context.Background())))
	assert.NoError(t, ignore(c.Me(context.Background())))
	assert.Len(t, live.seen(), 1)
	assert.Len(t, mock.seen(), 1)
}

func TestCheckLiveRoutes(t *testing.T) {
	t.Parallel()

	both := []string{"auth.login", "auth.refresh"}
	tests := []struct {
		name    string
		ids     []string
		wantErr string // "" = ok
	}{
		{name: "nil", ids: nil},
		{name: "empty", ids: []string{}},
		{name: "unknown", ids: []string{"me.nope"}, wantErr: "unknown"},
		{name: "unknown beside valid", ids: []string{"app.config", "x"}, wantErr: `"x"`},
		{name: "public only", ids: []string{"app.config", "auth.forgot", "tv.pair.start", "tv.pair.poll"}},
		{name: "login only", ids: []string{"auth.login"}},
		{name: "auth with no login or refresh", ids: []string{"me.home"}, wantErr: "me.home need"},
		{name: "auth with login only", ids: []string{"auth.login", "me.home"}, wantErr: "auth.refresh stay"},
		{name: "auth with refresh only", ids: []string{"auth.refresh", "me.home"}, wantErr: "auth.login stay"},
		{name: "auth with both", ids: append([]string{"me.home"}, both...)},
		{name: "plan example", ids: []string{"app.config", "auth.login", "auth.refresh", "me.home"}},
		{name: "duplicates", ids: []string{"auth.login", "auth.login", "auth.refresh", "me.get"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := CheckLiveRoutes(tc.ids)
			if tc.wantErr == "" {
				assert.NoError(t, err)

				return
			}
			assert.ErrorIs(t, err, core.ErrInvalid)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

// TestCheckLiveRoutesEveryRoute puts each of the 24 routes live alone and
// then with login and refresh.
func TestCheckLiveRoutesEveryRoute(t *testing.T) {
	t.Parallel()

	for _, route := range Routes() {
		t.Run(route.ID, func(t *testing.T) {
			t.Parallel()

			alone := CheckLiveRoutes([]string{route.ID})
			withAuth := CheckLiveRoutes([]string{route.ID, "auth.login", "auth.refresh"})
			assert.NoError(t, withAuth)
			switch {
			case route.ID == "auth.login" || route.ID == "auth.refresh":
				assert.NoError(t, alone)
			case route.Auth:
				assert.ErrorIs(t, alone, core.ErrInvalid)
				assert.ErrorContains(t, alone, route.ID)
			default:
				assert.NoError(t, alone)
			}
		})
	}
}

func TestCheckLiveRoutesDoesNotChangeInput(t *testing.T) {
	t.Parallel()

	ids := []string{"me.get", "auth.login"}
	_ = CheckLiveRoutes(ids)
	assert.Equal(t, []string{"me.get", "auth.login"}, ids)
}
