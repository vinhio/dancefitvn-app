//go:build android

// Adapted from jplayer-mobile/internal/sys/java_android.go at 49de3c2
// (the constants; lookupJava, newIntent, callString, parseURI, stringOf,
// startActivity, resolver, noJava), trimmed to what opening a URL and
// the photo picker need. Its lookups live in intents, not in java
// (paths_android.go), so that each file's methods are looked up by the
// first call that needs them.

package sys

import (
	"fmt"
	"sync"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The constants of android.jar (platform 35), read with javap -constants:
// Intent's actions, category and flag, MediaStore's ACTION_PICK_IMAGES
// (API 33) and ExifInterface's tag. Each is a string or int the API takes
// as is, so none needs a field lookup.
const (
	// actionView is Intent.ACTION_VIEW.
	actionView = "android.intent.action.VIEW"
	// actionGetContent is Intent.ACTION_GET_CONTENT, the photo picker
	// before API 33.
	actionGetContent = "android.intent.action.GET_CONTENT"
	// actionPickImages is MediaStore.ACTION_PICK_IMAGES, the Photo Picker.
	actionPickImages = "android.provider.action.PICK_IMAGES"
	// categoryOpenable is Intent.CATEGORY_OPENABLE: what GET_CONTENT
	// offers opens as a stream.
	categoryOpenable = "android.intent.category.OPENABLE"
	// tagOrientation is ExifInterface.TAG_ORIENTATION.
	tagOrientation = "Orientation"
	// flagActivityNewTask is Intent.FLAG_ACTIVITY_NEW_TASK, which starting
	// an activity from the application's context needs.
	flagActivityNewTask int32 = 0x10000000
	// pickImagesSince is the API of MediaStore.ACTION_PICK_IMAGES
	// (Build.VERSION_CODES.TIRAMISU).
	pickImagesSince = 33
)

// intents holds the classes (global references) and the method IDs of
// an ACTION_VIEW and of a picker's result, looked up at the first call on
// any thread: the system's classes are never unloaded, so they last for
// the process.
var intents struct {
	mu sync.Mutex
	ok bool

	intent, uri jni.Class

	sdkInt int32 // Build.VERSION.SDK_INT

	// android.content.Context and android.app.Activity
	ctxStartActivity, startActivity, getContentResolver jni.MethodID
	// android.content.Intent
	intentNew, intentNewURI, addFlags, setType, addCategory, getData, getClipData jni.MethodID
	// android.net.Uri and java.lang.Object
	uriParse, toString jni.MethodID
	// android.content.ClipData and ClipData.Item
	itemCount, itemAt, itemURI jni.MethodID
	// android.content.ContentResolver and android.os.ParcelFileDescriptor
	getType, openFD, detachFd, pfdClose jni.MethodID
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// lookupIntents looks the classes and methods of intents up, once.
func lookupIntents(env jni.Env) error {
	intents.mu.Lock()
	defer intents.mu.Unlock()
	if intents.ok {
		return nil
	}
	classes := []struct {
		c    *jni.Class
		name string
	}{
		{&intents.intent, "android/content/Intent"},
		{&intents.uri, "android/net/Uri"},
	}
	for _, c := range classes {
		local, err := env.FindClass(c.name)
		if err != nil {
			return err
		}
		*c.c = jni.Class(env.NewGlobalRef(jni.Object(local)))
		env.DeleteLocalRef(jni.Object(local))
	}
	version, err := env.FindClass("android/os/Build$VERSION")
	if err != nil {
		return err
	}
	sdk, err := env.StaticField(version, "SDK_INT", "I")
	if err != nil {
		env.DeleteLocalRef(jni.Object(version))

		return err
	}
	intents.sdkInt = env.StaticIntField(version, sdk)
	env.DeleteLocalRef(jni.Object(version))
	methods := []struct {
		id               *jni.MethodID
		class, name, sig string
		static           bool
	}{
		{id: &intents.ctxStartActivity, class: "android/content/Context", name: "startActivity", sig: "(Landroid/content/Intent;)V"},
		{id: &intents.startActivity, class: "android/app/Activity", name: "startActivity", sig: "(Landroid/content/Intent;)V"},
		{id: &intents.getContentResolver, class: "android/content/Context", name: "getContentResolver", sig: "()Landroid/content/ContentResolver;"},
		{id: &intents.intentNew, class: "android/content/Intent", name: "<init>", sig: "(Ljava/lang/String;)V"},
		{id: &intents.intentNewURI, class: "android/content/Intent", name: "<init>", sig: "(Ljava/lang/String;Landroid/net/Uri;)V"},
		{id: &intents.addFlags, class: "android/content/Intent", name: "addFlags", sig: "(I)Landroid/content/Intent;"},
		{id: &intents.setType, class: "android/content/Intent", name: "setType", sig: "(Ljava/lang/String;)Landroid/content/Intent;"},
		{id: &intents.addCategory, class: "android/content/Intent", name: "addCategory", sig: "(Ljava/lang/String;)Landroid/content/Intent;"},
		{id: &intents.getData, class: "android/content/Intent", name: "getData", sig: "()Landroid/net/Uri;"},
		{id: &intents.getClipData, class: "android/content/Intent", name: "getClipData", sig: "()Landroid/content/ClipData;"},
		{id: &intents.uriParse, class: "android/net/Uri", name: "parse", sig: "(Ljava/lang/String;)Landroid/net/Uri;", static: true},
		{id: &intents.toString, class: "java/lang/Object", name: "toString", sig: "()Ljava/lang/String;"},
		{id: &intents.itemCount, class: "android/content/ClipData", name: "getItemCount", sig: "()I"},
		{id: &intents.itemAt, class: "android/content/ClipData", name: "getItemAt", sig: "(I)Landroid/content/ClipData$Item;"},
		{id: &intents.itemURI, class: "android/content/ClipData$Item", name: "getUri", sig: "()Landroid/net/Uri;"},
		{id: &intents.getType, class: "android/content/ContentResolver", name: "getType", sig: "(Landroid/net/Uri;)Ljava/lang/String;"},
		{id: &intents.openFD, class: "android/content/ContentResolver", name: "openFileDescriptor", sig: "(Landroid/net/Uri;Ljava/lang/String;)Landroid/os/ParcelFileDescriptor;"},
		{id: &intents.detachFd, class: "android/os/ParcelFileDescriptor", name: "detachFd", sig: "()I"},
		{id: &intents.pfdClose, class: "android/os/ParcelFileDescriptor", name: "close", sig: "()V"},
	}
	for _, m := range methods {
		if err := lookupAnyMethod(env, m.id, m.class, m.name, m.sig, m.static); err != nil {
			return err
		}
	}
	intents.ok = true

	return nil
}

// lookupAnyMethod looks one method, static or not, up into id.
func lookupAnyMethod(env jni.Env, id *jni.MethodID, class, name, sig string, static bool) error {
	c, err := env.FindClass(class)
	if err != nil {
		return err
	}
	defer env.DeleteLocalRef(jni.Object(c))
	if static {
		*id, err = env.StaticMethod(c, name, sig)
	} else {
		*id, err = env.Method(c, name, sig)
	}

	return err
}

// newIntent returns a new Intent of action, of the URI uri when not 0, a
// local reference.
func newIntent(env jni.Env, action string, uri jni.Object) (jni.Object, error) {
	a, err := env.NewString(action)
	if err != nil {
		return 0, err
	}
	defer env.DeleteLocalRef(a)
	if uri != 0 {
		return env.NewObject(intents.intent, intents.intentNewURI, jni.Ref(a), jni.Ref(uri))
	}

	return env.NewObject(intents.intent, intents.intentNew, jni.Ref(a))
}

// callString calls a method of o taking one String and answering an
// object, which it deletes: Intent's setType, addCategory.
func callString(env jni.Env, o jni.Object, m jni.MethodID, s string) error {
	js, err := env.NewString(s)
	if err != nil {
		return err
	}
	defer env.DeleteLocalRef(js)
	r, err := env.CallObject(o, m, jni.Ref(js))
	env.DeleteLocalRef(r)

	return err
}

// parseURI returns the android.net.Uri of s, a local reference.
func parseURI(env jni.Env, s string) (jni.Object, error) {
	js, err := env.NewString(s)
	if err != nil {
		return 0, err
	}
	defer env.DeleteLocalRef(js)

	return env.CallStaticObject(intents.uri, intents.uriParse, jni.Ref(js))
}

// stringOf returns o.toString(), "" for null.
func stringOf(env jni.Env, o jni.Object) (string, error) {
	if o == 0 {
		return "", nil
	}
	s, err := env.CallObject(o, intents.toString)
	if err != nil {
		return "", err
	}
	defer env.DeleteLocalRef(s)

	return env.GoString(s)
}

// startActivity starts intent from the activity, or from the
// application's context as a new task while the app has none. Java's main
// thread only.
func startActivity(env jni.Env, intent jni.Object) error {
	if act := jni.Activity(); act != 0 {
		return env.CallVoid(act, intents.startActivity, jni.Ref(intent))
	}
	ctx := jni.Context()
	if ctx == 0 {
		return fmt.Errorf("no context: %w", core.ErrUnsupported)
	}
	r, err := env.CallObject(intent, intents.addFlags, jni.Int(flagActivityNewTask))
	env.DeleteLocalRef(r)
	if err != nil {
		return err
	}

	return env.CallVoid(ctx, intents.ctxStartActivity, jni.Ref(intent))
}

// resolver returns the application's ContentResolver, a local reference.
func resolver(env jni.Env) (jni.Object, error) {
	ctx := jni.Context()
	if ctx == 0 {
		return 0, fmt.Errorf("no context: %w", core.ErrUnsupported)
	}

	return env.CallObject(ctx, intents.getContentResolver)
}

// noJava is the error of a picker without gDesk's Java, which results of
// activities need.
func noJava(op string) error {
	return fmt.Errorf("sys: %s: gDesk's Java classes do not run in the app: %w", op, core.ErrUnsupported)
}
