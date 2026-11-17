//go:build !ios && !android

package sys

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// audioBase is DataDir off a phone (the tests, the desktop run): no
// system backs it up specially here.
func audioBase() (string, error) { return DataDir() }
