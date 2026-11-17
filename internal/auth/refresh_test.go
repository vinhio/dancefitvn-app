package auth_test

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/model"
)

// signedIn returns a rig with a user logged in through the mock.
func signedIn(t *testing.T) (*rig, string) {
	t.Helper()
	r := newRig(t, false)
	s, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err)

	return r, s.AccessToken
}

// TestRefreshScenarios runs every scenario of auth.refresh of routes.json.
func TestRefreshScenarios(t *testing.T) {
	t.Parallel()

	t.Run("default", func(t *testing.T) {
		t.Parallel()

		r, old := signedIn(t)
		tok, err := r.mgr.Refresh(t.Context(), old)
		require.NoError(t, err)
		assert.NotEqual(t, old, tok)
		assert.Equal(t, tok, r.mgr.Token())
		assert.Equal(t, 1, r.spy.count("auth.refresh"))

		s, ok := r.mgr.Session()
		require.True(t, ok)
		assert.Equal(t, "rt_mock_9b1d7e", s.RefreshToken, "the refresh token rotates")
		assert.Equal(t, "u_412", s.User.ID, "user and device stay")
		saved, err := r.vault.Load()
		require.NoError(t, err)
		assert.Equal(t, tok, saved.AccessToken, "the vault holds the new pair")
		assert.Equal(t, "rt_mock_9b1d7e", saved.RefreshToken)
		assert.Empty(t, r.outs())
	})

	t.Run("refresh_invalid", func(t *testing.T) {
		t.Parallel()

		r, old := signedIn(t)
		require.NoError(t, r.mock.SetScenario("auth.refresh", "refresh_invalid"))
		clears := r.clears()

		_, err := r.mgr.Refresh(t.Context(), old)
		require.ErrorIs(t, err, auth.ErrSignedOut)
		var ae *api.Error
		require.ErrorAs(t, err, &ae)
		assert.Equal(t, "refresh_invalid", ae.Code)

		assert.Empty(t, r.mgr.Token())
		assert.False(t, r.vault.Has(), "vault cleared")
		assert.Equal(t, clears+1, r.clears(), "cache cleared")
		outs := r.outs()
		require.Len(t, outs, 1, "OnSignedOut once")
		assert.ErrorIs(t, outs[0], auth.ErrSignedOut)

		_, err = r.mgr.Refresh(t.Context(), old)
		require.ErrorIs(t, err, auth.ErrSignedOut, "signed out: no second call")
		assert.Equal(t, 1, r.spy.count("auth.refresh"))
		assert.Len(t, r.outs(), 1)
	})
}

func TestRefreshOfflineKeepsSession(t *testing.T) {
	t.Parallel()

	r, old := signedIn(t)
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		if req.RouteID == "auth.refresh" {
			return nil, netErr, true
		}

		return nil, nil, false
	}
	_, err := r.mgr.Refresh(t.Context(), old)
	require.ErrorIs(t, err, api.ErrOffline)
	assert.NotErrorIs(t, err, auth.ErrSignedOut)
	assert.Equal(t, old, r.mgr.Token(), "session kept")
	assert.True(t, r.vault.Has())
	assert.Empty(t, r.outs())

	// back online: the same refresh token works
	r.spy.hook = nil
	tok, err := r.mgr.Refresh(t.Context(), old)
	require.NoError(t, err)
	assert.NotEqual(t, old, tok)
}

func TestRefreshServerErrorKeepsSession(t *testing.T) {
	t.Parallel()

	r, old := signedIn(t)
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		if req.RouteID == "auth.refresh" {
			return jsonResp(http.StatusInternalServerError, `{"error":{"code":"server_error","message":"x"}}`), nil, true
		}

		return nil, nil, false
	}
	_, err := r.mgr.Refresh(t.Context(), old)
	require.ErrorIs(t, err, api.ErrServer)
	assert.Equal(t, old, r.mgr.Token())
	assert.True(t, r.vault.Has())
	assert.Empty(t, r.outs())
}

func TestRefreshCanceledKeepsSession(t *testing.T) {
	t.Parallel()

	r, old := signedIn(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := r.mgr.Refresh(ctx, old)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, old, r.mgr.Token())
	assert.Empty(t, r.outs())
}

func TestRefreshWithoutNewRefreshTokenKeepsOld(t *testing.T) {
	t.Parallel()

	r, old := signedIn(t)
	before, _ := r.mgr.Session()
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		if req.RouteID == "auth.refresh" {
			return jsonResp(200, `{"access_token":"only-access","token_type":"Bearer","expires_in":900}`), nil, true
		}

		return nil, nil, false
	}
	tok, err := r.mgr.Refresh(t.Context(), old)
	require.NoError(t, err)
	assert.Equal(t, "only-access", tok)
	after, _ := r.mgr.Session()
	assert.Equal(t, before.RefreshToken, after.RefreshToken)

	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		return jsonResp(200, `{"token_type":"Bearer"}`), nil, req.RouteID == "auth.refresh"
	}
	_, err = r.mgr.Refresh(t.Context(), "only-access")
	require.ErrorIs(t, err, api.ErrServer, "an answer with no token is not a session")
	assert.Equal(t, "only-access", r.mgr.Token())
}

func TestRefreshStaleTokenReturnsCurrent(t *testing.T) {
	t.Parallel()

	r, old := signedIn(t)
	fresh, err := r.mgr.Refresh(t.Context(), old)
	require.NoError(t, err)

	got, err := r.mgr.Refresh(t.Context(), old)
	require.NoError(t, err)
	assert.Equal(t, fresh, got, "someone already refreshed")
	assert.Equal(t, 1, r.spy.count("auth.refresh"), "no second call")
}

// TestRefreshSingleFlight: ten callers that failed with the same token cause
// one request to the server. The server's answer is held until every caller
// has been told 401, so that they really are concurrent.
func TestRefreshSingleFlight(t *testing.T) {
	t.Parallel()

	const callers = 10
	r, old := signedIn(t)
	var refreshes atomic.Int32
	told401 := make(chan struct{}, callers)
	release := make(chan struct{})
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		switch req.RouteID {
		case "me.get":
			if req.Header.Get("Authorization") == "Bearer "+old {
				told401 <- struct{}{}

				return jsonResp(401, `{"error":{"code":"token_expired","message":"x"}}`), nil, true
			}
		case "auth.refresh":
			refreshes.Add(1)
			<-release
		default:
		}

		return nil, nil, false
	}

	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for range callers {
		wg.Go(func() {
			_, err := r.api.Me(context.Background())
			errs <- err
		})
	}
	for range callers {
		<-told401
	}
	close(release)
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}
	assert.EqualValues(t, 1, refreshes.Load(), "one refresh for ten callers")
	assert.NotEqual(t, old, r.mgr.Token())
	assert.Empty(t, r.outs())
}

// TestManagerRefreshSingleFlight drives Manager.Refresh itself with the
// api.Client out of the way: the Manager is single-flight by itself.
func TestManagerRefreshSingleFlight(t *testing.T) {
	t.Parallel()

	const callers = 10
	r, old := signedIn(t)
	var refreshes atomic.Int32
	entered := make(chan struct{}, callers)
	release := make(chan struct{})
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		if req.RouteID == "auth.refresh" {
			refreshes.Add(1)
			entered <- struct{}{}
			<-release
		}

		return nil, nil, false
	}

	toks := make(chan string, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			tok, err := r.mgr.Refresh(context.Background(), old)
			assert.NoError(t, err)
			toks <- tok
		})
	}
	<-entered // the leader is in the server call; the rest wait on it or on its result
	close(release)
	wg.Wait()
	close(toks)

	want := r.mgr.Token()
	for tok := range toks {
		assert.Equal(t, want, tok)
	}
	assert.EqualValues(t, 1, refreshes.Load())
}

func TestRefreshWaiterCanGiveUp(t *testing.T) {
	t.Parallel()

	r, old := signedIn(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		if req.RouteID == "auth.refresh" {
			close(entered)
			<-release
		}

		return nil, nil, false
	}
	leader := make(chan error, 1)
	go func() {
		_, err := r.mgr.Refresh(context.Background(), old)
		leader <- err
	}()
	<-entered

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := r.mgr.Refresh(ctx, old)
	require.ErrorIs(t, err, context.Canceled, "a waiter leaves when its own context ends")

	close(release)
	require.NoError(t, <-leader)
}

// TestTokenDoesNotWaitForTheVault: api.Client calls Token with its lock held,
// so it must answer while a refresh is saving to a slow vault.
func TestTokenDoesNotWaitForTheVault(t *testing.T) {
	t.Parallel()

	r, old := signedIn(t)
	slow := &blockingVault{MemoryVault: r.vault, entered: make(chan struct{}), release: make(chan struct{})}
	m, err := auth.New(auth.Options{Vault: slow})
	require.NoError(t, err)
	m.Bind(r.api)
	require.NoError(t, m.Adopt(session(old, "r1", "u_412")))
	slow.block.Store(true)

	done := make(chan error, 1)
	go func() {
		_, err := m.Refresh(context.Background(), old)
		done <- err
	}()
	<-slow.entered // the refresh is inside the vault's Save
	tok := m.Token()
	assert.NotEqual(t, old, tok, "the new token is already current")
	assert.NotEmpty(t, tok)
	close(slow.release)
	require.NoError(t, <-done)
}

type blockingVault struct {
	*auth.MemoryVault
	block   atomic.Bool
	entered chan struct{}
	release chan struct{}
}

func (v *blockingVault) Save(s model.Session) error {
	if v.block.Load() {
		close(v.entered)
		<-v.release
	}

	return v.MemoryVault.Save(s)
}

// TestLogoutDuringRefreshDoesNotResurrect: a refresh that finishes after the
// sign-out must not write the session back.
func TestLogoutDuringRefreshDoesNotResurrect(t *testing.T) {
	t.Parallel()

	r, old := signedIn(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		if req.RouteID == "auth.refresh" {
			close(entered)
			<-release
		}

		return nil, nil, false
	}
	done := make(chan error, 1)
	go func() {
		_, err := r.mgr.Refresh(context.Background(), old)
		done <- err
	}()
	<-entered
	require.NoError(t, r.mgr.Logout(t.Context()))
	close(release)

	require.ErrorIs(t, <-done, auth.ErrSignedOut)
	assert.Empty(t, r.mgr.Token())
	assert.False(t, r.vault.Has(), "the vault stays empty")
}

// TestMeTokenExpiredScenarioSignsOutAfterOneRefresh: me.get answers 401
// token_expired every time; the client refreshes once, is refused again, and
// the owner signs out through Handle.
func TestMeTokenExpiredScenarioSignsOutAfterOneRefresh(t *testing.T) {
	t.Parallel()

	r, _ := signedIn(t)
	require.NoError(t, r.mock.SetScenario("me.get", "token_expired"))

	_, err := r.api.Me(t.Context())
	require.ErrorIs(t, err, auth.ErrSignedOut)
	assert.Equal(t, 1, r.spy.count("auth.refresh"), "exactly one refresh")
	assert.Equal(t, 2, r.spy.count("me.get"), "the call and its retry")

	// the Manager has a fresh session until the owner acts on the error
	assert.NotEmpty(t, r.mgr.Token())
	require.True(t, r.mgr.Handle(err))
	assert.Empty(t, r.mgr.Token())
	assert.False(t, r.vault.Has())
	require.Len(t, r.outs(), 1)

	_, err = r.api.Me(t.Context())
	require.Error(t, err)
	assert.Equal(t, 1, r.spy.count("auth.refresh"), "signed out: no more refresh")
}

func TestRefreshInvalidThroughClient(t *testing.T) {
	t.Parallel()

	r, _ := signedIn(t)
	require.NoError(t, r.mock.SetScenario("me.get", "token_expired"))
	require.NoError(t, r.mock.SetScenario("auth.refresh", "refresh_invalid"))

	_, err := r.api.Me(t.Context())
	require.ErrorIs(t, err, auth.ErrSignedOut)
	assert.Empty(t, r.mgr.Token(), "signed out by the Manager itself")
	assert.False(t, r.vault.Has())
	assert.Len(t, r.outs(), 1)
}

func TestSignOutNilReason(t *testing.T) {
	t.Parallel()

	r, _ := signedIn(t)
	r.mgr.SignOut(nil)
	outs := r.outs()
	require.Len(t, outs, 1)
	assert.ErrorIs(t, outs[0], auth.ErrSignedOut)
	r.mgr.SignOut(nil)
	assert.Len(t, r.outs(), 1)
}

// TestLogoutIsNotAnExpiry runs the user's Logout while the server answers
// 401 token_expired to the logout call itself and refuses the refresh: the
// session ends silently (no OnSignedOut), for a phone and for a TV.
func TestLogoutIsNotAnExpiry(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		tv    bool
		route string
	}{{"phone", false, "auth.logout"}, {"tv", true, "tv.logout"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, tc.tv)
			_, err := r.mgr.Login(t.Context(), "a@b.c", "x")
			require.NoError(t, err)
			r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
				switch req.RouteID {
				case tc.route:
					return jsonResp(401, `{"error":{"code":"token_expired","message":"x"}}`), nil, true
				case "auth.refresh":
					return jsonResp(401, `{"error":{"code":"refresh_invalid","message":"x"}}`), nil, true
				}

				return nil, nil, false
			}

			require.NoError(t, r.mgr.Logout(t.Context()))
			_, ok := r.mgr.Session()
			assert.False(t, ok)
			assert.Empty(t, r.outs(), "a user's logout never tells OnSignedOut")

			// a late 401 of an old request finds nobody signed in
			assert.True(t, r.mgr.Handle(auth.ErrSignedOut))
			assert.Empty(t, r.outs())
		})
	}
}

// TestLogoutDropsLateExpiry holds a request's refresh refusal until the
// Logout is under way: the refusal is dropped, the logout completes.
func TestLogoutDropsLateExpiry(t *testing.T) {
	t.Parallel()

	r, _ := signedIn(t)
	inLogout, release := make(chan struct{}), make(chan struct{})
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		if req.RouteID == "auth.logout" {
			close(inLogout)
			<-release
		}

		return nil, nil, false
	}
	done := make(chan error, 1)
	go func() { done <- r.mgr.Logout(context.Background()) }()
	<-inLogout
	// an in-flight request is refused its refresh while the logout call is out
	r.mgr.SignOut(auth.ErrSignedOut)
	close(release)
	require.NoError(t, <-done)
	assert.Empty(t, r.outs())
	_, ok := r.mgr.Session()
	assert.False(t, ok)
}
