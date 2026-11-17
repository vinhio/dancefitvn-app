package store

import "os"

// FailSync makes the Dir's fsync fail with err (nil restores it).
func FailSync(d *Dir, err error) {
	if err == nil {
		d.fsync = func(f *os.File) error { return f.Sync() }

		return
	}
	d.fsync = func(*os.File) error { return err }
}

// NewCacheNoPrune returns a Cache that has not pruned.
func NewCacheNoPrune(dir string, max int64) *Cache { return newCache(dir, max) }

// Prune runs the prune of an opening now.
func (c *Cache) Prune() { c.prune() }

// CachePath returns the file of key.
func CachePath(c *Cache, key string) string { return c.path(key) }
