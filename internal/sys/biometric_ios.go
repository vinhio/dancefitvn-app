//go:build ios && !tvos

package sys

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"unsafe"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// laPolicyBiometrics is LAPolicyDeviceOwnerAuthenticationWithBiometrics:
// the biometry alone, no passcode (the login form is the fallback).
const laPolicyBiometrics = 1

// faceIDUsageKey is the Info.plist key without which iOS refuses Face ID
// (and Optic ID); kits/cli does not write it (G3a), scripts/
// ios-faceid-plist.sh does.
const faceIDUsageKey = "NSFaceIDUsageDescription"

// la is LocalAuthentication, loaded once: LAContext's class and whether
// the bundle has the usage sentence (it does not change while the app
// runs).
var la struct {
	once    sync.Once
	err     error
	context objc.Class
	usage   bool
}

// laSheet is one sheet showing: its LAContext (owned until the answer),
// and whether the answer came. UI thread only.
type laSheet struct {
	ctx      objc.ID
	finished bool
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// setupLA loads LocalAuthentication and reads the usage key, once.
func setupLA() error {
	la.once.Do(func() {
		if err := objc.Init(); err != nil {
			la.err = fmt.Errorf("sys: biometric: %w", err)

			return
		}
		_ = objc.LoadFramework("Foundation")
		if err := objc.LoadFramework("LocalAuthentication"); err != nil {
			la.err = fmt.Errorf("sys: biometric: %w: %w", core.ErrUnsupported, err)

			return
		}
		la.context = objc.GetClass("LAContext")
		if la.context == 0 {
			la.err = fmt.Errorf("sys: biometric: no LAContext: %w", core.ErrUnsupported)

			return
		}
		objc.WithPool(func() {
			bundle := objc.Call(objc.ID(objc.GetClass("NSBundle")), objc.Sel("mainBundle"))
			withString(faceIDUsageKey, func(key objc.ID) {
				la.usage = objc.Call(bundle, objc.Sel("objectForInfoDictionaryKey:"), uintptr(key)) != 0
			})
		})
	})

	return la.err
}

// biometry asks a new LAContext whether the biometry can be evaluated,
// and which it is. Any thread: LAContext is no UIKit object.
func biometry() (BiometryKind, error) {
	if err := setupLA(); err != nil {
		return BiometryNone, err
	}
	k, code, text := evaluate()
	switch err := laError(code, text); {
	case errors.Is(err, ErrBioLockout):
		return k, err
	case err != nil:
		return BiometryNone, nil
	case laNeedsUsage(k) && !la.usage:
		return BiometryNone, nil // Face ID without the usage sentence: hidden, never failing
	default:
		return k, nil
	}
}

// authenticate shows the sheet: evaluatePolicy:localizedReason:reply:
// on a new LAContext, whose reply comes on a private queue and is handed
// to the UI thread through d; the LAContext is released there. Face ID
// without the usage sentence is refused before iOS sees it.
func authenticate(d Dispatch, p BioPrompt, done func(error)) (func(), error) {
	if err := setupLA(); err != nil {
		return nil, err
	}
	if !objc.IsMainThread() {
		return nil, fmt.Errorf("sys: biometric: not on the UI thread: %w", core.ErrInvalid)
	}
	if k, _, _ := evaluate(); laNeedsUsage(k) && !la.usage {
		return nil, fmt.Errorf("sys: biometric: no %s in Info.plist: %w", faceIDUsageKey, ErrBioUnavailable)
	}
	sheet := &laSheet{}
	objc.WithPool(func() {
		sheet.ctx = objc.New(la.context)
		if p.Cancel != "" {
			withString(p.Cancel, func(ns objc.ID) {
				objc.Call(sheet.ctx, objc.Sel("setLocalizedCancelTitle:"), uintptr(ns))
			})
		}
		withString("", func(ns objc.ID) { // no "Enter Password": the login form is the fallback
			objc.Call(sheet.ctx, objc.Sel("setLocalizedFallbackTitle:"), uintptr(ns))
		})
		block := newBlock(func(args []reflect.Value) {
			var err error
			if !args[0].Bool() {
				code, text := laErrorOf(idArg(args[1]))
				if err = laError(code, text); err == nil {
					err = ErrBioFailed // NO without an error: not recognized
				}
			}
			d(func() {
				sheet.finish()
				done(err)
			})
		}, boolType, idType)
		withString(p.Reason, func(ns objc.ID) {
			objc.Call(sheet.ctx, objc.Sel("evaluatePolicy:localizedReason:reply:"), laPolicyBiometrics, uintptr(ns), uintptr(block))
		})
		objc.Release(block) // LocalAuthentication holds its copy until it replies
	})

	return sheet.cancel, nil
}

// evaluate asks a new LAContext canEvaluatePolicy:error: and its
// biometryType (set by the call, also when it fails): the kind, and the
// LAError code and text of a refusal (0 and "" when it can evaluate).
func evaluate() (kind BiometryKind, code int64, text string) {
	objc.WithPool(func() {
		ctx := objc.New(la.context)
		defer objc.Release(ctx)
		var errID objc.ID
		ok := objc.Send[bool](ctx, objc.Sel("canEvaluatePolicy:error:"), laPolicyBiometrics, unsafe.Pointer(&errID)) //nolint:gosec // G103: LocalAuthentication writes the error there during the call
		kind = laKind(int64(objc.Call(ctx, objc.Sel("biometryType"))))
		if !ok {
			if code, text = laErrorOf(errID); code == 0 {
				code = laBiometryNotAvailable // NO without an error
			}
		}
	})

	return kind, code, text
}

// cancel invalidates the context: iOS dismisses the sheet and replies
// LAErrorAppCancel. After the answer it does nothing.
func (s *laSheet) cancel() {
	if s.finished {
		return
	}
	objc.Call(s.ctx, objc.Sel("invalidate"))
}

// finish releases the context, once the answer came.
func (s *laSheet) finish() {
	if s.finished {
		return
	}
	s.finished = true
	objc.Release(s.ctx)
}

// laErrorOf returns the code and description of an NSError of the
// LAError domain; 0 and "" for nil. It runs in a pool (the description
// is autoreleased).
func laErrorOf(e objc.ID) (int64, string) {
	if e == 0 {
		return 0, ""
	}
	var (
		code int64
		text string
	)
	objc.WithPool(func() {
		code = int64(objc.Call(e, objc.Sel("code")))
		text = objc.GoString(objc.Call(e, objc.Sel("localizedDescription")))
	})

	return code, text
}
