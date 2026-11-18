package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"testing"

	"dancefitvn-app/internal/avatar"
	"dancefitvn-app/internal/safe"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/framework/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// brokenPictures are the bytes a server, a disk or a camera roll can hand over
// in place of a picture, as the tests of the picture paths feed them.
func brokenPictures(t testing.TB) []struct {
	name string
	data []byte
} {
	t.Helper()
	good := coverPNG(t, 64, 32)
	var jb bytes.Buffer
	require.NoError(t, jpeg.Encode(&jb, image.NewRGBA(image.Rect(0, 0, 40, 40)), nil))

	return []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"zero bytes", []byte{}},
		{"html error page", []byte("<!doctype html><html><body>403 Forbidden</body></html>")},
		{"json error body", []byte(`{"error":{"code":"expired"}}`)},
		{"png signature only", good[:8]},
		{"png cut in the header", good[:20]},
		{"png cut in the pixels", good[:len(good)/2]},
		{"png pixels spoilt", spoilt(good)},
		{"jpeg cut", jb.Bytes()[:jb.Len()/2]},
		{"jpeg header only", jb.Bytes()[:4]},
		{"random bytes", bytes.Repeat([]byte{0x9c, 0x01, 0xff, 0x00, 0x42}, 400)},
		{"png declaring 30000 x 30000", declaring(t, 30000, 30000)},
		{"png declaring 1 x 40000", declaring(t, 1, 40000)},
	}
}

// spoilt returns png with its compressed pixels overwritten.
func spoilt(png []byte) []byte {
	out := bytes.Clone(png)
	for i := 40; i < len(out)-16; i++ {
		out[i] ^= 0x5a
	}

	return out
}

// declaring returns the file of a 1 x 1 PNG whose header says w x h: a few
// dozen bytes that would ask for gigabytes of pixels.
func declaring(t testing.TB, w, h uint32) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))))
	b := buf.Bytes()
	binary.BigEndian.PutUint32(b[16:], w)
	binary.BigEndian.PutUint32(b[20:], h)
	binary.BigEndian.PutUint32(b[29:], crc32.ChecksumIEEE(b[12:29]))

	return b
}

// TestPicturePathsRefuseBrokenBytes feeds every picture path of the app (an
// avatar, a cover, the artwork of Now Playing, the photo picked for the avatar)
// the bytes of a missing, corrupt, truncated, wrong-type or oversized picture:
// each answers an error, none panics, none builds the pixels of a declared
// huge picture. The screens then keep the default person or the vector poster.
// (docs/plan/03-behavior-spec.md section 9; K14)
func TestPicturePathsRefuseBrokenBytes(t *testing.T) {
	t.Parallel()

	paths := []struct {
		name string
		run  func([]byte) error
	}{
		{"avatar picture", func(b []byte) error { _, err := decodePicture(b); return err }},
		{"cover", func(b []byte) error { _, err := decodeCover(b); return err }},
		{"now playing artwork", func(b []byte) error { _, err := fitArtwork(b); return err }},
		{"picked photo", func(b []byte) error { _, err := avatar.Decode(b); return err }},
	}
	for _, p := range paths {
		for _, c := range brokenPictures(t) {
			t.Run(p.name+"/"+c.name, func(t *testing.T) {
				t.Parallel()

				var err error
				require.NotPanics(t, func() { err = p.run(c.data) })
				require.Error(t, err)
				assert.True(t, errors.Is(err, core.ErrInvalid) || errors.Is(err, core.ErrUnsupported), "%v", err)
			})
		}
	}
}

// TestPicturePathsKeepGoodPictures is the other side of the table above: a
// small picture is decoded by every path.
func TestPicturePathsKeepGoodPictures(t *testing.T) {
	t.Parallel()

	good := coverPNG(t, 64, 32)
	img, err := decodePicture(good)
	require.NoError(t, err)
	assert.Equal(t, [2]int{64, 32}, [2]int{img.Width, img.Height})
	_, err = decodeCover(good)
	require.NoError(t, err)
	_, err = fitArtwork(good)
	require.NoError(t, err)
}

// TestPictureLoadsSurviveAPanic makes the decoder, or the fetch before it, panic
// on the goroutine that components.ImageCache runs a load on (nobody recovers
// it there): the load answers an error, the process goes on, and a later load of
// the same address tries again.
// (a panic in a goroutine of the app ends the app: docs/plan/03 section 9)
func TestPictureLoadsSurviveAPanic(t *testing.T) {
	t.Parallel()
	safe.Expect("app.cover")

	boom := true
	f := newCoverFixture(t, nil)
	f.l.fetch = func(context.Context, string, int64) ([]byte, error) {
		if boom {
			panic("decoder: index out of range [7] with length 3")
		}

		return f.body, nil
	}
	f.body = coverPNG(t, 64, 32)
	url := "https://cdn.example.test/panics.png"
	_, err := f.l.Load(t.Context(), url)
	require.ErrorIs(t, err, safe.ErrPanic)
	assert.ErrorIs(t, err, core.ErrInvalid)

	boom = false
	img, err := f.l.Load(t.Context(), url)
	require.NoError(t, err, "a failed load is not kept")
	assert.NotNil(t, img)
}

// TestCoverLoaderHealsASpoiltFile checks both ways a cover can be spoilt on its way
// to the disk: bytes cut short by the network are never filed, so a later load asks
// the server again; and a file already on the disk whose header reads but whose
// pixels do not decode is removed and fetched again, at once, not kept to answer an
// error for a week. (K14; the vector poster shows meanwhile)
func TestCoverLoaderHealsASpoiltFile(t *testing.T) {
	t.Parallel()

	whole := coverPNG(t, 64, 32)
	cut := whole[:len(whole)/2]
	tests := []struct {
		name     string
		onDisk   []byte // filed before the load; nil none
		served   []byte
		wantErr  bool
		wantCall int32
	}{
		{"cut by the network", nil, cut, true, 1},
		{"html instead of a picture", nil, []byte("<html>502</html>"), true, 1},
		{"spoilt file on the disk, good one on the server", cut, whole, false, 1},
		{"spoilt file on the disk, bad one on the server", cut, cut, true, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newCoverFixture(t, nil)
			url := "https://cdn.example.test/heal.png"
			if tc.onDisk != nil {
				require.NoError(t, f.disk.Put(artKey(url), tc.onDisk), "its header reads as a picture")
			}
			f.body = tc.served
			img, err := f.l.Load(t.Context(), url)
			assert.EqualValues(t, tc.wantCall, f.calls.Load())
			if tc.wantErr {
				require.ErrorIs(t, err, core.ErrInvalid)
				_, _, filed := f.disk.Get(artKey(url))
				assert.False(t, filed, "nothing spoilt stays on the disk")

				f.l.images = &components.ImageCache{} // a later load, the server better
				f.body = whole
				_, err = f.l.Load(t.Context(), url)
				require.NoError(t, err)
				assert.EqualValues(t, tc.wantCall+1, f.calls.Load(), "asked again")

				return
			}
			require.NoError(t, err)
			assert.NotNil(t, img)
			data, _, filed := f.disk.Get(artKey(url))
			require.True(t, filed)
			assert.Equal(t, whole, data, "the good file replaced it")
		})
	}
}
