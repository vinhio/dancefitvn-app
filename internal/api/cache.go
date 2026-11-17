package api

import (
	"net/url"
	"sync"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Cache keeps the body of a GET answer under its ETag (05 §1). Methods are
// safe for concurrent use. The store package implements it on disk.
type Cache interface {
	// Get returns the entry of key.
	Get(key string) (etag string, body []byte, ok bool)
	// Put saves the entry of key. Failure is not fatal: the client goes on.
	Put(key, etag string, body []byte) error
}

// MemoryCache is a Cache in memory.
type MemoryCache struct {
	mu sync.Mutex
	m  map[string]memoryEntry
}

type memoryEntry struct {
	etag string
	body []byte
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewMemoryCache returns an empty MemoryCache.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{m: map[string]memoryEntry{}}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Get implements Cache. The body is a copy.
func (c *MemoryCache) Get(key string) (string, []byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.m[key]
	if !ok {
		return "", nil, false
	}

	return e.etag, append([]byte(nil), e.body...), true
}

// Put implements Cache. The body is copied.
func (c *MemoryCache) Put(key, etag string, body []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.m[key] = memoryEntry{etag: etag, body: append([]byte(nil), body...)}

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// cacheKey is "method path?query" with the query sorted.
func cacheKey(method, path string, q url.Values) string {
	return method + " " + path + "?" + q.Encode()
}
