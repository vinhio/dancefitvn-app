//go:build android

// Adapted from jplayer-mobile/internal/sys/keepawake_android.go at
// 49de3c2 (fallback F8 while gDesk's Android backend has no
// platform.PowerDriver, proposal G9).

package sys

import (
	"fmt"
	"log"
	"sync"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// flagKeepScreenOn is WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON of
// android.jar (platform 35, javap -constants): 128, since API 1.
const flagKeepScreenOn int32 = 0x80

// screenOn is what the activity's window was last given here: the
// activity (the global reference jni.Activity returned, which a new
// activity changes) and whether its window has FLAG_KEEP_SCREEN_ON. Java's
// main thread writes it, the UI thread reads it to skip a post.
var screenOn struct {
	sync.Mutex
	activity jni.Object
	on       bool
}

// keepAwakeIDs are the methods a keep-awake calls, looked up at the
// first one: the system's classes are never unloaded, so their IDs last.
// Java's main thread only.
var keepAwakeIDs struct {
	found      bool
	getWindow  jni.MethodID // Activity.getWindow()Landroid/view/Window;
	addFlags   jni.MethodID // Window.addFlags(I)V
	clearFlags jni.MethodID // Window.clearFlags(I)V
}

// postKeepAwake is the function PostMain runs, made once: it sets on the
// activity's window what awake holds then, the latest asked for.
var postKeepAwake = func(env jni.Env) {
	if err := applyKeepAwake(env, awake.Load()); err != nil {
		log.Printf("dancefitvn: keep awake: %v", err)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// keepAwake queues the flag's change on Java's main thread, where the
// window is touched, and returns without waiting (jni.PostMain: the UI
// thread must never wait for the main thread). It posts nothing when the
// current activity's window has the value already, unless force is set.
// The driver is not needed: the flag is the activity's.
func keepAwake(_ platform.Driver, on, force bool) error {
	if !force {
		act := jni.Activity()
		screenOn.Lock()
		same := act != 0 && act == screenOn.activity && screenOn.on == on
		screenOn.Unlock()
		if same {
			return nil
		}
	}
	if err := jni.PostMain(postKeepAwake); err != nil {
		return fmt.Errorf("sys: keep awake: %w", err)
	}

	return nil
}

// applyKeepAwake calls activity.getWindow().addFlags or clearFlags of
// FLAG_KEEP_SCREEN_ON and records it. Without an activity (the app
// playing on behind no screen) there is no window: nothing is done, and
// ReapplyKeepAwake sets the next activity's. Its references are local to
// the frame PostMain runs it in; the window's is deleted here.
func applyKeepAwake(env jni.Env, on bool) error {
	act := jni.Activity()
	if act == 0 {
		return nil
	}
	if !keepAwakeIDs.found {
		if err := findKeepAwakeIDs(env); err != nil {
			return err
		}
	}
	win, err := env.CallObject(act, keepAwakeIDs.getWindow)
	if err != nil {
		return err
	}
	if win == 0 {
		return nil
	}
	defer env.DeleteLocalRef(win)
	m := keepAwakeIDs.clearFlags
	if on {
		m = keepAwakeIDs.addFlags
	}
	if err := env.CallVoid(win, m, jni.Int(flagKeepScreenOn)); err != nil {
		return err
	}
	screenOn.Lock()
	screenOn.activity, screenOn.on = act, on
	screenOn.Unlock()

	return nil
}

// findKeepAwakeIDs looks the methods of a keep-awake up.
func findKeepAwakeIDs(env jni.Env) error {
	actCls, err := env.FindClass("android/app/Activity")
	if err != nil {
		return err
	}
	defer env.DeleteLocalRef(jni.Object(actCls))
	getWindow, err := env.Method(actCls, "getWindow", "()Landroid/view/Window;")
	if err != nil {
		return err
	}
	winCls, err := env.FindClass("android/view/Window")
	if err != nil {
		return err
	}
	defer env.DeleteLocalRef(jni.Object(winCls))
	add, err := env.Method(winCls, "addFlags", "(I)V")
	if err != nil {
		return err
	}
	clearFlags, err := env.Method(winCls, "clearFlags", "(I)V")
	if err != nil {
		return err
	}
	keepAwakeIDs.getWindow, keepAwakeIDs.addFlags, keepAwakeIDs.clearFlags = getWindow, add, clearFlags
	keepAwakeIDs.found = true

	return nil
}
