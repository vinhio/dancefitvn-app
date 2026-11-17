//go:build ios

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// deviceName reads [[UIDevice currentDevice] name] inside a pool of its
// own: both objects are autoreleased, none is retained, and the text is
// copied before the pool drains.
func deviceName() (string, error) {
	if err := objc.Init(); err != nil {
		return "", fmt.Errorf("sys: device name: %w", err)
	}
	if err := objc.LoadFramework("UIKit"); err != nil {
		return "", fmt.Errorf("sys: device name: %w", err)
	}
	cls := objc.GetClass("UIDevice")
	if cls == 0 {
		return "", fmt.Errorf("sys: device name: no UIDevice: %w", core.ErrUnsupported)
	}
	var name string
	objc.WithPool(func() {
		dev := objc.Call(objc.ID(cls), objc.Sel("currentDevice"))
		if dev != 0 {
			name = objc.GoString(objc.Call(dev, objc.Sel("name")))
		}
	})

	return name, nil
}
