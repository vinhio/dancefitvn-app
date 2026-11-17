// Adapted from jplayer-mobile/e2e/sys/pick_ios_test.go at 49de3c2
// (TestPickVideosKeepsThem's answer: UIDragItems of NSItemProviders of
// files standing for PHPickerResults). New: the pictures the provider
// gives, the ImageIO path of the large ones, and a pick by the user
// through the real PHPicker (TestPickImageByUser, driven by
// local/p6/pick-tap.sh).

//go:build ios && !tvos

package sys

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/ffi/objc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/avatar"
	"dancefitvn-app/internal/sys"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The colours of a sample picture, upright: red above, blue below.
var (
	sampleTop    = color.RGBA{R: 220, G: 40, B: 50, A: 255}
	sampleBottom = color.RGBA{R: 40, G: 70, B: 210, A: 255}
)

// pickTapEnv names the rounds TestPickImageByUser waits for (set through
// SIMCTL_CHILD_DFV_PICK_TAP by local/p6/pick-tap.sh); unset, it skips.
const pickTapEnv = "DFV_PICK_TAP"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestPickImageGivesPicture answers the photo picker as PhotosUI does
// when the user picks one picture: a result whose item provider writes
// it to a file of its own. A JPEG or PNG of at most 4096 px comes as it
// is (the camera's EXIF orientation kept, for internal/avatar to apply);
// a larger one comes as a JPEG ImageIO made, at most 4096 px, upright.
// Each decodes to a working picture red above and blue below.
func TestPickImageGivesPicture(t *testing.T) { //nolint:paralleltest // one app a process
	dir := t.TempDir()
	tests := []struct {
		name   string
		file   string
		data   []byte
		mime   string
		asIs   bool
		upW    int // the working picture's size, upright
		upH    int
		maxOut int // the longest side handed over
	}{
		{"camera JPEG, EXIF 6", "camera.jpg", exif6JPEG(t, 2400, 3200), "image/jpeg", true, 1560, 2080, 3200},
		{"screenshot PNG", "screen.png", encodePNG(t, sample(1170, 2532)), "image/png", true, 1170, 2532, 2532},
		{"27 MP JPEG, EXIF 6", "large.jpg", exif6JPEG(t, 4500, 6000), "image/jpeg", false, 1560, 2080, 4096},
		{"panorama PNG", "pano.png", encodePNG(t, sample(5000, 1000)), "image/jpeg", false, 4096, 819, 4096},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.file)
			require.NoError(t, os.WriteFile(path, tc.data, 0o600))
			ch := make(chan picture, 1)
			var err error
			onUI(t, func() {
				err = sys.PickImage(canvas(), dispatch, func(b []byte, mime string, err error) { ch <- picture{b, mime, err} })
			})
			require.NoError(t, err)
			expectShown(t, shownImage)
			answerPicker(t, path)
			got := await(t, ch, within, "the photo picker's answer")
			require.NoError(t, got.err)
			assert.Equal(t, tc.mime, got.mime)
			if tc.asIs {
				assert.Equal(t, tc.data, got.data, "handed over as it is")
			}
			cfg, _, err := image.DecodeConfig(bytes.NewReader(got.data))
			require.NoError(t, err)
			assert.LessOrEqual(t, max(cfg.Width, cfg.Height), tc.maxOut)
			t.Logf("%s: %d bytes %s, %d×%d", tc.name, len(got.data), got.mime, cfg.Width, cfg.Height)
			work, err := avatar.Decode(got.data)
			require.NoError(t, err)
			assert.InDelta(t, tc.upW, work.Bounds().Dx(), 2, "upright width")
			assert.InDelta(t, tc.upH, work.Bounds().Dy(), 2, "upright height")
			expectUpright(t, work)
			expectGone(t)
		})
	}
}

// TestPickImageByUser opens the real photo picker and waits for the user
// (or local/p6/pick-tap.sh's XCUITest taps) to pick a photo of the
// simulator's library, once per round DFV_PICK_TAP names. Each answer is
// decoded and exported as the avatar would be; both are saved in the
// app's tmp folder (picked-<n>.raw, picked-<n>.jpg) for the script to
// copy. It skips without DFV_PICK_TAP.
func TestPickImageByUser(t *testing.T) { //nolint:paralleltest // one app a process
	rounds, _ := strconv.Atoi(os.Getenv(pickTapEnv))
	if rounds <= 0 {
		t.Skip("a pick by the user: run local/p6/pick-tap.sh")
	}
	for n := 1; n <= rounds; n++ {
		ch := make(chan picture, 1)
		var err error
		onUI(t, func() {
			err = sys.PickImage(canvas(), dispatch, func(b []byte, mime string, err error) { ch <- picture{b, mime, err} })
		})
		require.NoError(t, err)
		expectShown(t, shownImage)
		fmt.Printf("devtest-pick: shown %d\n", n)
		start := time.Now()
		got := await(t, ch, 3*time.Minute, "the user's pick")
		require.NoError(t, got.err, "round %d", n)
		cfg, _, err := image.DecodeConfig(bytes.NewReader(got.data))
		require.NoError(t, err)
		decodeStart := time.Now()
		work, err := avatar.Decode(got.data)
		require.NoError(t, err)
		decodeTook := time.Since(decodeStart)
		out, err := avatar.Export(work, avatar.Picked)
		require.NoError(t, err)
		tmp := os.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmp, fmt.Sprintf("picked-%d.raw", n)), got.data, 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmp, fmt.Sprintf("picked-%d.jpg", n)), out, 0o600))
		fmt.Printf("devtest-pick: answer %d mime=%s bytes=%d size=%dx%d upright=%dx%d decode=%v export=%d after=%v\n",
			n, got.mime, len(got.data), cfg.Width, cfg.Height, work.Bounds().Dx(), work.Bounds().Dy(),
			decodeTook.Round(time.Millisecond), len(out), time.Since(start).Round(time.Millisecond))
		expectUpright(t, work)
		expectGone(t)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// answerPicker answers the photo picker showing with one result whose
// item provider gives the file path (a UIDragItem stands for a
// PHPickerResult: both answer itemProvider), as PhotosUI does once the
// user tapped a photo.
func answerPicker(t *testing.T, path string) {
	t.Helper()

	onUI(t, func() {
		objc.WithPool(func() {
			p := objc.NSString(path)
			u := objc.Call(objc.ID(objc.GetClass("NSURL")), objc.Sel("fileURLWithPath:"), uintptr(p))
			objc.Release(p)
			provider := objc.Call(objc.Call(objc.ID(objc.GetClass("NSItemProvider")), objc.Sel("alloc")), objc.Sel("initWithContentsOfURL:"), uintptr(u))
			if provider == 0 {
				t.Errorf("no item provider of %s", path) // not Fatal: the UI thread must go on

				return
			}
			item := objc.Call(objc.Call(objc.ID(objc.GetClass("UIDragItem")), objc.Sel("alloc")), objc.Sel("initWithItemProvider:"), uintptr(provider))
			objc.Release(provider)
			results := objc.Call(objc.ID(objc.GetClass("NSMutableArray")), objc.Sel("array"))
			objc.Call(results, objc.Sel("addObject:"), uintptr(item))
			objc.Release(item)
			vc := presentedController()
			objc.Call(objc.Call(vc, objc.Sel("delegate")), objc.Sel("picker:didFinishPicking:"), uintptr(vc), uintptr(results))
		})
	})
}

// expectUpright checks a working picture shows a sample upright: red at
// the top, blue at the bottom.
func expectUpright(t *testing.T, img *image.RGBA) {
	t.Helper()

	b := img.Bounds()
	// Left of the centre: the samples of local/p6/_genphoto draw an arrow
	// up the middle.
	top, bottom := img.RGBAAt(b.Dx()/8, b.Dy()/10), img.RGBAAt(b.Dx()/8, b.Dy()*9/10)
	assert.True(t, near(top, sampleTop, 24), "top %v", top)
	assert.True(t, near(bottom, sampleBottom, 24), "bottom %v", bottom)
}

// sample returns a w × h picture, upright: red above, blue below, a white
// band in the middle.
func sample(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		c := sampleTop
		switch {
		case y >= h*45/100 && y < h*55/100:
			c = color.RGBA{R: 255, G: 255, B: 255, A: 255}
		case y >= h/2:
			c = sampleBottom
		}
		for x := range w {
			img.SetRGBA(x, y, c)
		}
	}

	return img
}

// exif6JPEG returns a JPEG of a w × h sample stored as a phone's camera
// stores a portrait photo: on its side (h × w), turned back by EXIF
// orientation 6 (rotate 90° clockwise to show).
func exif6JPEG(t *testing.T, w, h int) []byte {
	t.Helper()

	up := sample(w, h)
	stored := image.NewRGBA(image.Rect(0, 0, h, w))
	for v := range w {
		for u := range h {
			stored.SetRGBA(u, v, up.RGBAAt(w-1-v, u))
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, stored, &jpeg.Options{Quality: 90}))

	return withEXIF6(buf.Bytes())
}

// withEXIF6 returns the JPEG j with an APP1 EXIF segment (big-endian, as
// an iPhone writes it) whose first IFD holds Orientation 6, after its SOI.
func withEXIF6(j []byte) []byte {
	tiff := []byte("MM")
	tiff = binary.BigEndian.AppendUint16(tiff, 42)
	tiff = binary.BigEndian.AppendUint32(tiff, 8)
	tiff = binary.BigEndian.AppendUint16(tiff, 1)
	tiff = binary.BigEndian.AppendUint16(tiff, 0x0112)
	tiff = binary.BigEndian.AppendUint16(tiff, 3)
	tiff = binary.BigEndian.AppendUint32(tiff, 1)
	tiff = binary.BigEndian.AppendUint16(tiff, 6)
	tiff = binary.BigEndian.AppendUint16(tiff, 0)
	tiff = binary.BigEndian.AppendUint32(tiff, 0)
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := binary.BigEndian.AppendUint16([]byte{0xFF, 0xE1}, uint16(2+len(payload)))
	out := append(append([]byte{}, j[:2]...), append(seg, payload...)...)

	return append(out, j[2:]...)
}

// encodePNG returns img as a PNG.
func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	return buf.Bytes()
}

// near tells whether two colours differ by at most d in each channel.
func near(a, b color.RGBA, d int) bool {
	diff := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }

	return diff(a.R, b.R) <= d && diff(a.G, b.G) <= d && diff(a.B, b.B) <= d
}
