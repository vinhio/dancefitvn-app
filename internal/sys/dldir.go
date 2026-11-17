package sys

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// audioFolder is the name of the folder of the downloaded audio files
// (download.Options.Dir), inside the base audioBase gives; it was
// DataDir()/audio before P5.1.
const audioFolder = "audio"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AudioDir returns the folder of the downloaded audio files and their
// index, a folder no backup takes (tens of megabytes; docs/plan/05 §6):
// on iOS DataDir()/audio (Library/Application Support), which
// PrepareAudioDir flags with NSURLIsExcludedFromBackupKey; on Android
// Context.getNoBackupFilesDir()/audio, out of Auto Backup; elsewhere
// DataDir()/audio. tvOS downloads nothing: core.ErrUnsupported. It is
// not made: PrepareAudioDir makes it.
func AudioDir() (string, error) {
	base, err := audioBase()
	if err != nil {
		return "", fmt.Errorf("sys: audio dir: %w", err)
	}

	return filepath.Join(base, audioFolder), nil
}

// PrepareAudioDir returns AudioDir made (0700), flagged out of backups
// where the system backs files up (iOS), and holding what the folder of
// before, DataDir()/audio, held: that folder's entries are moved in once
// (a rename, on the same volume), and the folder removed once empty, so
// that a later call finds nothing to move. It fails with "" when the
// folder cannot be made. A move or the flag failing still returns the
// folder, made, with the error (an entry whose name the new folder
// already has stays where it was: core.ErrDuplicate): the caller logs it
// and uses the folder. It does disk I/O: call it off the UI thread,
// before download.NewManager.
func PrepareAudioDir() (string, error) {
	dir, err := AudioDir()
	if err != nil {
		return "", err
	}
	data, err := DataDir()
	if err != nil {
		return "", fmt.Errorf("sys: audio dir: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("sys: audio dir: %w", err)
	}
	if err := settleAudioDir(filepath.Join(data, audioFolder), dir); err != nil {
		return dir, fmt.Errorf("sys: audio dir: %w", err)
	}

	return dir, nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// settleAudioDir moves the entries of legacy into dir, which exists, and
// flags dir out of backups where the system has the flag. A failed move
// does not keep the flag from being set; both are reported.
func settleAudioDir(legacy, dir string) error {
	moveErr := moveAudio(legacy, dir)

	return errors.Join(excludeIfSupported(dir), moveErr)
}

// moveAudio moves the entries of the folder from into the folder to,
// which exists, and removes from once empty. A missing from, or from
// being to, is nothing to move. An entry to already has is left in from
// and reported (core.ErrDuplicate); a failed rename is reported too, the
// other entries moved all the same.
func moveAudio(from, to string) error {
	if filepath.Clean(from) == filepath.Clean(to) {
		return nil
	}
	entries, err := os.ReadDir(from)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("move %s: %w", from, err)
	}
	var errs []error
	for _, e := range entries {
		src, dst := filepath.Join(from, e.Name()), filepath.Join(to, e.Name())
		if _, err := os.Lstat(dst); err == nil {
			errs = append(errs, fmt.Errorf("move %s: %s exists: %w", src, dst, core.ErrDuplicate))

			continue
		}
		if err := os.Rename(src, dst); err != nil {
			errs = append(errs, fmt.Errorf("move %s: %w", src, err))
		}
	}
	if len(errs) == 0 {
		if err := os.Remove(from); err != nil {
			errs = append(errs, fmt.Errorf("remove %s: %w", from, err))
		}
	}

	return errors.Join(errs...)
}
