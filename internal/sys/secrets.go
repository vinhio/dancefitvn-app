package sys

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gdeskdev/desk/bridge/system/secret"
	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The files of the secret folder (SecretDir).
const (
	secretsFile = "secrets.json" // the F1 store, AES-256-GCM sealed
	secretsKey  = "secrets.key"  // its random key (secret.FileKey.KeyPath)
	damagedTag  = ".bad-"        // what a store moved aside is suffixed with, then the Unix time
	// clearedMark tells that the keychain's items of an earlier install
	// were cleared (clearStaleKeychain): iOS keeps Keychain items after
	// the app is deleted, while SecretDir goes with it.
	clearedMark = "keychain-cleared"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Secrets keeps the app's secrets (the session's tokens, 05 §6): in the
// system's keychain where gDesk's driver keeps secrets (G1: the iOS and
// tvOS Keychain, Android's Keystore), otherwise in the F1 file store of
// SecretDir, which the system does not back up (an iOS app signed
// without the keychain's entitlement, desktop systems without one). It
// is secret.WithFallback(secret.NewStore(d, AppID), the file store), so
// values written to the file before G1 stay readable after it.
//
// The keychain's calls run on the UI thread (bridge/system.Service):
// where the driver keeps secrets, each call goes through invoke, which
// is host.App.Invoke (it runs fn at once on the UI thread, and waits for
// it from another goroutine). Without such a driver the calls reach only
// the file store, which any goroutine may use. Never call it from the UI
// thread while holding a lock a goroutine waiting in invoke holds.
type Secrets struct {
	store   secret.Store
	invoke  func(func()) error // nil when every call may run where it is made
	dir     string
	damaged string // the store moved aside as it opened, "" when none was
}

// OpenSecrets opens the app's secrets: it makes SecretDir (0700, out of
// backups on iOS) and opens the file store there, the keychain of d in
// front of it. invoke runs a function on the UI thread
// (host.App.Invoke); it may be nil only while d keeps no secrets. A
// store that its key does not open (damaged, or its key lost) is moved
// aside and an empty one used: the user signs in again (Damaged tells).
// The first open of an install (SecretDir without its mark) clears what
// the keychain kept for AppID before: iOS keeps Keychain items after the
// app is deleted, so a reinstalled app would find an earlier install's
// tokens.
func OpenSecrets(d platform.Driver, invoke func(func()) error) (*Secrets, error) {
	dir, err := SecretDir()
	if err != nil {
		return nil, err
	}

	return openSecrets(secret.NewStore(d, AppID), keepsSecrets(d), invoke, dir)
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Set keeps value for account, replacing what was kept.
func (s *Secrets) Set(account string, value []byte) error {
	var err error
	if ierr := s.call(func() { err = s.store.Set(account, value) }); ierr != nil {
		return fmt.Errorf("sys: secrets: set: %w", ierr)
	}
	if err != nil {
		return fmt.Errorf("sys: secrets: set: %w", err)
	}

	return nil
}

// Get returns the value kept for account; the error wraps
// core.ErrNotFound when none is.
func (s *Secrets) Get(account string) ([]byte, error) {
	var (
		v   []byte
		err error
	)
	if ierr := s.call(func() { v, err = s.store.Get(account) }); ierr != nil {
		return nil, fmt.Errorf("sys: secrets: get: %w", ierr)
	}
	if err != nil {
		return nil, fmt.Errorf("sys: secrets: get: %w", err)
	}

	return v, nil
}

// Delete forgets the value of account, in the keychain and the file
// store; none kept is no error.
func (s *Secrets) Delete(account string) error {
	var err error
	if ierr := s.call(func() { err = s.store.Delete(account) }); ierr != nil {
		return fmt.Errorf("sys: secrets: delete: %w", ierr)
	}
	if err != nil {
		return fmt.Errorf("sys: secrets: delete: %w", err)
	}

	return nil
}

// Dir is the folder of the file store.
func (s *Secrets) Dir() string { return s.dir }

// Damaged is the path the store was moved aside to as it opened, because
// its key did not open it; "" when it opened.
func (s *Secrets) Damaged() string { return s.damaged }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// openSecrets opens the file store of dir behind primary. systemKeeps
// tells primary needs the UI thread (a keychain), so calls go through
// invoke.
func openSecrets(primary secret.Store, systemKeeps bool, invoke func(func()) error, dir string) (*Secrets, error) {
	if systemKeeps && invoke == nil {
		return nil, fmt.Errorf("sys: secrets: no invoke for the keychain's UI thread: %w", core.ErrInvalid)
	}
	if err := prepareSecretDir(dir); err != nil {
		return nil, fmt.Errorf("sys: secrets: %w", err)
	}
	s := &Secrets{dir: dir}
	if systemKeeps {
		s.invoke = invoke
		if err := s.clearStaleKeychain(primary); err != nil {
			slog.Warn("sys: secrets: the keychain's items of an earlier install were not cleared", "err", err)
		}
	}
	file, err := openFileStore(dir)
	if isUnreadable(err) {
		s.damaged, err = moveAside(dir, time.Now())
		if err == nil {
			slog.Warn("sys: secrets: the secret store did not open; moved aside, starting empty", "path", s.damaged)
			file, err = openFileStore(dir)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("sys: secrets: %w", err)
	}
	s.store = secret.WithFallback(primary, file)

	return s, nil
}

// openFileStore opens the F1 store of dir with its key file.
func openFileStore(dir string) (*secret.FileStore, error) {
	return secret.OpenFileStore(filepath.Join(dir, secretsFile), secret.FileKey{KeyPath: filepath.Join(dir, secretsKey)})
}

// isUnreadable tells err is OpenFileStore's for a store it cannot read:
// a wrong key (secret.ErrWrongPassword), a file or key that is not a
// store's (core.ErrInvalid), a store whose key is gone (core.ErrNotFound).
func isUnreadable(err error) bool {
	return err != nil && (errors.Is(err, core.ErrInvalid) || errors.Is(err, core.ErrNotFound))
}

// moveAside renames the store and its key of dir to <name>.bad-<unix>,
// and returns the store's new path.
func moveAside(dir string, now time.Time) (string, error) {
	suffix := damagedTag + strconv.FormatInt(now.Unix(), 10)
	store := filepath.Join(dir, secretsFile)
	if err := os.Rename(store, store+suffix); err != nil {
		return "", err
	}
	key := filepath.Join(dir, secretsKey)
	if err := os.Rename(key, key+suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	return store + suffix, nil
}

// clearStaleKeychain deletes every account primary (the keychain) holds,
// once per install: when s.dir has no clearedMark, which it then gets.
// The keychain's items outlive the app on iOS, its folders do not, so
// the items found then are an earlier install's. A keychain that cannot
// list (core.ErrUnsupported: an app signed without the keychain's
// entitlement) leaves the mark unwritten, to try again at the next
// launch; the fallback file store is never touched.
func (s *Secrets) clearStaleKeychain(primary secret.Store) error {
	mark := filepath.Join(s.dir, clearedMark)
	if _, err := os.Stat(mark); err == nil {
		return nil
	}
	var (
		accounts []string
		err      error
	)
	if ierr := s.call(func() { accounts, err = primary.List() }); ierr != nil {
		return ierr
	}
	if errors.Is(err, core.ErrUnsupported) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, a := range accounts {
		var derr error
		if ierr := s.call(func() { derr = primary.Delete(a) }); ierr != nil {
			return ierr
		}
		if derr != nil {
			return derr
		}
	}
	if len(accounts) > 0 {
		slog.Info("sys: secrets: cleared the keychain's items of an earlier install", "count", len(accounts))
	}

	return writeAtomic(mark, []byte("1\n"))
}

// call runs fn through invoke where the keychain needs the UI thread,
// and at once otherwise.
func (s *Secrets) call(fn func()) error {
	if s.invoke == nil {
		fn()

		return nil
	}

	return s.invoke(fn)
}

// keepsSecrets tells whether d keeps secrets in the system's keychain.
func keepsSecrets(d platform.Driver) bool {
	_, ok := d.(platform.SecretDriver)

	return ok
}
