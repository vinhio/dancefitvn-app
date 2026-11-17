package auth_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/store"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The answers a biometric sheet gives, as internal/sys makes them (auth
// does not import sys): each wraps what sys's sentinel wraps.
var (
	errSheetCanceled = fmt.Errorf("sheet: canceled: %w", context.Canceled)
	errSheetLockout  = fmt.Errorf("sheet: locked out: %w", core.ErrInvalid)
	errSheetFailed   = fmt.Errorf("sheet: not recognized: %w", core.ErrInvalid)
	errSheetNone     = fmt.Errorf("sheet: not enrolled: %w", core.ErrUnsupported)
)

// bioRig is a rig whose Manager keeps its session in a BioVault: main is
// rig.vault, slot the copy.
type bioRig struct {
	*rig
	slot *auth.MemoryVault
	bio  *auth.BioVault
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func TestBioVaultKeepsTheCopyThroughLogout(t *testing.T) {
	t.Parallel()

	r := newBioRig(t, nil)
	assert.False(t, r.mgr.HasBio(), "nothing kept before a login")
	s, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err)

	kept, err := r.bio.LoadBio()
	require.NoError(t, err)
	assert.Equal(t, s.RefreshToken, kept.RefreshToken)
	assert.Equal(t, s.User, kept.User)
	assert.Equal(t, s.Device, kept.Device)
	assert.Empty(t, kept.AccessToken, "the copy never holds the access token")
	assert.True(t, r.mgr.HasBio())

	require.NoError(t, r.mgr.Logout(t.Context()))
	assert.False(t, r.vault.Has(), "logout clears the session")
	assert.True(t, r.slot.Has(), "logout keeps the copy (K13)")
	assert.True(t, r.mgr.HasBio())
}

func TestBioVaultFollowsRotation(t *testing.T) {
	t.Parallel()

	r := newBioRig(t, nil)
	s, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err)
	_, err = r.mgr.Refresh(t.Context(), s.AccessToken)
	require.NoError(t, err)

	kept, err := r.bio.LoadBio()
	require.NoError(t, err)
	assert.Equal(t, "rt_mock_9b1d7e", kept.RefreshToken, "the copy holds the rotated refresh token")
}

func TestBioVaultSave(t *testing.T) {
	t.Parallel()

	slotErr := errors.New("slot full")
	for _, tc := range []struct {
		name     string
		keep     func() bool
		prior    bool  // a copy of another user is kept before
		slotSave error // the slot's Save fails
		wantErr  error
		wantUser string // "" for no copy
	}{
		{name: "another user overwrites", prior: true, wantUser: "u_new"},
		{name: "keep nil keeps", wantUser: "u_new"},
		{name: "keep yes keeps", keep: func() bool { return true }, wantUser: "u_new"},
		{name: "keep no clears", keep: func() bool { return false }, prior: true},
		{name: "slot failure reported", slotSave: slotErr, wantErr: slotErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			main, slot := auth.NewMemoryVault(), auth.NewMemoryVault()
			if tc.prior {
				slot.Put(session("", "rt-old", "u_old"))
			}
			slot.SaveErr = tc.slotSave
			v := auth.NewBioVault(main, slot, tc.keep)

			err := v.Save(session("at-new", "rt-new", "u_new"))
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				assert.True(t, main.Has(), "the session is saved even when the copy is not")
				assert.False(t, v.HasBio())

				return
			}
			require.NoError(t, err)
			got, err := main.Load()
			require.NoError(t, err)
			assert.Equal(t, "at-new", got.AccessToken)
			kept, err := v.LoadBio()
			if tc.wantUser == "" {
				require.ErrorIs(t, err, core.ErrNotFound)
				assert.False(t, v.HasBio())

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantUser, kept.User.ID)
			assert.Equal(t, "rt-new", kept.RefreshToken)
			assert.True(t, v.HasBio())
		})
	}
}

func TestBioVaultClearBio(t *testing.T) {
	t.Parallel()

	main, slot := auth.NewMemoryVault(), auth.NewMemoryVault()
	v := auth.NewBioVault(main, slot, nil)
	require.NoError(t, v.Save(session("at", "rt", "u")))
	require.NoError(t, v.Clear())
	assert.True(t, v.HasBio())
	require.NoError(t, v.ClearBio())
	assert.False(t, v.HasBio())
	_, err := v.LoadBio()
	require.ErrorIs(t, err, core.ErrNotFound)
	assert.False(t, main.Has())
}

func TestBioVaultHasBioReadsTheSlotOnce(t *testing.T) {
	t.Parallel()

	slot := auth.NewMemoryVault()
	slot.Put(session("", "rt", "u"))
	v := auth.NewBioVault(auth.NewMemoryVault(), slot, nil)
	assert.True(t, v.HasBio(), "a copy of an earlier launch counts")

	empty := auth.NewMemoryVault()
	empty.Put(session("", "", "u"))
	assert.False(t, auth.NewBioVault(auth.NewMemoryVault(), empty, nil).HasBio(), "a copy without refresh token is none")
}

func TestBioLogin(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		sheet     error  // what the check answers
		scenario  string // auth.refresh's mock scenario, "" for default
		offline   bool   // auth.refresh fails on the network
		noCopy    bool   // nothing kept
		want      []error
		signed    bool // signed in after
		keepsCopy bool
		refreshes int
	}{
		{name: "success", signed: true, keepsCopy: true, refreshes: 1},
		{name: "cancel", sheet: errSheetCanceled, want: []error{context.Canceled, errSheetCanceled}, keepsCopy: true},
		{name: "lockout", sheet: errSheetLockout, want: []error{core.ErrInvalid, errSheetLockout}, keepsCopy: true},
		{name: "not recognized", sheet: errSheetFailed, want: []error{errSheetFailed}, keepsCopy: true},
		{name: "not enrolled", sheet: errSheetNone, want: []error{core.ErrUnsupported}, keepsCopy: true},
		{name: "revoked refresh", scenario: "refresh_invalid", want: []error{auth.ErrBioExpired, auth.ErrSignedOut, api.ErrUnauthorized}, refreshes: 1},
		{name: "offline", offline: true, want: []error{api.ErrOffline}, keepsCopy: true, refreshes: 1},
		{name: "no copy", noCopy: true, want: []error{auth.ErrNoBio, core.ErrNotFound}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newBioRig(t, nil)
			if !tc.noCopy {
				signInAndOut(t, r)
			}
			if tc.scenario != "" {
				require.NoError(t, r.mock.SetScenario("auth.refresh", tc.scenario))
			}
			if tc.offline {
				r.spy.hook = func(req *api.Request) (*api.Response, error, bool) {
					if req.RouteID == "auth.refresh" {
						return nil, netErr, true
					}

					return nil, nil, false
				}
			}
			var checks atomic.Int32
			s, err := r.mgr.BioLogin(t.Context(), func(context.Context) error {
				checks.Add(1)

				return tc.sheet
			})
			for _, w := range tc.want {
				require.ErrorIs(t, err, w)
			}
			if len(tc.want) == 0 {
				require.NoError(t, err)
				assert.NotEmpty(t, s.AccessToken)
				assert.Equal(t, "u_412", s.User.ID, "the user of the copy")
			}
			_, signed := r.mgr.Session()
			assert.Equal(t, tc.signed, signed)
			assert.Equal(t, tc.signed, r.vault.Has(), "the session is saved when signed in")
			assert.Equal(t, tc.keepsCopy, r.mgr.HasBio())
			assert.Equal(t, tc.refreshes, r.spy.count("auth.refresh"))
			if tc.noCopy {
				assert.Zero(t, checks.Load(), "no sheet without a copy")
			}
			assert.Empty(t, r.outs(), "a biometric sign-in never reports a sign-out")
		})
	}
}

func TestBioLoginRotatesTheCopy(t *testing.T) {
	t.Parallel()

	r := newBioRig(t, nil)
	signInAndOut(t, r)
	s, err := r.mgr.BioLogin(t.Context(), func(context.Context) error { return nil })
	require.NoError(t, err)
	kept, err := r.bio.LoadBio()
	require.NoError(t, err)
	assert.Equal(t, s.RefreshToken, kept.RefreshToken)
	assert.Equal(t, "rt_mock_9b1d7e", kept.RefreshToken)
	assert.Equal(t, s.AccessToken, r.mgr.Token())
}

func TestBioLoginRefused(t *testing.T) {
	t.Parallel()

	t.Run("no bio slot", func(t *testing.T) {
		t.Parallel()

		r := newRig(t, false)
		_, err := r.mgr.BioLogin(t.Context(), func(context.Context) error { return nil })
		require.ErrorIs(t, err, auth.ErrNoBio)
		assert.False(t, r.mgr.HasBio())
	})
	t.Run("no check", func(t *testing.T) {
		t.Parallel()

		r := newBioRig(t, nil)
		signInAndOut(t, r)
		_, err := r.mgr.BioLogin(t.Context(), nil)
		require.ErrorIs(t, err, core.ErrInvalid)
	})
	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		r := newBioRig(t, nil)
		signInAndOut(t, r)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := r.mgr.BioLogin(ctx, func(ctx context.Context) error { return ctx.Err() })
		require.ErrorIs(t, err, context.Canceled)
		assert.True(t, r.mgr.HasBio())
	})
}

func TestForcedSignOutClearsTheCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		end  func(t *testing.T, r *bioRig, s model.Session)
		keep bool
	}{
		{name: "refresh refused", end: func(t *testing.T, r *bioRig, s model.Session) {
			t.Helper()
			require.NoError(t, r.mock.SetScenario("auth.refresh", "refresh_invalid"))
			_, err := r.mgr.Refresh(t.Context(), s.AccessToken)
			require.ErrorIs(t, err, auth.ErrSignedOut)
		}},
		{name: "sign out", end: func(_ *testing.T, r *bioRig, _ model.Session) { r.mgr.SignOut(nil) }},
		{name: "handle", end: func(t *testing.T, r *bioRig, _ model.Session) {
			t.Helper()
			assert.True(t, r.mgr.Handle(fmt.Errorf("call: %w", auth.ErrSignedOut)))
		}},
		{name: "logout keeps it", keep: true, end: func(t *testing.T, r *bioRig, _ model.Session) {
			t.Helper()
			require.NoError(t, r.mgr.Logout(t.Context()))
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newBioRig(t, nil)
			s, err := r.mgr.Login(t.Context(), "a@b.c", "x")
			require.NoError(t, err)
			require.True(t, r.slot.Has())
			tc.end(t, r, s)
			assert.False(t, r.vault.Has(), "the session is gone")
			assert.Equal(t, tc.keep, r.slot.Has())
			assert.Equal(t, tc.keep, r.mgr.HasBio())
			assert.Len(t, r.outs(), map[bool]int{true: 0, false: 1}[tc.keep])
		})
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// newBioRig is newRig with a BioVault: rig.vault is its main Vault.
func newBioRig(t *testing.T, keep func() bool) *bioRig {
	t.Helper()
	m, err := mock.New(os.DirFS(mockDir), mock.Options{
		Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close() })

	cache := store.OpenCache(filepath.Join(t.TempDir(), "api"), 0)
	t.Cleanup(cache.Close)
	r := &bioRig{
		rig:  &rig{mock: m, spy: &spy{inner: m}, vault: auth.NewMemoryVault(), cache: cache},
		slot: auth.NewMemoryVault(),
	}
	r.bio = auth.NewBioVault(r.vault, r.slot, keep)
	r.mgr, err = auth.New(auth.Options{
		Vault:      r.bio,
		ClearCache: cache.Clear,
		OnSignedOut: func(reason error) {
			r.mu.Lock()
			r.signedOut = append(r.signedOut, reason)
			r.mu.Unlock()
		},
	})
	require.NoError(t, err)
	r.api, err = api.New(api.Options{
		Transport: r.spy, Version: "1.0.0", Platform: "ios", InstallID: "install-1",
		Tokens: r.mgr, Cache: cache,
	})
	require.NoError(t, err)
	r.mgr.Bind(r.api)

	return r
}

// signInAndOut logs in with the password, then out: the copy stays.
func signInAndOut(t *testing.T, r *bioRig) model.Session {
	t.Helper()
	s, err := r.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err)
	require.NoError(t, r.mgr.Logout(t.Context()))
	require.True(t, r.mgr.HasBio())
	require.Zero(t, r.spy.count("auth.refresh"))

	return s
}
