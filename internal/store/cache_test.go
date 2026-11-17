package store_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/store"
)

var _ api.Cache = (*store.Cache)(nil)

func openCache(t *testing.T) (*store.Cache, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "api")
	c := store.OpenCache(dir, 0)
	t.Cleanup(c.Close)

	return c, dir
}

func TestCacheRoundTrip(t *testing.T) {
	t.Parallel()

	c, _ := openCache(t)
	_, _, ok := c.Get("GET /me?")
	assert.False(t, ok)

	require.NoError(t, c.Put("GET /me?", `"abc"`, []byte("{\n \"a\": [1, 2]\n}")))
	etag, body, ok := c.Get("GET /me?")
	require.True(t, ok)
	assert.Equal(t, `"abc"`, etag)
	assert.JSONEq(t, `{"a":[1,2]}`, string(body))

	require.NoError(t, c.Put("GET /me?", `"def"`, []byte(`{"a":3}`)))
	etag, body, _ = c.Get("GET /me?")
	assert.Equal(t, `"def"`, etag)
	assert.JSONEq(t, `{"a":3}`, string(body))
	_, _, ok = c.Get("GET /me/home?")
	assert.False(t, ok, "other key")
}

func TestCacheFileName(t *testing.T) {
	t.Parallel()

	c, dir := openCache(t)
	require.NoError(t, c.Put("GET /me?", "t", []byte(`{}`)))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Regexp(t, `^[0-9a-f]{64}\.json$`, entries[0].Name())
}

func TestCachePutRejects(t *testing.T) {
	t.Parallel()

	c, _ := openCache(t)
	assert.ErrorIs(t, c.Put("k", "", []byte(`{}`)), core.ErrInvalid, "no etag")
	assert.ErrorIs(t, c.Put("k", "t", []byte(`not json`)), core.ErrInvalid)
	assert.ErrorIs(t, c.Put("k", "t", nil), core.ErrInvalid)
	_, _, ok := c.Get("k")
	assert.False(t, ok)
}

func TestCacheCorruptIsMiss(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		edit func(raw string) string
	}{
		{"truncated", func(raw string) string { return raw[:len(raw)/2] }},
		{"garbage", func(string) string { return "\x00\x01 not json" }},
		{"empty", func(string) string { return "" }},
		{"other key", func(raw string) string { return strings.Replace(raw, `"key":"k"`, `"key":"zz"`, 1) }},
		{"no etag", func(raw string) string { return strings.Replace(raw, `"etag":"t"`, `"etag":""`, 1) }},
		{"no body", func(raw string) string { return strings.Replace(raw, `"body":{"a":1}`, `"body":null`, 1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, dir := openCache(t)
			require.NoError(t, c.Put("k", "t", []byte(`{"a":1}`)))
			files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
			require.Len(t, files, 1)
			raw, err := os.ReadFile(files[0])
			require.NoError(t, err)
			edited := tt.edit(string(raw))
			require.NotEqual(t, string(raw), edited)
			require.NoError(t, os.WriteFile(files[0], []byte(edited), 0o600))

			_, _, ok := c.Get("k")
			assert.False(t, ok)
			_, err = os.Stat(files[0])
			assert.ErrorIs(t, err, os.ErrNotExist, "the bad file is deleted")
		})
	}
}

func TestCacheClear(t *testing.T) {
	t.Parallel()

	c, dir := openCache(t)
	c.Clear() // nothing yet: no error, no panic
	for i := range 5 {
		require.NoError(t, c.Put(fmt.Sprint("k", i), "t", []byte(`{}`)))
	}
	c.Clear()
	for i := range 5 {
		_, _, ok := c.Get(fmt.Sprint("k", i))
		assert.False(t, ok)
	}
	require.NoError(t, c.Put("again", "t", []byte(`{}`)), "writes go to a new folder")
	_, _, ok := c.Get("again")
	assert.True(t, ok)

	c.Close() // waits for the deletion
	sibs, err := filepath.Glob(dir + ".old-*")
	require.NoError(t, err)
	assert.Empty(t, sibs, "cleared folder deleted")
}

func TestCachePruneOldestFirst(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "api")
	c := store.NewCacheNoPrune(dir, 2500)
	pad := `{"p":"` + strings.Repeat("x", 800) + `"}`
	base := time.Now().Add(-time.Hour)
	for i := range 4 {
		key := fmt.Sprint("k", i)
		require.NoError(t, c.Put(key, "t", []byte(pad)))
	}
	// age the files: k0 oldest.
	for i, f := range filesByKey(t, dir, 4) {
		require.NoError(t, os.Chtimes(f, base.Add(time.Duration(i)*time.Minute), base.Add(time.Duration(i)*time.Minute)))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tmp-leftover"), []byte("x"), 0o600))
	require.NoError(t, os.Chtimes(filepath.Join(dir, ".tmp-leftover"), base, base))

	c.Prune()

	_, _, ok := c.Get("k0")
	assert.False(t, ok, "oldest went")
	_, _, ok = c.Get("k3")
	assert.True(t, ok, "newest stays")
	assert.NoFileExists(t, filepath.Join(dir, ".tmp-leftover"))
	var total int64
	left, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, f := range left {
		info, err := os.Stat(f)
		require.NoError(t, err)
		total += info.Size()
	}
	assert.LessOrEqual(t, total, int64(2500))
}

// filesByKey returns the file of k0..k(n-1), in that order.
func filesByKey(t *testing.T, dir string, n int) []string {
	t.Helper()
	out := make([]string, n)
	c := store.NewCacheNoPrune(dir, 0)
	for i := range n {
		out[i] = store.CachePath(c, fmt.Sprint("k", i))
	}

	return out
}

func TestCacheClosedWritesNothing(t *testing.T) {
	t.Parallel()

	c, dir := openCache(t)
	c.Close()
	c.Close()
	assert.ErrorIs(t, c.Put("k", "t", []byte(`{}`)), core.ErrClosed)
	c.Clear()
	_, err := os.Stat(dir)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCacheConcurrent(t *testing.T) {
	t.Parallel()

	c, _ := openCache(t)
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			key := fmt.Sprint("k", i%4)
			_ = c.Put(key, "t", []byte(`{"a":1}`))
			c.Get(key)
			if i%8 == 0 {
				c.Clear()
			}
		})
	}
	wg.Wait()
}
