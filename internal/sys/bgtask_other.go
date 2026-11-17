//go:build !ios || tvos

package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// beginBackgroundTask has no time to ask for off the iPhone: Android keeps
// the process running in the background, tvOS downloads nothing, the Mac
// never suspends the app.
func beginBackgroundTask(string, func()) (func(), error) {
	return nil, fmt.Errorf("sys: background task: %w", core.ErrUnsupported)
}
