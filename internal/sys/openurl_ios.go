//go:build ios && !tvos

// Adapted from jplayer-mobile/internal/sys/openurl_ios.go at 49de3c2.

package sys

import (
	"fmt"
	"reflect"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// openURL asks UIApplication to open the URL, with no options; the
// completion handler, which UIKit calls on the main thread with whether
// an app took it, answers done. NSURL refuses what is not a URL to it
// (core.ErrInvalid).
func openURL(_ platform.Canvas, d Dispatch, rawURL string, done func(error)) error {
	if err := onMain("open URL"); err != nil {
		return err
	}
	var err error
	objc.WithPool(func() {
		var u objc.ID
		withString(rawURL, func(ns objc.ID) {
			u = objc.Call(objc.ID(objc.GetClass("NSURL")), objc.Sel("URLWithString:"), uintptr(ns))
		})
		if u == 0 {
			err = fmt.Errorf("sys: open URL: %q is no URL to NSURL: %w", rawURL, core.ErrInvalid)

			return
		}
		app := objc.Call(objc.ID(objc.GetClass("UIApplication")), objc.Sel("sharedApplication"))
		opts := objc.Call(objc.ID(objc.GetClass("NSDictionary")), objc.Sel("dictionary"))
		block := newBlock(func(args []reflect.Value) {
			var err error
			if !args[0].Bool() {
				err = fmt.Errorf("sys: open URL: no app opened %q: %w", rawURL, core.ErrNotFound)
			}
			d(func() { done(err) })
		}, boolType)
		objc.Call(app, objc.Sel("openURL:options:completionHandler:"), uintptr(u), uintptr(opts), uintptr(block))
		objc.Release(block) // UIApplication holds its copy until it calls it
	})

	return err
}
