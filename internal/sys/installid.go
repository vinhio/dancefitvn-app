package sys

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"uuid"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// installIDFile is the install id's file in SecretDir.
const installIDFile = "install-id"

// installIDRe is the form of an install id: a UUID version 4 (RFC 9562
// variant), lowercase, as uuid.UUID.String writes it.
var installIDRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// installIDs serializes InstallID, so that two first calls make one id.
var installIDs sync.Mutex

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// InstallID returns the id of this install of the app (05 §1, the
// X-Install-Id header and the secrets' account): a random UUID v4 made at
// the first call and kept in SecretDir, which the system never backs up,
// so a phone restored from this one makes its own. A file holding
// anything else than an id is replaced by a new one. It runs on any
// goroutine.
func InstallID() (string, error) {
	dir, err := SecretDir()
	if err != nil {
		return "", err
	}

	return installIDIn(dir)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// installIDIn reads the install id of dir, making it when missing or
// malformed.
func installIDIn(dir string) (string, error) {
	installIDs.Lock()
	defer installIDs.Unlock()
	path := filepath.Join(dir, installIDFile)
	raw, err := os.ReadFile(path) //nolint:gosec // G304: the app's own file
	switch {
	case err == nil:
		if id := strings.TrimSpace(string(raw)); installIDRe.MatchString(id) {
			return id, nil
		}
	case !errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("sys: install id: %w", err)
	default: // no file yet: made below
	}
	if err := prepareSecretDir(dir); err != nil {
		return "", fmt.Errorf("sys: install id: %w", err)
	}
	id := uuid.NewV4().String()
	if err := writeAtomic(path, []byte(id+"\n")); err != nil {
		return "", fmt.Errorf("sys: install id: %w", err)
	}

	return id, nil
}

// writeAtomic writes data to path through a temporary file of its folder,
// synced and renamed over it, both 0600 (as gDesk's
// bridge/system/secret/file.go writeFile).
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr, os.Chmod(name, 0o600)); err != nil {
		_ = os.Remove(name)

		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)

		return err
	}

	return nil
}
