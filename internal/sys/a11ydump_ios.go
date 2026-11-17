//go:build ios

// Adapted from jplayer-mobile/internal/sys/a11ydump_ios.go at 49de3c2:
// onMain and cgRect, which tvOS lacks here (objc_ios.go is the iPhone's), are
// a11yOnMain and a11yRect.

package sys

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/ffi/dl"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// a11yMaxDepth bounds the containers the dump goes down into.
const a11yMaxDepth = 8

// uikitPath is the UIKit binary whose exported trait constants the dump
// reads (UIAccessibilityConstants.h: extern variables, not methods).
const uikitPath = "/System/Library/Frameworks/UIKit.framework/UIKit"

// a11yTraitNames are UIKit's UIAccessibilityTraits constants and the
// names the dump gives them, in UIAccessibilityConstants.h's order.
var a11yTraitNames = [...]struct{ sym, name string }{
	{"UIAccessibilityTraitButton", "button"},
	{"UIAccessibilityTraitLink", "link"},
	{"UIAccessibilityTraitHeader", "header"},
	{"UIAccessibilityTraitSearchField", "searchField"},
	{"UIAccessibilityTraitImage", "image"},
	{"UIAccessibilityTraitSelected", "selected"},
	{"UIAccessibilityTraitPlaysSound", "playsSound"},
	{"UIAccessibilityTraitKeyboardKey", "keyboardKey"},
	{"UIAccessibilityTraitStaticText", "staticText"},
	{"UIAccessibilityTraitSummaryElement", "summaryElement"},
	{"UIAccessibilityTraitNotEnabled", "notEnabled"},
	{"UIAccessibilityTraitUpdatesFrequently", "updatesFrequently"},
	{"UIAccessibilityTraitStartsMediaSession", "startsMediaSession"},
	{"UIAccessibilityTraitAdjustable", "adjustable"},
	{"UIAccessibilityTraitAllowsDirectInteraction", "allowsDirectInteraction"},
	{"UIAccessibilityTraitCausesPageTurn", "causesPageTurn"},
	{"UIAccessibilityTraitTabBar", "tabBar"},
	{"UIAccessibilityTraitToggleButton", "toggle"},
	{"UIAccessibilityTraitSupportsZoom", "supportsZoom"},
}

// a11yFrames are the invocations of the frames' methods of a dump, by
// class of element and method; nil for a class without the method.
type a11yFrames map[a11yFrameKey]*objc.Invocation

// a11yFrameKey is a class of element and a method answering a CGRect.
type a11yFrameKey struct {
	cls objc.Class
	sel string
}

// a11yRect is a CGRect: origin and size in points, as doubles.
type a11yRect struct{ X, Y, W, H float64 }

// a11yTraits are the values of a11yTraitNames, read from UIKit once; a
// constant UIKit lacks stays 0 and is never named.
var a11yTraits struct {
	once sync.Once
	bits [len(a11yTraitNames)]uint64
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// accessibilityDump reads the elements of the canvas view of c.
func accessibilityDump(c platform.Canvas) ([]A11yElement, error) {
	const op = "accessibility dump"
	if c == nil {
		return nil, fmt.Errorf("sys: %s: no canvas: %w", op, core.ErrInvalid)
	}
	if err := a11yOnMain(op); err != nil {
		return nil, err
	}
	h := c.Handles()
	if h.Kind != platform.HandleUIKit || h.View == 0 {
		return nil, fmt.Errorf("sys: %s: not a UIKit view: %w", op, core.ErrUnsupported)
	}
	loadA11yTraits()
	frames := a11yFrames{}
	defer frames.release()
	var out []A11yElement
	objc.WithPool(func() { out = frames.appendA11y(out, objc.ID(h.View), 0) })

	return out, nil
}

// appendA11y appends the elements of the container obj, going down into
// those that are containers themselves (not elements), a11yMaxDepth deep
// at most.
func (f a11yFrames) appendA11y(out []A11yElement, obj objc.ID, depth int) []A11yElement {
	if depth > a11yMaxDepth || !objc.CallBool(obj, objc.Sel("respondsToSelector:"), uintptr(objc.Sel("accessibilityElements"))) {
		return out
	}
	list := objc.Call(obj, objc.Sel("accessibilityElements"))
	if list == 0 {
		return out
	}
	n := int(objc.Call(list, objc.Sel("count")))
	at, isElem := objc.Sel("objectAtIndex:"), objc.Sel("isAccessibilityElement")
	for i := range n {
		e := objc.Call(list, at, uintptr(i))
		if e == 0 {
			continue
		}
		if !objc.CallBool(e, isElem) {
			out = f.appendA11y(out, e, depth+1)

			continue
		}
		out = append(out, f.elementOf(e, depth))
	}

	return out
}

// elementOf reads the label, value, traits and frame of the element e.
func (f a11yFrames) elementOf(e objc.ID, depth int) A11yElement {
	bits := uint64(objc.Call(e, objc.Sel("accessibilityTraits")))
	r := f.rectOf(e, "accessibilityFrame")
	rc := f.rectOf(e, "accessibilityFrameInContainerSpace")

	return A11yElement{
		Label:     objc.GoString(objc.Call(e, objc.Sel("accessibilityLabel"))),
		Value:     objc.GoString(objc.Call(e, objc.Sel("accessibilityValue"))),
		Traits:    traitNames(bits),
		TraitBits: bits,
		Frame:     r.rect(),
		InView:    rc.rect(),
		Depth:     depth,
	}
}

// rectOf returns what the method sel of e answers, a CGRect of 32 bytes
// that comes back in four floating-point registers, out of objc.Call's
// reach: through an objc.Invocation made once per class of element and
// method; a zero rect when e's class has no such method.
func (f a11yFrames) rectOf(e objc.ID, sel string) a11yRect {
	key := a11yFrameKey{cls: objc.Class(objc.Call(e, objc.Sel("class"))), sel: sel}
	in, ok := f[key]
	if !ok {
		var err error
		if in, err = objc.NewInvocation(key.cls, sel); err != nil {
			in = nil
		}
		f[key] = in
	}
	if in == nil || in.Invoke(e) != nil {
		return a11yRect{}
	}
	r, err := in.Result[a11yRect]()
	if err != nil {
		return a11yRect{}
	}

	return r
}

// rect is r in the geometry of gDesk.
func (r a11yRect) rect() geom.Rect {
	return geom.Rect{X: float32(r.X), Y: float32(r.Y), W: float32(r.W), H: float32(r.H)}
}

// release releases the invocations.
func (f a11yFrames) release() {
	for _, in := range f {
		if in != nil {
			in.Release()
		}
	}
}

// traitNames names the traits set in bits.
func traitNames(bits uint64) []string {
	var out []string
	for i, b := range a11yTraits.bits {
		if b != 0 && bits&b == b {
			out = append(out, a11yTraitNames[i].name)
		}
	}

	return out
}

// a11yOnMain checks that the dump runs on the UI thread, where UIKit wants
// it, with the runtime and UIKit loaded (on the iPhone and the Apple TV).
func a11yOnMain(op string) error {
	if !objc.IsMainThread() {
		return fmt.Errorf("sys: %s: not on the UI thread: %w", op, core.ErrInvalid)
	}
	if err := objc.Init(); err != nil {
		return fmt.Errorf("sys: %s: %w", op, err)
	}
	if err := objc.LoadFramework("UIKit"); err != nil {
		return fmt.Errorf("sys: %s: %w", op, err)
	}

	return nil
}

// loadA11yTraits reads the values of a11yTraitNames from UIKit, once.
func loadA11yTraits() {
	a11yTraits.once.Do(func() {
		lib, err := dl.Open(uikitPath)
		if err != nil {
			return
		}
		for i, t := range a11yTraitNames {
			if !lib.Has(t.sym) {
				continue
			}
			var addr dl.Fn
			if lib.Bind(dl.Sym{Fn: &addr, Name: t.sym}) != nil || addr == 0 {
				continue
			}
			// The symbol's address holds the constant's value.
			a11yTraits.bits[i] = *(*uint64)(unsafe.Pointer(dl.Bytes(uintptr(addr)))) //nolint:gosec // G103: UIKit's constant
		}
	})
}
