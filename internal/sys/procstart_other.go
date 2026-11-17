//go:build !ios && !android

package sys

import (
	"fmt"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// processStart is not read on a desktop, which times no launch of the
// app: it fails with core.ErrUnsupported.
func processStart() (time.Time, error) {
	return time.Time{}, fmt.Errorf("sys: process start: %w", core.ErrUnsupported)
}
