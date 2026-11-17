// Adapted from jplayer-mobile/internal/sys/services_test.go at 49de3c2
// (TestErrCanceled, TestSampleSize, TestExifTransform, TestGoTakes,
// TestReadImage, zeros, encoded). Changed: no WebP; minSampledSide 1560.

package sys

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// zeros reads zeros for ever.
type zeros struct{}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Read fills p with zeros.
func (zeros) Read(p []byte) (int, error) {
	clear(p)

	return len(p), nil
}

// TestPickImageChecksArguments checks the errors every platform gives
// before the system is asked: no canvas, no dispatch, no done.
func TestPickImageChecksArguments(t *testing.T) {
	t.Parallel()

	done := func([]byte, string, error) { t.Error("done ran") }
	tests := []struct {
		name     string
		canvas   platform.Canvas
		dispatch bool
		done     func([]byte, string, error)
	}{
		{"no canvas", nil, true, done},
		{"no dispatch", stubCanvas{}, false, done},
		{"no done", stubCanvas{}, true, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var d Dispatch
			if tc.dispatch {
				d = noDispatch(t)
			}
			err := PickImage(tc.canvas, d, tc.done)
			require.ErrorIs(t, err, core.ErrInvalid)
			assert.True(t, strings.HasPrefix(err.Error(), "sys: pick image:"), err.Error())
		})
	}
}

// TestPickImageOffPhone checks that a desktop (and tvOS) has no photo
// picker: core.ErrUnsupported, nothing dispatched.
func TestPickImageOffPhone(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		t.Skip("the photo picker is checked by e2e/sys on the simulator and the emulator")
	}
	err := PickImage(stubCanvas{}, noDispatch(t), func([]byte, string, error) { t.Error("done ran") })
	require.ErrorIs(t, err, core.ErrUnsupported)
}

// TestErrCanceled checks that a dismissed picker is what the screens take
// for the user's choice (context.Canceled), and that a second picker is
// core.ErrDuplicate.
func TestErrCanceled(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, ErrCanceled, context.Canceled)
	assert.ErrorIs(t, busy("pick image"), core.ErrDuplicate)
	assert.True(t, strings.HasPrefix(busy("pick image").Error(), "sys: pick image: already showing: "))
}

// TestSampleSize checks BitmapFactory's inSampleSize: the largest power
// of two keeping the longest side at least 1560 px.
func TestSampleSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		w, h int
		want int32
	}{
		{100, 50, 1},
		{1560, 900, 1},
		{3119, 100, 1},
		{3120, 1600, 2},
		{4032, 3024, 2},
		{3024, 4032, 2},
		{6240, 10, 4},
		{8064, 6048, 4},
		{12480, 9360, 8},
	}
	for _, tc := range tests {
		got := sampleSize(tc.w, tc.h)
		assert.Equal(t, tc.want, got, "%d×%d", tc.w, tc.h)
		assert.GreaterOrEqual(t, max(tc.w, tc.h)/int(got), min(max(tc.w, tc.h), minSampledSide), "%d×%d keeps its pixels", tc.w, tc.h)
	}
}

// TestExifTransform checks the turn of each EXIF orientation, as
// Android's decoders make it.
func TestExifTransform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		o    int32
		deg  float32
		flip bool
	}{
		{0, 0, false},
		{1, 0, false},
		{2, 0, true},
		{3, 180, false},
		{4, 180, true},
		{5, 90, true},
		{6, 90, false},
		{7, -90, true},
		{8, -90, false},
		{9, 0, false},
		{-1, 0, false},
	}
	for _, tc := range tests {
		deg, flip := exifTransform(tc.o)
		assert.InDelta(t, tc.deg, deg, 0, "orientation %d", tc.o)
		assert.Equal(t, tc.flip, flip, "orientation %d", tc.o)
	}
}

// TestGoTakes checks which pictures go to Go as they are: a JPEG or PNG of
// its type, up to 4096 px; not a HEIC, not a WebP, not a mislabelled
// file, not a larger picture.
func TestGoTakes(t *testing.T) {
	t.Parallel()

	small := encoded(t, "png", 40, 30)
	tests := []struct {
		name string
		data []byte
		mime string
		want bool
	}{
		{"jpeg", encoded(t, "jpeg", 40, 30), mimeJPEG, true},
		{"png", small, mimePNG, true},
		{"png said to be jpeg", small, mimeJPEG, true}, // DecodeConfig sniffs the format
		{"heic", small, "image/heic", false},
		{"webp", small, "image/webp", false},
		{"no type", small, "", false},
		{"not a picture", []byte("hello"), mimeJPEG, false},
		{"4096 wide", encoded(t, "png", maxGoSide, 1), mimePNG, true},
		{"4097 wide", encoded(t, "png", maxGoSide+1, 1), mimePNG, false},
		{"4097 tall", encoded(t, "png", 1, maxGoSide+1), mimePNG, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, goTakes(tc.data, tc.mime))
		})
	}
}

// TestReadImage checks the bounds of a picked picture's bytes.
func TestReadImage(t *testing.T) {
	t.Parallel()

	b, err := readImage(strings.NewReader("abc"))
	require.NoError(t, err)
	assert.Equal(t, "abc", string(b))

	_, err = readImage(strings.NewReader(""))
	require.ErrorIs(t, err, core.ErrInvalid)

	_, err = readImage(io.LimitReader(zeros{}, maxImageBytes+1))
	require.ErrorIs(t, err, core.ErrInvalid)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// encoded returns a w × h picture encoded as format ("png", "jpeg").
func encoded(t *testing.T, format string, w, h int) []byte {
	t.Helper()

	img := image.NewGray(image.Rect(0, 0, w, h))
	img.SetGray(0, 0, color.Gray{Y: 200})
	var buf bytes.Buffer
	switch format {
	case "jpeg":
		require.NoError(t, jpeg.Encode(&buf, img, nil))
	default:
		require.NoError(t, png.Encode(&buf, img))
	}

	return buf.Bytes()
}
