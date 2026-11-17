package store_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/store"
)

// backends are the implementations of store.Backend the contract runs on.
func backends(t *testing.T) map[string]store.Backend {
	t.Helper()

	return map[string]store.Backend{
		"memory": store.NewMemory(),
		"dir":    store.NewDir(filepath.Join(t.TempDir(), "data")),
	}
}

func TestBackendContract(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"memory", "dir"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			b := backends(t)[name]
			_, err := b.Load("k")
			require.ErrorIs(t, err, core.ErrNotFound)

			require.NoError(t, b.Save("k", []byte(`{"a": 1}`)))
			got, err := b.Load("k")
			require.NoError(t, err)
			assert.Equal(t, `{"a": 1}`, string(got), "bytes as written")

			in := []byte("abc")
			require.NoError(t, b.Save("c", in))
			in[0] = 'X'
			got, _ = b.Load("c")
			assert.Equal(t, "abc", string(got), "Save keeps a copy")
			got[0] = 'Y'
			again, _ := b.Load("c")
			assert.Equal(t, "abc", string(again), "Load returns a copy")

			require.NoError(t, b.Save("k", []byte(`{"a":2}`)))
			got, _ = b.Load("k")
			assert.Equal(t, `{"a":2}`, string(got), "second save replaces")
		})
	}
}

func TestDirKeyRegex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key string
		ok  bool
	}{
		{"prefs", true}, {"mock-overlay", true}, {"progress-queue", true}, {"a.b_c-1", true}, {"0x", true},
		{"", false}, {"../x", false}, {"a/b", false}, {".hidden", false}, {"Upper", false}, {"a b", false}, {"-x", false},
	}
	d := store.NewDir(t.TempDir())
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()

			err := d.Save(tt.key, []byte(`{}`))
			_, lerr := d.Load(tt.key)
			if tt.ok {
				assert.NoError(t, err)
				assert.NoError(t, lerr)

				return
			}
			assert.ErrorIs(t, err, core.ErrInvalid)
			assert.ErrorIs(t, lerr, core.ErrInvalid)
		})
	}
}

func TestDirFileNameAndMode(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "a", "b")
	require.NoError(t, store.NewDir(dir).Save("prefs", []byte(`{}`)))
	b, err := os.ReadFile(filepath.Join(dir, "prefs.json"))
	require.NoError(t, err)
	assert.Equal(t, `{}`, string(b))
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestDirAtomicWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	d := store.NewDir(dir)
	require.NoError(t, d.Save("prefs", []byte(`{"old":true}`)))

	store.FailSync(d, assert.AnError)
	err := d.Save("prefs", []byte(`{"new":true}`))
	require.ErrorIs(t, err, assert.AnError)

	got, err := d.Load("prefs")
	require.NoError(t, err)
	assert.JSONEq(t, `{"old":true}`, string(got), "the old file is as it was")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "no partial or temporary file")
	assert.Equal(t, "prefs.json", entries[0].Name())

	store.FailSync(d, nil)
	require.NoError(t, d.Save("prefs", []byte(`{"new":true}`)))
	got, _ = d.Load("prefs")
	assert.JSONEq(t, `{"new":true}`, string(got))
}

func TestDirRemove(t *testing.T) {
	t.Parallel()

	d := store.NewDir(t.TempDir())
	require.NoError(t, d.Save("k", []byte(`{}`)))
	require.NoError(t, d.Remove("k"))
	require.NoError(t, d.Remove("k"), "missing is no error")
	_, err := d.Load("k")
	assert.ErrorIs(t, err, core.ErrNotFound)
	assert.ErrorIs(t, d.Remove("../x"), core.ErrInvalid)
}

func TestDirConcurrentSaves(t *testing.T) {
	t.Parallel()

	d := store.NewDir(t.TempDir())
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			assert.NoError(t, d.Save("k", []byte(`{"n":`+string(rune('0'+i%10))+`}`)))
			_, err := d.Load("k")
			assert.NoError(t, err)
		})
	}
	wg.Wait()
	got, err := d.Load("k")
	require.NoError(t, err)
	assert.Regexp(t, `^\{"n":\d\}$`, string(got), "whole file, never a mix")
}

func TestMemoryInjectsErrors(t *testing.T) {
	t.Parallel()

	m := store.NewMemory()
	m.SaveErr = assert.AnError
	assert.ErrorIs(t, m.Save("k", []byte("a")), assert.AnError)
	assert.Zero(t, m.Saves)
	m.Put("k", []byte("raw"))
	m.SaveErr = nil
	m.LoadErr = assert.AnError
	_, err := m.Load("k")
	assert.ErrorIs(t, err, assert.AnError)
	m.LoadErr = nil
	got, err := m.Load("k")
	require.NoError(t, err)
	assert.Equal(t, "raw", string(got))
}
