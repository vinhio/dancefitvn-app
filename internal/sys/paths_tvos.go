//go:build ios && tvos

// Adapted from jplayer-tv/internal/app/datadir_tvos.go at 781bee5
// (dataDir: the app's data under os.UserCacheDir on tvOS).

package sys

import "os"

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// dataBase is os.UserCacheDir, the app's Library/Caches: tvOS lets an app
// write there and promises nothing more durable than NSUserDefaults
// (jPlayer TV T0.5, DT10). The system may purge it when storage runs low:
// the TV then signs out and pairs again (R-P1-5, G1).
func dataBase() (string, error) { return os.UserCacheDir() }

// secretBase is DataDir, flagged out of backups as on iOS.
func secretBase() (string, error) { return DataDir() }
