//go:build android

package sys

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// audioBase is Context.getNoBackupFilesDir(), as the secret folder's
// base: Auto Backup would otherwise upload tens of megabytes of audio.
func audioBase() (string, error) { return secretBase() }
