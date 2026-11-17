// Adapted from jplayer-mobile/internal/app/diskcache.go and imgcache.go (the
// disk cache of the pictures: write, read, clear, prune) at 49de3c2.

package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg" // registers the JPEG decoder of the header check
	_ "image/png"  // and PNG's
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

const (
	// DefaultImagesMax is the most the pictures of an Images may weigh after
	// the prune of its opening: the least recently written go first. Covers
	// of the courses are some hundreds of KiB each, a few MiB at most.
	DefaultImagesMax = 64 << 20
	// maxImageFile bounds what Get reads of a file: a file past it is spoiled.
	maxImageFile = 32 << 20
)

// Images is the disk cache of the pictures the app downloads (the covers of
// the courses): one file per key, named by the hex SHA-256 of the key, holding
// the picture's bytes as the server sent them. A picture is filed only when
// its header reads as a picture (an error page, a cut file are not), and a
// file that does not read back as one is a miss and is deleted. Unlike Cache
// it keeps no user's data, so a sign-out does not clear it. Methods are safe
// from any goroutine; the prune runs on a goroutine Images owns, which Close
// waits for.
type Images struct {
	dir string
	max int64
	now func() time.Time
	at  time.Time // when the cache opened: temporary files from before are leftovers

	mu     sync.Mutex
	closed bool
	work   sync.WaitGroup
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// OpenImages returns the Images of dir (made on the first Put) and starts
// its prune on a goroutine: temporary files a process left go, then the
// oldest files until those kept weigh max at most (DefaultImagesMax when max
// is 0). Close it to wait for the goroutine.
func OpenImages(dir string, max int64) *Images {
	c := newImages(dir, max)
	c.work.Add(1)
	go func() {
		defer c.work.Done()
		c.prune()
	}()

	return c
}

// newImages returns an Images that has not pruned (the tests').
func newImages(dir string, max int64) *Images {
	if max <= 0 {
		max = DefaultImagesMax
	}

	return &Images{dir: dir, max: max, now: time.Now, at: time.Now()}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Get returns the bytes of the picture of key and when it was written; false
// when the cache has none or the file is spoiled (then deleted). A nil
// *Images has none.
func (c *Images) Get(key string) ([]byte, time.Time, bool) {
	if c == nil {
		return nil, time.Time{}, false
	}
	p := c.path(key)
	f, err := os.Open(p)
	if err != nil {
		return nil, time.Time{}, false
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxImageFile+1))
	if err != nil || len(data) == 0 || len(data) > maxImageFile || !readsAsPicture(data) {
		_ = os.Remove(p)

		return nil, time.Time{}, false
	}
	var mod time.Time
	if info, serr := f.Stat(); serr == nil {
		mod = info.ModTime()
	}

	return data, mod, true
}

// Put files data as the picture of key, unless its header does not read as a
// picture (core.ErrInvalid): a temporary file written, synced and renamed,
// so that a name never holds a part of one. A closed cache writes nothing
// (core.ErrClosed). A nil *Images writes nothing.
func (c *Images) Put(key string, data []byte) error {
	if c == nil {
		return nil
	}
	if !readsAsPicture(data) {
		return fmt.Errorf("store: images put: %d bytes: not a picture: %w", len(data), core.ErrInvalid)
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()

		return fmt.Errorf("store: images put: %w", core.ErrClosed)
	}
	c.work.Add(1)
	c.mu.Unlock()
	defer c.work.Done()

	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("store: images put: %w", err)
	}
	f, err := os.CreateTemp(c.dir, tempPrefix+"*")
	if err != nil {
		return fmt.Errorf("store: images put: %w", err)
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, c.path(key))
	}
	if err != nil {
		_ = os.Remove(tmp)

		return fmt.Errorf("store: images put: %w", err)
	}

	return nil
}

// Close waits for the prune and the writes under way. The cache writes
// nothing after. Safe to call twice, and on a nil *Images.
func (c *Images) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	c.work.Wait()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// path is the file of key: the hex SHA-256 of the key.
func (c *Images) path(key string) string {
	sum := sha256.Sum256([]byte(key))

	return filepath.Join(c.dir, hex.EncodeToString(sum[:]))
}

// readsAsPicture tells whether data starts as a picture of a format the
// program decodes (its header, without its pixels).
func readsAsPicture(data []byte) bool {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))

	return err == nil && cfg.Width > 0 && cfg.Height > 0
}

// prune deletes the temporary files written before the cache opened, then
// the oldest files until those kept weigh max at most.
func (c *Images) prune() {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	type file struct {
		path string
		size int64
		mod  time.Time
	}
	files := make([]file, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		p := filepath.Join(c.dir, e.Name())
		if strings.HasPrefix(e.Name(), tempPrefix) {
			if info.ModTime().Before(c.at) { // a write under way is newer
				_ = os.Remove(p)
			}

			continue
		}
		files = append(files, file{path: p, size: info.Size(), mod: info.ModTime()})
	}
	slices.SortFunc(files, func(a, b file) int { return b.mod.Compare(a.mod) }) // the newest first
	var total int64
	full := false
	for _, f := range files {
		if !full && total+f.size <= c.max {
			total += f.size

			continue
		}
		full = true // this one and every older one go
		_ = os.Remove(f.path)
	}
}
