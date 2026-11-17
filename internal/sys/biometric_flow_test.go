package sys_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/sys"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mockDir is the API's mock, the contract (05).
const mockDir = "../../mock/dancefitvn-mock-api"

// bioApp is the biometric sign-in as the app wires it (P3.1): a Manager
// over a BioVault whose keep is the device's biometry, the api.Client on
// the mock, and the fake sheet.
type bioApp struct {
	mock  *mock.Transport
	mgr   *auth.Manager
	main  *auth.MemoryVault
	sheet *sys.FakeBiometric
	outs  int
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestBioLoginFlow runs 03 §3 bioLogin over the fake sheet: password
// login, logout, then the sheet's answers.
func TestBioLoginFlow(t *testing.T) {
	t.Parallel()

	prompt := sys.BioPrompt{Reason: "reason", Cancel: "cancel"}
	for _, tc := range []struct {
		name     string
		answer   error  // the sheet's
		scenario string // auth.refresh's
		want     []error
		signed   bool
		hasBio   bool
	}{
		{name: "success", signed: true, hasBio: true},
		{name: "cancel", answer: sys.ErrBioCanceled, want: []error{context.Canceled}, hasBio: true},
		{name: "lockout", answer: sys.ErrBioLockout, want: []error{sys.ErrBioLockout, core.ErrInvalid}, hasBio: true},
		{name: "not recognized", answer: sys.ErrBioFailed, want: []error{sys.ErrBioFailed}, hasBio: true},
		{name: "not enrolled", answer: sys.ErrBioUnavailable, want: []error{core.ErrUnsupported}, hasBio: true},
		{name: "revoked refresh", scenario: "refresh_invalid", want: []error{auth.ErrBioExpired, auth.ErrSignedOut}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a := newBioApp(t, sys.BiometryFace)
			_, err := a.mgr.Login(t.Context(), "a@b.c", "x")
			require.NoError(t, err)
			require.NoError(t, a.mgr.Logout(t.Context()))
			require.True(t, a.mgr.HasBio() && a.sheet.Available(), "the login screen offers Face ID")

			if tc.scenario != "" {
				require.NoError(t, a.mock.SetScenario("auth.refresh", tc.scenario))
			}
			a.sheet.Answer(tc.answer)
			_, err = a.mgr.BioLogin(t.Context(), func(ctx context.Context) error {
				return a.sheet.Authenticate(ctx, prompt)
			})
			for _, w := range tc.want {
				require.ErrorIs(t, err, w)
			}
			if len(tc.want) == 0 {
				require.NoError(t, err)
			}
			assert.Equal(t, 1, a.sheet.Calls())
			assert.Equal(t, prompt, a.sheet.Prompt())
			_, signed := a.mgr.Session()
			assert.Equal(t, tc.signed, signed)
			assert.Equal(t, tc.hasBio, a.mgr.HasBio())
			assert.Equal(t, tc.signed, a.main.Has())
			assert.Zero(t, a.outs)
		})
	}
}

// TestBioCopyNeedsBiometry checks a device without biometry keeps no
// copy: the password login clears it.
func TestBioCopyNeedsBiometry(t *testing.T) {
	t.Parallel()

	a := newBioApp(t, sys.BiometryNone)
	_, err := a.mgr.Login(t.Context(), "a@b.c", "x")
	require.NoError(t, err)
	assert.False(t, a.mgr.HasBio())
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

func newBioApp(t *testing.T, kind sys.BiometryKind) *bioApp {
	t.Helper()
	m, err := mock.New(os.DirFS(mockDir), mock.Options{
		Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close() })
	cache := store.OpenCache(filepath.Join(t.TempDir(), "api"), 0)
	t.Cleanup(cache.Close)

	a := &bioApp{mock: m, main: auth.NewMemoryVault(), sheet: sys.NewFakeBiometric(kind)}
	a.mgr, err = auth.New(auth.Options{
		Vault:       auth.NewBioVault(a.main, auth.NewMemoryVault(), a.sheet.Available),
		ClearCache:  cache.Clear,
		OnSignedOut: func(error) { a.outs++ },
	})
	require.NoError(t, err)
	c, err := api.New(api.Options{
		Transport: m, Version: "1.0.0", Platform: "ios", InstallID: "install-1", Tokens: a.mgr, Cache: cache,
	})
	require.NoError(t, err)
	a.mgr.Bind(c)

	return a
}
