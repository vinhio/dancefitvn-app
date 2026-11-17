//go:build !ios && !android

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// freeSpace knows no space off a phone: the screens show none (Q-P5-9),
// and the tests stay the same on every machine.
func freeSpace(string) (free, total int64, err error) {
	return 0, 0, fmt.Errorf("free space: %w", core.ErrUnsupported)
}
