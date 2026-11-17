//go:build !android && (!ios || tvos)

// Adapted from jplayer-mobile/internal/sys/pickimage_other.go at 49de3c2;
// tvOS, which has no photo picker, is here too.

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// pickImage has no photo picker off a phone.
func pickImage(platform.Canvas, Dispatch, func([]byte, string, error)) error {
	return fmt.Errorf("sys: pick image: %w", core.ErrUnsupported)
}
