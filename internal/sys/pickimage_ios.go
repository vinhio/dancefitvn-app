//go:build ios && !tvos

// Adapted from jplayer-mobile/internal/sys/pickimage_ios.go at 49de3c2.
// Changed: iPhone only (tvOS has no photo picker), no WebP among the
// types asked for, the delegate is DFVPhotoPickerDelegate, and a picture
// Go does not take as it is (past 4096 px) is made a smaller JPEG by
// ImageIO in the provider's block (imageio_darwin.go).

package sys

import (
	"fmt"
	"os"
	"reflect"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// representationCompatible is PHPickerConfigurationAssetRepresentationModeCompatible
// of PhotosUI's PHPicker.h (Automatic 0, Current 1, Compatible 2): the
// picker transcodes what an app may not read, a HEIC to a JPEG.
const representationCompatible = 2

// imageTypes are the Uniform Type Identifiers of the pictures the picker
// hands over as they are, in the order it is asked for them, and their
// media types. Anything else (HEIC, GIF, WebP, RAW) is asked as
// public.jpeg, which the picker transcodes to.
var imageTypes = []struct{ uti, mime string }{
	{"public.jpeg", mimeJPEG},
	{"public.png", mimePNG},
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// pickImage presents a PHPickerViewController of one image, the
// compatible representation, over the top controller; its delegate,
// DFVPhotoPickerDelegate, answers (imagePicked).
func pickImage(c platform.Canvas, d Dispatch, done func([]byte, string, error)) error {
	const op = "pick image"
	if err := onMain(op); err != nil {
		return err
	}
	if kit.image.vc != 0 {
		if kit.image.waiting() {
			return busy(op)
		}
		imagePicked(kit.image.vc, 0) // gone without a word
	}
	cfgClass, pickerClass, filterClass := objc.GetClass("PHPickerConfiguration"), objc.GetClass("PHPickerViewController"), objc.GetClass("PHPickerFilter")
	if cfgClass == 0 || pickerClass == 0 || filterClass == 0 {
		return fmt.Errorf("sys: %s: no PHPickerViewController: %w", op, core.ErrUnsupported)
	}
	top, err := topController(op, c)
	if err != nil {
		return err
	}
	objc.WithPool(func() {
		cfg := objc.Call(objc.Call(objc.ID(cfgClass), objc.Sel("alloc")), objc.Sel("init"))
		objc.Call(cfg, objc.Sel("setFilter:"), uintptr(objc.Call(objc.ID(filterClass), objc.Sel("imagesFilter"))))
		objc.Call(cfg, objc.Sel("setSelectionLimit:"), 1)
		objc.Call(cfg, objc.Sel("setPreferredAssetRepresentationMode:"), representationCompatible)
		picker := objc.Call(objc.Call(objc.ID(pickerClass), objc.Sel("alloc")), objc.Sel("initWithConfiguration:"), uintptr(cfg))
		objc.Release(cfg) // the picker keeps a copy
		objc.Call(picker, objc.Sel("setDelegate:"), uintptr(kit.delegate))
		present(&kit.image, top, picker, d, done)
	})

	return nil
}

// imagePicked answers the photo picker picker, if it is the one waited
// on, and dismisses it (a PHPickerViewController leaves that to its
// delegate). results are its PHPickerResults, none or 0 for a cancel.
// The first result's item provider writes the picture to a file it
// deletes once the block returns, on a queue of its own: the block reads
// it there and hands the bytes on. The provider is retained until then.
func imagePicked(picker, results objc.ID) {
	if picker == 0 || kit.image.vc != picker {
		return
	}
	objc.Call(picker, objc.Sel("dismissViewControllerAnimated:completion:"), 1, 0)
	got := take(&kit.image)
	if results == 0 || objc.Call(results, objc.Sel("count")) == 0 {
		got.d(func() { got.done(nil, "", ErrCanceled) })

		return
	}
	provider := objc.Retain(objc.Call(objc.Call(results, objc.Sel("objectAtIndex:"), 0), objc.Sel("itemProvider")))
	uti, mime := imageTypeOf(provider)
	block := newBlock(func(args []reflect.Value) {
		data, mime, err := readPicked(idArg(args[0]), idArg(args[1]), mime)
		objc.Release(provider)
		if err != nil {
			data, mime = nil, ""
		}
		got.d(func() { got.done(data, mime, err) })
	}, idType, idType)
	withString(uti, func(ns objc.ID) {
		objc.Call(provider, objc.Sel("loadFileRepresentationForTypeIdentifier:completionHandler:"), uintptr(ns), uintptr(block))
	})
	objc.Release(block) // the provider holds its copy until it calls it
}

// imageTypeOf returns the type to ask provider for, and its media type:
// the first of imageTypes it has, or a JPEG, which the picker makes of
// anything else.
func imageTypeOf(provider objc.ID) (uti, mime string) {
	has := objc.Sel("hasItemConformingToTypeIdentifier:")
	for _, t := range imageTypes {
		ok := false
		withString(t.uti, func(ns objc.ID) { ok = objc.CallBool(provider, has, uintptr(ns)) })
		if ok {
			return t.uti, t.mime
		}
	}

	return imageTypes[0].uti, imageTypes[0].mime
}

// readPicked reads the file url the item provider wrote, said to be of
// type mime, in its block: the file is gone once the block returns.
// nsErr is the provider's failure. A picture Go does not take as it is
// comes back as the JPEG ImageIO made of it (systemJPEG).
func readPicked(url, nsErr objc.ID, mime string) ([]byte, string, error) {
	if url == 0 {
		var why string
		objc.WithPool(func() { why = nsErrorText(nsErr) })

		return nil, "", fmt.Errorf("sys: pick image: %s: %w", why, core.ErrNotFound)
	}
	var path string
	objc.WithPool(func() { path = objc.GoString(objc.Call(url, objc.Sel("path"))) })
	f, err := os.Open(path) //nolint:gosec // G304: the file the item provider wrote
	if err != nil {
		return nil, "", fmt.Errorf("sys: pick image: %w", err)
	}
	data, err := readImage(f)
	_ = f.Close() // read only
	if err != nil {
		return nil, "", err
	}
	if goTakes(data, mime) {
		return data, mime, nil
	}
	if data, err = systemJPEG(data); err != nil {
		return nil, "", fmt.Errorf("sys: pick image: %w", err)
	}

	return data, mimeJPEG, nil
}
