// Adapted from jplayer-tv/internal/ui/kit/export_shadow_test.go at 781bee5.

package kit

import "github.com/gdeskdev/desk/foundation/scene"

// Renders returns how many pictures the cache made itself: the rest of its
// lookups were found in the process's cache.
func (c *ShadowCache) Renders() int { return c.renders }

// ResetShadowPics empties the process's cache of pictures.
func ResetShadowPics() {
	shadowPics.mu.Lock()
	defer shadowPics.mu.Unlock()
	clear(shadowPics.m)
	shadowPics.bytes = 0
}

// PicCache is a picture cache with its own limits, for tests of the eviction.
type PicCache struct{ c picCache }

// NewPicCache returns a cache bounded by n pictures and b bytes.
func NewPicCache(n, b int) *PicCache { return &PicCache{c: picCache{maxPics: n, maxBytes: b}} }

// Put stores a picture of size bytes under the key named name and returns the
// picture that stands for it.
func (p *PicCache) Put(name string, size int) *scene.Image {
	pic := &shadowPic{}
	pic.img.Pix = make([]byte, size)

	return &p.c.put(shadowKey{text: name}, pic).img
}

// Get looks the key named name up and tells whether it is held.
func (p *PicCache) Get(name string) bool { return p.c.get(shadowKey{text: name}) != nil }

// Len returns the pictures held.
func (p *PicCache) Len() int { p.c.mu.Lock(); defer p.c.mu.Unlock(); return len(p.c.m) }

// Bytes returns the pixel bytes held.
func (p *PicCache) Bytes() int { p.c.mu.Lock(); defer p.c.mu.Unlock(); return p.c.bytes }

// ShadowPicsLen returns how many pictures the process's cache holds.
func ShadowPicsLen() int {
	shadowPics.mu.Lock()
	defer shadowPics.mu.Unlock()

	return len(shadowPics.m)
}

// Picture returns the image the cache draws, nil when it has none.
func (c *ShadowCache) Picture() *scene.Image {
	if c.pic == nil {
		return nil
	}

	return &c.pic.img
}
