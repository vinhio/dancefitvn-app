//go:build android

// Adapted from jplayer-mobile/internal/sys/pickfiles_android.go at
// 49de3c2 (startForResult, resultURIs, itemURI, openDocument): only what
// the photo picker needs; the lookups are intents' (intent_android.go).

package sys

import (
	"fmt"
	"io"
	"os"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// startForResult makes the intent of build and starts it for a result
// that fn gets. Java's main thread only.
func startForResult(env jni.Env, build func(jni.Env) (jni.Object, error), fn func(env jni.Env, code int32, data jni.Object)) error {
	if err := lookupIntents(env); err != nil {
		return err
	}
	intent, err := build(env)
	if err != nil {
		return err
	}

	return jni.StartActivityForResult(env, intent, fn)
}

// resultURIs returns the URIs of a picker's result as strings: the items
// of its ClipData (a pick of several), else its data. A result that is not
// RESULT_OK, or of no URI, is the user's cancel. The references made for
// each item are deleted as the loop goes.
func resultURIs(env jni.Env, code int32, data jni.Object) ([]string, error) {
	if code != jni.ResultOK || data == 0 {
		return nil, ErrCanceled
	}
	var uris []string
	clip, err := env.CallObject(data, intents.getClipData)
	if err != nil {
		return nil, err
	}
	if clip != 0 {
		n, err := env.CallInt(clip, intents.itemCount)
		if err != nil {
			env.DeleteLocalRef(clip)

			return nil, err
		}
		for i := range n {
			s, err := itemURI(env, clip, i)
			if err != nil {
				env.DeleteLocalRef(clip)

				return nil, err
			}
			if s != "" {
				uris = append(uris, s)
			}
		}
		env.DeleteLocalRef(clip)
	}
	if len(uris) == 0 {
		uri, err := env.CallObject(data, intents.getData)
		if err != nil {
			return nil, err
		}
		s, err := stringOf(env, uri)
		env.DeleteLocalRef(uri)
		if err != nil {
			return nil, err
		}
		if s != "" {
			uris = append(uris, s)
		}
	}
	if len(uris) == 0 {
		return nil, ErrCanceled
	}

	return uris, nil
}

// itemURI returns the URI of item i of clip as a string, deleting the
// references it makes.
func itemURI(env jni.Env, clip jni.Object, i int32) (string, error) {
	item, err := env.CallObject(clip, intents.itemAt, jni.Int(i))
	if err != nil || item == 0 {
		return "", err
	}
	defer env.DeleteLocalRef(item)
	uri, err := env.CallObject(item, intents.itemURI)
	if err != nil {
		return "", err
	}
	defer env.DeleteLocalRef(uri)

	return stringOf(env, uri)
}

// openDocument opens the document s for reading: the content resolver's
// file descriptor, detached from its ParcelFileDescriptor and handed to an
// os.File named name, which closes it.
func openDocument(s, name string) (io.ReadCloser, error) {
	var fd int32 = -1
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
		mode, err := env.NewString("r")
		if err != nil {
			return err
		}
		pfd, err := env.CallObject(res, intents.openFD, jni.Ref(uri), jni.Ref(mode))
		if err != nil {
			return err
		}
		if pfd == 0 {
			return fmt.Errorf("no file descriptor: %w", core.ErrNotFound)
		}
		if fd, err = env.CallInt(pfd, intents.detachFd); err != nil {
			return err
		}

		return env.CallVoid(pfd, intents.pfdClose) // closes nothing once detached
	})
	if err != nil {
		if fd >= 0 {
			_ = os.NewFile(uintptr(fd), name).Close()
		}

		return nil, fmt.Errorf("sys: open picked file: %w", err)
	}

	return os.NewFile(uintptr(fd), name), nil
}
