//go:build ios

// Adapted from jplayer-mobile/internal/sys/link_ios.go at 49de3c2
// (resourceValue: getResourceValue:forKey:error: after
// removeCachedResourceValueForKey:) and
// jplayer-mobile/internal/sys/a11ydump_ios.go at 49de3c2 (loadA11yTraits:
// a constant's value read through dl from its framework). The build tag
// ios covers tvOS too: GOOS=ios with -tags tvos.

package sys

import (
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

// foundationPath is Foundation's binary, which exports the resource keys.
const foundationPath = "/System/Library/Frameworks/Foundation.framework/Foundation"

// backupKeySym is the NSURLResourceKey of NSURL.h whose value
// excludedFromBackup reads: its value is not always its name
// (NSURLPathKey is "_NSURLPathKey"), so it is read from Foundation.
const backupKeySym = "NSURLIsExcludedFromBackupKey"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// backupKey is Foundation's NSURLIsExcludedFromBackupKey, read once; the
// NSString is a constant of the framework, never released.
var backupKey struct {
	once sync.Once
	key  objc.ID
	err  error
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// loadBackupKey loads Foundation and reads the key's value, once.
func loadBackupKey() (objc.ID, error) {
	backupKey.once.Do(func() {
		if err := objc.Init(); err != nil {
			backupKey.err = err

			return
		}
		if err := objc.LoadFramework("Foundation"); err != nil {
			backupKey.err = err

			return
		}
		lib, err := dl.Open(foundationPath)
		if err != nil {
			backupKey.err = err

			return
		}
		var addr dl.Fn
		if err := lib.Bind(dl.Sym{Fn: &addr, Name: backupKeySym}); err != nil || addr == 0 {
			backupKey.err = fmt.Errorf("%s: %w", backupKeySym, core.ErrNotFound)

			return
		}
		// The symbol's address holds the constant: an NSString *.
		backupKey.key = *(*objc.ID)(unsafe.Pointer(dl.Bytes(uintptr(addr)))) //nolint:gosec // G103: Foundation's constant
		if backupKey.key == 0 {
			backupKey.err = fmt.Errorf("%s: nil: %w", backupKeySym, core.ErrNotFound)
		}
	})

	return backupKey.key, backupKey.err
}

// excludeFromBackup sets NSURLIsExcludedFromBackupKey to YES on the
// file URL of path. Every object but the path's NSString is autoreleased
// in the pool.
func excludeFromBackup(path string) error {
	key, err := loadBackupKey()
	if err != nil {
		return err
	}
	objc.WithPool(func() {
		url := fileURL(path)
		if url == 0 {
			err = fmt.Errorf("no URL: %w", core.ErrInvalid)

			return
		}
		yes := objc.Call(objc.ID(objc.GetClass("NSNumber")), objc.Sel("numberWithBool:"), 1)
		var errID objc.ID
		if !objc.Send[bool](url, objc.Sel("setResourceValue:forKey:error:"), yes, key, unsafe.Pointer(&errID)) { //nolint:gosec // G103: Foundation writes the error there during the call
			err = nsError(errID)
		}
	})

	return err
}

// excludedFromBackup reads NSURLIsExcludedFromBackupKey of path's file
// URL, its cached value removed first.
func excludedFromBackup(path string) (bool, error) {
	key, err := loadBackupKey()
	if err != nil {
		return false, err
	}
	var on bool
	objc.WithPool(func() {
		url := fileURL(path)
		if url == 0 {
			err = fmt.Errorf("no URL: %w", core.ErrInvalid)

			return
		}
		objc.Call(url, objc.Sel("removeCachedResourceValueForKey:"), uintptr(key))
		var v, errID objc.ID
		if !objc.Send[bool](url, objc.Sel("getResourceValue:forKey:error:"), unsafe.Pointer(&v), key, unsafe.Pointer(&errID)) { //nolint:gosec // G103: Foundation writes the value and the error there during the call
			err = nsError(errID)

			return
		}
		on = v != 0 && objc.CallBool(v, objc.Sel("boolValue"))
	})

	return on, err
}

// fileURL returns +[NSURL fileURLWithPath:] of path, autoreleased; inside
// a pool. The path's NSString is released at once.
func fileURL(path string) objc.ID {
	ns := objc.NSString(path)
	if ns == 0 {
		return 0
	}
	defer objc.Release(ns)

	return objc.Call(objc.ID(objc.GetClass("NSURL")), objc.Sel("fileURLWithPath:"), uintptr(ns))
}

// nsError describes the NSError e (autoreleased, not ours to release).
func nsError(e objc.ID) error {
	if e == 0 {
		return fmt.Errorf("refused without an error: %w", core.ErrInvalid)
	}
	domain := objc.GoString(objc.Call(e, objc.Sel("domain")))
	code := int64(objc.Call(e, objc.Sel("code")))
	text := objc.GoString(objc.Call(e, objc.Sel("localizedDescription")))

	return fmt.Errorf("%s %d: %s: %w", domain, code, text, core.ErrInvalid)
}
