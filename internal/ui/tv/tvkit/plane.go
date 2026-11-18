//go:build tvos

// Adapted from jplayer-tv/internal/ui/watch/video.go at 781bee5 (a widget that
// tells a native plane where it is whenever its place changes, and hides the
// plane when it leaves the tree; it paints nothing itself while the plane
// shows). Changed: the surface is the two calls of env.Playback, and until the
// first frame the widget draws the course's cover darkened with a loading ring
// (K47), which cross-fades into the video.

package tvkit

import (
	"context"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// PlaneSurface is what a VideoPlane asks of the player: where the plane is
// (Place; the empty rectangle hides it) and whether the first frame is on it.
// env.Playback has both.
type PlaneSurface interface {
	// Place puts the video's plane at r in the window; an empty r hides it.
	Place(r geom.Rect)
	// PictureReady tells that the first frame of the video is on the plane.
	PictureReady() bool
}

// namePlane is the key layout.json would give the plane; the prototype's
// `.vstage` has no class the capture records.
const namePlane = "tv.player.plane"

// The loading look of the hole (K47, chủ sản phẩm yêu cầu 2026-11-18): the
// prototype has none on TV, its `.vstage` (index.html:139-142) being the stand-in
// that K5 kept.
const (
	// PlaneFade is the cross-fade from the cover to the video, the 250 ms of
	// `.vui{transition:opacity .25s}` (index.html:130), the app's other fade of
	// this screen (K46): CSS `ease`, at once under Reduce Motion.
	PlaneFade = 250 * time.Millisecond
	// planeScrim is the black laid over the cover: `rgba(0,0,0,.5)`.
	planeScrim = .5
	// planeSpinner and planeSpinnerLine are the ring's side and line, in DIPs.
	planeSpinner, planeSpinnerLine float32 = 88, 8
)

// planeBg is `<div class="scr" style="background:#0B0B0C">` of TS.player
// (index.html:681): what the hole is until the cover has loaded, or with none.
var planeBg = color.RGB(0x0b, 0x0b, 0x0c)

// VideoPlane is `.vstage` of TS.player (index.html:681, `left:656px;top:0;
// width:608px;height:1080px`) as the app has it: the hole the system's video
// plane shows through. It tells the plane where it is whenever that changes and
// hides the plane when it leaves the tree. Until the first frame is on the plane
// (K47) it draws #0B0B0C with the course's cover over it, filling the hole
// (centred crop) under a scrim of black at .5, and a loading ring in --pr in the
// middle; the picture arriving fades that out over PlaneFade, and the picture
// going away shows it again at once. Once PictureReady and faded it paints
// nothing, so that the view's transparent background and the plane under it
// show. Nothing over it may paint opaque while it shows. It is for the UI
// thread.
type VideoPlane struct {
	ui.Base
	// Decor: the cover and the ring are decoration; the lesson's texts say what
	// loads.
	kit.Decor

	surf   PlaneSurface
	spin   *kit.Spinner
	fade   ui.Animated[float32] // 0 the cover shows, 1 the video does
	placed geom.Rect            // the last rectangle given to the plane
	has    bool                 // placed holds a Place call, so the plane is showing

	// The cover: load fetches the picture at cover, from Mounted on; asked is the
	// address the picture shown or coming is of, stop ends the load under way.
	load    Loader
	cover   string
	asked   string
	image   *scene.Image
	stop    func()
	mounted bool

	// c, pt, drawLoading and drawImage paint the cover under a layer and a clip:
	// made once, so that a frame allocates nothing.
	c           *ui.Context
	pt          *ui.Painter
	drawLoading func()
	drawImage   func()
}

// Loader loads the cover of a course at url (env.Services.Cover): blocking, off
// the UI thread; an error is a cover that cannot be had.
type Loader = poster.Loader

var (
	_ ui.Mounter   = (*VideoPlane)(nil)
	_ ui.Unmounter = (*VideoPlane)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewVideoPlane returns a plane over the surface; nil draws the loading look
// always.
func NewVideoPlane(s PlaneSurface) *VideoPlane {
	v := &VideoPlane{surf: s, spin: kit.NewSpinner(planeSpinner, planeSpinnerLine)}
	v.Name = namePlane
	v.fade.Ease, v.fade.Duration = theme.CSSEase, PlaneFade
	if s != nil && s.PictureReady() {
		v.fade.Set(1) // a screen that opens on a playing video does not fade in
	}
	v.drawLoading = func() { v.paintLoading(v.c, v.pt) }
	v.drawImage = func() { v.paintImage(v.pt) }
	v.Add(v.spin)

	return v
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Placed returns the rectangle last given to the plane and whether the plane is
// showing.
func (v *VideoPlane) Placed() (geom.Rect, bool) { return v.placed, v.has }

// Showing tells whether the picture is on the plane, so that the widget paints
// nothing once it has faded in.
func (v *VideoPlane) Showing() bool { return v.surf != nil && v.surf.PictureReady() }

// Fading tells whether the cover is fading out over the picture.
func (v *VideoPlane) Fading() bool { return v.fade.Running() }

// Covered tells whether the loading look is the plane's paint: the picture is not
// on it, or is still fading in.
func (v *VideoPlane) Covered() bool { return v.fade.Value() < 1 }

// Cover returns the cover picture once it has loaded, nil until then.
func (v *VideoPlane) Cover() *scene.Image { return v.image }

// Picture is told the first frame came (or went): the cover fades out, or is
// drawn again.
func (v *VideoPlane) Picture() { v.Repaint() }

// SetCover has the plane show the cover at url, which load gets once the plane is
// in a view, over #0B0B0C until it is there, and for good when load fails or
// there is no url. A new url drops the picture of the old one and ends its load;
// the same url changes nothing. Call it on the UI thread.
func (v *VideoPlane) SetCover(load Loader, url string) {
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
func (v *VideoPlane) Mounted(*ui.Context) {
	v.mounted = true
	v.begin()
}

// Layout takes the room it is given, with the ring in the middle of it.
func (v *VideoPlane) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	ss := c.Layout(v.spin, ui.Tight(geom.Sz(planeSpinner, planeSpinner)))
	c.Place(v.spin, geom.Pt((size.W-ss.W)/2, (size.H-ss.H)/2))

	return size
}

// Paint tells the plane where it is when that changed, then draws the loading
// look until the picture is on it, fading it out as it arrives.
func (v *VideoPlane) Paint(c *ui.Context, p *ui.Painter) {
	if v.surf == nil {
		v.paintLoading(c, p)

		return
	}
	if r := v.WindowFrame(); !v.has || r != v.placed {
		v.surf.Place(r)
		v.placed, v.has = r, true
	}
	if !v.surf.PictureReady() {
		v.fade.Set(0) // a picture that went away is covered at once
		v.paintLoading(c, p)

		return
	}
	mix := v.fade.Track(v, 1)
	switch {
	case mix >= 1:
	case mix <= 0:
		v.paintLoading(c, p)
	default:
		v.c, v.pt = c, p
		p.Layer(1-mix, 0, v.drawLoading)
		v.c, v.pt = nil, nil
	}
}

// Unmounted hides the plane, ends the load of the cover and forgets where the plane
// was, so that joining a tree again places it anew.
func (v *VideoPlane) Unmounted(*ui.Context) {
	if v.has && v.surf != nil {
		v.surf.Place(geom.Rect{})
	}
	v.placed, v.has = geom.Rect{}, false
	v.mounted = false
	v.stop = nil // ui.Go has ended the load with the plane's leaving
	if v.image == nil {
		v.asked = ""
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// paintLoading draws the hole as it is until the picture: #0B0B0C, the cover
// aspect-filled over it under the scrim, and the ring.
func (v *VideoPlane) paintLoading(c *ui.Context, p *ui.Painter) {
	r := v.Bounds()
	p.Fill(r, scene.Solid(planeBg), 0)
	if img := v.image; img != nil && img.Width > 0 && img.Height > 0 {
		v.pt = p
		p.Clip(r, 0, v.drawImage)
	}
	p.Child(c, v.spin)
}

// paintImage draws the cover to fill the hole, centred, then the scrim.
func (v *VideoPlane) paintImage(p *ui.Painter) {
	r := v.Bounds()
	img := v.image
	k := max(r.W/float32(img.Width), r.H/float32(img.Height))
	w, h := float32(img.Width)*k, float32(img.Height)*k
	p.Image(img, geom.XYWH(r.X+(r.W-w)/2, r.Y+(r.H-h)/2, w, h), 1)
	p.Fill(r, scene.Solid(color.RGBA(0, 0, 0, planeScrim)), 0)
}

// setImage shows img, nil for none.
func (v *VideoPlane) setImage(img *scene.Image) {
	if v.image == img {
		return
	}
	v.image = img
	v.Repaint()
}

// begin starts the load of the cover when the plane is in a view, one is wanted
// and none is under way or done for this url.
func (v *VideoPlane) begin() {
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
func (v *VideoPlane) end() {
	if v.stop != nil {
		v.stop()
		v.stop = nil
	}
}
