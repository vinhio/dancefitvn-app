//go:build !ios && !android

package sys

import "os"

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// dataBase is os.UserConfigDir: on a Mac ~/Library/Application Support.
// The folders are plain ones here (tests, the desktop run).
func dataBase() (string, error) { return os.UserConfigDir() }

// secretBase is DataDir: no system backs it up specially here.
func secretBase() (string, error) { return DataDir() }
