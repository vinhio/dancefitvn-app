// Adapted from jplayer-mobile/internal/sys/services.go at 49de3c2.
// Dispatch, ErrCanceled, checkAsync, checkURL and busy are kept: the
// services that answer later (biometrics, opening a URL, the photo
// picker) hand their done to Dispatch.

package sys

import (
	"context"
	"fmt"
	"net/url"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Dispatch runs fn on the app's UI thread, later and without waiting: the
// app's host.App.Enqueue (ui.Go's in a test). The services that answer
// later (the biometric sheet, opening a URL) hand their done to it, so
// done always runs on the UI thread, never inside the call that started
// it.
type Dispatch func(fn func())

// ErrCanceled is what done gets when the user dismissed a picker without
// choosing anything. errors.Is(err, context.Canceled) holds, so the
// screens take it as the user's own choice, never as a failure to show.
var ErrCanceled = fmt.Errorf("sys: canceled by the user: %w", context.Canceled)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// checkAsync checks what every service answering later needs: a canvas
// (the window the service acts for), a dispatch and a done.
func checkAsync(op string, c platform.Canvas, d Dispatch, hasDone bool) error {
	switch {
	case c == nil:
		return fmt.Errorf("sys: %s: no canvas: %w", op, core.ErrInvalid)
	case d == nil:
		return fmt.Errorf("sys: %s: no dispatch: %w", op, core.ErrInvalid)
	case !hasDone:
		return fmt.Errorf("sys: %s: no done function: %w", op, core.ErrInvalid)
	default:
		return nil
	}
}

// checkURL checks that raw is an absolute URL, with a scheme, which the
// system can hand to an app: "https://…", "mailto:…".
func checkURL(op, raw string) error {
	u, err := url.Parse(raw)
	switch {
	case raw == "":
		return fmt.Errorf("sys: %s: empty URL: %w", op, core.ErrInvalid)
	case err != nil:
		return fmt.Errorf("sys: %s: %w: %w", op, core.ErrInvalid, err)
	case u.Scheme == "":
		return fmt.Errorf("sys: %s: %q has no scheme: %w", op, raw, core.ErrInvalid)
	default:
		return nil
	}
}

// busy is the error of a service asked while its sheet or picker shows
// already: one at a time.
func busy(op string) error {
	return fmt.Errorf("sys: %s: already showing: %w", op, core.ErrDuplicate)
}
