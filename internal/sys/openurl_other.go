//go:build !android && (!ios || tvos)

// Adapted from jplayer-mobile/internal/sys/openurl_other.go at 49de3c2;
// tvOS, which has no browser, is here too.

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// openURL has no system to hand a URL to off a phone.
func openURL(platform.Canvas, Dispatch, string, func(error)) error {
	return fmt.Errorf("sys: open URL: %w", core.ErrUnsupported)
}
