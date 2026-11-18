//go:build tvos

// Adapted from jplayer-tv/internal/ui/watch/video.go at 781bee5 (a widget that
// tells a native plane where it is whenever its place changes, and hides the
// plane when it leaves the tree; it paints nothing itself while the plane
// shows). Changed: the surface is the two calls of env.Playback, and until the
// first frame the widget draws the course's cover darkened with a loading ring
// (K47), which cross-fades into the video.

package tvkit

import (
	"dancefitvn-app/internal/ui/kit"

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

// The loading look of the hole (K47, chủ sản phẩm yêu cầu 2026-11-18) is kit.LoadingCover:
// the prototype has none on TV, its `.vstage` (index.html:139-142) being the stand-in
// that K5 kept.
const (
	// PlaneFade is the cross-fade from the cover to the video, the 250 ms of
	// `.vui{transition:opacity .25s}` (index.html:130), the app's other fade of
	// this screen (K46): CSS `ease`, at once under Reduce Motion.
	PlaneFade = kit.CoverFade
)

// planeStyle is the look's sizes on the stage: a ring of 88 with a line of 8, and
// the wordmark standing in for a missing cover at 56 (`.tlogo` is 40, index.html:186,
// in the corner; the hole is 608 wide) with 2 of letter spacing, 40 above the ring.
var planeStyle = kit.CoverStyle{Spinner: 88, SpinnerLine: 8, Logo: 56, LogoSpacing: 2, LogoGap: 40}

// VideoPlane is `.vstage` of TS.player (index.html:681, `left:656px;top:0;
// width:608px;height:1080px`) as the app has it: the hole the system's video
// plane shows through. It tells the plane where it is whenever that changes and
// hides the plane when it leaves the tree. Until the first frame is on the plane
// (K47) it draws #0B0B0C with the course's cover over it, filling the hole
// (centred crop) under a scrim of black at .5, and a loading ring in --pr in the
// middle (the DanceFITVN wordmark above the ring while there is no cover picture); the picture arriving fades that out over PlaneFade, and the picture
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
	look   *kit.LoadingCover
	placed geom.Rect // the last rectangle given to the plane
	has    bool      // placed holds a Place call, so the plane is showing
}

// Loader loads the cover of a course at url (env.Services.Cover): blocking, off
// the UI thread; an error is a cover that cannot be had.
type Loader = kit.CoverLoader

var (
	_ ui.Unmounter = (*VideoPlane)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewVideoPlane returns a plane over the surface, with brand as the wordmark that
// stands in for a missing cover (splash.brand); a nil surface draws the loading look
// always.
func NewVideoPlane(s PlaneSurface, brand string) *VideoPlane {
	v := &VideoPlane{surf: s, look: kit.NewLoadingCover(planeStyle, brand)}
	v.Name = namePlane
	if s != nil {
		// a screen that opens on a playing video does not fade in
		v.look.SetReady(s.PictureReady)
	}
	v.Add(v.look)

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
func (v *VideoPlane) Fading() bool { return v.look.Fading() }

// Covered tells whether the loading look is the plane's paint: the picture is not
// on it, or is still fading in.
func (v *VideoPlane) Covered() bool { return v.look.Covered() }

// Cover returns the cover picture once it has loaded, nil until then.
func (v *VideoPlane) Cover() *scene.Image { return v.look.Cover() }

// Look returns the loading look the plane draws until the picture (its wordmark and
// ring).
func (v *VideoPlane) Look() *kit.LoadingCover { return v.look }

// Picture is told the first frame came (or went): the cover fades out, or is
// drawn again.
func (v *VideoPlane) Picture() { v.Repaint() }

// SetCover has the plane show the cover at url, which load gets once the plane is
// in a view, over #0B0B0C with the wordmark until it is there, and for good when
// load fails or there is no url. A new url drops the picture of the old one and ends
// its load; the same url changes nothing. Call it on the UI thread.
func (v *VideoPlane) SetCover(load Loader, url string) { v.look.SetCover(load, url) }

// Layout takes the room it is given, with the look filling it.
func (v *VideoPlane) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	c.Layout(v.look, ui.Tight(size))
	c.Place(v.look, geom.Point{})

	return size
}

// Paint tells the plane where it is when that changed, then draws the loading
// look until the picture is on it, fading it out as it arrives.
func (v *VideoPlane) Paint(c *ui.Context, p *ui.Painter) {
	if v.surf != nil {
		if r := v.WindowFrame(); !v.has || r != v.placed {
			v.surf.Place(r)
			v.placed, v.has = r, true
		}
	}
	p.Child(c, v.look)
}

// Unmounted hides the plane and forgets where it was, so that joining a tree again
// places it anew; the look ends the load of the cover as it leaves.
func (v *VideoPlane) Unmounted(*ui.Context) {
	if v.has && v.surf != nil {
		v.surf.Place(geom.Rect{})
	}
	v.placed, v.has = geom.Rect{}, false
}
