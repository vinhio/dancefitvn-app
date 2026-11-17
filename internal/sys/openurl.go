// Adapted from jplayer-mobile/internal/sys/openurl.go at 49de3c2 (F7,
// until gDesk's G8 opens a URL itself).

package sys

import (
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// OpenURL hands rawURL to the system, which opens it in the app that
// takes it: the browser for an http(s) link ("Quên mật khẩu?", K15), the
// store for the store's page of an update.
// done, which may be nil, gets through d whether it opened: nil, or an
// error when no app takes the URL (core.ErrNotFound) or the system
// refused it. The app logs the error and goes on.
//
// On iOS it calls -[UIApplication openURL:options:completionHandler:]; on
// Android it starts an ACTION_VIEW intent of the URL from the activity
// (the application's context, as a new task, while there is none).
//
// Call it on the UI thread. A URL without a scheme is core.ErrInvalid. A
// returned error means done will not run. Elsewhere than iOS and Android
// (tvOS has no browser) it fails with core.ErrUnsupported.
func OpenURL(c platform.Canvas, d Dispatch, rawURL string, done func(error)) error {
	if err := checkAsync("open URL", c, d, true); err != nil {
		return err
	}
	if err := checkURL("open URL", rawURL); err != nil {
		return err
	}
	if done == nil {
		done = func(error) {}
	}

	return openURL(c, d, rawURL, done)
}
