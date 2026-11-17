package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tempPrefix starts the name of a file being written; Dir never reads one.
const tempPrefix = ".tmp-"

// keyPattern is what a key looks like (jplayer-tv/internal/store/document.go).
var keyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// Dir is a Backend of one JSON file per key, <dir>/<key>.json, written
// atomically (temporary file, fsync, rename; the write pattern of
// jplayer-mobile/internal/app/diskcache.go). Keys match
// ^[a-z0-9][a-z0-9._-]*$, else core.ErrInvalid. It is safe for concurrent
// use and may be called from any goroutine; the first write of a process can
// take a second or two on Android.
type Dir struct {
	dir string

	fsync func(*os.File) error // the sync of a written file; tests swap it
}

var _ Backend = (*Dir)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewDir returns a Dir keeping its files in dir, which Save makes (0700)
// when it is missing.
func NewDir(dir string) *Dir {
	return &Dir{dir: dir, fsync: func(f *os.File) error { return f.Sync() }}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Load reads <dir>/<key>.json as it was written. A missing file wraps
// core.ErrNotFound.
func (d *Dir) Load(key string) ([]byte, error) {
	p, err := d.path(key)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("store: load %s: %w", key, core.ErrNotFound)
		}

		return nil, fmt.Errorf("store: load %s: %w", key, err)
	}

	return b, nil
}

// Save writes <dir>/<key>.json: a temporary file is written, synced and
// renamed, so that the file's name never holds a part of a write. A failed
// Save leaves the old file as it was and no temporary file.
func (d *Dir) Save(key string, data []byte) error {
	p, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d.dir, 0o700); err != nil {
		return fmt.Errorf("store: save %s: %w", key, err)
	}
	f, err := os.CreateTemp(d.dir, tempPrefix+"*")
	if err != nil {
		return fmt.Errorf("store: save %s: %w", key, err)
	}
	tmp := f.Name()
	_, err = f.Write(slices.Clone(data))
	if err == nil {
		err = d.fsync(f)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, p)
	}
	if err != nil {
		_ = os.Remove(tmp)

		return fmt.Errorf("store: save %s: %w", key, err)
	}

	return nil
}

// Remove deletes the file of key; a missing file is no error.
func (d *Dir) Remove(key string) error {
	p, err := d.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("store: remove %s: %w", key, err)
	}

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// path returns the file of key, or core.ErrInvalid for a key that could
// leave dir.
func (d *Dir) path(key string) (string, error) {
	if !keyPattern.MatchString(key) {
		return "", fmt.Errorf("store: key %q: %w", key, core.ErrInvalid)
	}

	return filepath.Join(d.dir, key+".json"), nil
}
