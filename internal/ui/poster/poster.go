package poster

import (
	"context"
	"math"
	"sync"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Spec is what poster(cid, w, h, o) (index.html:291) is given besides the
// size: the course and the options.
type Spec struct {
	// ID is the API id of the course, which picks the palette row; an id the
	// palette does not hold draws `--sf2` to `--bg` with no figure.
	ID string
	// Title is the short name under "DANCEFIT": `c.short` without the brand,
	// "SET 35" (index.html:296). The caller gives it: it is the course's
	// Short(), and a Vietnamese one ("GIỚI THIỆU") is text, not palette.
	Title string
	// Radius is `o.r` (index.html:298); RadiusDefault, which is below zero,
	// reads as 16, and zero is a square poster.
	Radius float32
	// TitleSize is `o.ts`; 0 is `min(h*.2, w*.14)` (index.html:292).
	TitleSize float32
	// NoTitle is `o.title===false`: no title, and wider dancers (:295).
	NoTitle bool
	// NoFigure is `o.fig===false`: no dancers. A vinyl record is drawn anyway,
	// as poster() draws it before it looks at `o.fig` (:294).
	NoFigure bool
}

// Poster is the cover of a course, `div.poster` (index.html:65-71, :298): a
// box of Width x Height DIPs, or of the room it is given where a side is 0.
// Its title is a `div.ttl` holding the `s` and `b` of the prototype, as
// elements of the tree.
//
// Everything is drawn from the course's palette row, vector. With an Image
// set, the picture covers the box instead (the live mode of 02 section 5),
// with the radius and nothing over it.
type Poster struct {
	ui.Base
	// Decor: a poster is a picture, with its title painted in, and no element of
	// a screen reader's reading; a pressable around it is named by the course.
	kit.Decor
	// Width and Height are the poster's size in DIPs; 0 takes the room the
	// parent gives on that side.
	Width, Height float32

	spec   Spec
	colors Colors
	known  bool // the palette holds the course
	image  *scene.Image
	extra  ui.Widget
	ttl    *titleBox

	// The cover (K14): load fetches the picture at cover, from Mounted on;
	// asked is the address the picture shown or coming is of, stop ends the
	// load under way.
	load    Loader
	cover   string
	asked   string
	stop    func()
	mounted bool

	g     geo         // the numbers of the current size
	gSize geom.Size   // the size g was made for
	gSpec Spec        // the spec g was made for
	paint scene.Paint // the gradient, in the poster's coordinates
	figs  [2]dancer   // the dancers, flipped first
}

var (
	_ ui.Widget    = (*Poster)(nil)
	_ ui.Mounter   = (*Poster)(nil)
	_ ui.Unmounter = (*Poster)(nil)
)

// Loader loads the cover of a course at url (env.Services.Cover): blocking,
// off the UI thread; an error is a cover that cannot be had.
type Loader func(ctx context.Context, url string) (*scene.Image, error)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// brand is "DANCEFIT", the first line of the title (index.html:296): the
// brand of the app, which locales/vi.json holds as splash.brand.
var brand = sync.OnceValue(func() string { return locales.MustTexts()["splash.brand"] })

// New returns a poster of the spec at w x h DIPs (0: the room given).
func New(s Spec, w, h float32) *Poster {
	p := &Poster{Width: w, Height: h}
	p.Name = "div.poster"
	p.ttl = newTitleBox()
	p.Add(p.ttl)
	p.SetSpec(s)

	return p
}

// ForCourse returns the Spec of an API course: its id and its short name.
// The rest is the caller's (Radius is RadiusDefault).
func ForCourse(c model.Course) Spec {
	return Spec{ID: c.ID, Title: c.Short(), Radius: RadiusDefault}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Spec returns the spec of the poster.
func (p *Poster) Spec() Spec { return p.spec }

// SetSpec changes what the poster shows; the same spec changes nothing.
func (p *Poster) SetSpec(s Spec) {
	if p.spec == s && p.ttl.brand.Text != "" {
		return
	}
	p.spec = s
	p.colors, p.known = Lookup(s.ID)
	p.ttl.set(brand(), s.Title)
	kit.SetHidden(p.ttl, s.NoTitle || s.Title == "")
	p.Relayout()
}

// SetImage shows img, drawn to cover the box with the poster's radius, in
// place of the drawing; nil goes back to it. The image must stay unchanged
// until it is replaced (scene.Image).
func (p *Poster) SetImage(img *scene.Image) {
	if p.image == img {
		return
	}
	p.image = img
	p.Repaint()
}

// SetCover has the poster show the cover at url, which load gets once the
// poster is in a view (the live mode's real picture, K14), and the drawing
// until then, and for good when load fails or there is no url (the mock's
// covers: a Loader that refuses them). A new url drops the picture of the old
// one and ends its load; the same url changes nothing. The load ends with the
// poster leaving its view. Call it on the UI thread.
func (p *Poster) SetCover(load Loader, url string) {
	if url == "" {
		load = nil
	}
	p.load = load
	if p.cover == url {
		p.begin()

		return
	}
	p.cover = url
	p.end()
	p.asked = ""
	p.SetImage(nil)
	p.begin()
}

// Mounted starts the load of the cover, when one is wanted.
func (p *Poster) Mounted(*ui.Context) {
	p.mounted = true
	p.begin()
}

// Unmounted forgets the load: ui.Go has ended it with the poster's leaving,
// and a poster that comes back asks again (the picture stays, drawn at once).
func (p *Poster) Unmounted(*ui.Context) {
	p.mounted = false
	p.stop = nil
	if p.image == nil {
		p.asked = ""
	}
}

// SetExtra puts w over the drawing, centered in the box (the `o.extra` of
// index.html:457-458, an icon on a download's poster); nil takes it away.
func (p *Poster) SetExtra(w ui.Widget) {
	if p.extra == w {
		return
	}
	if p.extra != nil {
		p.Remove(p.extra)
	}
	p.extra = w
	if w != nil {
		p.Add(w)
	}
	p.Relayout()
}

// Radius returns the corner radius of the poster in DIPs.
func (p *Poster) Radius() float32 {
	if p.spec.Radius < 0 {
		return DefaultRadius
	}

	return p.spec.Radius
}

// Layout sizes the poster, makes what depends on the size, and places the
// title in the lower left and the extra in the middle.
func (p *Poster) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w, h := p.Width, p.Height
	if w <= 0 && cs.Max.W <= math.MaxFloat32 {
		w = cs.Max.W
	}
	if h <= 0 && cs.Max.H <= math.MaxFloat32 {
		h = cs.Max.H
	}
	size := cs.Constrain(geom.Sz(w, h))
	p.prepare(size)

	if !p.ttl.Hidden {
		p.ttl.configure(p.g.ts, p.colors.Ink)
		// `.ttl` is absolute at left:8% with no width: its box is as wide as
		// its text, to the right edge of the poster at most, where the text
		// wraps (:67). What is wider still is cut by `overflow:hidden` (:65).
		room := geom.Sz(max(size.W-p.g.titleX, 0), ui.Unbounded)
		ts := c.Layout(p.ttl, ui.Loose(room))
		c.Place(p.ttl, geom.Pt(p.g.titleX, p.g.titleB-ts.H))
	}
	if p.extra != nil {
		es := c.Layout(p.extra, ui.Loose(size))
		c.Place(p.extra, geom.Pt((size.W-es.W)/2, (size.H-es.H)/2))
	}

	return size
}

// Paint draws the poster: the gradient, the ring, the record or the dancers,
// then the title and the extra over them.
func (p *Poster) Paint(c *ui.Context, pt *ui.Painter) {
	f := p.Frame()
	if f.W <= 0 || f.H <= 0 {
		return
	}
	r := geom.XYWH(0, 0, f.W, f.H)
	radius := p.Radius()
	if img := p.image; img != nil && img.Width > 0 && img.Height > 0 {
		pt.Clip(r, radius, func() {
			pt.Image(img, coverRect(r, img), 1)
			if p.extra != nil {
				pt.Child(c, p.extra)
			}
		})

		return
	}

	pt.Fill(r, p.paint, radius)
	ring := theme.From(c).C.RingWhite13
	// `.poster{overflow:hidden}` (:65) clips the ring, the figures, the title
	// and the extra to the rounded box.
	pt.Clip(r, radius, func() {
		pt.Border(p.g.ring, geom.Insets{Top: p.g.ringWidth, Right: p.g.ringWidth, Bottom: p.g.ringWidth, Left: p.g.ringWidth}, ring, p.g.ringRadius)
		switch {
		case p.colors.Vinyl:
			drawVinyl(pt, p.g.vinyl)
		case !p.spec.NoFigure && p.known:
			p.paintDancers(pt)
		default: // no figure for this poster
		}
		if !p.ttl.Hidden {
			pt.Child(c, p.ttl)
		}
		if p.extra != nil {
			pt.Child(c, p.extra)
		}
	})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// begin starts the load of the cover when the poster is in a view, one is
// wanted and none is under way or done for this url.
func (p *Poster) begin() {
	if !p.mounted || p.load == nil || p.cover == "" || p.asked == p.cover {
		return
	}
	url, load := p.cover, p.load
	p.asked = url
	p.stop = ui.Go(p, func(ctx context.Context) (*scene.Image, error) {
		return load(ctx, url)
	}, func(_ *ui.Context, img *scene.Image, err error) {
		p.stop = nil
		if err == nil && img != nil && p.cover == url {
			p.SetImage(img)
		}
	})
}

// end ends the load under way.
func (p *Poster) end() {
	if p.stop != nil {
		p.stop()
		p.stop = nil
	}
}

// prepare makes the numbers and the paths of a poster of the size, unless
// they are those of the last call.
func (p *Poster) prepare(size geom.Size) {
	if p.gSize == size && p.gSpec == p.spec && p.paint.Gradient != nil {
		return
	}
	p.gSize, p.gSpec = size, p.spec
	p.g = newGeo(size.W, size.H, p.spec.TitleSize, p.spec.NoTitle)
	p.paint = scene.Linear(p.g.from, p.g.to,
		scene.Stop{Color: p.colors.From, Offset: 0}, scene.Stop{Color: p.colors.To, Offset: 1})
	p.figs[0].build(p.g.figX, p.g.figY, p.g.figH)
	p.figs[1].build(p.g.figX+p.g.figH*(1-figOverlap), p.g.figY, p.g.figH)
}

// paintDancers draws the two dancers as one group at opacity .92.
func (p *Poster) paintDancers(pt *ui.Painter) {
	ink := scene.Solid(p.colors.Ink)
	st := stroke(p.g.figH)
	pt.Layer(figOpacity, 0, func() {
		for i := range p.figs {
			pt.FillPath(&p.figs[i].head, ink, scene.NonZero)
			pt.StrokePath(&p.figs[i].body, st, ink)
		}
	})
}

// coverRect returns where img is drawn to cover r, centered: scaled by the
// larger of the two ratios, as `background-size: cover` does.
func coverRect(r geom.Rect, img *scene.Image) geom.Rect {
	k := max(r.W/float32(img.Width), r.H/float32(img.Height))
	w, h := float32(img.Width)*k, float32(img.Height)*k

	return geom.XYWH(r.X+(r.W-w)/2, r.Y+(r.H-h)/2, w, h)
}

// titleBox is `div.ttl` (index.html:67): the brand over the short name,
// shrink-wrapped, drawn with the text shadow.
type titleBox struct {
	ui.Base
	brand, name *kit.Label // the `s` and the `b`
	ink         color.Color
	shadow      kit.TextShadow // of the name
	brandShadow kit.TextShadow // of the brand, which is at .85 as a whole
	// brandPic and namePic hold the shadows as pictures: a poster is drawn on every
	// frame its grid slides, and a blurred layer that moves is blurred again each
	// time (the CPU does it), where a picture is a texture drawn as it is.
	brandPic, namePic kit.ShadowCache
}

// newTitleBox returns the title block: `s` is Barlow 700 with 1.5 px
// spacing, `b` Barlow 800 upper-case at line-height .95 (index.html:68-69).
func newTitleBox() *titleBox {
	t := &titleBox{
		brand: &kit.Label{Role: theme.RoleDisplay, Weight: brandWeight, Spacing: brandSpacing},
		// `b` is a block of normal wrapping (:68): a name wider than the room
		// goes on a second line, and the poster cuts what is still too much.
		name:        &kit.Label{Role: theme.RoleDisplay, Weight: nameWeight, LineHeight: nameLine, Upper: true, Wrap: true},
		shadow:      kit.PosterTitleShadow,
		brandShadow: kit.PosterTitleShadow,
	}
	t.Name = "div.ttl"
	t.brand.Name = "s"
	t.name.Name = "b"
	t.brandShadow.Color = t.brandShadow.Color.WithAlpha(float32(kit.PosterTitleShadow.Color.A) / 255 * brandOpacity)
	t.Add(t.brand, t.name)

	return t
}

// set puts the brand and the name in the labels.
func (t *titleBox) set(brandText, name string) {
	t.brand.SetText(brandText)
	t.name.SetText(name)
}

// configure sizes the labels for a title of size ts and sets the ink, which
// the brand shows at .85. A size that changes lays the labels out again.
func (t *titleBox) configure(ts float32, ink color.Color) {
	t.ink = ink
	// `div.ttl{color:...}` is inherited by `s` and `b`; the box draws them itself,
	// the labels only carry the color layout.json records.
	t.brand.Color, t.name.Color = ink, ink
	if sz := ts * brandSize; t.brand.Size != sz {
		t.brand.Size = sz
		t.brand.Relayout()
	}
	if t.name.Size != ts {
		t.name.Size = ts
		t.name.Relayout()
	}
}

// Layout stacks the brand over the name at the left, each as the block it
// is in CSS: as wide as the wider of the two (`div.ttl` is shrink-wrapped, `s`
// and `b` are `display:block`).
func (t *titleBox) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	room := ui.Loose(geom.Sz(cs.Max.W, ui.Unbounded))
	w := max(c.Layout(t.brand, room).W, c.Layout(t.name, room).W)
	room.Min.W = w
	b := c.Layout(t.brand, room)
	c.Place(t.brand, geom.Pt(0, 0))
	n := c.Layout(t.name, room)
	c.Place(t.name, geom.Pt(0, b.H))
	// The pictures of the shadows are made here, where a screen is built, and not
	// by the first frame that draws them while its grid slides.
	t.brandPic.Prepare(c, t.brand.TextLayout(), t.brandShadow)
	t.namePic.Prepare(c, t.name.TextLayout(), t.shadow)

	return geom.Sz(w, b.H+n.H)
}

// Paint draws the two lines, each over its shadow: the text-shadow is
// inherited by `s` and `b`, and the brand's is at the brand's .85.
func (t *titleBox) Paint(c *ui.Context, p *ui.Painter) {
	t.line(c, p, t.brand, &t.brandPic, t.brandShadow, t.ink.WithAlpha(brandOpacity))
	t.line(c, p, t.name, &t.namePic, t.shadow, t.ink)
}

// line draws one label where its own Paint would, with the shadow s from the
// picture pic holds.
func (t *titleBox) line(c *ui.Context, p *ui.Painter, l *kit.Label, pic *kit.ShadowCache, s kit.TextShadow, col color.Color) {
	ly := l.TextLayout()
	if ly == nil || len(ly.Lines) == 0 {
		return
	}
	f := l.Frame()
	pic.Draw(c, p, ly, geom.Pt(f.X, f.Y+l.Baseline()-ly.Lines[0].Baseline), s, col)
}
