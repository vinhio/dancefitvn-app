package sys

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// IdleTimerDisabled reads UIApplication's isIdleTimerDisabled: true while
// the system keeps the screen on (no dimming on an iPhone, no screen
// saver on an Apple TV), which KeepAwake asks for through gDesk's
// power.KeepAwake. It is the probe of a developer's build and of the
// device tests (local/plans/P7.md P7.5c): gDesk tells no such value.
// Call it on the UI thread. Elsewhere than iOS and tvOS it fails with
// core.ErrUnsupported.
func IdleTimerDisabled() (bool, error) { return idleTimerDisabled() }
