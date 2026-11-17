//go:build android

package sys

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// freeSpace is statfs on Android: StatFs.getAvailableBytes reads the same
// f_bavail · f_bsize, so no JNI call is needed.
func freeSpace(path string) (free, total int64, err error) { return statfsSpace(path) }
