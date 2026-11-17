//go:build ios && !tvos

package sys

import "os"

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// dataBase is os.UserConfigDir: the app's Library/Application Support.
func dataBase() (string, error) { return os.UserConfigDir() }

// secretBase is DataDir: the secret folder is flagged out of backups as
// it is made (prepareSecretDir).
func secretBase() (string, error) { return DataDir() }
