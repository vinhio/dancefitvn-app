// Adapted from internal/ui/tv/tvkit/plane.go (K47: the loading look of the video
// hole: #0B0B0C, the course's cover aspect-filled under a black scrim, a ring, a
// cross-fade out when the picture arrives), lifted into the kit so that the phone's
// screen 06 and the TV plane share one. Changed: it asks for the picture through a
// func (Ready) instead of the TV's surface, it shows the brand's wordmark above the
// ring while there is no cover (the owner's addition of 2026-11-18), and the
// background, the ring and the wordmark are a style.

package kit

import (
	"context"
	"time"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// CoverFade is the cross-fade from the cover to the video, the 250 ms of
// `.vui{transition:opacity .25s}` (index.html:130): CSS `ease`, at once under
// Reduce Motion.
const CoverFade = 250 * time.Millisecond

const (
	// coverScrim is the black laid over the cover: `rgba(0,0,0,.5)`.
	coverScrim = .5
	// coverLogoAlpha dims the wordmark that stands in for a cover (the owner's
	// choice, K47: "dimmed slightly").
	coverLogoAlpha = .6
	// coverLogoWeight is `.tlogo{font-weight:800}` (index.html:186), the weight of
	// the brand everywhere (splash `.d.acc`).
	coverLogoWeight = 800
)

// DefaultCoverBg is `<div class="scr" style="background:#0B0B0C">` of TS.player
// (index.html:681): what the look is until a cover has loaded, or with none.
var DefaultCoverBg = color.RGB(0x0b, 0x0b, 0x0c)

// CoverLoader loads the cover of a course at url (env.Services.Cover): blocking,
// off the UI thread; an error is a cover that cannot be had.
type CoverLoader func(ctx context.Context, url string) (*scene.Image, error)

// CoverStyle is the sizes of the look, in DIPs: the ring's side and line, the
// wordmark's font size and letter spacing, and the air between the wordmark and
// the ring.
type CoverStyle struct {
	Spinner, SpinnerLine, Logo, LogoSpacing, LogoGap float32
}

// LoadingCover is what a video's box is until its first frame (K47): the Bg with
// the course's cover over it, filling the box (centred crop) under a scrim of
// black at .5, and the loading ring in the middle; with no cover (no address,
// the load failed, it has not come yet) the DanceFITVN wordmark stands above the
// ring in its place, dimmed. When Ready says the picture is there the look fades
// out over CoverFade and then paints nothing, so the picture under it shows; when
// Ready goes false it is back at once. It is for the UI thread.
type LoadingCover struct {
	ui.Base
	// Decor: the cover, the wordmark and the ring are decoration; the lesson's
	// texts say what loads.
	Decor
	// Bg is the colour under the cover; DefaultCoverBg unless set.
	Bg color.Color

	ready func() bool
	style CoverStyle
	spin  *Spinner
	logo  *Label
	fade  ui.Animated[float32] // 0 the look shows, 1 the picture does

	// The cover: load fetches the picture at cover, from Mounted on; asked is the
	// address the picture shown or coming is of, stop ends the load under way.
	load    CoverLoader
	cover   string
	asked   string
	image   *scene.Image
	stop    func()
	mounted bool

	// c, pt and the funcs paint the look under a layer and a clip: made once, so
	// that a frame allocates nothing.
	c         *ui.Context
	pt        *ui.Painter
	drawLook  func()
	drawImage func()
	drawLogo  func()
}

var (
	_ ui.Mounter   = (*LoadingCover)(nil)
	_ ui.Unmounter = (*LoadingCover)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewLoadingCover returns the look in the style, with brand as the wordmark's
// text (splash.brand).
func NewLoadingCover(st CoverStyle, brand string) *LoadingCover {
	v := &LoadingCover{Bg: DefaultCoverBg, style: st, spin: NewSpinner(st.Spinner, st.SpinnerLine)}
	v.logo = &Label{
		Text: brand, Role: theme.RoleDisplay, Size: st.Logo, Weight: coverLogoWeight,
		Spacing: st.LogoSpacing, LineHeight: 1, Upper: true, Ink: theme.InkPrimary,
	}
	v.fade.Ease, v.fade.Duration = theme.CSSEase, CoverFade
	v.drawLook = func() { v.paintLook(v.c, v.pt) }
	v.drawImage = func() { v.paintImage(v.pt) }
	v.drawLogo = func() { v.pt.Child(v.c, v.logo) }
	v.Add(v.spin, v.logo)

	return v
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetReady sets what tells that the picture is on: nil is never. A picture that is
// there already is not faded in.
func (v *LoadingCover) SetReady(fn func() bool) {
	v.ready = fn
	if fn != nil && fn() {
		v.fade.Set(1)
	}
}

// Covered tells whether the look is what paints: the picture is not on, or is
// still fading in.
func (v *LoadingCover) Covered() bool { return v.fade.Value() < 1 }

// Fading tells whether the look is fading out over the picture.
func (v *LoadingCover) Fading() bool { return v.fade.Running() }

// Cover returns the cover picture once it has loaded, nil until then.
func (v *LoadingCover) Cover() *scene.Image { return v.image }

// Logo returns the wordmark label; it paints only while there is no cover
// picture.
func (v *LoadingCover) Logo() *Label { return v.logo }

// Spinner returns the ring.
func (v *LoadingCover) Spinner() *Spinner { return v.spin }

// SetCover has the look show the cover at url, which load gets once the widget is
// in a view, the wordmark until it is there, and for good when load fails or there
// is no url. A new url drops the picture of the old one and ends its load; the
// same url changes nothing. Call it on the UI thread.
func (v *LoadingCover) SetCover(load CoverLoader, url string) {
	if url == "" {
		load = nil
	}
	v.load = load
	if v.cover == url {
		v.begin()

		return
	}
	v.cover = url
	v.end()
	v.asked = ""
	v.setImage(nil)
	v.begin()
}

// Mounted starts the load of the cover, when one is wanted.
func (v *LoadingCover) Mounted(*ui.Context) {
	v.mounted = true
	v.begin()
}

// Unmounted ends the interest in the load: ui.Go has ended it with the widget's
// leaving.
func (v *LoadingCover) Unmounted(*ui.Context) {
	v.mounted = false
	v.stop = nil
	if v.image == nil {
		v.asked = ""
	}
}

// Layout takes the room it is given: the ring in the middle of it, the wordmark
// above the ring.
func (v *LoadingCover) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	ss := c.Layout(v.spin, ui.Tight(geom.Sz(v.style.Spinner, v.style.Spinner)))
	top := (size.H - ss.H) / 2
	c.Place(v.spin, geom.Pt((size.W-ss.W)/2, top))
	ls := c.Layout(v.logo, ui.Loose(size))
	c.Place(v.logo, geom.Pt((size.W-ls.W)/2, max(top-v.style.LogoGap-ls.H, 0)))

	return size
}

// Paint draws the look until the picture is on, fading it out as it arrives.
func (v *LoadingCover) Paint(c *ui.Context, p *ui.Painter) {
	if v.ready == nil || !v.ready() {
		v.fade.Set(0) // a picture that went away is covered at once
		v.paintLook(c, p)

		return
	}
	mix := v.fade.Track(v, 1)
	switch {
	case mix >= 1:
	case mix <= 0:
		v.paintLook(c, p)
	default:
		v.c, v.pt = c, p
		p.Layer(1-mix, 0, v.drawLook)
		v.c, v.pt = nil, nil
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// paintLook draws the box as it is until the picture: the Bg, then the cover
// aspect-filled under the scrim, or the wordmark when there is none, and the ring.
func (v *LoadingCover) paintLook(c *ui.Context, p *ui.Painter) {
	r := v.Bounds()
	p.Fill(r, scene.Solid(v.Bg), 0)
	if img := v.image; img != nil && img.Width > 0 && img.Height > 0 {
		v.pt = p
		p.Clip(r, 0, v.drawImage)
	} else {
		v.c, v.pt = c, p
		p.Layer(coverLogoAlpha, 0, v.drawLogo)
	}
	p.Child(c, v.spin)
	v.c, v.pt = nil, nil
}

// paintImage draws the cover to fill the box, centred, then the scrim.
func (v *LoadingCover) paintImage(p *ui.Painter) {
	r := v.Bounds()
	img := v.image
	k := max(r.W/float32(img.Width), r.H/float32(img.Height))
	w, h := float32(img.Width)*k, float32(img.Height)*k
	p.Image(img, geom.XYWH(r.X+(r.W-w)/2, r.Y+(r.H-h)/2, w, h), 1)
	p.Fill(r, scene.Solid(color.RGBA(0, 0, 0, coverScrim)), 0)
}

// setImage shows img, nil for none.
func (v *LoadingCover) setImage(img *scene.Image) {
	if v.image == img {
		return
	}
	v.image = img
	v.Repaint()
}

// begin starts the load of the cover when the widget is in a view, one is wanted
// and none is under way or done for this url.
func (v *LoadingCover) begin() {
	if !v.mounted || v.load == nil || v.cover == "" || v.asked == v.cover {
		return
	}
	url, load := v.cover, v.load
	v.asked = url
	v.stop = ui.Go(v, func(ctx context.Context) (*scene.Image, error) {
		return load(ctx, url)
	}, func(_ *ui.Context, img *scene.Image, err error) {
		v.stop = nil
		if err == nil && img != nil && v.cover == url {
			v.setImage(img)
		}
	})
}

// end ends the load under way.
func (v *LoadingCover) end() {
	if v.stop != nil {
		v.stop()
		v.stop = nil
	}
}
