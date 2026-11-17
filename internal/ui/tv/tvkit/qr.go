//go:build tvos

package tvkit

import (
	"math"

	"dancefitvn-app/internal/qr"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the QR box of TS.code (index.html:656, K1).
const (
	// QRArea is `qrSvg(300)`: the side of the symbol's square.
	QRArea float32 = 300
	// QRPad is `padding:20px` of the white box.
	QRPad float32 = 20
	// QRRadius is `border-radius:20px`.
	QRRadius float32 = 20
	// QRSide is the white box's width: `width:340px` of the column, which is the
	// symbol and its padding.
	QRSide = QRArea + 2*QRPad
	// QRDescent is what the box has below the symbol more than the padding: the
	// `<svg>` is inline, so its line box reaches below the baseline by the
	// strut's descent (Inter 16 px: 4 px), and the box is 344 high, not 340
	// (the next block of the capture, `p.muted#3`, is at y 678).
	QRDescent float32 = 4
)

// QR is the white box of TS.code holding the QR symbol of the pairing address
// (K1: a real symbol, where the prototype draws 25 by 25 random squares). The
// symbol is drawn with a module of floor(300 / size) DIPs, a whole number, in
// the 300 square of the box, centered there (on a whole DIP); the modules are --bg (`fill="#141518"`)
// on #fff. The modules are one path made when the code is set, so a repaint
// draws it and allocates nothing.
//
// The box is 340 wide and 344 high (QRDescent). With no code it is the empty
// white box.
type QR struct {
	ui.Base
	// Decor: the symbol is the address "dancefitvn.com/tv", which the line under
	// it says in words ("Mở dancefitvn.com/mobile/tv trên điện thoại").
	kit.Decor

	code *qr.Code
	path scene.Path
	mod  float32 // the module in DIPs; 0 with no code
	org  geom.Point
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewQR returns the empty box.
func NewQR() *QR { return &QR{} }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Code returns the symbol drawn, nil when there is none.
func (q *QR) Code() *qr.Code { return q.code }

// Module returns the side of a module in DIPs, 0 with no symbol.
func (q *QR) Module() float32 { return q.mod }

// Origin returns where the symbol's top-left module is, in the box's own
// coordinates.
func (q *QR) Origin() geom.Point { return q.org }

// Set shows code; nil empties the box. The same code changes nothing. The
// code must not be changed after.
func (q *QR) Set(code *qr.Code) {
	if code == q.code {
		return
	}
	q.code = code
	q.path.Reset()
	q.mod = 0
	if code != nil && code.Size() > 0 {
		q.build(code)
	}
	q.Repaint()
}

// Layout makes the box: 340 by 344.
func (q *QR) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Sz(QRSide, QRSide+QRDescent))
}

// Paint draws the white box and the modules.
func (q *QR) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	p.Fill(q.Bounds(), scene.Solid(theme.InkWhite.Of(look)), QRRadius)
	if q.mod > 0 {
		p.FillPath(&q.path, scene.Solid(look.T.Bg), scene.NonZero)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// build makes the path of the dark modules: each run of dark modules in a row
// is one rectangle.
func (q *QR) build(code *qr.Code) {
	n := code.Size()
	q.mod = float32(int(QRArea) / n)
	if q.mod < 1 { // a symbol too large for the box is drawn at one DIP a module
		q.mod = 1
	}
	side := q.mod * float32(n)
	// centered, on a whole DIP so that no module edge is blurred at scale 1
	off := QRPad + float32(math.Floor(float64(QRArea-side)/2))
	q.org = geom.Pt(off, off)
	for y := range n {
		for x := 0; x < n; x++ {
			if !code.Black(x, y) {
				continue
			}
			start := x
			for x+1 < n && code.Black(x+1, y) {
				x++
			}
			q.path.Rect(geom.XYWH(q.org.X+float32(start)*q.mod, q.org.Y+float32(y)*q.mod, float32(x-start+1)*q.mod, q.mod))
		}
	}
}
