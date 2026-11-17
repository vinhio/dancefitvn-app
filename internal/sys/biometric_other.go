//go:build !android && (!ios || tvos)

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// biometry reports core.ErrUnsupported: only the iPhone and Android have
// a biometric sheet (tvOS has none; the desktop and tests none here).
func biometry() (BiometryKind, error) {
	return BiometryNone, fmt.Errorf("sys: biometric: %w", core.ErrUnsupported)
}

// authenticate reports core.ErrUnsupported, as biometry.
func authenticate(Dispatch, BioPrompt, func(error)) (func(), error) {
	return nil, fmt.Errorf("sys: biometric: %w", core.ErrUnsupported)
}
