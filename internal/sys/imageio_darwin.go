//go:build darwin && !tvos

// New in this app (no jPlayer source): jPlayer's iOS picker hands Go any
// picture, so a 48 MP one is decoded whole. ImageIO plays here the part
// BitmapFactory plays on Android (pickimage_android.go, systemJPEG):
// decode smaller, turn by the EXIF orientation, make a JPEG. Its C
// functions are called through foundation/ffi/dl and its constants read
// with foundation/ffi/cf, as backup_ios.go reads Foundation's. Built on
// the Mac too (darwin covers GOOS=ios), where imageio_darwin_test.go
// checks it; tvOS has no picker.

package sys

import (
	"fmt"
	"sync"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/cf"
	"github.com/gdeskdev/desk/foundation/ffi/dl"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The binaries of the frameworks the JPEG is made with.
const (
	imageIOPath = "/System/Library/Frameworks/ImageIO.framework/ImageIO"
	cfPath      = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
)

// jpegUTI is the type of the destination ImageIO writes.
const jpegUTI = "public.jpeg"

// imageIO holds ImageIO's functions and option keys, looked up once. The
// keys are the framework's constants, never released.
var imageIO struct {
	once sync.Once
	err  error

	createSource, createThumbnail, createDest, addImage, finalize, dataCreateMutable dl.Fn

	keyAlways, keyTransform, keyMaxSize, keyQuality cf.Ref
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// systemJPEG decodes the picture of data with ImageIO, its longest side
// at most maxGoSide (CGImageSourceCreateThumbnailAtIndex, which decodes
// a large JPEG or HEIC smaller instead of whole), turned upright by its
// EXIF orientation (kCGImageSourceCreateThumbnailWithTransform), and
// returns it as a JPEG of quality systemJPEGQuality. Any thread: ImageIO
// is thread-safe. Every object it makes is released before it returns.
func systemJPEG(data []byte) ([]byte, error) {
	if err := loadImageIO(); err != nil {
		return nil, err
	}
	iio := &imageIO
	in := cf.Data(data)
	defer cf.Release(in)
	src := cf.Ref(iio.createSource.Call(uintptr(in), 0))
	if src == 0 {
		return nil, fmt.Errorf("not a picture ImageIO reads: %w", core.ErrInvalid)
	}
	defer cf.Release(src)
	side := cf.Int(maxGoSide)
	opts := cf.Dictionary(iio.keyAlways, cf.True(), iio.keyTransform, cf.True(), iio.keyMaxSize, side)
	cf.Release(side) // the dictionary retains it
	img := cf.Ref(iio.createThumbnail.Call(uintptr(src), 0, uintptr(opts)))
	cf.Release(opts)
	if img == 0 {
		return nil, fmt.Errorf("not a picture ImageIO decodes: %w", core.ErrInvalid)
	}
	defer cf.Release(img)
	out := cf.Ref(iio.dataCreateMutable.Call(0, 0))
	if out == 0 {
		return nil, fmt.Errorf("no CFMutableData: %w", core.ErrInvalid)
	}
	defer cf.Release(out)
	typ := cf.String(jpegUTI)
	dest := cf.Ref(iio.createDest.Call(uintptr(out), uintptr(typ), 1, 0))
	cf.Release(typ)
	if dest == 0 {
		return nil, fmt.Errorf("no JPEG destination: %w", core.ErrUnsupported)
	}
	defer cf.Release(dest)
	q := cf.Float(systemJPEGQuality / 100.0)
	props := cf.Dictionary(iio.keyQuality, q)
	cf.Release(q)
	iio.addImage.Call(uintptr(dest), uintptr(img), uintptr(props))
	cf.Release(props)
	if !iio.finalize.Bool(uintptr(dest)) {
		return nil, fmt.Errorf("no JPEG from the picture: %w", core.ErrInvalid)
	}

	return cf.Bytes(out), nil
}

// loadImageIO opens ImageIO and CoreFoundation and binds what systemJPEG
// calls, once; its failure is kept.
func loadImageIO() error {
	imageIO.once.Do(func() {
		imageIO.err = bindImageIO()
	})

	return imageIO.err
}

// bindImageIO binds ImageIO's functions and reads its option keys.
func bindImageIO() error {
	if err := cf.Init(); err != nil {
		return err
	}
	lib, err := dl.Open(imageIOPath)
	if err != nil {
		return err
	}
	iio := &imageIO
	if err := lib.Bind(
		dl.Sym{Fn: &iio.createSource, Name: "CGImageSourceCreateWithData"},
		dl.Sym{Fn: &iio.createThumbnail, Name: "CGImageSourceCreateThumbnailAtIndex"},
		dl.Sym{Fn: &iio.createDest, Name: "CGImageDestinationCreateWithData"},
		dl.Sym{Fn: &iio.addImage, Name: "CGImageDestinationAddImage"},
		dl.Sym{Fn: &iio.finalize, Name: "CGImageDestinationFinalize"},
	); err != nil {
		return err
	}
	keys := []struct {
		ref  *cf.Ref
		name string
	}{
		{&iio.keyAlways, "kCGImageSourceCreateThumbnailFromImageAlways"},
		{&iio.keyTransform, "kCGImageSourceCreateThumbnailWithTransform"},
		{&iio.keyMaxSize, "kCGImageSourceThumbnailMaxPixelSize"},
		{&iio.keyQuality, "kCGImageDestinationLossyCompressionQuality"},
	}
	for _, k := range keys {
		if *k.ref, err = cf.Constant(lib.Handle(), k.name); err != nil {
			return fmt.Errorf("%s: %w: %w", k.name, core.ErrNotFound, err)
		}
	}
	cfLib, err := dl.Open(cfPath)
	if err != nil {
		return err
	}

	return cfLib.Bind(dl.Sym{Fn: &iio.dataCreateMutable, Name: "CFDataCreateMutable"})
}
