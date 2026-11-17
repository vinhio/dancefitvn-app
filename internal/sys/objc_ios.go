//go:build ios && !tvos

// Adapted from jplayer-mobile/internal/sys/present_ios.go at 49de3c2
// (boolType, withString, onMain; setupKit with its delegate class, of
// which only picker:didFinishPicking: is kept; presented, presentTimeout,
// topController, present, waiting, take, nsErrorText). blockType,
// idType, newBlock and idArg are in block_ios.go, which tvOS shares.
// iPhone only: the services using them (biometrics, opening a URL, the
// photo picker) have no tvOS side.

package sys

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// delegateClassName is the Objective-C class of the photo picker's
// delegate, defined once per process (a class name is the process's for
// its life).
const delegateClassName = "DFVPhotoPickerDelegate"

// boolType is the type of a block's BOOL argument.
var boolType = reflect.TypeFor[bool]()

// presented is a controller the package presented and waits on: the
// controller (owned: alloc/init, released once it answered), where its
// answer goes, when it was presented and whether UIKit has finished
// presenting it. UI thread only.
type presented[T any] struct {
	vc    objc.ID
	d     Dispatch
	done  T
	since time.Time
	shown bool
}

// presentTimeout is how long a presentation may take before the package
// takes it for one UIKit refused (presenting over a controller that is
// presenting already logs and shows nothing).
const presentTimeout = 10 * time.Second

// kit is the package's UIKit state: the frameworks and the one delegate,
// made at the first use and kept for the process, and the photo picker
// showing. The fields past setup are for the UI thread, the main thread
// on iOS, where UIKit calls the delegate too.
var kit struct {
	once sync.Once
	err  error

	delegate objc.ID // the DFVPhotoPickerDelegate, never released

	image presented[func([]byte, string, error)]
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// setupKit loads the runtime and the frameworks the services use and
// defines the picker's delegate class, once; its failure is kept. A
// framework the system lacks leaves its classes missing, which the
// service that needs one reports.
func setupKit() error {
	kit.once.Do(func() {
		if err := objc.Init(); err != nil {
			kit.err = fmt.Errorf("sys: %w", err)

			return
		}
		for _, fw := range []string{"Foundation", "UIKit"} {
			if err := objc.LoadFramework(fw); err != nil {
				kit.err = fmt.Errorf("sys: %w", err)

				return
			}
		}
		for _, fw := range []string{"UniformTypeIdentifiers", "PhotosUI"} {
			// There on every iOS the app runs on (17+); a failure shows as
			// a missing class.
			_ = objc.LoadFramework(fw)
		}
		cls, err := objc.DefineClass(delegateClassName, "NSObject",
			[]string{"PHPickerViewControllerDelegate"}, delegateMethods()...)
		if err != nil {
			kit.err = fmt.Errorf("sys: %w", err)

			return
		}
		kit.delegate = objc.Call(objc.Call(objc.ID(cls), objc.Sel("alloc")), objc.Sel("init"))
	})

	return kit.err
}

// delegateMethods are the methods of DFVPhotoPickerDelegate:
// PHPickerViewControllerDelegate's one, which UIKit calls on the main
// thread.
func delegateMethods() []objc.Method {
	return []objc.Method{
		{Selector: "picker:didFinishPicking:", Fn: func(_ objc.ID, _ objc.SEL, picker, results objc.ID) {
			imagePicked(picker, results)
		}},
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// withString hands fn a new NSString of s, released after it.
func withString(s string, fn func(ns objc.ID)) {
	ns := objc.NSString(s)
	defer objc.Release(ns)
	fn(ns)
}

// onMain checks that the call runs on the UI thread, where UIKit wants
// it, and loads the frameworks the first time.
func onMain(op string) error {
	if !objc.IsMainThread() {
		return fmt.Errorf("sys: %s: not on the UI thread: %w", op, core.ErrInvalid)
	}

	return setupKit()
}

// topController returns the controller on top of c's window: its root
// controller (gDesk's GDeskViewController), or the last one presented
// over it that is not on its way out. The package presents over it.
func topController(op string, c platform.Canvas) (objc.ID, error) {
	h := c.Handles()
	if h.Kind != platform.HandleUIKit || h.Window == 0 {
		return 0, fmt.Errorf("sys: %s: not a UIKit window: %w", op, core.ErrUnsupported)
	}
	vc := objc.Call(objc.ID(h.Window), objc.Sel("rootViewController"))
	if vc == 0 {
		return 0, fmt.Errorf("sys: %s: the window has no root controller: %w", op, core.ErrNotFound)
	}
	presentedSel, dismissing := objc.Sel("presentedViewController"), objc.Sel("isBeingDismissed")
	for {
		next := objc.Call(vc, presentedSel)
		if next == 0 || objc.CallBool(next, dismissing) {
			return vc, nil
		}
		vc = next
	}
}

// present presents vc over top, animated, as a sheet does, and records
// in p when UIKit has finished (the completion block, on the main thread).
func present[T any](p *presented[T], top, vc objc.ID, d Dispatch, done T) {
	*p = presented[T]{vc: vc, d: d, done: done, since: time.Now()}
	block := newBlock(func([]reflect.Value) {
		if p.vc == vc {
			p.shown = true
		}
	})
	objc.Call(top, objc.Sel("presentViewController:animated:completion:"), uintptr(vc), 1, uintptr(block))
	objc.Release(block) // UIKit holds its copy until it calls it
}

// waiting tells whether p's controller still waits for the user: being
// presented for less than presentTimeout, or presented and not dismissed
// since (UIKit clears the presenting controller of a dismissed one). A
// controller that is not waiting but was never answered went without a
// word.
func (p *presented[T]) waiting() bool {
	switch {
	case p.vc == 0:
		return false
	case !p.shown:
		return time.Since(p.since) < presentTimeout
	default:
		return objc.Call(p.vc, objc.Sel("presentingViewController")) != 0
	}
}

// take returns what p waits on and clears it, releasing its controller.
func take[T any](p *presented[T]) presented[T] {
	got := *p
	*p = presented[T]{}
	if got.vc != 0 {
		objc.Release(got.vc)
	}

	return got
}

// nsErrorText returns the localized description of an NSError.
func nsErrorText(e objc.ID) string {
	if e == 0 {
		return ""
	}

	return objc.GoString(objc.Call(e, objc.Sel("localizedDescription")))
}
