package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"strings"
	"time"

	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/components"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

const (
	// coversFolder, under the cache folder, is the disk cache of the covers.
	coversFolder = "covers"
	// maxCoverBytes is the largest cover read: a production cover of a course
	// weighs more than 2 MiB.
	maxCoverBytes = 16 << 20
	// coverMaxSide is the longest side a decoded cover keeps: the widest
	// place a cover is drawn is the TV's 1920 DIP hero, 640 DIP wide, and a
	// phone's hero is 353 DIP at 3 px a DIP; more pixels only fill the image
	// cache (a decoded 12 MP picture is 48 MB).
	coverMaxSide = 1280
	// coverFresh is how long a cover filed on the disk is taken without
	// asking the server; past it the server is asked first and the file kept
	// when it cannot answer (offline).
	coverFresh = 7 * 24 * time.Hour
	// coverKeyPrefix keeps the decoded covers apart from the avatars in the
	// image cache: a cover is reduced, an avatar is not.
	coverKeyPrefix = "cover:"
)

// coverFetch reads at most limit bytes of the file at url, blocking.
type coverFetch func(ctx context.Context, url string, limit int64) ([]byte, error)

// coverLoader is the app's way to a course's cover (K14, docs/plan/02 §5):
// the live mode's real picture, from the disk cache when it is there (so that
// it shows offline and after a relaunch) or from the network, filed then,
// decoded off the UI thread and reduced to coverMaxSide. The mock's own
// pictures are no covers: they say core.ErrUnsupported, and the screens keep
// the vector poster of the prototype.
type coverLoader struct {
	images *components.ImageCache
	disk   *store.Images
	fetch  coverFetch
	isMock func(url string) bool
	now    func() time.Time
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newCoverLoader returns a loader over the shared decoded-image cache, the
// disk cache (nil: none) and the fetch of the wire; isMock tells a URL of the
// mock's media (nil: none is).
func newCoverLoader(images *components.ImageCache, disk *store.Images, fetch coverFetch, isMock func(string) bool) *coverLoader {
	return &coverLoader{images: images, disk: disk, fetch: fetch, isMock: isMock, now: time.Now}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Load returns the cover at rawURL, once per URL. Blocks: call it from ui.Go.
func (l *coverLoader) Load(ctx context.Context, rawURL string) (*scene.Image, error) {
	if rawURL == "" || (l.isMock != nil && l.isMock(rawURL)) || !strings.HasPrefix(rawURL, "http") {
		return nil, fmt.Errorf("app: cover: no real cover at this address: %w", core.ErrUnsupported)
	}

	return l.images.Loader(coverKeyPrefix+rawURL, func(ctx context.Context) (*scene.Image, error) {
		return l.read(ctx, rawURL)
	})(ctx)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// read gets the bytes of the cover, from the disk or the network, and
// decodes them.
func (l *coverLoader) read(ctx context.Context, rawURL string) (*scene.Image, error) {
	key := artKey(rawURL) // a CDN's signature left out, as for the artwork
	data, saved, ok := l.disk.Get(key)
	fresh := ok && l.now().Sub(saved) < coverFresh
	if !fresh {
		got, err := l.fetch(ctx, rawURL, maxCoverBytes)
		switch {
		case err == nil:
			// Filing is best effort (a full disk, bytes that are no picture: the
			// decode below tells): the cover shows either way.
			_ = l.disk.Put(key, got)
			data, ok = got, true
		case !ok:
			return nil, err
		default: // offline or the server failed: the stale file is better than none
		}
	}
	if !ok {
		return nil, fmt.Errorf("app: cover: %w", core.ErrNotFound)
	}
	return decodeCover(data)
}

// decodeCover decodes a cover (a JPEG or a PNG of artMaxSide a side and
// artMaxPixels at most, as the artwork of Now Playing is) and reduces it, by
// the area average of boxReduce, to coverMaxSide on its longer side: a cover of
// 3000 px is drawn in a box of some hundreds of DIPs, and a decoded 12 MP
// picture would hold 48 MB of pixels in the image cache. The decoded original
// is dropped before it returns. Run it off the UI thread.
func decodeCover(data []byte) (*scene.Image, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("app: cover: %w: %w", core.ErrInvalid, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || max(cfg.Width, cfg.Height) > artMaxSide || cfg.Width*cfg.Height > artMaxPixels {
		return nil, fmt.Errorf("app: cover: %d×%d px, at most %d a side and %d px: %w",
			cfg.Width, cfg.Height, artMaxSide, artMaxPixels, core.ErrInvalid)
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("app: cover: %w: %w", core.ErrInvalid, err)
	}
	w, h := fitSide(cfg.Width, cfg.Height, coverMaxSide)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	boxReduce(dst, src)

	return &scene.Image{Pix: dst.Pix, Stride: dst.Stride, Width: w, Height: h}, nil
}

// isMockMedia tells whether rawURL is a picture of the mock's media server
// (or still has its mock-media:// scheme): in the mock's mode that is the
// MOCK COVER of the files, which is no cover (K14).
func isMockMedia(m inProcMock, rawURL string) bool {
	if m == nil {
		return false
	}
	if strings.HasPrefix(rawURL, mockMediaScheme) {
		return true
	}
	base := m.Media().BaseURL()

	return base != "" && strings.HasPrefix(rawURL, base+"/")
}
