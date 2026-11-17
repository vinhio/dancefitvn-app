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

// idleTimerDisabled asks [[UIApplication sharedApplication]
// isIdleTimerDisabled]; the shared application is not retained (it lives
// for the process) and nothing is autoreleased.
func idleTimerDisabled() (bool, error) {
	if err := objc.Init(); err != nil {
		return false, fmt.Errorf("sys: idle timer: %w", err)
	}
	if err := objc.LoadFramework("UIKit"); err != nil {
		return false, fmt.Errorf("sys: idle timer: %w", err)
	}
	cls := objc.GetClass("UIApplication")
	if cls == 0 {
		return false, fmt.Errorf("sys: idle timer: no UIApplication: %w", core.ErrUnsupported)
	}
	app := objc.Call(objc.ID(cls), objc.Sel("sharedApplication"))
	if app == 0 {
		return false, fmt.Errorf("sys: idle timer: app not launched: %w", core.ErrInvalid)
	}

	return objc.CallBool(app, objc.Sel("isIdleTimerDisabled")), nil
}
