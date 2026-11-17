// Adapted from jplayer-mobile/e2e/sys/platform_ios_test.go at 49de3c2
// (shownClass, presentedController, expectShown, cancelShown,
// expectGone), trimmed to the photo picker. iPhone only: tvOS has none.

//go:build ios && !tvos

package sys

import (
	"testing"

	"github.com/gdeskdev/desk/foundation/ffi/objc"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// shownClass is the controller class each service presents.
var shownClass = map[shown]string{
	shownImage: "PHPickerViewController",
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// presentedController returns the controller presented over the window's
// root controller, 0 for none; UI thread.
func presentedController() objc.ID {
	root := objc.Call(objc.ID(canvas().Handles().Window), objc.Sel("rootViewController"))

	return objc.Call(root, objc.Sel("presentedViewController"))
}

// expectShown waits until the controller of what shows over the app, its
// presentation over.
func expectShown(t *testing.T, what shown) {
	t.Helper()

	cls := objc.GetClass(shownClass[what])
	require.NotZero(t, cls, shownClass[what])
	eventually(t, within, shownClass[what]+" presented", func() bool {
		ok := false
		onUI(t, func() {
			vc := presentedController()
			ok = vc != 0 && objc.CallBool(vc, objc.Sel("isKindOfClass:"), uintptr(cls)) &&
				!objc.CallBool(vc, objc.Sel("isBeingPresented"))
		})

		return ok
	})
}

// cancelShown dismisses what shows as the user's Cancel does, telling its
// delegate what UIKit tells it: the photo picker's
// picker:didFinishPicking: of no results (its delegate dismisses it).
func cancelShown(t *testing.T, what shown) {
	t.Helper()

	var vc objc.ID
	onUI(t, func() { vc = presentedController() })
	require.NotZero(t, vc)
	onUI(t, func() {
		switch what {
		case shownImage:
			none := objc.Call(objc.ID(objc.GetClass("NSArray")), objc.Sel("array"))
			objc.Call(objc.Call(vc, objc.Sel("delegate")), objc.Sel("picker:didFinishPicking:"), uintptr(vc), uintptr(none))
		default:
			t.Errorf("nothing to cancel: %d", what) // not Fatal: the UI thread must go on
		}
	})
}

// expectGone waits until nothing shows over the app.
func expectGone(t *testing.T) {
	t.Helper()

	eventually(t, within, "nothing presented", func() bool {
		gone := false
		onUI(t, func() { gone = presentedController() == 0 })

		return gone
	})
}
