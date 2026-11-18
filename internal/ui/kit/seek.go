package kit

import (
	"dancefitvn-app/internal/format"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// SeekStep is the 10 s that a screen reader's increment and decrement move a
// position bar by: the 10 of `data-v="-10"` and `data-v="10"` (index.html:430).
const SeekStep = 10

// SeekTouchHeight is the height of the touch target of a position bar, 44 DIPs
// (the smallest a finger hits with ease): the bars are 18 and 40 DIPs tall as
// the prototype draws them (index.html:151, :164), and SeekPad catches the
// press over the rest without changing what is drawn or laid out (K38).
const SeekTouchHeight float32 = 44

// SeekBar is a position bar a finger drags: Scrub and Wave. The share is the
// place along the bar's width, 0 to 1. A drag begins at the press, follows the
// finger and ends at the release (commit: the bar asks for the position it
// stands at, once) or when the system takes the touch away (no commit: the bar
// goes back to where the player says it is). While it lasts the bar shows the
// drag's share, not the player's (SetPos only remembers).
type SeekBar interface {
	ui.Widget
	// SeekBegin, SeekMove and SeekEnd are the three phases above.
	SeekBegin(share float32)
	SeekMove(share float32)
	SeekEnd(commit bool)
	// Dragging tells whether a drag is under way.
	Dragging() bool
}

// BMarker is a bar with a second handle, the B end of the A-B band (K38): a press
// that BHit says takes it takes the pad instead of the playhead, and a drag of it
// only moves B (BEnd's commit asks once for the new B; nothing seeks).
type BMarker interface {
	// BHit tells whether a press at x, in the bar's coordinates, takes the marker.
	BHit(x float32) bool
	// BBegin, BMove and BEnd are the drag of the marker, as SeekBar's.
	BBegin(share float32)
	BMove(share float32)
	BEnd(commit bool)
}

// seekCore is the drag state and the callbacks that Scrub and Wave share.
type seekCore struct {
	// OnDrag is called with the share the finger is at, when the drag begins
	// and each time the share changes: the screen shows the time of it.
	OnDrag func(share float32)
	// OnSeek is called once with the share where the finger lets go, the one
	// request for a position: a tap is a drag that lets go where it began.
	OnSeek func(share float32)
	// OnDragEnd is called when the drag is over, after OnSeek when there was
	// one; moved tells whether the finger went anywhere (a tap did not).
	OnDragEnd func(moved bool)

	on, moved   bool
	share, from float32
}

// Dragging tells whether a finger is on the bar.
func (d *seekCore) Dragging() bool { return d.on }

// DragShare returns the share the finger is at, or 0 when none is.
func (d *seekCore) DragShare() float32 {
	if !d.on {
		return 0
	}

	return d.share
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// begin starts a drag at share.
func (d *seekCore) begin(share float32) {
	share = clamp01(share)
	d.on, d.moved, d.share, d.from = true, false, share, share
	if d.OnDrag != nil {
		d.OnDrag(share)
	}
}

// move follows the finger to share; it tells whether the share changed.
func (d *seekCore) move(share float32) bool {
	share = clamp01(share)
	if !d.on || share == d.share {
		return false
	}
	d.share, d.moved = share, d.moved || share != d.from
	if d.OnDrag != nil {
		d.OnDrag(share)
	}

	return true
}

// end finishes the drag and returns the share it stood at; the callbacks run
// with the drag already over, so that the bar shows what the player says.
func (d *seekCore) end(commit bool) (share float32, was bool) {
	if !d.on {
		return 0, false
	}
	share, moved := d.share, d.moved
	d.on, d.moved = false, false
	if commit && d.OnSeek != nil {
		d.OnSeek(share)
	}
	if d.OnDragEnd != nil {
		d.OnDragEnd(moved)
	}

	return share, true
}

// handleSeek is the pointer of a position bar: it takes the press at once, so
// that no scroll, sheet drag or edge swipe around the bar can take it (the
// range of jPlayer Mobile, kit/range.go, which grabs and has no slop), follows
// the finger along the width and lets go at the release. A touch the system
// cancels does not seek.
func handleSeek(e *ui.PointerEvent, bar SeekBar, width float32) {
	switch e.Kind {
	case ui.PointerDown:
		if e.Button != platform.ButtonPrimary && e.Button != platform.ButtonNone {
			return
		}
		e.Grab()
		if width > 0 {
			bar.SeekBegin(e.Pos.X / width)
		}
	case ui.PointerMove:
		if bar.Dragging() {
			e.Handle()
			bar.SeekMove(e.Pos.X / width)
		}
	case ui.PointerUp:
		if bar.Dragging() {
			e.Handle()
			bar.SeekMove(e.Pos.X / width) // where the finger lifted, if it moved since the last move
			bar.SeekEnd(true)
		}
	case ui.PointerCancel:
		bar.SeekEnd(false)
	case ui.PointerUnknown, ui.PointerEnter, ui.PointerLeave, ui.PointerScroll:
	default:
	}
}

// SeekPad is the touch target of a position bar: a widget with no drawing that
// lies over the bar, as wide, and at least SeekTouchHeight tall, centered on it.
// The bar keeps the size the prototype gives it (the layout, the layout.json and
// the picture do not change); the pad takes the presses on and around it. The
// screen adds it after the bar and before the controls that may lie under its
// ends (a button wins where they meet), and calls Place after placing the bar.
type SeekPad struct {
	ui.Base
	bar SeekBar
	// markB tells that the press under way took the bar's B marker.
	markB bool
}

var _ ui.PointerHandler = (*SeekPad)(nil)

// NewSeekPad returns the pad of bar.
func NewSeekPad(bar SeekBar) *SeekPad { return &SeekPad{bar: bar} }

// Place lays the pad out over the bar and puts it there. Call it after the bar
// was placed, in the same layout.
func (p *SeekPad) Place(c *ui.Context) {
	f := p.bar.Node().Frame()
	h := max(SeekTouchHeight, f.H)
	c.Layout(p, ui.Tight(geom.Sz(f.W, h)))
	c.Place(p, geom.Pt(f.X, f.Y-(h-f.H)/2))
}

// Layout is as large as it is given.
func (p *SeekPad) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(cs.Max)
}

// Hit tells whether the point, in the window's coordinates, is on the pad.
func (p *SeekPad) Hit(win geom.Point) bool { return p.Node().WindowFrame().Contains(win) }

// Begin starts a drag at x, in the pad's coordinates (the bar's too, they share
// the left edge); a screen that takes the press itself (the hidden controls of
// the video) feeds the pad with Begin, Move and End.
func (p *SeekPad) Begin(x float32) {
	if w := p.Frame().W; w > 0 {
		p.bar.SeekBegin(x / w)
	}
}

// Move follows the finger to x.
func (p *SeekPad) Move(x float32) {
	if w := p.Frame().W; w > 0 && p.bar.Dragging() {
		p.bar.SeekMove(x / w)
	}
}

// End lets go (commit) or gives the touch up.
func (p *SeekPad) End(commit bool) { p.bar.SeekEnd(commit) }

// HandlePointer is the press, the drag and the release on the pad: a press on the
// B marker of a bar that has one (BMarker) drags B, any other the playhead.
func (p *SeekPad) HandlePointer(_ *ui.Context, e *ui.PointerEvent) {
	w := p.Frame().W
	m, hasB := p.bar.(BMarker)
	if e.Kind == ui.PointerDown && hasB && w > 0 && m.BHit(e.Pos.X) && (e.Button == platform.ButtonPrimary || e.Button == platform.ButtonNone) {
		p.markB = true
		e.Grab()
		m.BBegin(e.Pos.X / w)

		return
	}
	if !p.markB {
		handleSeek(e, p.bar, w)

		return
	}
	switch e.Kind {
	case ui.PointerMove:
		e.Handle()
		m.BMove(e.Pos.X / w)
	case ui.PointerUp:
		e.Handle()
		p.markB = false
		m.BMove(e.Pos.X / w)
		m.BEnd(true)
	case ui.PointerCancel:
		p.markB = false
		m.BEnd(false)
	case ui.PointerUnknown, ui.PointerDown, ui.PointerEnter, ui.PointerLeave, ui.PointerScroll:
	default:
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// describeSeek describes a position bar as a slider named label, from 0 to the
// length of the media, at the position, which it speaks as "elapsed / total"
// ("01:12 / 03:40"). A bar with no reading is a slider at 0 of 0.
func describeSeek(n *platform.SemanticNode, label string, reading func() (pos, dur float64)) {
	n.Role, n.Label = platform.RoleSlider, label
	n.Actions |= platform.ActIncrement | platform.ActDecrement
	if reading == nil {
		return
	}
	pos, dur := reading()
	n.Min, n.Max, n.Now = 0, dur, pos
	n.Value = format.Clock(pos) + " / " + format.Clock(dur)
}

// performSeek is the SemanticActor of a position bar: an increment moves the
// position SeekStep s on, a decrement as many back, through step; it tells
// whether it took the action.
func performSeek(a platform.SemanticAction, step func(delta float64)) bool {
	if step == nil {
		return false
	}
	switch a {
	case platform.ActIncrement:
		step(SeekStep)
	case platform.ActDecrement:
		step(-SeekStep)
	default:
		return false
	}

	return true
}
