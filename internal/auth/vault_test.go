package auth_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/auth"
)

// fakeSecrets is a SecretStore in memory.
type fakeSecrets struct {
	mu     sync.Mutex
	m      map[string][]byte
	setErr error
}

func (f *fakeSecrets) Get(a string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.m[a]
	if !ok {
		return nil, fmt.Errorf("fake: %w", core.ErrNotFound)
	}

	return v, nil
}

func (f *fakeSecrets) Set(a string, v []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setErr != nil {
		return f.setErr
	}
	if f.m == nil {
		f.m = map[string][]byte{}
	}
	f.m[a] = v

	return nil
}

func (f *fakeSecrets) Delete(a string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, a)

	return nil
}

func TestSecretVaultRoundTrip(t *testing.T) {
	t.Parallel()

	fs := &fakeSecrets{}
	v := auth.NewSecretVault(fs, "install-1")
	_, err := v.Load()
	require.ErrorIs(t, err, core.ErrNotFound)

	in := session("acc", "ref", "u_1")
	in.User.FullName = "Nguyễn Thảo Vy"
	in.Device.ID = "d_1"
	require.NoError(t, v.Save(in))
	assert.Contains(t, fs.m, "install-1", "the account is the install id")

	out, err := v.Load()
	require.NoError(t, err)
	assert.Equal(t, in, out)

	require.NoError(t, v.Clear())
	require.NoError(t, v.Clear(), "clearing nothing is no error")
	_, err = v.Load()
	assert.ErrorIs(t, err, core.ErrNotFound)
}

func TestSecretVaultBadValue(t *testing.T) {
	t.Parallel()

	for name, raw := range map[string]string{
		"garbage":       "\x00 nope",
		"other version": `{"version":99,"session":{}}`,
		"empty":         ``,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fs := &fakeSecrets{m: map[string][]byte{"a": []byte(raw)}}
			_, err := auth.NewSecretVault(fs, "a").Load()
			assert.ErrorIs(t, err, core.ErrInvalid)
		})
	}
}

func TestSecretVaultSaveFailure(t *testing.T) {
	t.Parallel()

	fs := &fakeSecrets{setErr: assert.AnError}
	assert.ErrorIs(t, auth.NewSecretVault(fs, "a").Save(session("a", "r", "u")), assert.AnError)
}

func TestSecretVaultWithManager(t *testing.T) {
	t.Parallel()

	fs := &fakeSecrets{}
	m1, err := auth.New(auth.Options{Vault: auth.NewSecretVault(fs, "i")})
	require.NoError(t, err)
	require.NoError(t, m1.Adopt(session("acc", "ref", "u_1")))

	// a new launch
	m2, err := auth.New(auth.Options{Vault: auth.NewSecretVault(fs, "i")})
	require.NoError(t, err)
	s, ok := m2.Restore()
	require.True(t, ok)
	assert.Equal(t, "u_1", s.User.ID)
	assert.Equal(t, "acc", m2.Token())
}

func TestSessionNeverPrints(t *testing.T) {
	t.Parallel()

	s := session("secret-access", "secret-refresh", "u")
	for _, out := range []string{fmt.Sprintf("%v", s), fmt.Sprintf("%+v", s), fmt.Sprintf("%#v", s), fmt.Sprint(s.Tokens)} {
		assert.NotContains(t, out, "secret-")
	}
}
