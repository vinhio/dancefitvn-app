// Adapted from jplayer-mobile/internal/ui/sheets/{host,entry,sheet}.go at
// 49de3c2 (the in-tree host, the dim that closes it, the panel that follows a
// finger down, the keyboard padding); its frost, its action sheets and its
// dialogs are not in the prototype.

package kit

import (
	"time"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit/gesture"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of .sheet (index.html:124-129).
const (
	// sheetRadius is `.sheet{border-radius:24px 24px 0 0}` (:125).
	sheetRadius float32 = 24
	// sheetPadTop, sheetPadSide are `.sheet{padding:14px 20px 40px}` (:125);
	// sheetPadBottom is its 40.
	sheetPadTop, sheetPadSide, sheetPadBottom float32 = 14, 20, 40
	// sheetMaxShare is `.sheet{max-height:92%}` (:125): of the screen.
	sheetMaxShare float32 = .92
	// grabW, grabH, grabRadius, grabGap are `.sheet .grab{width:36px;height:5px;
	// border-radius:3px;margin:0 auto 14px}` (:126).
	grabW, grabH, grabRadius, grabGap float32 = 36, 5, 3, 14
	// grabTop is where the content of a sheet starts: padding, grab, margin.
	grabTop = sheetPadTop + grabH + grabGap
	// SheetHeaderGap is `.sheet .sh{margin-bottom:16px}` (:127): the room a
	// sheet's content leaves under the header.
	SheetHeaderGap float32 = 16
	// sheetHeaderH is the height of `.sh`: its tallest part, the 32 round
	// button of `style="width:32px;height:32px"` (:506).
	sheetHeaderH float32 = 32
	// sheetTitleSize, sheetTitleSpacing are `.sheet .sh .d{font-size:22px;
	// letter-spacing:1px}` (:128).
	sheetTitleSize, sheetTitleSpacing float32 = 22, 1
	// sheetCloseIcon is `ic('x',18)` in the close button (:506).
	sheetCloseIcon float32 = 18
	// sheetKeyboardReveal is the time the keyboard takes to rise before the
	// focused field is scrolled into view (jPlayer Mobile KM19).
	sheetKeyboardReveal = 250 * time.Millisecond
)

// The names layout.json gives the parts of a sheet.
const (
	nameSheetDim    = "closeSheet" // `.dim-bg` data-act (:547)
	nameSheetPanel  = "div.sheet"
	nameSheetGrab   = "div.grab"
	nameSheetHeader = "div.sh"
	nameSheetTitle  = "span.d"
)

// SheetOptions are what a sheet is opened with.
type SheetOptions struct {
	// OnClose is called once when the sheet closes, however it closes: a
	// button of its content, the dim, a pull down, Back.
	OnClose func()
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Sheet is the sheet host: the `.dim-bg` and the `.sheet` of renderM
// (index.html:547) as one overlay as large as its room, hidden while no sheet is
// open so that it takes no touch (GM28). One sheet is open at a time, as in the
// prototype (`S.m.sheet`). Put it over the screen and under the Toast.
//
// Opening fades the dim in over 200 ms (CSS `ease`) and slides the panel up
// over 250 ms (`ease-out`); closing is at once, with no animation, and what
// happens inside an open sheet replays nothing (Q-P2-3, K17). The panel is
// --sf with its top corners rounded 24, padded 14 20 40, at most 92 % of the
// screen high and scrolling inside, with a 36 x 5 grab. A tap on the dim, Back
// (Back) and a pull down by 80 DIPs or faster than 0.6 DIP/ms close it. With
// the on-screen keyboard up the bottom padding grows to the keyboard's height
// (Q-P2-7).
type Sheet struct {
	ui.Base

	dim    sheetDim
	panel  sheetPanel
	opts   SheetOptions
	isOpen bool
	gen    uint32 // counts the sheets opened, for handles

	// scrim and pos are the animations of opening: the dim's opacity and how
	// far the panel is from its place as a share of its height (1 below the
	// screen, 0 at rest).
	scrim, pos ui.Animated[float32]
}

// SheetHandle is an open sheet, to close it from outside. It stands for the
// sheet it was returned for: once that has closed it does nothing.
type SheetHandle struct {
	s   *Sheet
	gen uint32
}

// sheetDim is `.dim-bg` (index.html:124): the scrim over the screen, whose tap
// closes the sheet.
type sheetDim struct {
	ui.Base

	s    *Sheet
	down bool // the press that is down began on the dim
}

var _ ui.PointerHandler = (*sheetDim)(nil)

// sheetPanel is `.sheet`: the grab and the content, scrolling inside. It is a
// PointerInterceptor: a press that the content's scroll leaves alone (it is at
// its top), and that is not on a text field, follows the finger down.
type sheetPanel struct {
	ui.Base

	s      *Sheet
	scroll ui.Scroll
	body   sheetBody

	drag     gesture.Drag
	armed    bool // a press that may become the drag is down
	dragging bool // the panel follows the finger

	kb      float32 // the keyboard's height the last layout saw
	kbTimer ui.Timer
	kbFn    func()
}

var (
	_ ui.PointerInterceptor = (*sheetPanel)(nil)
	_ ui.PointerHandler     = (*sheetPanel)(nil)
)

// sheetBody is what the panel scrolls: the grab, then the content 20 in from
// each side, then the bottom padding.
type sheetBody struct {
	ui.Base

	grab    sheetGrab
	content ui.Widget
	bottom  float32 // 40, or the keyboard's height when higher
}

// sheetGrab is `.sheet .grab`: the 36 x 5 bar.
type sheetGrab struct {
	ui.Base
}

// SheetHeader is `.sh` (index.html:127-128, :506): the title in Barlow
// Condensed 22 on the left, the 32 round close button on the right, 32 tall.
// Leave SheetHeaderGap under it.
type SheetHeader struct {
	ui.Base
	// Title is the sheet's title, upper-cased.
	Title string
	// Close is the close button; set OnPress on it, or use NewSheetHeader.
	Close RoundButton

	title    Label
	attached bool
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewSheet returns the host with no sheet open.
func NewSheet() *Sheet {
	s := &Sheet{}
	s.Hidden = true
	s.dim.s, s.dim.Name = s, nameSheetDim
	s.panel.s, s.panel.Name = s, nameSheetPanel
	s.panel.drag = gesture.SheetDrag()
	s.panel.kbFn = s.panel.revealFocus
	s.panel.scroll.Axes, s.panel.scroll.HideBars = ui.ScrollVertical, true
	s.panel.scroll.Add(&s.panel.body)
	s.panel.Add(&s.panel.scroll)
	s.panel.body.grab.Name = nameSheetGrab
	s.panel.body.Add(&s.panel.body.grab)
	s.pos.Resizes = true
	s.Add(&s.dim, &s.panel)

	return s
}

// Open shows content in a sheet and returns its handle. Opening while a sheet
// is open replaces its content at once, with no animation, and tells the
// replaced sheet's OnClose.
func (s *Sheet) Open(content ui.Widget, o SheetOptions) *SheetHandle {
	if s.isOpen {
		s.release()
	}
	s.gen++
	s.opts = o
	s.panel.body.setContent(content)
	s.panel.scroll.ScrollTo(geom.Point{}, false)
	if !s.isOpen {
		s.isOpen = true
		s.Hidden = false
		s.begin()
	}
	s.Relayout()
	s.screenChanged()

	return &SheetHandle{s: s, gen: s.gen}
}

// Close closes the open sheet at once and tells its OnClose; with none open it
// does nothing.
func (s *Sheet) Close() {
	if !s.isOpen {
		return
	}
	if s.panel.FocusWithin() {
		if v := s.View(); v != nil {
			v.Focus(nil) // the keyboard goes down with the sheet
		}
	}
	onClose := s.opts.OnClose
	s.isOpen, s.Hidden = false, true
	s.opts = SheetOptions{}
	s.panel.reset()
	s.scrim.Set(0)
	s.pos.Set(1)
	s.panel.body.setContent(nil)
	s.screenChanged()
	if onClose != nil {
		onClose()
	}
}

// Back closes the open sheet and tells whether there was one: the first step
// of Back (docs/plan/03 section 1).
func (s *Sheet) Back() bool {
	if !s.isOpen {
		return false
	}
	s.Close()

	return true
}

// IsOpen tells whether a sheet is open.
func (s *Sheet) IsOpen() bool { return s.isOpen }

// Top returns the handle of the open sheet, nil when there is none.
func (s *Sheet) Top() *SheetHandle {
	if !s.isOpen {
		return nil
	}

	return &SheetHandle{s: s, gen: s.gen}
}

// Content returns the open sheet's content, nil when none is open.
func (s *Sheet) Content() ui.Widget { return s.panel.body.content }

// Close closes the sheet this handle stands for, if it is still the open one.
func (h *SheetHandle) Close() {
	if h != nil && h.s != nil && h.s.isOpen && h.s.gen == h.gen {
		h.s.Close()
	}
}

// Closed tells whether the sheet has closed.
func (h *SheetHandle) Closed() bool {
	return h == nil || h.s == nil || !h.s.isOpen || h.s.gen != h.gen
}

// Layout gives the dim the whole room and puts the panel at the bottom, moved
// down by its slide.
func (s *Sheet) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	room := cs.Max
	c.Layout(&s.dim, ui.Tight(room))
	c.Place(&s.dim, geom.Point{})
	ps := c.Layout(&s.panel, ui.Loose(room))
	c.Place(&s.panel, geom.Pt(0, room.H-ps.H+s.pos.Value()*ps.H))

	return room
}

// Layout makes the header as wide as it gets, 32 tall, with the title at the
// start and the close button at the end, each centered on the row.
func (h *SheetHeader) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !h.attached {
		h.attached = true
		h.Name = nameSheetHeader
		h.title.Name = nameSheetTitle
		h.Close.Name, h.Close.Size, h.Close.IconSize, h.Close.Icon = nameSheetDim, sheetHeaderH, sheetCloseIcon, icon.X
		h.title.Heading = 1 // the sheet is a screen of its own, as in jPlayer Mobile
		h.Add(&h.title, &h.Close)
	}
	h.Close.Bg = theme.From(c).T.Surface2
	labelStyle{Role: theme.RoleDisplay, Size: sheetTitleSize, Weight: 700, LineHeight: 1, Spacing: sheetTitleSpacing,
		Upper: true, Ink: theme.InkText, Wrap: true}.apply(&h.title)
	h.title.SetText(h.Title)
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = 0
	}
	cb := c.Layout(&h.Close, ui.Tight(geom.Sz(sheetHeaderH, sheetHeaderH)))
	room := max(w-cb.W-rowGap, 0)
	ts := c.Layout(&h.title, ui.Loose(geom.Sz(room, ui.Unbounded)))
	hh := max(sheetHeaderH, ts.H)
	size := cs.Constrain(geom.Sz(w, hh))
	c.Place(&h.title, geom.Pt(0, centeredY(hh, ts.H)))
	c.Place(&h.Close, geom.Pt(size.W-cb.W, centeredY(hh, cb.H)))

	return size
}

// NewSheetHeader returns a header with the title and the close button's label
// and action.
func NewSheetHeader(title, closeLabel string, onClose func()) *SheetHeader {
	h := &SheetHeader{Title: title}
	h.Close.Label, h.Close.OnPress = closeLabel, onClose

	return h
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// begin starts the dim fading in and the panel sliding up.
func (s *Sheet) begin() {
	s.scrim.Set(0)
	s.pos.Set(1)
	s.scrim.Duration, s.scrim.Ease = theme.FadeIn, theme.CSSEase
	s.pos.Duration, s.pos.Ease = theme.SheetIn, ui.EaseOut
	s.scrim.To(s, 1)
	s.pos.To(s, 0)
}

// release tells the open sheet's OnClose without closing the host: the content
// is being replaced.
func (s *Sheet) release() {
	onClose := s.opts.OnClose
	s.opts = SheetOptions{}
	s.panel.reset()
	if onClose != nil {
		onClose()
	}
}

// screenChanged tells a screen reader that a panel came or went over the app.
func (s *Sheet) screenChanged() {
	if v := s.View(); v != nil {
		v.Context().SemanticsChanged(platform.ChangeScreen)
	}
}

// dimColor returns the scrim's color at the opacity of its fade-in.
func dimColor(c color.Color, k float32) color.Color {
	c.A = uint8(float32(c.A)*min(max(k, 0), 1) + .5)

	return c
}

// HandlePointer swallows every touch on the dim and closes the sheet when a
// press and its release both lie on it (`data-act="closeSheet"`, :547).
func (d *sheetDim) HandlePointer(_ *ui.Context, e *ui.PointerEvent) {
	switch e.Kind {
	case ui.PointerDown:
		e.Handle()
		d.down = true
	case ui.PointerMove:
		e.Handle()
	case ui.PointerUp:
		e.Handle()
		down := d.down
		d.down = false
		if down && e.Inside {
			d.s.Close()
		}
	case ui.PointerCancel:
		d.down = false
	case ui.PointerUnknown, ui.PointerEnter, ui.PointerLeave, ui.PointerScroll:
	default:
	}
}

// Paint fills the room with the scrim, `rgba(0,0,0,.55)`, at the fade-in's
// opacity.
func (d *sheetDim) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	p.Fill(d.Bounds(), scene.Solid(dimColor(look.T.Scrim, d.s.scrim.Value())), 0)
}

// Layout sizes the panel to its content, at most 92 % of the room, and keeps
// the bottom padding above the on-screen keyboard.
func (p *sheetPanel) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w, room := cs.Max.W, cs.Max.H
	kb := c.Keyboard().Bottom
	p.body.bottom = max(sheetPadBottom, kb)
	natural := c.Layout(&p.body, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}).H
	h := min(natural, room*sheetMaxShare)
	c.Layout(&p.scroll, ui.Tight(geom.Sz(w, h)))
	c.Place(&p.scroll, geom.Point{})
	if kb != p.kb {
		p.kb = kb
		p.kbTimer.Stop()
		p.kbTimer = c.AfterFunc(sheetKeyboardReveal, p.kbFn)
	}

	return geom.Sz(w, h)
}

// Paint draws the surface, rounded 24 at the top, under the content.
func (p *sheetPanel) Paint(c *ui.Context, pt *ui.Painter) {
	look := theme.From(c)
	pt.FillShape(scene.Shape{Rect: p.Bounds(), Radii: scene.Radii{sheetRadius, sheetRadius, 0, 0}}, scene.Solid(look.T.Surface))
	p.Base.Paint(c, pt)
}

// InterceptPointer notes a press that may become the pull down: one the
// content's scroll is at its top for and that is not on a text field. The
// scroll inside is asked first (innermost first) and takes a drag that goes its
// way; the panel takes the pull it has no room for, once the finger has moved 8
// DIPs and mostly down or up.
func (p *sheetPanel) InterceptPointer(c *ui.Context, ev *ui.PointerEvent) bool {
	switch ev.Kind {
	case ui.PointerDown:
		p.armed, p.dragging = false, false
		if !p.s.isOpen || (ev.Button != platform.ButtonPrimary && ev.Button != platform.ButtonNone) {
			return false
		}
		at := ev.Pos.Add(p.WindowFrame().Min())
		if p.scroll.Offset().Y <= 0 && !gesture.Exempted(&p.scroll, at) {
			p.drag.PressAt(at, ev.Time, c.Now())
			p.armed = true
		}
	case ui.PointerMove:
		if !p.armed || p.dragging {
			return false
		}
		switch p.drag.MoveAt(ev.Pos.Add(p.WindowFrame().Min()), ev.Time, c.Now()) {
		case gesture.StepStart:
			p.dragging = true
			p.follow()

			return true
		case gesture.StepRefuse:
			p.armed = false
		case gesture.StepUnknown, gesture.StepWait, gesture.StepMove:
		default:
		}
	case ui.PointerUnknown, ui.PointerUp, ui.PointerEnter, ui.PointerLeave, ui.PointerCancel, ui.PointerScroll:
	default:
	}

	return false
}

// HandlePointer holds a press nothing else held, so that its moves reach
// InterceptPointer; follows the finger of a drag the panel took; and ends it.
func (p *sheetPanel) HandlePointer(c *ui.Context, ev *ui.PointerEvent) {
	switch ev.Kind {
	case ui.PointerDown:
		if p.armed {
			ev.Handle()
		}
	case ui.PointerMove:
		if p.dragging {
			ev.Handle()
			p.drag.MoveAt(ev.Pos.Add(p.WindowFrame().Min()), ev.Time, c.Now())
			p.follow()
		}
	case ui.PointerUp:
		if p.dragging {
			ev.Handle()
			p.release(c, ev)
		}
		p.armed, p.dragging = false, false
		p.drag.Cancel()
	case ui.PointerCancel:
		if p.dragging {
			p.snapBack()
		}
		p.armed, p.dragging = false, false
		p.drag.Cancel()
	case ui.PointerUnknown, ui.PointerEnter, ui.PointerLeave, ui.PointerScroll:
	default:
	}
}

// follow puts the panel where the drag has it, with no transition.
func (p *sheetPanel) follow() {
	if h := p.Frame().H; h > 0 {
		p.s.pos.Set(p.drag.Offset() / h)
		p.s.Relayout()
	}
}

// release ends the drag: the sheet closes when pulled down more than 80 DIPs or
// let go faster than 0.6 DIP/ms; else it slides back.
func (p *sheetPanel) release(c *ui.Context, ev *ui.PointerEvent) {
	if p.drag.ReleaseAt(ev.Pos.Add(p.WindowFrame().Min()), ev.Time, c.Now()) {
		p.s.Close()

		return
	}
	p.snapBack()
}

// snapBack slides the panel from where it is back to its place.
func (p *sheetPanel) snapBack() {
	p.s.pos.Duration, p.s.pos.Ease = theme.SheetIn, ui.EaseOut
	p.s.pos.To(p.s, 0)
}

// reset forgets a drag under way.
func (p *sheetPanel) reset() {
	p.armed, p.dragging = false, false
	p.drag.Cancel()
	p.kbTimer.Stop()
}

// revealFocus scrolls the field that has the focus into view once the bottom
// padding has followed the keyboard.
func (p *sheetPanel) revealFocus() {
	if !p.FocusWithin() {
		return
	}
	if v := p.View(); v != nil {
		if f := v.Focused(); f != nil {
			f.Node().ScrollIntoView()
		}
	}
}

// setContent replaces the content of the body.
func (b *sheetBody) setContent(w ui.Widget) {
	if b.content == w {
		return
	}
	if b.content != nil {
		b.Remove(b.content)
	}
	b.content = w
	if w != nil {
		b.Add(w)
	}
	b.Relayout()
}

// Layout puts the grab at the top, the content 20 in from the sides under it,
// and the bottom padding after.
func (b *sheetBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	c.Layout(&b.grab, ui.Tight(geom.Sz(grabW, grabH)))
	c.Place(&b.grab, geom.Pt((w-grabW)/2, sheetPadTop))
	h := grabTop
	if b.content != nil {
		inner := max(w-2*sheetPadSide, 0)
		cs2 := c.Layout(b.content, ui.Constraints{Min: geom.Sz(inner, 0), Max: geom.Sz(inner, ui.Unbounded)})
		c.Place(b.content, geom.Pt(sheetPadSide, grabTop))
		h += cs2.H
	}

	return cs.Constrain(geom.Sz(w, h+b.bottom))
}

// Paint draws the bar in --tx3 (`background:var(--tx3)`).
func (g *sheetGrab) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(g.Bounds(), scene.Solid(theme.From(c).T.Text3), grabRadius)
}
