// Adapted from jplayer-tv/internal/ui/kit/shadowcache.go at 781bee5.

package kit

import (
	"math"
	"sync"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/raster"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// ShadowCache holds the shadow of one text as a picture, so that a widget that
// repaints often (a card, 60 times a second while its focus ring grows) blurs
// it once and not on every frame, as [DrawTextShadow] does.
//
// The picture is the layer DrawTextShadow records, rendered by the CPU
// renderer into a transparent image and blurred as the renderer blurs a layer
// (a Gaussian of Blur/2 device pixels); drawing it is one image op, which a GPU
// renderer keeps as a texture while its Version stands. It is looked up when
// the key changes: the text, its face and size, the shaper, the shadow
// (offset, blur, colour) and the device scale.
//
// The pictures are shared by the process: a small cache, [shadowPics], keyed
// as above, holds the last [ShadowPicsMax] of them and at most
// [ShadowPicsBytes] bytes of pixels, so that a screen that is
// rebuilt (TV04 when the player closes, a mobile list on its return, their
// cards new widgets with the same texts) finds the pictures of the cards it
// replaced and renders nothing. A picture is never changed once made, so the cards
// that share one, and an eviction, do not disturb each other.
//
// The zero ShadowCache is empty and ready. It is used on the UI thread only,
// so a widget must not be copied once it has drawn.
type ShadowCache struct {
	pic *shadowPic
	key shadowKey
	// keyed tells that key is the key of the last lookup, ok that the lookup
	// found or made the picture in pic; a build that failed leaves ok false
	// and the draw falls back to the layer, without trying again.
	keyed, ok bool
	// builds counts the keys looked up, renders the pictures this cache made
	// itself (the rest came from the shared cache), for tests.
	builds, renders int
}

// ShadowPicsMax is how many pictures the process keeps for the caches: the
// titles of a home screen are a dozen, and of the other screens a few more.
const ShadowPicsMax = 64

// ShadowPicsBytes is the most pixel memory the process keeps in the caches'
// pictures: 24 MiB, the budget jPlayer measured for a set of titles at scale 2,
// kept here as a ceiling for the 64 pictures of ShadowPicsMax (a title 700 DIPs wide
// and 100 high is 1.1 MB at scale 2). The least recently used
// pictures go until the cache is under both limits. A picture bigger than the budget is not kept
// at all, so that it does not flush the rest; its cache still draws it.
const ShadowPicsBytes = 24 << 20

// shadowKey is what a picture depends on. The text, the face and the box are
// the layout's: the same text in the same face and size, by the same shaper,
// is the same glyphs.
type shadowKey struct {
	text   string
	face   text.Face
	size   geom.Size
	lines  int
	shaped float32 // the scale the text was shaped at
	shaper *text.Shaper
	shadow TextShadow
	scale  float32
}

// shadowPic is a picture of a shadow, made once. pad is how far, in points,
// it reaches left of and above the text: the blur's reach.
type shadowPic struct {
	img scene.Image
	pad float32
	// used is the tick of its last use, which the eviction reads.
	used uint64
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// shadowRender is the renderer and the scene the pictures are made with: one
// of each for the process, as a build happens on a UI thread now and then, and
// small (a name and its blur). One worker, so that no helper goroutine starts.
var shadowRender struct {
	mu    sync.Mutex
	r     *raster.Renderer
	sc    scene.Scene
	tmp   []uint32
	every geom.Rect
}

// shadowPics is the process's pictures by key, at most [ShadowPicsMax] and
// [ShadowPicsBytes] bytes; the least recently used goes first. Its lock is held for a lookup and a store,
// never for a render.
var shadowPics picCache

// picCache is the pictures by key, bounded by count and by bytes of pixels.
type picCache struct {
	mu   sync.Mutex
	m    map[shadowKey]*shadowPic
	tick uint64
	// bytes is the pixels of the pictures in m.
	bytes int
	// maxPics and maxBytes are the limits; zero is [ShadowPicsMax] and
	// [ShadowPicsBytes].
	maxPics, maxBytes int
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Prepare makes the picture of l's shadow now if the key changed, so that the
// first frame that draws it only draws. It is for Layout, which is not a hot
// path; a draw does the same lazily when it was not called.
func (c *ShadowCache) Prepare(ctx *ui.Context, l *text.Layout, s TextShadow) {
	if l == nil || s.Color.A == 0 {
		return
	}
	c.ensure(ctx.Shaper(), l, s, ctx.Scale())
}

// Draw is [DrawTextShadow] with the shadow drawn from the picture: it draws l
// at at in colour col over its shadow s.
func (c *ShadowCache) Draw(ctx *ui.Context, p *ui.Painter, l *text.Layout, at geom.Point, s TextShadow, col color.Color) {
	if l == nil {
		return
	}
	at = snapTextShadow(p, l, at)
	if s.Color.A > 0 {
		scale := p.Scene().Scale()
		if !c.ensure(ctx.Shaper(), l, s, scale) {
			drawTextShadow(p, l, at, s, col)

			return
		}
		pic := c.pic
		w, h := float32(pic.img.Width)/scale, float32(pic.img.Height)/scale
		p.Image(&pic.img, geom.XYWH(at.X+s.Offset.X-pic.pad, at.Y+s.Offset.Y-pic.pad, w, h), 1)
	}
	p.Text(l, at, col)
}

// Reset forgets the picture, which the next draw looks up again (and finds in
// the shared cache, unless it was evicted).
func (c *ShadowCache) Reset() { c.keyed, c.ok, c.pic, c.key = false, false, nil, shadowKey{} }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// ensure finds or makes the picture of the key, unless it is there, and tells
// whether there is one. Nothing is allocated while the key stands.
func (c *ShadowCache) ensure(sh *text.Shaper, l *text.Layout, s TextShadow, scale float32) bool {
	k := shadowKey{
		text: l.Text, face: l.Face, size: l.Size, lines: len(l.Lines), shaped: l.Scale,
		shaper: sh, shadow: s, scale: scale,
	}
	if c.keyed && c.key == k {
		return c.ok
	}
	c.keyed, c.ok, c.key, c.pic = true, false, k, nil
	c.builds++
	if sh == nil || l.Size.W <= 0 || l.Size.H <= 0 {
		return false
	}
	// A paragraph in several faces is not told by its first one: the picture
	// is the cache's own and not shared.
	shared := len(l.Runs) == 0
	if shared {
		if c.pic = shadowPics.get(k); c.pic != nil {
			c.ok = true

			return true
		}
	}
	pic, err := buildShadow(sh, l, s, scale)
	if err != nil {
		return false
	}
	c.pic, c.ok = pic, true
	c.renders++
	if shared {
		c.pic = shadowPics.put(k, pic)
	}

	return true
}

// get returns the picture of k and marks it used, nil when there is none.
func (sp *picCache) get(k shadowKey) *shadowPic {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	pic := sp.m[k]
	if pic != nil {
		sp.tick++
		pic.used = sp.tick
	}

	return pic
}

// put stores pic under k, then evicts the least recently used pictures until
// the cache is within its count and byte limits, and returns the picture that
// stands for k: another cache may have made it meanwhile, which then wins. A
// picture bigger than the byte budget is returned and not stored. An evicted
// picture is only dropped from the map: whoever holds it keeps drawing it.
func (sp *picCache) put(k shadowKey, pic *shadowPic) *shadowPic {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	maxPics, maxBytes := sp.maxPics, sp.maxBytes
	if maxPics <= 0 {
		maxPics = ShadowPicsMax
	}
	if maxBytes <= 0 {
		maxBytes = ShadowPicsBytes
	}
	if sp.m == nil {
		sp.m = make(map[shadowKey]*shadowPic, maxPics+1)
	}
	sp.tick++
	if old := sp.m[k]; old != nil {
		old.used = sp.tick

		return old
	}
	size := len(pic.img.Pix)
	if size > maxBytes {
		return pic
	}
	pic.used = sp.tick
	sp.m[k] = pic
	sp.bytes += size
	for len(sp.m) > maxPics || sp.bytes > maxBytes {
		var oldest shadowKey
		var least uint64
		first := true
		for key, p := range sp.m {
			if first || p.used < least {
				oldest, least, first = key, p.used, false
			}
		}
		sp.bytes -= len(sp.m[oldest].img.Pix)
		delete(sp.m, oldest)
	}

	return pic
}

// buildShadow renders the text in the shadow's colour into a transparent image
// big enough for the blur's reach all round, and blurs it.
func buildShadow(sh *text.Shaper, l *text.Layout, s TextShadow, scale float32) (*shadowPic, error) {
	// The renderer grows a layer by the blur's reach (scene.LayerOp.Reach); the
	// image does too, in whole pixels, so that the text starts on one.
	padPx := int(math.Ceil(float64((&scene.LayerOp{Blur: s.Blur}).Reach() * scale)))
	w := int(math.Ceil(float64(l.Size.W*scale))) + 2*padPx
	h := int(math.Ceil(float64(l.Size.H*scale))) + 2*padPx
	pic := &shadowPic{pad: float32(padPx) / scale}

	img := &pic.img
	img.Stride, img.Width, img.Height = 4*w, w, h
	img.Pix = make([]byte, img.Stride*h)
	img.Version++

	sr := &shadowRender
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if sr.r == nil {
		sr.r = raster.New(raster.WithWorkers(1))
		sr.every = geom.XYWH(-1e9, -1e9, 2e9, 2e9)
	}
	sr.sc.Reset(scale)
	if err := sh.DrawInZoomed(&sr.sc, l, geom.Pt(pic.pad, pic.pad), s.Color, sr.every, 1); err != nil {
		return nil, err
	}
	t := raster.Target{Pix: img.Pix, Stride: img.Stride, Width: w, Height: h, Format: raster.FormatRGBA}
	if err := sr.r.Render(&sr.sc, t, color.Transparent); err != nil {
		return nil, err
	}
	sr.sc.Reset(scale) // the masks are not kept
	sr.tmp = scene.BlurRGBA(img.Pix, w, h, img.Stride, s.Blur/2*scale, sr.tmp)

	return pic, nil
}
