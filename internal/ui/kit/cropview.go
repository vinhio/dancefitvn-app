// jPlayer Mobile and TV have no crop view to copy (checked at 49de3c2 and 781bee5):
// this one is written from `.crop` and the code of the avatar sheet
// (index.html:167-170, :520-526, :571-575). The math of the picture's place is
// internal/avatar's.

package kit

import (
	"dancefitvn-app/internal/avatar"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of `.crop` (index.html:167-170).
const (
	// cropHeight, cropRadius are `.crop{height:230px;border-radius:16px}`.
	cropHeight, cropRadius float32 = 230, 16
	// cropMaskHole is where `radial-gradient(circle at 50% 50%, transparent 99px,
	// rgba(0,0,0,.55) 100px)` turns: it ramps over the pixel from 99 to 100, so the
	// edge of the hole is at its middle.
	cropMaskHole float32 = 99.5
	// cropMaskAlpha is the 0.55 of that gradient's black.
	cropMaskAlpha float32 = .55
	// cropRingWidth is `.mask:after{border:2.5px solid #fff}`; its box is
	// avatar.RingSide wide (200, the border inside, box-sizing).
	cropRingWidth float32 = 2.5
)

// nameCrop is the key of the crop view in layout.json: `<div class="crop"
// id="crop">` is not recorded by the prototype's capture.
const nameCrop = "crop"

// cropBg is `.crop{background:#2A2D33}`: a literal of the prototype, not a token,
// and what the export fills where the picture does not reach (avatar.Fill).
var cropBg = color.RGB(0x2A, 0x2D, 0x33)

// CropView is `.crop` of the avatar sheet (index.html:167-170, :521): a frame 230
// tall, 16 rounded, of #2A2D33, over which a 260 square box of the picture (its
// centred square, cover-scaled; the person figure of the avatar when there is no
// picture) is moved by View.X, View.Y and scaled by View.Zoom about its centre,
// the whole clipped to the frame; over that the mask (black at .55 outside a hole
// of radius 100) and the ring (200 across, 2.5 white). A finger down on it moves
// the picture by what it moves, with no limit (Q-P6-8); OnMove hears the new View.
// A press on it never starts the drag of a sheet around it (DragExempt), and takes
// the finger from the scrolls around it (`touch-action:none`).
//
// Painting and moving allocate nothing.
type CropView struct {
	ui.Base
	// Image is the picture; nil draws the person. It must stay unchanged until
	// replaced (scene.Image).
	Image *scene.Image
	// View is where the picture is: set it with SetView.
	View avatar.View
	// OnMove is told the View after every move of the finger.
	OnMove func(v avatar.View)
	// Label names the view for assistive technologies.
	Label string

	figure   Avatar
	mask     scene.Path
	maskFor  geom.Size
	drag     bool
	last     geom.Point
	inFrame  func()
	inBox    func()
	drawImg  func()
	c        *ui.Context
	p        *ui.Painter
	imgDst   geom.Rect // where the whole picture goes in the box, from the last paint
	boxClip  geom.Rect
	attached bool
}

var (
	_ ui.PointerHandler = (*CropView)(nil)
	_ ui.Labeler        = (*CropView)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewCropView returns a crop view of the person at v.
func NewCropView(v avatar.View) *CropView {
	return &CropView{View: v}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetImage shows img in place of the person (nil: the person); the same picture
// changes nothing.
func (v *CropView) SetImage(img *scene.Image) {
	if v.Image != img {
		v.Image = img
		v.Repaint()
	}
}

// SetView moves the picture to view; the same view changes nothing.
func (v *CropView) SetView(view avatar.View) {
	if v.View != view {
		v.View = view
		v.Repaint()
	}
}

// Dragging tells whether a finger is moving the picture.
func (v *CropView) Dragging() bool { return v.drag }

// AccessibleLabel returns the view's label.
func (v *CropView) AccessibleLabel() string { return v.Label }

// DragExempt tells that a press on the view never starts the drag of a sheet
// around it.
func (v *CropView) DragExempt() bool { return true }

// Cursor is the hand of `.crop{cursor:grab}` as near as gDesk has one (no grab
// cursor in platform.Cursor; a phone has none).
func (v *CropView) Cursor() platform.Cursor { return platform.CursorMove }

// Layout takes the room's width and 230 DIPs.
func (v *CropView) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	if v.Name == "" {
		v.Name = nameCrop
	}
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = 0
	}

	return cs.Constrain(geom.Sz(w, cropHeight))
}

// HandlePointer moves the picture with the finger: from the press, by every move.
func (v *CropView) HandlePointer(_ *ui.Context, e *ui.PointerEvent) {
	switch e.Kind {
	case ui.PointerDown:
		if e.Button != platform.ButtonPrimary && e.Button != platform.ButtonNone {
			return
		}
		e.Grab()
		v.drag, v.last = true, e.Pos
		v.Repaint()
	case ui.PointerMove:
		if !v.drag {
			return
		}
		e.Handle()
		// 1 DIP is 1 CSS px: `phoneScale` is 1 (index.html:571-575)
		v.View.X += float64(e.Pos.X - v.last.X)
		v.View.Y += float64(e.Pos.Y - v.last.Y)
		v.last = e.Pos
		v.Repaint()
		if v.OnMove != nil {
			v.OnMove(v.View)
		}
	case ui.PointerUp, ui.PointerCancel:
		if v.drag {
			v.drag = false
			e.Handle()
			v.Repaint()
		}
	case ui.PointerUnknown, ui.PointerEnter, ui.PointerLeave, ui.PointerScroll:
	}
}

// Paint draws the frame, the picture, the mask and the ring.
func (v *CropView) Paint(c *ui.Context, p *ui.Painter) {
	if !v.attached {
		v.attached = true
		v.inFrame, v.inBox, v.drawImg = v.paintFrame, v.paintBox, v.paintImage
		v.figure.Square, v.figure.Size = true, avatar.BoxSide
	}
	r := v.Bounds()
	v.c, v.p = c, p
	p.Clip(r, capRadius(r, cropRadius), v.inFrame)
	v.c, v.p = nil, nil
}

// paintFrame draws what is inside the frame.
func (v *CropView) paintFrame() {
	p := v.p
	r := v.Bounds()
	p.Fill(r, scene.Solid(cropBg), 0)
	// `transform:translate(x,y) scale(zoom)` about the box's centre, the box centred
	// in the frame: box-local (130,130) lands on the frame's centre moved by (x,y)
	z := float32(v.View.Zoom)
	bx := r.W/2 + float32(v.View.X) - avatar.BoxSide/2*z
	by := r.H/2 + float32(v.View.Y) - avatar.BoxSide/2*z
	p.Transform(scene.Affine{XX: z, YY: z, X0: bx, Y0: by}, v.inBox)

	// the mask: black at .55 outside the hole, then the ring
	if sz := r.Size(); sz != v.maskFor {
		v.maskFor = sz
		v.mask.Reset()
		v.mask.Rect(geom.XYWH(0, 0, sz.W, sz.H))
		v.mask.Circle(sz.W/2, sz.H/2, cropMaskHole)
	}
	p.FillPath(&v.mask, scene.Solid(color.RGBA(0, 0, 0, cropMaskAlpha)), scene.EvenOdd)
	ring := geom.XYWH(r.W/2-avatar.RingSide/2, r.H/2-avatar.RingSide/2, avatar.RingSide, avatar.RingSide)
	p.Border(ring, geom.Uniform(cropRingWidth), color.White, avatar.RingSide/2)
}

// paintBox draws the box, in its own coordinates (260 across): the picture's
// centred square, or the figure.
func (v *CropView) paintBox() {
	img := v.Image
	if img == nil || img.Width <= 0 || img.Height <= 0 {
		v.figure.preparePaths(avatar.BoxSide)
		v.figure.Paint(v.c, v.p)

		return
	}
	// `background-size:cover;background-position:center`: the picture's shorter side
	// fills the box, the longer one runs over it and is cut at the box's edge
	sq := avatar.Square(img.Width, img.Height)
	k := avatar.BoxSide / float32(sq.Dx())
	v.imgDst = geom.XYWH(-float32(sq.Min.X)*k, -float32(sq.Min.Y)*k, float32(img.Width)*k, float32(img.Height)*k)
	v.p.Clip(geom.XYWH(0, 0, avatar.BoxSide, avatar.BoxSide), 0, v.drawImg)
}

// paintImage draws the picture in its place in the box.
func (v *CropView) paintImage() { v.p.Image(v.Image, v.imgDst, 1) }
