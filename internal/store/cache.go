// Adapted from jplayer-mobile/internal/app/diskcache.go (write, clear, prune).

package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/api"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// DefaultCacheMax is the most the files of a Cache may weigh after the prune
// of its opening (docs/plan P1.5).
const DefaultCacheMax = 5 << 20

const (
	// cacheOldSuffix ends the name a cleared folder is renamed to before it
	// is deleted on a goroutine; an opening deletes those a process left.
	cacheOldSuffix = ".old-"
	cacheExt       = ".json"
)

// Cache is the api.Cache on disk: one file per key, <dir>/<hex sha256(key)>.json
// holding {etag, key, saved_at, body}. The body is JSON and comes back
// compacted. A file that does not read back as what was written (cut short,
// spoiled, another key) is a miss and is deleted.
//
// Keys do not carry the user, so the owner clears the Cache when the account
// signs out or changes (auth.Options.ClearCache). Methods are safe from any
// goroutine; the prune and the deletion of a cleared folder run on goroutines
// the Cache owns, which Close waits for.
type Cache struct {
	dir string
	max int64
	now func() time.Time
	at  time.Time // when the Cache opened: temporary files from before are leftovers

	mu     sync.Mutex
	closed bool
	work   sync.WaitGroup
}

var _ api.Cache = (*Cache)(nil)

// entry is a file of the Cache.
type entry struct {
	ETag    string         `json:"etag"`
	Key     string         `json:"key"`
	SavedAt time.Time      `json:"saved_at"`
	Body    jsontext.Value `json:"body"`
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// OpenCache returns the Cache of dir (made on the first Put) and starts its
// prune on a goroutine: temporary files and cleared folders a process left
// go, then the oldest files until those kept weigh max at most (DefaultCacheMax
// when max is 0). Close it to wait for the goroutines.
func OpenCache(dir string, max int64) *Cache {
	c := newCache(dir, max)
	c.work.Add(1)
	go func() {
		defer c.work.Done()
		c.prune()
	}()

	return c
}

// newCache returns a Cache that has not pruned (the tests').
func newCache(dir string, max int64) *Cache {
	if max <= 0 {
		max = DefaultCacheMax
	}

	return &Cache{dir: dir, max: max, now: time.Now, at: time.Now()}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Get implements api.Cache. The body is a copy.
func (c *Cache) Get(key string) (string, []byte, bool) {
	p := c.path(key)
	raw, err := os.ReadFile(p)
	if err != nil {
		return "", nil, false
	}
	var e entry
	if err := json.Unmarshal(raw, &e); err != nil || e.Key != key || e.ETag == "" || len(e.Body) == 0 || string(e.Body) == "null" || !e.Body.IsValid() {
		_ = os.Remove(p)

		return "", nil, false
	}

	return e.ETag, []byte(e.Body), true
}

// Put implements api.Cache: the body (JSON) is written to a temporary file,
// synced and renamed. An empty etag or a body that is not JSON is
// core.ErrInvalid. A closed Cache writes nothing (core.ErrClosed).
func (c *Cache) Put(key, etag string, body []byte) error {
	if etag == "" || !jsontext.Value(body).IsValid() {
		return fmt.Errorf("store: cache put: %w", core.ErrInvalid)
	}
	raw, err := json.Marshal(entry{ETag: etag, Key: key, SavedAt: c.now().UTC(), Body: body})
	if err != nil {
		return fmt.Errorf("store: cache put: %w: %w", core.ErrInvalid, err)
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()

		return fmt.Errorf("store: cache put: %w", core.ErrClosed)
	}
	c.work.Add(1)
	c.mu.Unlock()
	defer c.work.Done()

	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("store: cache put: %w", err)
	}
	f, err := os.CreateTemp(c.dir, tempPrefix+"*")
	if err != nil {
		return fmt.Errorf("store: cache put: %w", err)
	}
	tmp := f.Name()
	_, err = f.Write(raw)
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

		return fmt.Errorf("store: cache put: %w", err)
	}

	return nil
}

// Clear empties the Cache: its folder is renamed at once, so that nothing
// reads a file of it any more and later writes go to a new one, then the old
// folder is deleted on a goroutine.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	old := c.dir + cacheOldSuffix + strconv.FormatInt(c.now().UnixNano(), 36)
	if err := os.Rename(c.dir, old); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		old = c.dir // rename failed: delete in place
	}
	c.work.Add(1)
	go func() {
		defer c.work.Done()
		_ = os.RemoveAll(old)
	}()
}

// Close waits for the prune, the deletions and the writes under way. The
// Cache clears and writes nothing after. Safe to call twice.
func (c *Cache) Close() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	c.work.Wait()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// path is the file of key: the hex SHA-256 of the key.
func (c *Cache) path(key string) string {
	sum := sha256.Sum256([]byte(key))

	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+cacheExt)
}

// prune deletes temporary files, cleared folders a process left, and the
// oldest files until the rest weighs max at most.
func (c *Cache) prune() {
	if parent, base := filepath.Dir(c.dir), filepath.Base(c.dir); parent != "" {
		if sibs, err := os.ReadDir(parent); err == nil {
			for _, s := range sibs {
				if strings.HasPrefix(s.Name(), base+cacheOldSuffix) {
					_ = os.RemoveAll(filepath.Join(parent, s.Name()))
				}
			}
		}
	}
	files, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	type kept struct {
		path string
		size int64
		mod  time.Time
	}
	var all []kept
	var total int64
	for _, f := range files {
		p := filepath.Join(c.dir, f.Name())
		info, err := f.Info()
		if err != nil {
			continue
		}
		if strings.HasPrefix(f.Name(), tempPrefix) {
			if info.ModTime().Before(c.at) { // a write under way is newer
				_ = os.Remove(p)
			}

			continue
		}
		if f.IsDir() || !strings.HasSuffix(f.Name(), cacheExt) {
			_ = os.RemoveAll(p)

			continue
		}
		all = append(all, kept{p, info.Size(), info.ModTime()})
		total += info.Size()
	}
	sort.Slice(all, func(i, j int) bool { return all[i].mod.Before(all[j].mod) })
	for _, k := range all {
		if total <= c.max {
			break
		}
		if os.Remove(k.path) == nil {
			total -= k.size
		}
	}
}
