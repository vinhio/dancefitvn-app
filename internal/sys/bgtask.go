package sys

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// BeginBackgroundTask asks the system for time to finish work after the
// app left the screen: the downloads under way go on for the grace the
// system grants (about 30 s on iOS) instead of stopping as the app is
// suspended (fallback F5a of docs/plan/06-gdesk-extensions.md, until
// gDesk's G5a bridge/system/background). end gives the time back; call
// it once the work is over. expired, which may be nil, runs on the UI
// thread when the grace runs out: the work must stop then (pause the
// downloads, keeping their bytes), and the task is ended right after it.
// end may be called more than once, and after the expiry; it does
// nothing then.
//
// On iOS it is -[UIApplication beginBackgroundTaskWithName:
// expirationHandler:] and endBackgroundTask:. Call it on the UI thread.
// A system that grants no time (UIBackgroundTaskInvalid) is
// core.ErrClosed.
// Elsewhere (Android keeps the process running, tvOS downloads nothing,
// the Mac) it fails with core.ErrUnsupported.
func BeginBackgroundTask(name string, expired func()) (end func(), err error) {
	if name == "" {
		return nil, fmt.Errorf("sys: background task: no name: %w", core.ErrInvalid)
	}
	if expired == nil {
		expired = func() {}
	}

	return beginBackgroundTask(name, expired)
}
