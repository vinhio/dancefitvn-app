//go:build ios

// Adapted from jplayer-mobile/internal/app/tz_ios.go at 49de3c2 (build tag
// ios covers tvOS too: GOOS=ios with -tags tvos).

package app

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// localZone returns the zone of +[NSTimeZone localTimeZone], read from
// its TZif data (-[NSTimeZone data]) so that its daylight saving rules
// come with it: Go on iOS has no zone database to load a name from.
func localZone() (loc *time.Location, err error) {
	objc.WithPool(func() {
		// A class property's getter: the object is not ours to release.
		tz := objc.Call(objc.ID(objc.GetClass("NSTimeZone")), objc.Sel("localTimeZone"))
		if tz == 0 {
			err = fmt.Errorf("local zone: %w", core.ErrNotFound)

			return
		}
		name := objc.GoString(objc.Call(tz, objc.Sel("name")))
		data := objc.Call(tz, objc.Sel("data"))
		if data == 0 {
			err = fmt.Errorf("local zone %q: no data: %w", name, core.ErrNotFound)

			return
		}
		n := int(objc.Call(data, objc.Sel("length")))
		p := objc.Send[*byte](data, objc.Sel("bytes"))
		if n == 0 || p == nil {
			err = fmt.Errorf("local zone %q: empty data: %w", name, core.ErrNotFound)

			return
		}
		// The bytes belong to the NSData: copied before the pool drains.
		tzif := append([]byte(nil), unsafe.Slice(p, n)...) //nolint:gosec // G103: NSData's bytes, alive while data is
		loc, err = time.LoadLocationFromTZData(name, tzif)
		if err != nil {
			err = fmt.Errorf("local zone %q: %w: %w", name, core.ErrInvalid, err)
		}
	})

	return loc, err
}
