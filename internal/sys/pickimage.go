// Adapted from jplayer-mobile/internal/sys/pickimage.go at 49de3c2 (F2,
// until gDesk's G2 picks a photo itself). Changed: no WebP (no
// golang.org/x/image here: a WebP comes to Go as a JPEG the system
// made), minSampledSide is 1560, and iOS makes a JPEG of a picture Go
// does not take as Android does (pickimage_ios.go, imageio_darwin.go).

package sys

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // image.DecodeConfig of a picked JPEG
	_ "image/png"  // and PNG
	"io"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// maxImageBytes bounds the bytes of a picked image: a 48 MP HEIC is about
// 12 MB, a JPEG of it 20 MB; past this the picture is refused, not read.
const maxImageBytes = 64 << 20

// maxGoSide is the longest side of a picture handed to Go as it is: past
// it, the system decodes it smaller first (Android's inSampleSize, iOS's
// ImageIO thumbnail), so Go's decoder never holds a 48 MP picture.
const maxGoSide = 4096

// minSampledSide is the longest side a picture Android decodes smaller
// keeps at least: twice the 260 DIP of the crop's picture at 3 px a DIP
// (sheet 15D, Q-P6-10), so the working picture still has pixels to
// average.
const minSampledSide = 1560

// systemJPEGQuality is the quality of the JPEG the system makes of a
// picture Go does not take as it is; the avatar is encoded again at 90.
const systemJPEGQuality = 95

// The image types Go decodes (internal/avatar): any other comes to Go as
// a JPEG the system made.
const (
	mimeJPEG = "image/jpeg"
	mimePNG  = "image/png"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// PickImage opens the system's picker of one picture (the prototype's
// #filePick, accept image/*, behind "Chụp ảnh mới" and "Chọn từ thư
// viện" of sheet 15D: index.html:628) and hands its bytes and media type
// to done, through d: a JPEG or a PNG of at most 4096 px on its longest
// side, which internal/avatar decodes and turns upright by the JPEG's
// EXIF orientation. A picture the system had to decode (a HEIC, a WebP,
// a larger one) comes upright already, as a JPEG of quality 95 without
// EXIF orientation. A user who canceled gives no bytes and ErrCanceled
// (errors.Is context.Canceled).
//
// On iOS it presents a PHPickerViewController (images only, one, the
// compatible representation, so that a HEIC comes as a JPEG): it needs no
// permission to the photo library. The picker's item provider writes the
// picture to a temporary file it deletes once its block returns, on a
// queue of its own: the bytes are read there, in the block, and a picture
// past 4096 px made smaller by ImageIO there too. On Android it starts the
// Photo Picker (MediaStore.ACTION_PICK_IMAGES) from API 33,
// ACTION_GET_CONTENT of image/* before; it reads the picture's bytes
// through a file descriptor off Java's main thread. A picture Go does not
// decode (HEIC, HEIF, GIF, WebP) or larger than 4096 px is decoded by
// BitmapFactory, smaller (inSampleSize) where it is large, turned by its
// EXIF orientation and made a JPEG at quality 95.
//
// Call it on the UI thread. A picker showing already is
// core.ErrDuplicate; on Android without gDesk's Java, core.ErrUnsupported;
// more than 64 MB of picture, core.ErrInvalid through done. A returned
// error means done will not run; otherwise done runs once, on the UI
// thread. Elsewhere than iOS (not tvOS) and Android it fails with
// core.ErrUnsupported.
func PickImage(c platform.Canvas, d Dispatch, done func(data []byte, mime string, err error)) error {
	if err := checkAsync("pick image", c, d, done != nil); err != nil {
		return err
	}

	return pickImage(c, d, done)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// readImage reads a picked picture from r, refusing one past
// maxImageBytes.
func readImage(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxImageBytes+1))
	switch {
	case err != nil:
		return nil, fmt.Errorf("sys: pick image: read: %w", err)
	case len(b) > maxImageBytes:
		return nil, fmt.Errorf("sys: pick image: larger than %d bytes: %w", maxImageBytes, core.ErrInvalid)
	case len(b) == 0:
		return nil, fmt.Errorf("sys: pick image: no bytes: %w", core.ErrInvalid)
	default:
		return b, nil
	}
}

// goTakes tells whether Go decodes the picture of data, said to be of
// type mime, as it is: a JPEG or PNG whose header Go reads, of at most
// maxGoSide on its longest side.
func goTakes(data []byte, mime string) bool {
	switch mime {
	case mimeJPEG, mimePNG:
	default:
		return false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return false
	}

	return cfg.Width > 0 && cfg.Height > 0 && max(cfg.Width, cfg.Height) <= maxGoSide
}

// sampleSize returns BitmapFactory's inSampleSize for a picture of w × h:
// the largest power of two that keeps its longest side at least
// minSampledSide (1 for a small picture).
func sampleSize(w, h int) int32 {
	side := max(w, h)
	s := int32(1)
	for side/int(s*2) >= minSampledSide {
		s *= 2
	}

	return s
}

// exifTransform returns how a picture of EXIF orientation o turns
// upright, as Android's own decoders do it (androidx ExifInterface,
// Glide): a rotation in degrees, clockwise, then a horizontal flip. An
// orientation of none or out of 1–8 is upright already.
func exifTransform(o int32) (degrees float32, flip bool) {
	switch o {
	case 2: // ORIENTATION_FLIP_HORIZONTAL
		return 0, true
	case 3: // ORIENTATION_ROTATE_180
		return 180, false
	case 4: // ORIENTATION_FLIP_VERTICAL
		return 180, true
	case 5: // ORIENTATION_TRANSPOSE
		return 90, true
	case 6: // ORIENTATION_ROTATE_90
		return 90, false
	case 7: // ORIENTATION_TRANSVERSE
		return -90, true
	case 8: // ORIENTATION_ROTATE_270
		return -90, false
	default:
		return 0, false
	}
}
