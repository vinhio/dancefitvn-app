//go:build darwin && !tvos

package sys

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestSystemJPEG checks the JPEG ImageIO makes of a picture Go does not
// take as it is: at most 4096 px, upright (red above), no EXIF left to
// turn it again; a HEIC (made by sips, where the Mac has it) too.
func TestSystemJPEG(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
		w, h int
	}{
		{"27 MP JPEG on its side", sideways(t, 4500, 6000), 3072, 4096},
		{"panorama PNG", pngOf(t, upright(5000, 1000)), 4096, 819},
		{"small PNG", pngOf(t, upright(300, 200)), 300, 200},
	}
	if heic := heicOf(t, sideways(t, 600, 800)); heic != nil {
		tests = append(tests, struct {
			name string
			data []byte
			w, h int
		}{"HEIC on its side", heic, 600, 800})
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			out, err := systemJPEG(tc.data)
			require.NoError(t, err)
			img, err := jpeg.Decode(bytes.NewReader(out))
			require.NoError(t, err)
			assert.InDelta(t, tc.w, img.Bounds().Dx(), 1)
			assert.InDelta(t, tc.h, img.Bounds().Dy(), 1)
			b := img.Bounds()
			assert.True(t, reddish(img.At(b.Dx()/2, b.Dy()/10)), "top %v", img.At(b.Dx()/2, b.Dy()/10))
			assert.False(t, reddish(img.At(b.Dx()/2, b.Dy()*9/10)), "bottom %v", img.At(b.Dx()/2, b.Dy()*9/10))
			assert.True(t, goTakes(out, mimeJPEG), "Go takes what ImageIO made")
		})
	}
	_, err := systemJPEG([]byte("not a picture"))
	require.ErrorIs(t, err, core.ErrInvalid)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// upright returns a w × h picture: red above, blue below.
func upright(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		c := color.RGBA{R: 220, G: 40, B: 50, A: 255}
		if y >= h/2 {
			c = color.RGBA{R: 40, G: 70, B: 210, A: 255}
		}
		for x := range w {
			img.SetRGBA(x, y, c)
		}
	}

	return img
}

// sideways returns a JPEG of upright(w, h) stored on its side with EXIF
// orientation 6, as a phone's camera stores a portrait.
func sideways(t *testing.T, w, h int) []byte {
	t.Helper()

	up := upright(w, h)
	stored := image.NewRGBA(image.Rect(0, 0, h, w))
	for v := range w {
		for u := range h {
			stored.SetRGBA(u, v, up.RGBAAt(w-1-v, u))
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, stored, &jpeg.Options{Quality: 90}))
	j := buf.Bytes()
	tiff := []byte("MM")
	for _, v := range []any{uint16(42), uint32(8), uint16(1), uint16(0x0112), uint16(3), uint32(1), uint16(6), uint16(0), uint32(0)} {
		tiff, _ = binary.Append(tiff, binary.BigEndian, v)
	}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := binary.BigEndian.AppendUint16([]byte{0xFF, 0xE1}, uint16(2+len(payload)))
	out := append(append([]byte{}, j[:2]...), append(seg, payload...)...)

	return append(out, j[2:]...)
}

// pngOf returns img as a PNG.
func pngOf(t *testing.T, img image.Image) []byte {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	return buf.Bytes()
}

// heicOf returns the HEIC sips makes of the JPEG j, nil where the Mac
// has no sips (the simulator, a bare Mac).
func heicOf(t *testing.T, j []byte) []byte {
	t.Helper()

	sips, err := exec.LookPath("sips")
	if err != nil {
		return nil
	}
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.jpg"), filepath.Join(dir, "out.heic")
	require.NoError(t, os.WriteFile(in, j, 0o600))
	if err := exec.Command(sips, "-s", "format", "heic", in, "--out", out).Run(); err != nil { //nolint:gosec // G204: sips of the test's own files
		t.Logf("no HEIC: %v", err)

		return nil
	}
	b, err := os.ReadFile(out) //nolint:gosec // G304: the test's own file
	require.NoError(t, err)

	return b
}

// reddish tells whether c is the sample's red.
func reddish(c color.Color) bool {
	r, g, b, _ := c.RGBA()

	return r>>8 > 150 && g>>8 < 100 && b>>8 < 110
}
