//go:build android

// Adapted from jplayer-mobile/internal/sys/pickimage_android.go at
// 49de3c2. Changed: the lookups are intents' (intent_android.go), not
// java's; systemJPEGQuality is in pickimage.go.

package sys

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// pickingImage is set from the start of a photo picker to its answer: one
// shows at a time. An activity started for a result always answers (its
// cancel is a result too), so it never stays set while the process lives.
var pickingImage atomic.Bool

// pictures holds the classes (global references), methods and fields
// that decoding a picture in Java needs, looked up at the first picture
// Go does not take as it is.
var pictures struct {
	mu sync.Mutex
	ok bool

	factory, options, exif, matrix, bitmap, byteIn, byteOut jni.Class

	decodeByteArray, optionsNew, exifNew, getAttributeInt, matrixNew, postRotate, postScale jni.MethodID
	createBitmap, compress, width, height, recycle, byteInNew, byteOutNew, toByteArray      jni.MethodID
	justBounds, sample, outWidth, outHeight                                                 jni.FieldID
	jpeg                                                                                    jni.Object // Bitmap.CompressFormat.JPEG, global
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// pickImage starts the photo picker for a result on Java's main thread;
// the picture is read, and made a JPEG where Go does not take it, on a
// goroutine of its own, which ends with it.
func pickImage(_ platform.Canvas, d Dispatch, done func([]byte, string, error)) error {
	const op = "pick image"
	if !jni.HasJava() {
		return noJava(op)
	}
	if !pickingImage.CompareAndSwap(false, true) {
		return busy(op)
	}
	finish := func(data []byte, mime string, err error) {
		pickingImage.Store(false)
		if err != nil {
			data, mime = nil, ""
		}
		d(func() { done(data, mime, err) })
	}
	err := jni.PostMain(func(env jni.Env) {
		err := startForResult(env, imageIntent, func(env jni.Env, code int32, data jni.Object) {
			uris, err := resultURIs(env, code, data)
			if err != nil {
				finish(nil, "", err)

				return
			}
			go func() { finish(readPicture(uris[0])) }()
		})
		if err != nil {
			finish(nil, "", fmt.Errorf("sys: %s: %w", op, err))
		}
	})
	if err != nil {
		pickingImage.Store(false)

		return fmt.Errorf("sys: %s: %w", op, err)
	}

	return nil
}

// imageIntent returns the intent of the photo picker: the system's Photo
// Picker (ACTION_PICK_IMAGES) from API 33, one picture; before it,
// ACTION_GET_CONTENT of an openable image.
func imageIntent(env jni.Env) (jni.Object, error) {
	action := actionPickImages
	if intents.sdkInt < pickImagesSince {
		action = actionGetContent
	}
	intent, err := newIntent(env, action, 0)
	if err != nil {
		return 0, err
	}
	if err := callString(env, intent, intents.setType, "image/*"); err != nil {
		return 0, err
	}
	if action == actionGetContent {
		if err := callString(env, intent, intents.addCategory, categoryOpenable); err != nil {
			return 0, err
		}
	}

	return intent, nil
}

// readPicture reads the picture of the document s and its type; one Go
// does not decode as it is comes back as a JPEG Java made (systemJPEG).
func readPicture(s string) ([]byte, string, error) {
	var mime string
	err := jni.Do(func(env jni.Env) error {
		if err := lookupIntents(env); err != nil {
			return err
		}
		uri, err := parseURI(env, s)
		if err != nil {
			return err
		}
		res, err := resolver(env)
		if err != nil {
			return err
		}
		t, err := env.CallObject(res, intents.getType, jni.Ref(uri))
		if err != nil {
			return err
		}
		mime, err = env.GoString(t)

		return err
	})
	if err != nil {
		return nil, "", fmt.Errorf("sys: pick image: %w", err)
	}
	r, err := openDocument(s, "picture")
	if err != nil {
		return nil, "", err
	}
	data, err := readImage(r)
	_ = r.Close()
	if err != nil {
		return nil, "", err
	}
	if goTakes(data, mime) {
		return data, mime, nil
	}
	if err := jni.Do(func(env jni.Env) error {
		var err error
		data, err = systemJPEG(env, data)

		return err
	}); err != nil {
		return nil, "", fmt.Errorf("sys: pick image: %w", err)
	}

	return data, mimeJPEG, nil
}

// systemJPEG decodes the picture of data with BitmapFactory, smaller by
// sampleSize, turns it upright by its EXIF orientation (ExifInterface,
// which reads HEIF from API 28) and returns it as a JPEG of quality
// systemJPEGQuality. Every Java object it makes is local to the Do frame;
// the bitmaps are recycled as soon as they are done with.
func systemJPEG(env jni.Env, data []byte) ([]byte, error) {
	if err := lookupPictures(env); err != nil {
		return nil, err
	}
	p := &pictures
	arr, err := env.NewByteArray(data)
	if err != nil {
		return nil, err
	}
	opts, err := env.NewObject(p.options, p.optionsNew)
	if err != nil {
		return nil, err
	}
	n := jni.Int(int32(len(data)))
	env.SetBoolField(opts, p.justBounds, true)
	if _, err := env.CallStaticObject(p.factory, p.decodeByteArray, jni.Ref(arr), jni.Int(0), n, jni.Ref(opts)); err != nil {
		return nil, err
	}
	w, h := env.IntField(opts, p.outWidth), env.IntField(opts, p.outHeight)
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("not a picture Android decodes: %w", core.ErrInvalid)
	}
	env.SetBoolField(opts, p.justBounds, false)
	env.SetIntField(opts, p.sample, sampleSize(int(w), int(h)))
	bmp, err := env.CallStaticObject(p.factory, p.decodeByteArray, jni.Ref(arr), jni.Int(0), n, jni.Ref(opts))
	if err != nil {
		return nil, err
	}
	if bmp == 0 {
		return nil, fmt.Errorf("not a picture Android decodes: %w", core.ErrInvalid)
	}
	if bmp, err = upright(env, arr, bmp); err != nil {
		return nil, err
	}
	defer func() { _ = env.CallVoid(bmp, p.recycle) }()
	out, err := env.NewObject(p.byteOut, p.byteOutNew)
	if err != nil {
		return nil, err
	}
	ok, err := env.CallBool(bmp, p.compress, jni.Ref(p.jpeg), jni.Int(systemJPEGQuality), jni.Ref(out))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("no JPEG from the bitmap: %w", core.ErrInvalid)
	}
	b, err := env.CallObject(out, p.toByteArray)
	if err != nil {
		return nil, err
	}

	return env.Bytes(b, nil)
}

// upright returns bmp turned by the EXIF orientation of the picture in arr,
// recycling bmp when it made another; a picture without EXIF, or one
// ExifInterface cannot read, stays as it is.
func upright(env jni.Env, arr, bmp jni.Object) (jni.Object, error) {
	p := &pictures
	in, err := env.NewObject(p.byteIn, p.byteInNew, jni.Ref(arr))
	if err != nil {
		return bmp, err
	}
	exif, err := env.NewObject(p.exif, p.exifNew, jni.Ref(in))
	if err != nil {
		return bmp, nil //nolint:nilerr // no EXIF Android reads: as it is
	}
	tag, err := env.NewString(tagOrientation)
	if err != nil {
		return bmp, err
	}
	o, err := env.CallInt(exif, p.getAttributeInt, jni.Ref(tag), jni.Int(1))
	if err != nil {
		return bmp, nil //nolint:nilerr // as it is
	}
	deg, flip := exifTransform(o)
	if deg == 0 && !flip {
		return bmp, nil
	}
	m, err := env.NewObject(p.matrix, p.matrixNew)
	if err != nil {
		return bmp, err
	}
	if deg != 0 {
		if _, err := env.CallBool(m, p.postRotate, jni.Float(deg)); err != nil {
			return bmp, err
		}
	}
	if flip {
		if _, err := env.CallBool(m, p.postScale, jni.Float(-1), jni.Float(1)); err != nil {
			return bmp, err
		}
	}
	w, err := env.CallInt(bmp, p.width)
	if err != nil {
		return bmp, err
	}
	h, err := env.CallInt(bmp, p.height)
	if err != nil {
		return bmp, err
	}
	turned, err := env.CallStaticObject(p.bitmap, p.createBitmap, jni.Ref(bmp), jni.Int(0), jni.Int(0), jni.Int(w), jni.Int(h), jni.Ref(m), jni.Bool(true))
	if err != nil || turned == 0 {
		return bmp, err
	}
	if !env.IsSameObject(turned, bmp) {
		_ = env.CallVoid(bmp, p.recycle)
	}

	return turned, nil
}

// lookupPictures looks the members of pictures up, once.
func lookupPictures(env jni.Env) error {
	p := &pictures
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ok {
		return nil
	}
	for _, c := range []struct {
		c    *jni.Class
		name string
	}{
		{&p.factory, "android/graphics/BitmapFactory"},
		{&p.options, "android/graphics/BitmapFactory$Options"},
		{&p.exif, "android/media/ExifInterface"},
		{&p.matrix, "android/graphics/Matrix"},
		{&p.bitmap, "android/graphics/Bitmap"},
		{&p.byteIn, "java/io/ByteArrayInputStream"},
		{&p.byteOut, "java/io/ByteArrayOutputStream"},
	} {
		local, err := env.FindClass(c.name)
		if err != nil {
			return err
		}
		*c.c = jni.Class(env.NewGlobalRef(jni.Object(local)))
		env.DeleteLocalRef(jni.Object(local))
	}
	var err error
	for _, m := range []struct {
		id        *jni.MethodID
		c         jni.Class
		name, sig string
		static    bool
	}{
		{&p.decodeByteArray, p.factory, "decodeByteArray", "([BIILandroid/graphics/BitmapFactory$Options;)Landroid/graphics/Bitmap;", true},
		{&p.optionsNew, p.options, "<init>", "()V", false},
		{&p.exifNew, p.exif, "<init>", "(Ljava/io/InputStream;)V", false},
		{&p.getAttributeInt, p.exif, "getAttributeInt", "(Ljava/lang/String;I)I", false},
		{&p.matrixNew, p.matrix, "<init>", "()V", false},
		{&p.postRotate, p.matrix, "postRotate", "(F)Z", false},
		{&p.postScale, p.matrix, "postScale", "(FF)Z", false},
		{&p.createBitmap, p.bitmap, "createBitmap", "(Landroid/graphics/Bitmap;IIIILandroid/graphics/Matrix;Z)Landroid/graphics/Bitmap;", true},
		{&p.compress, p.bitmap, "compress", "(Landroid/graphics/Bitmap$CompressFormat;ILjava/io/OutputStream;)Z", false},
		{&p.width, p.bitmap, "getWidth", "()I", false},
		{&p.height, p.bitmap, "getHeight", "()I", false},
		{&p.recycle, p.bitmap, "recycle", "()V", false},
		{&p.byteInNew, p.byteIn, "<init>", "([B)V", false},
		{&p.byteOutNew, p.byteOut, "<init>", "()V", false},
		{&p.toByteArray, p.byteOut, "toByteArray", "()[B", false},
	} {
		if m.static {
			*m.id, err = env.StaticMethod(m.c, m.name, m.sig)
		} else {
			*m.id, err = env.Method(m.c, m.name, m.sig)
		}
		if err != nil {
			return err
		}
	}
	for _, f := range []struct {
		id        *jni.FieldID
		name, sig string
	}{
		{&p.justBounds, "inJustDecodeBounds", "Z"},
		{&p.sample, "inSampleSize", "I"},
		{&p.outWidth, "outWidth", "I"},
		{&p.outHeight, "outHeight", "I"},
	} {
		if *f.id, err = env.Field(p.options, f.name, f.sig); err != nil {
			return err
		}
	}
	format, err := env.FindClass("android/graphics/Bitmap$CompressFormat")
	if err != nil {
		return err
	}
	defer env.DeleteLocalRef(jni.Object(format))
	jpeg, err := env.StaticField(format, "JPEG", "Landroid/graphics/Bitmap$CompressFormat;")
	if err != nil {
		return err
	}
	local := env.StaticObjectField(format, jpeg)
	p.jpeg = env.NewGlobalRef(local)
	env.DeleteLocalRef(local)
	p.ok = true

	return nil
}
