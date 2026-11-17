// Adapted from jplayer-mobile/internal/app/nowplaying_art.go at 49de3c2
// (artwork, pictureJPEG: a PNG or a JPEG of 2 MB at most handed over as it
// is, another picture decoded and written as a JPEG of quality 85 within a
// longest side). Changed: a cover over 2 MB, or of more than artPassSide
// pixels a side, is reduced (an area average, standard library only, in
// place of golang.org/x/image/draw) rather than dropped for a drawn cover;
// the covers made are kept by URL (artCache).

package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png" // the PNG decoder of fitArtwork
	"net/url"
	"sync"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The artwork the app hands the system's media controls (F9).
const (
	// artReadMaxBytes is the most a cover may weigh to be read for Now
	// Playing at all: a production cover may weigh more than 2 MB.
	artReadMaxBytes = 16 << 20
	// artPassSide is the longest side a PNG or a JPEG of artMaxBytes at
	// most may have to be handed over as it is: Android's session scales
	// it to 1024 px itself (GDeskMedia.ART_SIZE, by powers of two), iOS
	// draws it at about 300 pt.
	artPassSide = 2048
	// artSide is the longest side of a reduced cover, Android's ART_SIZE.
	artSide = 1024
	// artQuality is the JPEG quality of a reduced cover.
	artQuality = 85
	// artMaxSide and artMaxPixels bound what is decoded to be reduced: a
	// 40 MP JPEG decodes to some 60 MB (YCbCr) for a moment.
	artMaxSide   = 8192
	artMaxPixels = 40_000_000
	// artKept is how many covers artCache keeps: a few courses.
	artKept = 8
)

// artFetch reads limit bytes at most of the file at rawURL (the wires'
// fetchBytes). Blocks.
type artFetch func(ctx context.Context, rawURL string, limit int64) ([]byte, error)

// artCache keeps the last artKept covers made for Now Playing by URL
// (artKey: a CDN's signature left out), so that coming back to a course
// reads and reduces nothing. Any goroutine.
type artCache struct {
	mu   sync.Mutex
	urls [artKept]string
	bufs [artKept][]byte
	next int // the slot put fills next
}

// artworks is the app's artCache, shared by the phone's and the TV's wire.
var artworks artCache

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// loadArtwork returns the cover at rawURL as the system's media controls take
// it (fitArtwork): the one kept in c, else the one read with fetch, then
// kept. A cover over artReadMaxBytes, one that cannot be read and one that
// does not decode are an error (no artwork). logf tells of a reduction.
// Blocks: run it off the UI thread.
func loadArtwork(ctx context.Context, c *artCache, rawURL string, fetch artFetch, logf func(string, ...any)) ([]byte, error) {
	if b, ok := c.get(rawURL); ok {
		return b, nil
	}
	b, err := fetch(ctx, rawURL, artReadMaxBytes+1)
	if err != nil {
		return nil, err
	}
	if len(b) > artReadMaxBytes {
		return nil, fmt.Errorf("app: artwork: over %d bytes: %w", artReadMaxBytes, core.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("app: artwork: %w", err)
	}
	out, err := fitArtwork(b)
	if err != nil {
		return nil, err
	}
	if logf != nil && len(out) != len(b) {
		logf("dancefitvn: now playing: artwork of %d B reduced to %d B", len(b), len(out))
	}
	c.put(rawURL, out)

	return out, nil
}

// fitArtwork returns b as the system's media controls take it: b itself
// when it is a PNG or a JPEG of artMaxBytes at most and artPassSide pixels
// a side at most; else b decoded (a JPEG or a PNG of artMaxSide a side and
// artMaxPixels at most), reduced by an area average to artSide on its
// longer side and written as a JPEG of quality artQuality, transparent
// parts over black. A picture that does not decode, or is too large to,
// is core.ErrInvalid. Run it off the UI thread.
func fitArtwork(b []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("app: artwork: %w: %w", core.ErrInvalid, err)
	}
	w, h := cfg.Width, cfg.Height
	switch {
	case w <= 0 || h <= 0 || max(w, h) > artMaxSide || w*h > artMaxPixels:
		return nil, fmt.Errorf("app: artwork: %d×%d px, at most %d a side and %d px: %w",
			w, h, artMaxSide, artMaxPixels, core.ErrInvalid)
	case len(b) <= artMaxBytes && isPNGOrJPEG(b) && max(w, h) <= artPassSide:
		return b, nil
	default:
	}
	src, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("app: artwork: %w: %w", core.ErrInvalid, err)
	}
	dw, dh := fitSide(w, h, artSide)
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	boxReduce(dst, src)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: artQuality}); err != nil {
		return nil, fmt.Errorf("app: artwork: %w: %w", core.ErrInvalid, err)
	}
	if buf.Len() > artMaxBytes {
		return nil, fmt.Errorf("app: artwork: %d bytes made: %w", buf.Len(), core.ErrInvalid)
	}

	return buf.Bytes(), nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// get returns the cover kept for rawURL.
func (c *artCache) get(rawURL string) ([]byte, bool) {
	key := artKey(rawURL)
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, u := range c.urls {
		if u == key && u != "" {
			return c.bufs[i], true
		}
	}

	return nil, false
}

// put keeps b for rawURL, in place of the oldest cover once artKept are kept.
func (c *artCache) put(rawURL string, b []byte) {
	key := artKey(rawURL)
	if key == "" || len(b) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, u := range c.urls {
		if u == key {
			c.bufs[i] = b

			return
		}
	}
	c.urls[c.next], c.bufs[c.next] = key, b
	c.next = (c.next + 1) % artKept
}

// artKey is the key of the cover at rawURL in artCache: the address
// without its query and fragment, which a CDN signs anew at each request
// for the same file.
func artKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.RawQuery, u.Fragment, u.User = "", "", nil

	return u.String()
}

// fitSide returns w × h scaled down to side on its longer side, each at
// least 1; as it is when it fits.
func fitSide(w, h, side int) (int, int) {
	m := max(w, h)
	if m <= side {
		return w, h
	}

	return max((w*side+m/2)/m, 1), max((h*side+m/2)/m, 1)
}

// boxReduce draws src into dst, no larger, each pixel of dst the mean of
// the block of src it covers (the blocks' edges rounded to whole source
// pixels), premultiplied. src is read a row at a time through image/draw,
// whose fast paths read YCbCr, NRGBA, Gray and Paletted, so that it is
// never held twice.
func boxReduce(dst *image.RGBA, src image.Image) {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	dw, dh := dst.Bounds().Dx(), dst.Bounds().Dy()
	xs := make([]int, dw+1) // the source columns of dst's column i: xs[i]..xs[i+1]
	for i := range xs {
		xs[i] = i * sw / dw
	}
	row := image.NewRGBA(image.Rect(0, 0, sw, 1))
	acc := make([]uint32, 4*dw)
	for j := range dh {
		y0, y1 := j*sh/dh, (j+1)*sh/dh
		clear(acc)
		for y := y0; y < y1; y++ {
			draw.Draw(row, row.Bounds(), src, image.Pt(sb.Min.X, sb.Min.Y+y), draw.Src)
			p := row.Pix
			for i := range dw {
				a := acc[4*i : 4*i+4 : 4*i+4]
				for x := xs[i]; x < xs[i+1]; x++ {
					s := p[4*x : 4*x+4 : 4*x+4]
					a[0] += uint32(s[0])
					a[1] += uint32(s[1])
					a[2] += uint32(s[2])
					a[3] += uint32(s[3])
				}
			}
		}
		out := dst.Pix[j*dst.Stride : j*dst.Stride+4*dw]
		for i := range dw {
			n := uint32((xs[i+1] - xs[i]) * (y1 - y0))
			for k := range 4 {
				out[4*i+k] = uint8((acc[4*i+k] + n/2) / n)
			}
		}
	}
}
