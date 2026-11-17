// jPlayer Mobile and TV have no avatar widget to copy (checked at 49de3c2 and
// 781bee5): this one is written from photo() of index.html:302-306.

package kit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of photo() (index.html:302-306) and the camera badge (:464).
const (
	// avatarRing is `box-shadow:0 0 0 3px <ring>`: a ring of 3 outside the
	// circle.
	avatarRing float32 = 3
	// avatarBox is the 100 unit box of the default person.
	avatarBox float32 = 100
	// avatarCam is the camera badge's size: `width:28px;height:28px`, with a
	// 2 px border, the camera at 15 with a stroke of 2 (`ic('cam',15,2)`).
	avatarCam, avatarCamBorder, avatarCamIcon, avatarCamStroke float32 = 28, 2, 15, 2
	// avatarCamRight, avatarCamBottom are `right:-4px;bottom:-2px`.
	avatarCamRight, avatarCamBottom float32 = -4, -2
)

// The colors of the default person (`photo()` :305) and of the photo behind a
// picture (`#ccc`, :304); literals of the SVG, not tokens.
var (
	personBg    = color.RGB(0xdc, 0xd8, 0xd2) // <circle r=50 fill>
	personShirt = color.RGB(0xd3, 0x20, 0x2a) // body path
	personSkin  = color.RGB(0xe3, 0xae, 0x8c) // neck rect
	personFace  = color.RGB(0xe8, 0xb8, 0x94) // face circle
	personHair  = color.RGB(0x2a, 0x1f, 0x1a) // hair path
	photoBack   = color.RGB(0xcc, 0xcc, 0xcc) // `#ccc url(...)`
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Avatar is photo(size, ring) (index.html:302-306): a round picture of Size DIPs
// with an optional ring of 3 outside it. With no Image it draws the default
// person; with one, the picture over #ccc, `background-size: Zoom*100%` centered
// and moved by (X, Y) of 200 units of its size (the crop of the avatar sheet),
// clipped to the circle. The ring is a shadow: it takes no room.
type Avatar struct {
	ui.Base
	// Image is the user's picture; nil draws the default person. It must stay
	// unchanged until replaced (scene.Image).
	Image *scene.Image
	// Size is the diameter; 0 reads as 44.
	Size float32
	// Ring is the color of the ring; the zero color has none.
	Ring color.Color
	// Zoom is the picture's scale, 1 to 3; 0 reads as 1. X and Y move it, in
	// units of 1/200 of Size.
	Zoom, X, Y float32
	// Square draws the figure over a square, with no ring and no circle: the
	// picture of the avatar sheet before a photo is picked, `photo(260)` with its
	// `border-radius:50%` taken out (index.html:521).
	Square bool

	body, hair, sash scene.Path // the person's paths for the size laid out
	pathsFor         float32
	person           func() // drawPerson as a method value: Clip takes it
	c                *ui.Context
	p                *ui.Painter
}

// AvatarButton is a pressable Avatar that is not a flex item, as the buttons of
// the prototype holding photo() are (`<button>${photo(44,...)}</button>`,
// :363-464): Size wide and Size + 4 tall, the 4 being the descent of the line the
// inline svg sits on (F-4 of local/plans/P2.md; `go.account` 44 x 48, `sheet:avatar`
// 76 x 80). With Camera it carries the white camera badge at the lower right.
// Set the Name to the data key.
type AvatarButton struct {
	ui.Pressable
	// Avatar is the picture.
	Avatar Avatar
	// Camera adds the 28 DIP camera badge, `right:-4px;bottom:-2px` of the
	// button (:464).
	Camera bool
	// Label names the button for assistive technologies.
	Label string

	cam      avatarCamera
	attached bool
}

// avatarCamera is the camera badge: a white disc with a 2 px #141518 border and
// the icon in #141518.
type avatarCamera struct {
	ui.Base

	icon Icon
}

var _ ui.Labeler = (*AvatarButton)(nil)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewAvatar returns the default person of size DIPs (0: 44) with a ring.
func NewAvatar(size float32, ring color.Color) *Avatar { return &Avatar{Size: size, Ring: ring} }

// NewAvatarButton returns a button of an avatar of size DIPs with a ring and
// the label of the button.
func NewAvatarButton(size float32, ring color.Color, label string) *AvatarButton {
	b := &AvatarButton{Label: label}
	b.Avatar.Size, b.Avatar.Ring = size, ring

	return b
}

// SetImage shows img in place of the person (nil: the person).
func (a *Avatar) SetImage(img *scene.Image) {
	if a.Image != img {
		a.Image = img
		a.Repaint()
	}
}

// radius is the corner radius of the picture: a circle, or none when Square.
func (a *Avatar) radius() float32 {
	if a.Square {
		return 0
	}

	return a.diameter() / 2
}

// diameter returns the size, 44 by default.
func (a *Avatar) diameter() float32 {
	if a.Size <= 0 {
		return 44
	}

	return a.Size
}

// Layout is a square of the size.
func (a *Avatar) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	d := a.diameter()
	a.preparePaths(d)

	return cs.Constrain(geom.Sz(d, d))
}

// Paint draws the ring, then the picture or the person clipped to the circle.
func (a *Avatar) Paint(c *ui.Context, p *ui.Painter) {
	d := a.diameter()
	if a.Ring.A > 0 && !a.Square {
		p.Fill(geom.XYWH(-avatarRing, -avatarRing, d+2*avatarRing, d+2*avatarRing), scene.Solid(a.Ring), d/2+avatarRing)
	}
	if a.person == nil {
		a.person = a.drawPerson
	}
	a.c, a.p = c, p
	p.Clip(geom.XYWH(0, 0, d, d), a.radius(), a.person)
	a.c, a.p = nil, nil
}

// drawPerson draws what fills the circle: the picture, or the default person.
func (a *Avatar) drawPerson() {
	p := a.p
	d := a.diameter()
	if img := a.Image; img != nil && img.Width > 0 && img.Height > 0 {
		p.Fill(geom.XYWH(0, 0, d, d), scene.Solid(photoBack), 0)
		zoom := a.Zoom
		if zoom <= 0 {
			zoom = 1
		}
		w := d * zoom
		h := w * float32(img.Height) / float32(img.Width)
		p.Image(img, geom.XYWH((d-w)/2+a.X*d/200, (d-h)/2+a.Y*d/200, w, h), 1)

		return
	}
	k := d / avatarBox
	p.Fill(geom.XYWH(0, 0, d, d), scene.Solid(personBg), a.radius())
	p.FillPath(&a.body, scene.Solid(personShirt), scene.NonZero)
	p.StrokePath(&a.sash, scene.Stroke{Width: 4 * k}, scene.Solid(color.White)) // <path d="M25 81 L75 75" stroke="#fff" stroke-width="4"/>
	p.Fill(geom.XYWH(43*k, 50*k, 14*k, 14*k), scene.Solid(personSkin), 0)
	p.Fill(geom.XYWH(35*k, 26*k, 30*k, 30*k), scene.Solid(personFace), 15*k)
	p.FillPath(&a.hair, scene.Solid(personHair), scene.NonZero)
}

// AccessibleLabel returns the button's label.
func (b *AvatarButton) AccessibleLabel() string { return b.Label }

// Layout makes the button its avatar's width and the avatar's height plus the
// strut, and puts the camera badge at the lower right.
func (b *AvatarButton) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !b.attached {
		b.attached = true
		b.Look = noLook
		b.cam.icon.Name = iconName
		b.cam.Add(&b.cam.icon)
		b.cam.Hidden = true
		b.Add(&b.Avatar, &b.cam)
	}
	d := b.Avatar.diameter()
	c.Layout(&b.Avatar, ui.Tight(geom.Sz(d, d)))
	c.Place(&b.Avatar, geom.Point{})
	size := cs.Constrain(geom.Sz(d, d+inlineStrut))
	b.cam.Hidden = !b.Camera
	if b.Camera {
		c.Layout(&b.cam, ui.Tight(geom.Sz(avatarCam, avatarCam)))
		c.Place(&b.cam, geom.Pt(size.W-avatarCamRight-avatarCam, size.H-avatarCamBottom-avatarCam))
	}

	return size
}

// Layout centers the camera in the badge.
func (a *avatarCamera) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(avatarCam, avatarCam))
	a.icon.Glyph, a.icon.Size, a.icon.Stroke, a.icon.Ink = icon.Cam, avatarCamIcon, avatarCamStroke, theme.InkOnPrimary
	is := c.Layout(&a.icon, ui.Tight(geom.Sz(avatarCamIcon, avatarCamIcon)))
	c.Place(&a.icon, geom.Pt(centeredY(size.W, is.W), centeredY(size.H, is.H)))

	return size
}

// Paint draws the white disc with its #141518 border (the value of --on).
func (a *avatarCamera) Paint(c *ui.Context, p *ui.Painter) {
	r := a.Bounds()
	p.Fill(r, scene.Solid(color.White), r.W/2)
	p.Border(r, geom.Uniform(avatarCamBorder), theme.From(c).T.OnPrimary, r.W/2)
	a.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// preparePaths builds the body and hair of the default person for a circle of d
// DIPs, unless they are those of the last call.
func (a *Avatar) preparePaths(d float32) {
	if a.pathsFor == d {
		return
	}
	a.pathsFor = d
	k := d / avatarBox
	a.body.Reset()
	// <path d="M3 105 C5 72 28 62 50 62 C72 62 95 72 97 105Z" fill="#D3202A"/>
	a.body.MoveTo(3*k, 105*k)
	a.body.CubicTo(5*k, 72*k, 28*k, 62*k, 50*k, 62*k)
	a.body.CubicTo(72*k, 62*k, 95*k, 72*k, 97*k, 105*k)
	a.body.Close()
	a.sash.Reset()
	a.sash.MoveTo(25*k, 81*k)
	a.sash.LineTo(75*k, 75*k)
	a.hair.Reset()
	// <path d="M34 40 C33 22 66 20 66 38 C56 31 44 32 34 40Z" fill="#2A1F1A"/>
	a.hair.MoveTo(34*k, 40*k)
	a.hair.CubicTo(33*k, 22*k, 66*k, 20*k, 66*k, 38*k)
	a.hair.CubicTo(56*k, 31*k, 44*k, 32*k, 34*k, 40*k)
	a.hair.Close()
}
