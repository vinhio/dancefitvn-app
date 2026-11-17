//go:build ios && !tvos

// The objc pattern is the one of openurl_ios.go (adapted from
// jplayer-mobile/internal/sys/openurl_ios.go at 49de3c2) and of gDesk's
// examples/video-frames/video_frames_ios_test.go (backgroundTask); jPlayer
// has no background task.

package sys

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// beginBackgroundTask begins a UIKit background task named name. The
// expiration handler, which UIKit calls on the main thread, runs expired
// then ends the task; end ends it at once. The block is released once
// UIKit holds its copy.
func beginBackgroundTask(name string, expired func()) (func(), error) {
	if err := onMain("background task"); err != nil {
		return nil, err
	}
	var (
		app  objc.ID
		task uintptr
		end  func()
	)
	end = onceEnd(func() {
		objc.Call(app, objc.Sel("endBackgroundTask:"), task)
	})
	objc.WithPool(func() {
		app = objc.Call(objc.ID(objc.GetClass("UIApplication")), objc.Sel("sharedApplication"))
		block := newBlock(func([]reflect.Value) {
			expired()
			end()
		})
		withString(name, func(ns objc.ID) {
			task = uintptr(objc.Call(app, objc.Sel("beginBackgroundTaskWithName:expirationHandler:"), uintptr(ns), uintptr(block)))
		})
		objc.Release(block) // UIApplication holds its copy until the task ends
	})
	if task == 0 { // UIBackgroundTaskInvalid
		return nil, fmt.Errorf("sys: background task %q: no time granted: %w", name, core.ErrClosed)
	}

	return end, nil
}

// onceEnd returns end made to run once, whoever calls it first: the app
// or the expiry.
func onceEnd(end func()) func() {
	var once sync.Once

	return func() { once.Do(end) }
}
