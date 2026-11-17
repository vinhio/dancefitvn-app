//go:build ios

// The build tag ios covers tvOS too (GOOS=ios with -tags tvos), whose
// Foundation lacks NSURLVolumeAvailableCapacityForImportantUsageKey
// (API_UNAVAILABLE(tvos)): statfs answers there, as the Apple TV
// simulator showed. The keys are read from Foundation as backup_ios.go
// reads its own.

package sys

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/dl"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The NSURLResourceKeys of NSURL.h whose values freeSpace reads (iOS 11):
// read from Foundation, as their values need not be their names.
const (
	importantUsageKeySym = "NSURLVolumeAvailableCapacityForImportantUsageKey"
	totalCapacityKeySym  = "NSURLVolumeTotalCapacityKey"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// spaceKeys are Foundation's two keys, read once; the NSStrings are
// constants of the framework, never released.
var spaceKeys struct {
	once      sync.Once
	important objc.ID
	total     objc.ID
	err       error
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// freeSpace asks Foundation the volume's capacity for important usage
// and its size; statfs answers when Foundation cannot (no key, an error
// of the URL), free then counting no purgeable space.
func freeSpace(path string) (free, total int64, err error) {
	free, total, err = volumeSpace(path)
	if err == nil {
		return free, total, nil
	}
	free, total, serr := statfsSpace(path)
	if serr != nil {
		return 0, 0, errors.Join(err, serr)
	}

	return free, total, nil
}

// volumeSpace reads the two keys of path's file URL. Every object but
// the path's NSString is autoreleased in the pool.
func volumeSpace(path string) (free, total int64, err error) {
	if err := loadSpaceKeys(); err != nil {
		return 0, 0, err
	}
	objc.WithPool(func() {
		url := fileURL(path)
		if url == 0 {
			err = fmt.Errorf("no URL: %w", core.ErrInvalid)

			return
		}
		if free, err = int64Resource(url, spaceKeys.important); err != nil {
			return
		}
		total, err = int64Resource(url, spaceKeys.total)
	})
	if err == nil && (total <= 0 || free < 0) {
		err = fmt.Errorf("volume of %d bytes, %d free: %w", total, free, core.ErrInvalid)
	}

	return free, total, err
}

// int64Resource reads the NSNumber value of key of url, inside a pool.
func int64Resource(url, key objc.ID) (int64, error) {
	var v, errID objc.ID
	if !objc.Send[bool](url, objc.Sel("getResourceValue:forKey:error:"), unsafe.Pointer(&v), key, unsafe.Pointer(&errID)) { //nolint:gosec // G103: Foundation writes the value and the error there during the call
		return 0, nsError(errID)
	}
	if v == 0 {
		return 0, fmt.Errorf("no value: %w", core.ErrNotFound)
	}

	return int64(objc.Call(v, objc.Sel("longLongValue"))), nil //nolint:gosec // G115: a long long in the result register
}

// loadSpaceKeys loads Foundation and reads the keys' values, once.
func loadSpaceKeys() error {
	spaceKeys.once.Do(func() {
		if err := objc.Init(); err != nil {
			spaceKeys.err = err

			return
		}
		if err := objc.LoadFramework("Foundation"); err != nil {
			spaceKeys.err = err

			return
		}
		lib, err := dl.Open(foundationPath)
		if err != nil {
			spaceKeys.err = err

			return
		}
		var important, total dl.Fn
		if err := lib.Bind(
			dl.Sym{Fn: &important, Name: importantUsageKeySym},
			dl.Sym{Fn: &total, Name: totalCapacityKeySym},
		); err != nil || important == 0 || total == 0 {
			spaceKeys.err = fmt.Errorf("volume capacity keys: %w", errors.Join(err, core.ErrNotFound))

			return
		}
		// Each symbol's address holds the constant: an NSString *.
		spaceKeys.important = *(*objc.ID)(unsafe.Pointer(dl.Bytes(uintptr(important)))) //nolint:gosec // G103: Foundation's constant
		spaceKeys.total = *(*objc.ID)(unsafe.Pointer(dl.Bytes(uintptr(total))))         //nolint:gosec // G103: Foundation's constant
		if spaceKeys.important == 0 || spaceKeys.total == 0 {
			spaceKeys.err = fmt.Errorf("volume capacity keys: nil: %w", core.ErrNotFound)
		}
	})

	return spaceKeys.err
}
