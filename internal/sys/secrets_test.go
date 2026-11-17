package sys

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gdeskdev/desk/bridge/system/secret"
	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/platform/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// testAccount and testValue are made-up values: no real token here.
const (
	testAccount = "00000000-0000-4000-8000-000000000000"
	testValue   = "test-value-not-a-token"
)

// noSecretsDriver is a driver without a keychain, as gDesk's uikit and
// android drivers are until G1.
type noSecretsDriver struct{ platform.UnimplementedDriver }

// nopHost is an AppHost that does nothing, for starting a fake driver.
type nopHost struct{}

func (nopHost) OnLaunched()                         {}
func (nopHost) OnScheduled()                        {}
func (nopHost) OnQuitRequest() bool                 { return true }
func (nopHost) OnLifecycle(platform.LifecycleState) {}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func TestSecretsRoundTripFallback(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "secret")
	d := noSecretsDriver{}
	s, err := openSecrets(secret.NewStore(d, AppID), keepsSecrets(d), nil, dir)
	require.NoError(t, err)
	assert.Empty(t, s.Damaged())
	assert.Equal(t, dir, s.Dir())

	_, err = s.Get(testAccount)
	require.ErrorIs(t, err, core.ErrNotFound)
	require.NoError(t, s.Set(testAccount, []byte(testValue)))
	got, err := s.Get(testAccount)
	require.NoError(t, err)
	assert.Equal(t, testValue, string(got))

	// Sealed on disk: the value is not in the clear.
	raw, err := os.ReadFile(filepath.Join(dir, secretsFile))
	require.NoError(t, err)
	assert.False(t, bytes.Contains(raw, []byte(testValue)))
	for _, name := range []string{secretsFile, secretsKey} {
		info, err := os.Stat(filepath.Join(dir, name))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), name)
	}

	// Another open, as after a restart, reads it back.
	again, err := openSecrets(secret.NewStore(d, AppID), false, nil, dir)
	require.NoError(t, err)
	got, err = again.Get(testAccount)
	require.NoError(t, err)
	assert.Equal(t, testValue, string(got))

	require.NoError(t, again.Delete(testAccount))
	require.NoError(t, again.Delete(testAccount), "deleting nothing is no error")
	_, err = again.Get(testAccount)
	require.ErrorIs(t, err, core.ErrNotFound)
}

func TestSecretsKeychainGoesThroughInvoke(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "secret")
	primary := secret.NewMemoryStore()
	var calls atomic.Int32
	invoke := func(fn func()) error {
		calls.Add(1)
		fn()

		return nil
	}
	s, err := openSecrets(primary, true, invoke, dir)
	require.NoError(t, err)

	require.NoError(t, s.Set(testAccount, []byte(testValue)))
	got, err := s.Get(testAccount)
	require.NoError(t, err)
	assert.Equal(t, testValue, string(got))
	inPrimary, err := primary.Get(testAccount)
	require.NoError(t, err)
	assert.Equal(t, testValue, string(inPrimary))
	_, err = os.Stat(filepath.Join(dir, secretsFile))
	require.ErrorIs(t, err, os.ErrNotExist, "the keychain holds it: nothing in the file")
	require.NoError(t, s.Delete(testAccount))
	assert.Equal(t, int32(3), calls.Load())
}

func TestSecretsReadsFileValuesAfterKeychainArrives(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "secret")
	before, err := openSecrets(secret.NewStore(noSecretsDriver{}, AppID), false, nil, dir)
	require.NoError(t, err)
	require.NoError(t, before.Set(testAccount, []byte(testValue)))

	// G1 lands: the keychain is empty, the F1 value still answers.
	after, err := openSecrets(secret.NewMemoryStore(), true, func(fn func()) error { fn(); return nil }, dir)
	require.NoError(t, err)
	got, err := after.Get(testAccount)
	require.NoError(t, err)
	assert.Equal(t, testValue, string(got))
}

func TestSecretsWithFakeDriverKeychain(t *testing.T) {
	t.Parallel()

	d := fake.New()
	invoke, stop := uiThread(t, d)
	defer stop()

	// Off the UI thread the keychain refuses, which is why invoke exists.
	require.ErrorIs(t, secret.Set(d, AppID, testAccount, []byte(testValue)), core.ErrInvalid)

	dir := filepath.Join(t.TempDir(), "secret")
	s, err := openSecrets(secret.NewStore(d, AppID), keepsSecrets(d), invoke, dir)
	require.NoError(t, err)
	require.NoError(t, s.Set(testAccount, []byte(testValue)))
	got, err := s.Get(testAccount)
	require.NoError(t, err)
	assert.Equal(t, testValue, string(got))
	inKeychain, err := d.Secret(AppID, testAccount)
	require.NoError(t, err)
	assert.Equal(t, testValue, string(inKeychain))
	require.NoError(t, s.Delete(testAccount))
	_, err = d.Secret(AppID, testAccount)
	require.ErrorIs(t, err, core.ErrNotFound)
}

func TestSecretsInvokeErrorIsReturned(t *testing.T) {
	t.Parallel()

	closed := func(func()) error { return core.ErrClosed }
	s, err := openSecrets(secret.NewMemoryStore(), true, closed, filepath.Join(t.TempDir(), "secret"))
	require.NoError(t, err)
	require.ErrorIs(t, s.Set(testAccount, []byte(testValue)), core.ErrClosed)
	_, err = s.Get(testAccount)
	require.ErrorIs(t, err, core.ErrClosed)
	require.ErrorIs(t, s.Delete(testAccount), core.ErrClosed)
}

func TestOpenSecretsNeedsInvokeForKeychain(t *testing.T) {
	t.Parallel()

	_, err := openSecrets(secret.NewMemoryStore(), true, nil, filepath.Join(t.TempDir(), "secret"))
	require.ErrorIs(t, err, core.ErrInvalid)
	_, err = OpenSecrets(fake.New(), nil)
	require.ErrorIs(t, err, core.ErrInvalid)
}

func TestSecretsWrongKeyStartsEmpty(t *testing.T) {
	t.Parallel()

	randomKey := make([]byte, 32)
	_, _ = rand.Read(randomKey)
	cases := []struct {
		name   string
		damage func(t *testing.T, dir string)
	}{
		{name: "another key", damage: func(t *testing.T, dir string) {
			t.Helper()
			require.NoError(t, os.WriteFile(filepath.Join(dir, secretsKey), randomKey, 0o600))
		}},
		{name: "key gone", damage: func(t *testing.T, dir string) {
			t.Helper()
			require.NoError(t, os.Remove(filepath.Join(dir, secretsKey)))
		}},
		{name: "key cut short", damage: func(t *testing.T, dir string) {
			t.Helper()
			require.NoError(t, os.WriteFile(filepath.Join(dir, secretsKey), randomKey[:7], 0o600))
		}},
		{name: "store not json", damage: func(t *testing.T, dir string) {
			t.Helper()
			require.NoError(t, os.WriteFile(filepath.Join(dir, secretsFile), []byte("{not json"), 0o600))
		}},
		{name: "store sealed data changed", damage: func(t *testing.T, dir string) {
			t.Helper()
			path := filepath.Join(dir, secretsFile)
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			raw = bytes.Replace(raw, []byte(`"data":"`), []byte(`"data":"AAAA`), 1)
			require.NoError(t, os.WriteFile(path, raw, 0o600))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "secret")
			require.NoError(t, prepareSecretDir(dir))
			file, err := openFileStore(dir)
			require.NoError(t, err)
			require.NoError(t, file.Set(testAccount, []byte(testValue)))
			tc.damage(t, dir)

			s, err := openSecrets(secret.NewStore(noSecretsDriver{}, AppID), false, nil, dir)
			require.NoError(t, err)
			require.NotEmpty(t, s.Damaged())
			assert.True(t, strings.HasPrefix(filepath.Base(s.Damaged()), secretsFile+damagedTag), s.Damaged())
			_, err = os.Stat(s.Damaged())
			require.NoError(t, err, "the damaged store is kept aside")
			_, err = s.Get(testAccount)
			require.ErrorIs(t, err, core.ErrNotFound)

			require.NoError(t, s.Set(testAccount, []byte("fresh")))
			again, err := openSecrets(secret.NewStore(noSecretsDriver{}, AppID), false, nil, dir)
			require.NoError(t, err)
			assert.Empty(t, again.Damaged())
			got, err := again.Get(testAccount)
			require.NoError(t, err)
			assert.Equal(t, "fresh", string(got))
		})
	}
}

func TestSecretDirMode0700(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	_, err := openSecrets(secret.NewMemoryStore(), false, nil, dir)
	require.NoError(t, err)
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())

	fresh := filepath.Join(t.TempDir(), "a", "secret")
	_, err = openSecrets(secret.NewMemoryStore(), false, nil, fresh)
	require.NoError(t, err)
	info, err = os.Stat(fresh)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestOpenSecretsFailsOnUnwritableDir(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	_, err := openSecrets(secret.NewMemoryStore(), false, nil, filepath.Join(file, "secret"))
	require.Error(t, err)
}

func TestExcludeFromBackupUnsupportedOffIOS(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.ErrorIs(t, ExcludeFromBackup(dir), core.ErrUnsupported)
	_, err := ExcludedFromBackup(dir)
	require.ErrorIs(t, err, core.ErrUnsupported)
	require.NoError(t, excludeIfSupported(dir))
}

func TestDirsOffMobile(t *testing.T) {
	t.Parallel()

	cfg, err := os.UserConfigDir()
	require.NoError(t, err)
	cache, err := os.UserCacheDir()
	require.NoError(t, err)
	cases := []struct {
		name string
		dir  func() (string, error)
		want string
	}{
		{name: "data", dir: DataDir, want: filepath.Join(cfg, AppID)},
		{name: "cache", dir: CacheDir, want: filepath.Join(cache, AppID)},
		{name: "secret", dir: SecretDir, want: filepath.Join(cfg, AppID, "secret")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.dir()
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// uiThread starts d on a goroutine of its own, which serves the
// functions invoke hands it, as host.App.Invoke does; stop ends it.
func uiThread(t *testing.T, d *fake.Driver) (invoke func(func()) error, stop func()) {
	t.Helper()
	work := make(chan func())
	started := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		started <- d.Start(nopHost{}, platform.AppConfig{})
		for fn := range work {
			fn()
		}
	}()
	require.NoError(t, <-started)
	invoke = func(fn func()) error {
		finished := make(chan struct{})
		work <- func() {
			defer close(finished)
			fn()
		}
		<-finished

		return nil
	}

	return invoke, func() {
		close(work)
		<-done
	}
}
