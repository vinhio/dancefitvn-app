package sys

import (
	"fmt"
	"os"
	"path/filepath"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// AppID is the app's identifier: the name of its folders and the service
// of its secrets (docs/plan/05-data.md §6).
const AppID = "com.jivecode.dancefitvn"

// secretFolder is the name of the folder holding the secret store, its
// key and the install id, inside the base secretBase gives.
const secretFolder = "secret"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// DataDir returns the app's data folder, <base>/com.jivecode.dancefitvn:
// on iOS and Android the base is os.UserConfigDir (iOS: Library/
// Application Support; Android: files/.config, since gDesk sets HOME to
// the app's files folder), on tvOS os.UserCacheDir (Library/Caches, the
// folder tvOS lets an app write, as jPlayer TV found). It is not made.
func DataDir() (string, error) {
	base, err := dataBase()
	if err != nil {
		return "", fmt.Errorf("sys: data dir: %w", err)
	}

	return filepath.Join(base, AppID), nil
}

// CacheDir returns the app's cache folder,
// <os.UserCacheDir>/com.jivecode.dancefitvn, which the system may empty.
// It is not made.
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("sys: cache dir: %w", err)
	}

	return filepath.Join(base, AppID), nil
}

// SecretDir returns the folder of the secret store (F1), its key and the
// install id: a folder the system never backs up, so that a phone
// restored from this one neither holds its tokens nor takes its install
// id. On Android it is Context.getNoBackupFilesDir()/secret; on iOS and
// tvOS DataDir()/secret, which OpenSecrets and InstallID flag with
// NSURLIsExcludedFromBackupKey; elsewhere DataDir()/secret. It is not
// made.
func SecretDir() (string, error) {
	base, err := secretBase()
	if err != nil {
		return "", fmt.Errorf("sys: secret dir: %w", err)
	}

	return filepath.Join(base, secretFolder), nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// prepareSecretDir makes dir (0700) when missing, forces its mode to
// 0700 and, where the system backs files up (iOS), keeps it out of
// backups. A system without the flag (ErrUnsupported) is no failure.
func prepareSecretDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}

	return excludeIfSupported(dir)
}
