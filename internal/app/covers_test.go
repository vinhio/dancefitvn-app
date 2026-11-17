package app

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"sync/atomic"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/framework/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// coverPNG makes a w x h picture of one color, the stand-in of a cover.
func coverPNG(t testing.TB, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 10, 200, 30, 255
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	return buf.Bytes()
}

// coverFixture is a loader over a temporary disk cache and a counting fetch.
type coverFixture struct {
	l     *coverLoader
	disk  *store.Images
	calls atomic.Int32
	body  []byte
	err   error
	now   time.Time
}

func newCoverFixture(t *testing.T, mock func(string) bool) *coverFixture {
	t.Helper()
	f := &coverFixture{now: time.Now()}
	f.disk = store.OpenImages(t.TempDir(), 0)
	t.Cleanup(f.disk.Close)
	fetch := func(_ context.Context, _ string, limit int64) ([]byte, error) {
		f.calls.Add(1)
		if f.err != nil {
			return nil, f.err
		}
		require.Equal(t, int64(maxCoverBytes), limit)

		return f.body, nil
	}
	f.l = newCoverLoader(&components.ImageCache{}, f.disk, fetch, mock)
	f.l.now = func() time.Time { return f.now }

	return f
}

// TestCoverLoaderDecodesReducesAndKeepsOnDisk loads a cover larger than it is drawn:
// it comes back reduced to coverMaxSide, is filed, and a second loader over the same
// folder (a relaunch) draws it with the network gone.
func TestCoverLoaderDecodesReducesAndKeepsOnDisk(t *testing.T) {
	t.Parallel()

	f := newCoverFixture(t, nil)
	f.body = coverPNG(t, 2400, 1200)
	url := "https://cdn.example.test/covers/a.png?sig=1"
	img, err := f.l.Load(t.Context(), url)
	require.NoError(t, err)
	assert.Equal(t, coverMaxSide, img.Width, "the longer side is reduced")
	assert.Equal(t, coverMaxSide/2, img.Height, "the shape is kept")
	assert.Equal(t, uint8(10), img.Pix[0])
	assert.Equal(t, uint8(200), img.Pix[1])
	assert.EqualValues(t, 1, f.calls.Load())

	// a relaunch: a new image cache, the same folder, no network
	again := newCoverLoader(&components.ImageCache{}, f.disk, func(context.Context, string, int64) ([]byte, error) {
		return nil, api.ErrOffline
	}, nil)
	img, err = again.Load(t.Context(), "https://cdn.example.test/covers/a.png?sig=2") // the CDN signs anew
	require.NoError(t, err, "the file on the disk answers, whatever the signature")
	assert.Equal(t, coverMaxSide, img.Width)
}

// TestCoverLoaderOnce loads one address once, however often it is asked for.
func TestCoverLoaderOnce(t *testing.T) {
	t.Parallel()

	f := newCoverFixture(t, nil)
	f.body = coverPNG(t, 64, 32)
	for range 3 {
		_, err := f.l.Load(t.Context(), "https://cdn.example.test/b.png")
		require.NoError(t, err)
	}
	assert.EqualValues(t, 1, f.calls.Load())
}

// TestCoverLoaderRefusesTheMocksAndFailures keeps the vector poster: the mock's own
// pictures, no address, a failing network with nothing on the disk and bytes that
// are no picture are errors; the mock's is core.ErrUnsupported and never fetched.
func TestCoverLoaderRefusesTheMocksAndFailures(t *testing.T) {
	t.Parallel()

	f := newCoverFixture(t, func(u string) bool { return u == "http://127.0.0.1:1/covers/set-35.jpg" })
	_, err := f.l.Load(t.Context(), "http://127.0.0.1:1/covers/set-35.jpg")
	assert.ErrorIs(t, err, core.ErrUnsupported)
	_, err = f.l.Load(t.Context(), "")
	assert.ErrorIs(t, err, core.ErrUnsupported)
	_, err = f.l.Load(t.Context(), "mock-media://covers/set-35.jpg")
	assert.ErrorIs(t, err, core.ErrUnsupported)
	assert.Zero(t, f.calls.Load(), "nothing was fetched")

	f.err = errors.New("no network")
	_, err = f.l.Load(t.Context(), "https://cdn.example.test/c.png")
	assert.Error(t, err)

	f.err, f.body = nil, []byte("<html>error page</html>")
	_, err = f.l.Load(t.Context(), "https://cdn.example.test/d.png")
	assert.ErrorIs(t, err, core.ErrInvalid)
}

// TestCoverLoaderStaleFileAnswersOffline asks the server again once a file is older
// than coverFresh, and keeps the old file when the server cannot answer.
func TestCoverLoaderStaleFileAnswersOffline(t *testing.T) {
	t.Parallel()

	f := newCoverFixture(t, nil)
	f.body = coverPNG(t, 64, 32)
	url := "https://cdn.example.test/e.png"
	_, err := f.l.Load(t.Context(), url)
	require.NoError(t, err)
	assert.EqualValues(t, 1, f.calls.Load())

	f.now = f.now.Add(coverFresh + time.Hour)
	f.err = api.ErrOffline
	fresh := newCoverLoader(&components.ImageCache{}, f.disk, f.l.fetch, nil)
	fresh.now = f.l.now
	img, err := fresh.Load(t.Context(), url)
	require.NoError(t, err, "a stale file beats none")
	assert.Equal(t, 64, img.Width)
	assert.EqualValues(t, 2, f.calls.Load(), "the server was asked first")
}
