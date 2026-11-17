//go:build tvos

// Adapted from jplayer-tv/internal/ui/watch/video.go at 781bee5 (a widget that
// tells a native plane where it is whenever its place changes, and hides the
// plane when it leaves the tree; it paints nothing itself while the plane
// shows). Changed: the surface is the two calls of env.Playback, and until the
// first frame the widget draws the K5 stage over the hole.

package tvkit

import (
	"dancefitvn-app/internal/ui/kit"

	"github.com/gdeskdev/desk/foundation/core/geom"
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

// VideoPlane is `.vstage` of TS.player (index.html:681, `left:656px;top:0;
// width:608px;height:1080px`) as the app has it: the hole the system's video
// plane shows through. It tells the plane where it is whenever that changes and
// hides the plane when it leaves the tree. Until the first frame is on the plane
// (K5) it draws the stage, which the video then replaces: once PictureReady it
// paints nothing, so that the view's transparent background and the plane under
// it show. Nothing over it may paint opaque while it shows. It is for the UI
// thread.
type VideoPlane struct {
	ui.Base

	surf   PlaneSurface
	stage  *kit.Stage
	placed geom.Rect // the last rectangle given to the plane
	has    bool      // placed holds a Place call, so the plane is showing
}

var (
	_ ui.Unmounter = (*VideoPlane)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewVideoPlane returns a plane over the surface; nil draws the stage always.
func NewVideoPlane(s PlaneSurface) *VideoPlane {
	v := &VideoPlane{surf: s, stage: kit.NewStage()}
	v.Name = namePlane
	v.Add(v.stage)

	return v
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Placed returns the rectangle last given to the plane and whether the plane is
// showing.
func (v *VideoPlane) Placed() (geom.Rect, bool) { return v.placed, v.has }

// Showing tells whether the picture is on the plane, so that the widget paints
// nothing.
func (v *VideoPlane) Showing() bool { return v.surf != nil && v.surf.PictureReady() }

// Picture is told the first frame came (or went): the stage is drawn again or
// not.
func (v *VideoPlane) Picture() { v.Repaint() }

// Layout takes the room it is given, which the stage fills.
func (v *VideoPlane) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	c.Layout(v.stage, ui.Tight(size))
	c.Place(v.stage, geom.Point{})

	return size
}

// Paint tells the plane where it is when that changed, then draws the stage
// until the picture is on it.
func (v *VideoPlane) Paint(c *ui.Context, p *ui.Painter) {
	if v.surf == nil {
		v.Base.Paint(c, p)

		return
	}
	if r := v.WindowFrame(); !v.has || r != v.placed {
		v.surf.Place(r)
		v.placed, v.has = r, true
	}
	if !v.surf.PictureReady() {
		v.Base.Paint(c, p)
	}
}

// Unmounted hides the plane and forgets where it was, so that joining a tree
// again places it anew.
func (v *VideoPlane) Unmounted(*ui.Context) {
	if v.has && v.surf != nil {
		v.surf.Place(geom.Rect{})
	}
	v.placed, v.has = geom.Rect{}, false
}
