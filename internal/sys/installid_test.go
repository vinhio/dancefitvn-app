package sys

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func TestInstallIDStable(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "secret")
	first, err := installIDIn(dir)
	require.NoError(t, err)
	second, err := installIDIn(dir)
	require.NoError(t, err)
	assert.Equal(t, first, second)

	raw, err := os.ReadFile(filepath.Join(dir, installIDFile))
	require.NoError(t, err)
	assert.Equal(t, first+"\n", string(raw))
	info, err := os.Stat(filepath.Join(dir, installIDFile))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	info, err = os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	leftovers, err := filepath.Glob(filepath.Join(dir, ".install-id.*"))
	require.NoError(t, err)
	assert.Empty(t, leftovers, "no temporary file stays behind")
}

func TestInstallIDFormat(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for range 20 {
		id, err := installIDIn(filepath.Join(t.TempDir(), "secret"))
		require.NoError(t, err)
		assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, id)
		u, err := uuid.Parse(id)
		require.NoError(t, err)
		assert.Equal(t, byte(4), u[6]>>4, "version 4")
		assert.Equal(t, byte(0b10), u[8]>>6, "RFC 9562 variant")
		assert.False(t, seen[id], "ids differ")
		seen[id] = true
	}
}

func TestInstallIDRegeneratesMalformed(t *testing.T) {
	t.Parallel()

	const kept = "3f0c9a7e-1b2d-4c5e-8f90-a1b2c3d4e5f6"
	cases := []struct {
		name     string
		content  string
		keepsOld bool
	}{
		{name: "empty", content: ""},
		{name: "garbage", content: "not an id"},
		{name: "version 1", content: "f81d4fae-7dec-11d0-a765-00a0c91e6bf6"},
		{name: "uppercase", content: strings.ToUpper(kept)},
		{name: "braces", content: "{" + kept + "}"},
		{name: "wrong variant", content: "3f0c9a7e-1b2d-4c5e-cf90-a1b2c3d4e5f6"},
		{name: "valid", content: kept, keepsOld: true},
		{name: "valid with spaces", content: "  " + kept + "\n\n", keepsOld: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "secret")
			require.NoError(t, os.MkdirAll(dir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, installIDFile), []byte(tc.content), 0o600))
			id, err := installIDIn(dir)
			require.NoError(t, err)
			if tc.keepsOld {
				assert.Equal(t, kept, id)

				return
			}
			assert.Regexp(t, installIDRe, id)
			again, err := installIDIn(dir)
			require.NoError(t, err)
			assert.Equal(t, id, again, "the new id is kept")
		})
	}
}

func TestInstallIDConcurrentFirstCallsAgree(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "secret")
	ids := make([]string, 8)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Go(func() {
			id, err := installIDIn(dir)
			assert.NoError(t, err)
			ids[i] = id
		})
	}
	wg.Wait()
	for _, id := range ids {
		assert.Equal(t, ids[0], id)
	}
}

func TestInstallIDFailsOnUnreadableFile(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, installIDFile), 0o700), "a folder where the file should be")
	_, err := installIDIn(dir)
	require.Error(t, err)
}
