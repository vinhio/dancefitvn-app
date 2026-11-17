//go:build android

// Adapted from jplayer-mobile/internal/sys/openurl_android.go at 49de3c2.

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// openURL starts an ACTION_VIEW intent of the URL on Java's main thread,
// where activities start; done gets, through d, nil or what Java threw
// (ActivityNotFoundException when no app takes it).
func openURL(_ platform.Canvas, d Dispatch, rawURL string, done func(error)) error {
	err := jni.PostMain(func(env jni.Env) {
		err := viewURL(env, rawURL)
		if err != nil {
			err = fmt.Errorf("sys: open URL: %w", err)
		}
		d(func() { done(err) })
	})
	if err != nil {
		return fmt.Errorf("sys: open URL: %w", err)
	}

	return nil
}

// viewURL starts the ACTION_VIEW intent of rawURL.
func viewURL(env jni.Env, rawURL string) error {
	if err := lookupIntents(env); err != nil {
		return err
	}
	uri, err := parseURI(env, rawURL)
	if err != nil {
		return err
	}
	defer env.DeleteLocalRef(uri)
	intent, err := newIntent(env, actionView, uri)
	if err != nil {
		return err
	}
	defer env.DeleteLocalRef(intent)

	return startActivity(env, intent)
}
