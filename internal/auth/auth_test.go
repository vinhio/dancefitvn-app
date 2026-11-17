package auth_test

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/auth"
)

func TestNewRequiresVault(t *testing.T) {
	t.Parallel()

	_, err := auth.New(auth.Options{})
	assert.ErrorIs(t, err, core.ErrInvalid)
}

func TestErrSignedOutIsTheAPIOne(t *testing.T) {
	t.Parallel()

	assert.Same(t, api.ErrSignedOut, auth.ErrSignedOut)
}

// TestLoginScenarios runs every scenario of auth.login of routes.json.
func TestLoginScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		scenario string
		status   int
		code     string
		sentinel error
		devices  int
	}{
		{scenario: "default"},
		{scenario: "invalid_credentials", status: 401, code: "invalid_credentials", sentinel: api.ErrUnauthorized},
		{scenario: "account_locked", status: 423, code: "account_locked", sentinel: api.ErrLocked},
		{scenario: "device_limit", status: 409, code: "device_limit", sentinel: api.ErrConflict, devices: 3},
	}
	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, false)
			require.NoError(t, r.mock.SetScenario("auth.login", tt.scenario))

			s, err := r.mgr.Login(t.Context(), "thaovy.hlv@gmail.com", "pw-not-logged")
			if tt.status == 0 {
				require.NoError(t, err)
				assert.Equal(t, "u_412", s.User.ID)
				assert.NotEmpty(t, r.mgr.Token())
				assert.Equal(t, s.AccessToken, r.mgr.Token())
				got, ok := r.mgr.Session()
				require.True(t, ok)
				assert.Equal(t, "u_412", got.User.ID)
				assert.True(t, r.vault.Has(), "saved")
				saved, lerr := r.vault.Load()
				require.NoError(t, lerr)
				assert.Equal(t, s.RefreshToken, saved.RefreshToken)
				assert.Equal(t, 1, r.clears(), "a new user starts on an empty cache")
				assert.Empty(t, r.outs())

				return
			}
			require.Error(t, err)
			var ae *api.Error
			require.ErrorAs(t, err, &ae)
			assert.Equal(t, tt.status, ae.Status)
			assert.Equal(t, tt.code, ae.Code)
			require.ErrorIs(t, err, tt.sentinel)
			assert.NotErrorIs(t, err, auth.ErrSignedOut, "a refused login is not a signed-out session")
			assert.Len(t, ae.Body.Devices, tt.devices)
			assert.Empty(t, r.mgr.Token())
			_, ok := r.mgr.Session()
			assert.False(t, ok)
			assert.False(t, r.vault.Has())
			assert.Zero(t, r.clears())
			assert.Empty(t, r.outs(), "not a sign-out")
			assert.Zero(t, r.spy.count("auth.refresh"), "a login failure never refreshes")
		})
	}
}

// TestLoginReplacing checks K31: the 409 device_limit lists the devices; the login sent again with
// one of them as replace_device_id signs in and keeps the session; the first body has no such field,
// the second has it with the same email and password; a refused one keeps nothing.
func TestLoginReplacing(t *testing.T) {
	t.Parallel()

	r := newRig(t, false)
	var bodies []string
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		if req.RouteID == "auth.login" {
			bodies = append(bodies, string(req.Body))
		}

		return nil, nil, false
	}
	require.NoError(t, r.mock.SetScenario("auth.login", "device_limit"))
	_, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	var ae *api.Error
	require.ErrorAs(t, err, &ae)
	require.Len(t, ae.Body.Devices, 3)
	assert.False(t, r.vault.Has())

	s, err := r.mgr.LoginReplacing(t.Context(), "a@b.c", "x", ae.Body.Devices[1].ID)
	require.NoError(t, err)
	assert.NotEmpty(t, s.AccessToken)
	assert.True(t, r.vault.Has(), "the session is kept")
	got, ok := r.mgr.Session()
	require.True(t, ok)
	assert.Equal(t, s.User.ID, got.User.ID)

	require.Len(t, bodies, 2)
	assert.NotContains(t, bodies[0], "replace_device_id")
	assert.Contains(t, bodies[1], `"replace_device_id":"d_102"`)
	assert.Contains(t, bodies[1], `"email":"a@b.c"`)
	assert.Contains(t, bodies[1], `"password":"x"`)
}

// TestLoginReplacingRefused checks the 400 on the field: the server's error with its fields comes
// back, nothing is kept and the cache is not cleared.
func TestLoginReplacingRefused(t *testing.T) {
	t.Parallel()

	r := newRig(t, false)
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		if req.RouteID != "auth.login" {
			return nil, nil, false
		}

		return &api.Response{Status: 400, Header: http.Header{}, Body: []byte(
			`{"error":{"code":"validation_failed","message":"m","fields":{"replace_device_id":"x"}}}`)}, nil, true
	}
	_, err := r.mgr.LoginReplacing(t.Context(), "a@b.c", "x", "d_nope")
	var ae *api.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 400, ae.Status)
	assert.Equal(t, "validation_failed", ae.Code)
	assert.Contains(t, ae.Body.Fields, "replace_device_id")
	assert.False(t, r.vault.Has())
	_, ok := r.mgr.Session()
	assert.False(t, ok)
}

func TestLoginOffline(t *testing.T) {
	t.Parallel()

	r := newRig(t, false)
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) { return nil, netErr, true }
	_, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	require.ErrorIs(t, err, api.ErrOffline)
	assert.False(t, r.vault.Has())
}

func TestLoginSendsDevice(t *testing.T) {
	t.Parallel()

	r := newRig(t, false)
	var body string
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		if req.RouteID == "auth.login" {
			body = string(req.Body)
		}

		return nil, nil, false
	}
	_, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err)
	assert.Contains(t, body, `"platform":"ios"`)
	assert.Contains(t, body, `"install_id":"install-1"`)
}

func TestLoginWithoutRemote(t *testing.T) {
	t.Parallel()

	m, err := auth.New(auth.Options{Vault: auth.NewMemoryVault()})
	require.NoError(t, err)
	_, err = m.Login(t.Context(), "a@b.c", "x")
	assert.ErrorIs(t, err, core.ErrInvalid)
	_, err = m.Refresh(t.Context(), "x")
	assert.ErrorIs(t, err, auth.ErrSignedOut)
}

func TestLoginVaultFailureKeepsSession(t *testing.T) {
	t.Parallel()

	r := newRig(t, false)
	r.vault.SaveErr = assert.AnError
	_, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err, "the server accepted the login")
	assert.NotEmpty(t, r.mgr.Token())
	assert.Equal(t, []string{"save"}, r.vaultOps())
}

func TestSecondLoginSameUserKeepsCache(t *testing.T) {
	t.Parallel()

	r := newRig(t, false)
	_, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err)
	_, err = r.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err)
	assert.Equal(t, 1, r.clears(), "same user: cache kept")
}

func TestAccountSwitchClearsCache(t *testing.T) {
	t.Parallel()

	r := newRig(t, false)
	_, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err)
	require.Equal(t, 1, r.clears())

	require.NoError(t, r.mgr.Adopt(session("acc2", "ref2", "u_other")))
	assert.Equal(t, 2, r.clears(), "another user: cache cleared")
	assert.Equal(t, "acc2", r.mgr.Token())
}

// TestRestore covers what the vault may hold at launch.
func TestRestore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		setup  func(v *auth.MemoryVault)
		ok     bool
		errOps []string
	}{
		{name: "empty", setup: func(*auth.MemoryVault) {}},
		{name: "saved", setup: func(v *auth.MemoryVault) { v.Put(session("a1", "r1", "u1")) }, ok: true},
		{name: "no refresh token", setup: func(v *auth.MemoryVault) { v.Put(session("a1", "", "u1")) }},
		{name: "unreadable", setup: func(v *auth.MemoryVault) {
			v.Put(session("a1", "r1", "u1"))
			v.LoadErr = core.ErrInvalid
		}, errOps: []string{"load"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, false)
			tt.setup(r.vault)
			s, ok := r.mgr.Restore()
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, "u1", s.User.ID)
				assert.Equal(t, "a1", r.mgr.Token())
			} else {
				assert.Empty(t, r.mgr.Token())
			}
			assert.Equal(t, tt.ok, r.vault.Has(), "what cannot be used is cleared")
			assert.Equal(t, tt.errOps, r.vaultOps())
			assert.Empty(t, r.outs(), "Restore is not a sign-out")
		})
	}
}

func TestRestoreThenRequestUsesToken(t *testing.T) {
	t.Parallel()

	r := newRig(t, false)
	r.vault.Put(session("restored-token", "r1", "u_412"))
	_, ok := r.mgr.Restore()
	require.True(t, ok)
	var got string
	r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
		got = req.Header.Get("Authorization")

		return nil, nil, false
	}
	_, err := r.api.Me(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "Bearer restored-token", got)
}

func TestAdoptValidates(t *testing.T) {
	t.Parallel()

	r := newRig(t, true)
	assert.ErrorIs(t, r.mgr.Adopt(session("", "r", "u")), core.ErrInvalid)
	assert.ErrorIs(t, r.mgr.Adopt(session("a", "", "u")), core.ErrInvalid)
	assert.Empty(t, r.mgr.Token())
	require.NoError(t, r.mgr.Adopt(session("a", "r", "u")))
	assert.Equal(t, "a", r.mgr.Token())
	assert.True(t, r.vault.Has())
}

// TestLogoutScenarios runs every scenario of auth.logout, and the TV's
// tv.logout, and the cases a server cannot answer.
func TestLogoutScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		tv        bool
		offline   bool
		scenario  string
		serverHit string
	}{
		{name: "default", serverHit: "auth.logout"},
		{name: "offline is ignored", offline: true, serverHit: "auth.logout"},
		{name: "tv", tv: true, serverHit: "tv.logout"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, tt.tv)
			_, err := r.mgr.Login(t.Context(), "a@b.c", "x")
			require.NoError(t, err)
			clears := r.clears()
			if tt.offline {
				r.spy.hook = func(*api.Request) (*api.Response, error, bool) { return nil, netErr, true }
			}

			require.NoError(t, r.mgr.Logout(t.Context()))

			assert.Equal(t, 1, r.spy.count(tt.serverHit))
			assert.Empty(t, r.mgr.Token())
			_, ok := r.mgr.Session()
			assert.False(t, ok)
			assert.False(t, r.vault.Has(), "vault cleared")
			assert.Equal(t, clears+1, r.clears(), "cache cleared")
			assert.Empty(t, r.outs(), "the user asked: no OnSignedOut")
		})
	}
}

func TestLogoutWithoutSessionCallsNoServer(t *testing.T) {
	t.Parallel()

	r := newRig(t, false)
	r.vault.Put(session("stray", "r", "u"))
	require.NoError(t, r.mgr.Logout(t.Context()))
	assert.Zero(t, r.spy.count("auth.logout"))
	assert.False(t, r.vault.Has(), "a stray vault entry goes too")
}

func TestLogoutKeepsDownloadsDir(t *testing.T) {
	t.Parallel()

	downloads := filepath.Join(t.TempDir(), "audio")
	require.NoError(t, os.MkdirAll(downloads, 0o700))
	f := filepath.Join(downloads, "l_3503.mp3")
	require.NoError(t, os.WriteFile(f, []byte("mp3"), 0o600))

	r := newRig(t, false)
	_, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err)
	require.NoError(t, r.mgr.Logout(t.Context()))
	assert.FileExists(t, f)
}

func TestLogoutClearsCache(t *testing.T) {
	t.Parallel()

	r := newRig(t, false)
	ctx := t.Context()
	_, err := r.mgr.Login(ctx, "a@b.c", "x")
	require.NoError(t, err)
	_, err = r.api.Me(ctx)
	require.NoError(t, err)
	_, _, hit := r.cache.Get("GET /me?")
	require.True(t, hit, "the answer of me.get is cached")

	require.NoError(t, r.mgr.Logout(ctx))
	_, _, hit = r.cache.Get("GET /me?")
	assert.False(t, hit, "no answer of the user outlives the session")
}

func TestLogoutVaultFailureIsReported(t *testing.T) {
	t.Parallel()

	r := newRig(t, false)
	_, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err)
	r.vault.ClearErr = assert.AnError
	err = r.mgr.Logout(t.Context())
	require.ErrorIs(t, err, assert.AnError)
	assert.Empty(t, r.mgr.Token(), "memory is signed out even so")
	assert.Equal(t, []string{"clear"}, r.vaultOps())
}

func TestHandle(t *testing.T) {
	t.Parallel()

	r := newRig(t, false)
	_, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err)

	assert.False(t, r.mgr.Handle(nil))
	assert.False(t, r.mgr.Handle(errors.New("other")))
	assert.False(t, r.mgr.Handle(api.ErrOffline))
	assert.NotEmpty(t, r.mgr.Token())

	assert.True(t, r.mgr.Handle(&wrapped{api.ErrSignedOut}))
	assert.True(t, r.mgr.Handle(api.ErrSignedOut), "second call: idempotent")
	assert.Empty(t, r.mgr.Token())
	assert.False(t, r.vault.Has())
	outs := r.outs()
	require.Len(t, outs, 1, "OnSignedOut once per session")
	assert.ErrorIs(t, outs[0], auth.ErrSignedOut)
}

type wrapped struct{ err error }

func (w *wrapped) Error() string { return "wrapped: " + w.err.Error() }
func (w *wrapped) Unwrap() error { return w.err }

// TestTokenDoesNotAllocate is not parallel: AllocsPerRun forbids it.
func TestTokenDoesNotAllocate(t *testing.T) {
	m, err := auth.New(auth.Options{Vault: auth.NewMemoryVault()})
	require.NoError(t, err)
	assert.Empty(t, m.Token())
	assert.Zero(t, testing.AllocsPerRun(100, func() { _ = m.Token() }))
}
