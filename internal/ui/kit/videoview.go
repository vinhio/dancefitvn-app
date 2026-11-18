// Adapted from jplayer-mobile/internal/ui/np/video.go at 49de3c2 (VideoView:
// one latch of the surface's frame a paint, the frame the surface kept, interest
// in the frames only while mounted and shown, the funcs a paint reuses so that
// no frame allocates). Changed: it reads a VideoSource, not a *media.Surface
// (the kit does not import internal/media, internal/archtest), it has Mirror
// (the flip of 04 sections 2.3 and 3.3, proven in cmd/avspike/video.go), it has
// no radius, no fallback widget and no Scaled: screen 06's picture is a plain
// box under the loading look (LoadingCover, K47: it replaced the K5 stage), which
// the screen draws over it until the first frame (HasPicture tells it).

package kit

import (
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/event"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// NameVideo is the Name layout.json finds the picture's box by.
const NameVideo = "video"

// aspectW and aspectH are the shape of a box that is given a width only: 16 by
// 9, the shape of the sample the engines were measured with.
const aspectW, aspectH float32 = 16, 9

// videoBG is `background:#000` of `.scr.vid` (index.html:415): what shows around
// a contained picture, and in place of a frame that is not there, unless Under.
var videoBG = color.RGB(0, 0, 0)

// maxExtent is where a constraint counts as unbounded.
const maxExtent = 3.4e38

// VideoSource is the frames of a video as VideoView reads them: media.Surface.
// Every method but Watch and Frames is for the UI thread, as the Surface's.
type VideoSource interface {
	// Frame is the newest frame, latched once for the paint numbered paint.
	Frame(paint uint64) (img *scene.Image, seq uint64)
	// Rotation is the clockwise turn, in degrees, that stands the frame upright.
	Rotation() int
	// Kept is the frame kept while the engine has none.
	Kept() (img *scene.Image, rot int)
	// Active says the engine plays now; Pulls says the UI asks it for frames;
	// Pull asks for one.
	Active() bool
	Pulls() bool
	Pull()
	// Watch tells the producer a view wants frames; the func it returns ends the
	// interest.
	Watch() (unwatch func())
	// Frames announces each frame the producer published.
	Frames() *event.Topic[uint64]
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// VideoView shows a VideoSource: black, with the newest frame fitted inside
// (FitContain) or over (FitCover) its box, turned upright by the source's
// Rotation and flipped left to right when Mirror is set. When the engine has no
// frame (paused after a Free, stalled, reloading, failed) it draws the frame
// the source kept; with none kept it draws nothing when Under, so that what lies under it
// shows, and black otherwise.
//
// Paint is the one place the UI latches a frame (VideoSource.Frame), at the
// start, with the number of the window's paint (ui.View.FramesDrawn). For an
// engine the UI pulls from (iOS) a view that is Active pulls a frame and asks
// for the next paint, so it paints every display frame while the media plays;
// otherwise a paint follows a frame the producer announces on Frames, and none
// follows when nothing moves. A view that is not mounted or not shown (SetShown)
// does not ask the producer for frames.
//
// Every method runs on the UI thread. The zero Fit paints no picture.
type VideoView struct {
	ui.Base
	// Fit is object-fit: FitContain for screen 06.
	Fit Fit
	// Mirror flips the picture left to right about the middle of the box:
	// "Lật gương" (`.vstage.mir`). Change it with SetMirror.
	Mirror bool
	// Under makes the view paint nothing when it has no picture, so that what
	// lies under it shows.
	Under bool
	// pic is whether the last paint had a picture to draw (the engine's frame
	// or the kept one).
	pic bool

	src     VideoSource
	unwatch func() // from VideoSource.Watch while attached
	unsub   func() // from VideoSource.Frames().Subscribe while attached
	mounted bool
	hidden  bool // SetShown(false)

	// p, img, dst, rot and mirrored are what the paint of the picture reads:
	// Clip and Transform take a func without arguments, so Paint leaves them
	// here for the funcs below, made once, and no paint allocates a closure.
	p          *ui.Painter
	img        *scene.Image
	dst        geom.Rect
	cx         float32
	rot        int
	inClip     func()
	drawTurned func()
	drawFlip   func()
	onFrameFn  func(uint64)
}

var (
	_ ui.Mounter   = (*VideoView)(nil)
	_ ui.Unmounter = (*VideoView)(nil)
)

// NewVideoView returns a view of s (nil shows what lies under it, or black) with
// the fit.
func NewVideoView(s VideoSource, fit Fit) *VideoView {
	v := &VideoView{Fit: fit, src: s}
	v.Name = NameVideo
	v.init()
	if s != nil {
		img, _ := s.Kept()
		v.pic = img != nil
	}

	return v
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetSource shows s, nil for none: the old source is let go of and the new one
// watched at once when the view is mounted and shown.
func (v *VideoView) SetSource(s VideoSource) {
	if s == v.src {
		return
	}
	v.detach()
	v.src = s
	v.pic = false
	if s != nil {
		img, _ := s.Kept()
		v.pic = img != nil // a screen that opens on a kept frame has a picture
	}
	v.attach()
	v.Repaint()
}

// Source returns the source shown.
func (v *VideoView) Source() VideoSource { return v.src }

// HasPicture tells whether a picture is on the view: the last paint drew a frame
// (the engine's, or the kept one); before the first paint, whether the source
// keeps one. It is false from the Reset of a new media until its first frame.
// Allocates nothing.
func (v *VideoView) HasPicture() bool { return v.pic }

// SetMirror flips the picture, or not; the same value changes nothing.
func (v *VideoView) SetMirror(on bool) {
	if v.Mirror != on {
		v.Mirror = on
		v.Repaint()
	}
}

// SetShown shows or hides the view and, with it, whether it asks the producer
// for frames: hiding unwatches the source.
func (v *VideoView) SetShown(on bool) {
	if v.hidden == !on && v.Hidden == !on {
		return
	}
	v.hidden, v.Hidden = !on, !on
	if on {
		v.attach()
	} else {
		v.detach()
	}
	v.Relayout()
}

// Mounted starts the interest in the source's frames.
func (v *VideoView) Mounted(*ui.Context) {
	v.mounted = true
	v.attach()
}

// Unmounted ends it: the source is no longer watched for this view.
func (v *VideoView) Unmounted(*ui.Context) {
	v.detach()
	v.mounted = false
}

// Layout fills the constraints, as `inset:0` fills its parent. A box with no
// height to fill is 16:9.
func (v *VideoView) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	w, h := cs.Max.W, cs.Max.H
	if w > maxExtent {
		w = cs.Min.W
	}
	if h > maxExtent {
		h = w * aspectH / aspectW
	}

	return cs.Constrain(geom.Sz(w, h))
}

// Paint draws the black box, then the newest frame fitted into it: the
// engine's, else the one the source kept; without a picture, nothing (Under) or
// the black box.
func (v *VideoView) Paint(ctx *ui.Context, p *ui.Painter) {
	b := v.Frame()
	box := geom.XYWH(0, 0, b.W, b.H)
	s := v.src
	if s != nil && s.Active() && s.Pulls() {
		// The engine's frames come when asked for: the UI asks at each display
		// frame while the media plays (04 section 2.2).
		s.Pull()
		v.Repaint()
	}
	var img *scene.Image
	rot := 0
	if s != nil {
		img, _ = s.Frame(PaintNumber(ctx)) // one latch a paint
		rot = s.Rotation()
		if img == nil {
			img, rot = s.Kept() // the engine has none: the frame kept
		}
	}
	v.pic = img != nil
	if img == nil {
		if !v.Under {
			p.Fill(box, scene.Solid(videoBG), 0)
		}

		return
	}
	p.Fill(box, scene.Solid(videoBG), 0)
	if box.Empty() {
		return
	}
	dst := fitRect(img.Width, img.Height, rot, box, v.Fit)
	if dst.Empty() {
		return
	}
	v.img, v.dst, v.rot, v.cx = img, dst, rot, box.X+box.W/2
	if v.inClip == nil {
		v.init()
	}
	v.p = p
	if v.Fit == FitCover {
		p.Clip(box, 0, v.inClip) // overflow: hidden
	} else {
		v.inClip() // a contained picture lies inside the box
	}
	v.p, v.img = nil, nil
}

// PaintNumber is the number VideoSource.Frame takes for the paint ctx runs: the
// frames its view drew, which the view counts up before it paints, so the
// widgets of one paint share it and the next paint has another. It is 0, a latch
// at every call, without a view.
func PaintNumber(ctx *ui.Context) uint64 {
	if ctx == nil || ctx.View() == nil {
		return 0
	}

	return uint64(ctx.View().FramesDrawn())
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// init makes the funcs a paint and a subscription reuse.
func (v *VideoView) init() {
	v.inClip = v.paintPicture
	v.drawTurned = v.drawPicture
	v.drawFlip = v.paintTurned
	v.onFrameFn = v.onFrame
}

// paintPicture draws the picture of this paint, flipped when Mirror: the flip
// is about the middle of the box, the turn about the middle of the picture.
func (v *VideoView) paintPicture() {
	if v.Mirror {
		v.p.Transform(mirrorAbout(v.cx), v.drawFlip)

		return
	}
	v.paintTurned()
}

// paintTurned draws the picture upright: the image is laid out in the rectangle
// that the turn about the picture's center maps to the fitted one.
func (v *VideoView) paintTurned() {
	if v.rot == 0 {
		v.p.Image(v.img, v.dst, 1)

		return
	}
	v.p.Transform(turn(v.rot, geom.Pt(v.dst.X+v.dst.W/2, v.dst.Y+v.dst.H/2)), v.drawTurned)
}

// drawPicture draws the image unturned, inside the Transform of paintTurned.
func (v *VideoView) drawPicture() { v.p.Image(v.img, unturned(v.dst, v.rot), 1) }

// onFrame is the listener of VideoSource.Frames: a frame was published.
func (v *VideoView) onFrame(uint64) { v.Repaint() }

// attach watches the source and listens for its frames, if the view is mounted,
// shown and has one, and is not already.
func (v *VideoView) attach() {
	if !v.mounted || v.hidden || v.src == nil || v.unwatch != nil {
		return
	}
	if v.onFrameFn == nil {
		v.init()
	}
	v.unwatch = v.src.Watch()
	v.unsub = v.src.Frames().Subscribe(v.onFrameFn)
}

// detach undoes attach.
func (v *VideoView) detach() {
	if v.unwatch == nil {
		return
	}
	v.unsub()
	v.unwatch()
	v.unsub, v.unwatch = nil, nil
}
