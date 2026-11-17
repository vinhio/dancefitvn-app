package store

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tinyPNG(t testing.TB) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))))

	return buf.Bytes()
}

// TestImagesPutGet files a picture and reads it back; what is no picture is refused
// and a spoiled file is a miss that goes.
func TestImagesPutGet(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	c := newImages(dir, 0)
	data := tinyPNG(t)
	require.NoError(t, c.Put("https://x.test/a.png", data))
	got, saved, ok := c.Get("https://x.test/a.png")
	require.True(t, ok)
	assert.Equal(t, data, got)
	assert.WithinDuration(t, time.Now(), saved, time.Minute)

	assert.ErrorIs(t, c.Put("k", []byte("<html></html>")), core.ErrInvalid)
	_, _, ok = c.Get("k")
	assert.False(t, ok)

	require.NoError(t, os.WriteFile(c.path("https://x.test/bad"), []byte("junk"), 0o600))
	_, _, ok = c.Get("https://x.test/bad")
	assert.False(t, ok)
	assert.NoFileExists(t, c.path("https://x.test/bad"), "the spoiled file is deleted")

	var nilCache *Images
	assert.NoError(t, nilCache.Put("k", data))
	_, _, ok = nilCache.Get("k")
	assert.False(t, ok)
	nilCache.Close()

	c.Close()
	assert.ErrorIs(t, c.Put("k2", data), core.ErrClosed)
}

// TestImagesPruneKeepsTheNewest drops the oldest files past the limit.
func TestImagesPruneKeepsTheNewest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	data := tinyPNG(t)
	c := newImages(dir, int64(2*len(data)))
	for i, k := range []string{"old", "mid", "new"} {
		require.NoError(t, c.Put(k, data))
		at := time.Now().Add(time.Duration(i-3) * time.Hour)
		require.NoError(t, os.Chtimes(c.path(k), at, at))
	}
	c.prune()
	_, _, old := c.Get("old")
	_, _, mid := c.Get("mid")
	_, _, newest := c.Get("new")
	assert.False(t, old)
	assert.True(t, mid)
	assert.True(t, newest)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}
