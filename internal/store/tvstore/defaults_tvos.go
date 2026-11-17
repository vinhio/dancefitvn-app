// Adapted from jplayer-tv/internal/store/tvstore/defaults_tvos.go at 781bee5.

//go:build ios && tvos

package tvstore

import (
	"fmt"
	"unsafe"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// errSync reports that NSUserDefaults' synchronize answered NO: the
// system did not write the changes now. It wraps core.ErrUnsupported, the
// sentinel of a request the platform declines (the changes stay pending
// in cfprefsd).
var errSync = fmt.Errorf("tvstore: synchronize returned false: %w", core.ErrUnsupported)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewDefaults loads Foundation and checks that NSUserDefaults and NSData
// are there; it fails with an error wrapping core.ErrUnsupported when they
// are not. prefix goes before every key (Prefix for the app).
func NewDefaults(prefix string) (*Defaults, error) {
	if err := objc.Init(); err != nil {
		return nil, fmt.Errorf("tvstore: new defaults: %w: %w", core.ErrUnsupported, err)
	}
	if err := objc.LoadFramework("Foundation"); err != nil {
		return nil, fmt.Errorf("tvstore: new defaults: %w: %w", core.ErrUnsupported, err)
	}
	for _, name := range []string{"NSUserDefaults", "NSData"} {
		if objc.GetClass(name) == 0 {
			return nil, fmt.Errorf("tvstore: new defaults: no class %s: %w", name, core.ErrUnsupported)
		}
	}
	var ok bool
	objc.WithPool(func() { ok = standard() != 0 })
	if !ok {
		return nil, fmt.Errorf("tvstore: new defaults: no standardUserDefaults: %w", core.ErrUnsupported)
	}

	return &Defaults{prefix: prefix}, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Load returns a copy of the NSData under prefix + key. An absent key
// wraps core.ErrNotFound; a value that is not an NSData (written by
// someone else) wraps core.ErrInvalid.
func (d *Defaults) Load(key string) ([]byte, error) {
	if d == nil {
		return nil, fmt.Errorf("tvstore: load %s: nil defaults: %w", key, core.ErrInvalid)
	}
	var (
		out []byte
		err error
	)
	objc.WithPool(func() {
		k := objc.NSString(d.prefix + key)
		defer objc.Release(k)
		obj := objc.Call(standard(), objc.Sel("objectForKey:"), uintptr(k))
		switch {
		case obj == 0:
			err = fmt.Errorf("tvstore: load %s: %w", key, core.ErrNotFound)
		case !objc.CallBool(obj, objc.Sel("isKindOfClass:"), uintptr(objc.GetClass("NSData"))):
			err = fmt.Errorf("tvstore: load %s: value is %s, not NSData: %w", key, className(obj), core.ErrInvalid)
		default:
			out = goBytes(obj)
		}
	})

	return out, err
}

// Save stores a copy of data, which must not be empty, as an NSData under
// prefix + key, then synchronizes. A synchronize that answers NO is an
// error, though NSUserDefaults keeps the value in memory and writes it
// later.
func (d *Defaults) Save(key string, data []byte) error {
	switch {
	case d == nil:
		return fmt.Errorf("tvstore: save %s: nil defaults: %w", key, core.ErrInvalid)
	case len(data) == 0:
		return fmt.Errorf("tvstore: save %s: empty data: %w", key, core.ErrInvalid)
	default:
	}
	objc.WithPool(func() {
		k := objc.NSString(d.prefix + key)
		defer objc.Release(k)
		objc.Call(standard(), objc.Sel("setObject:forKey:"), uintptr(bytesData(data)), uintptr(k))
	})
	if err := d.Sync(); err != nil {
		return fmt.Errorf("tvstore: save %s: %w", key, err)
	}

	return nil
}

// Sync asks NSUserDefaults to write its pending changes now
// (synchronize). On tvOS the write goes through cfprefsd and may still
// reach the disk later (T0.5): this narrows the window, it does not close
// it.
func (d *Defaults) Sync() error {
	if d == nil {
		return fmt.Errorf("tvstore: sync: nil defaults: %w", core.ErrInvalid)
	}
	var ok bool
	objc.WithPool(func() { ok = objc.CallBool(standard(), objc.Sel("synchronize")) })
	if !ok {
		return fmt.Errorf("tvstore: sync: %w", errSync)
	}

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// standard returns [NSUserDefaults standardUserDefaults], not owned. In a
// pool.
func standard() objc.ID {
	return objc.Call(objc.ID(objc.GetClass("NSUserDefaults")), objc.Sel("standardUserDefaults"))
}

// bytesData returns an autoreleased NSData copy of b, which must not be
// empty (bridge/backend/darwin/clipimage.go). In a pool.
func bytesData(b []byte) objc.ID {
	return objc.Send[objc.ID](objc.ID(objc.GetClass("NSData")), objc.Sel("dataWithBytes:length:"), unsafe.Pointer(&b[0]), uint64(len(b))) //nolint:gosec // G103: b is alive during the call, which copies it
}

// goBytes copies an NSData's bytes into Go memory; an empty NSData gives
// an empty, non-nil slice.
func goBytes(data objc.ID) []byte {
	n := int(objc.Call(data, objc.Sel("length")))
	if n == 0 {
		return []byte{}
	}
	ptr := objc.Send[*byte](data, objc.Sel("bytes"))

	return append([]byte(nil), unsafe.Slice(ptr, n)...) //nolint:gosec // G103: NSData's bytes, alive while data is
}

// className names obj's class, for messages. In a pool.
func className(obj objc.ID) string {
	return objc.GoString(objc.Call(objc.Call(obj, objc.Sel("class")), objc.Sel("description")))
}
