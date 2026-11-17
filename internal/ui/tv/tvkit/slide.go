//go:build tvos

package tvkit

import (
	"time"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of scrollTV (index.html:729) and of the two lists it slides.
const (
	// SlideTime is `transition:transform .25s` of `#tvGrid` and `#tvList`
	// (:674, :678): CSS `ease`.
	SlideTime = 250 * time.Millisecond
	// SlideMargin is the `- 80` of `lim=g.parentElement.clientHeight-
	// el.offsetHeight-80` (:729).
	SlideMargin float32 = 80
)

// Slide is the clip of scrollTV (index.html:729): a rectangle of `overflow:
// hidden` holding an optional heading and the content that slides, `#tvGrid`
// or `#tvList`. It is not a ui.Scroll: the offset is not clamped, it follows
// the focus alone, and it moves over the heading.
//
// When the focus goes to an item inside Content the offset becomes
// max(0, top - (clipHeight - itemHeight - 80)), top being the item's top
// relative to the clip with the content unshifted (`el.offsetTop`). The
// content is placed at that offset at once, so that the geometry the focus
// picks by is final, and painted gliding there over 250 ms. A focus that goes
// outside Content leaves the offset where it was, as the prototype does; a
// content that is replaced (SetContent: the prototype re-renders the DOM) starts
// at the offset of the focused item with no glide, 0 when the focus is
// elsewhere.
//
// Put it in a parent that gives it its size (tight); it lays the heading out
// at the top and the content below it, as wide as itself and as tall as it
// needs.
type Slide struct {
	ui.Base

	head    ui.Widget
	content ui.Widget

	off    ui.Animated[float32] // the offset the content is shown at; it glides to target
	target float32              // the offset the content is placed at, 0 or more
	fresh  bool                 // the next layout sets the offset without a glide
	headH  float32              // the heading's height, where the content starts
	stop   func()

	// inner, drawContent, c and p paint the clip's content: made once, so that
	// a frame allocates nothing.
	inner       func()
	drawContent func()
	c           *ui.Context
	p           *ui.Painter
}

var (
	_ ui.Mounter   = (*Slide)(nil)
	_ ui.Unmounter = (*Slide)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewSlide returns a Slide of head (which may be nil) and content.
func NewSlide(head, content ui.Widget) *Slide {
	s := &Slide{fresh: true}
	s.off.Ease, s.off.Duration = theme.CSSEase, SlideTime
	s.inner, s.drawContent = s.paintInner, s.paintContent
	if head != nil {
		s.head = head
		s.Add(head)
	}
	if content != nil {
		s.content = content
		s.Add(content)
	}

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Content returns the sliding content.
func (s *Slide) Content() ui.Widget { return s.content }

// Offset returns where the content is placed: 0 or more DIPs up.
func (s *Slide) Offset() float32 { return s.target }

// Shown returns where the content is painted now, between the offset it left
// and Offset while it glides.
func (s *Slide) Shown() float32 { return s.off.Value() }

// Gliding tells whether the content is on its way to Offset.
func (s *Slide) Gliding() bool { return s.off.Running() }

// SetContent replaces the sliding content with w, as a re-render does: the
// new content has no glide, it starts where the focused item puts it, or at 0
// when the focus is outside it.
func (s *Slide) SetContent(w ui.Widget) {
	if s.content != nil {
		s.Remove(s.content)
	}
	s.content = w
	s.target = 0
	s.fresh = true
	if w != nil {
		s.Add(w)
	}
	s.Relayout()
}

// Mounted starts following the focus.
func (s *Slide) Mounted(*ui.Context) {
	if v := s.View(); v != nil && s.stop == nil {
		s.stop = v.OnFocusChange(s.focusChanged)
	}
}

// Unmounted stops following it.
func (s *Slide) Unmounted(*ui.Context) {
	if s.stop != nil {
		s.stop()
		s.stop = nil
	}
}

// Layout fills the room it is given, lays the heading at the top and the
// content below it at the offset.
func (s *Slide) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	s.headH = 0
	if s.head != nil {
		hs := c.Layout(s.head, ui.Loose(geom.Sz(size.W, ui.Unbounded)))
		c.Place(s.head, geom.Point{})
		s.headH = hs.H
	}
	if s.content == nil {
		return size
	}
	c.Layout(s.content, ui.Constraints{Min: geom.Sz(size.W, 0), Max: geom.Sz(size.W, ui.Unbounded)})
	if s.fresh {
		s.fresh = false
		s.target = 0
		c.Place(s.content, geom.Pt(0, s.headH))
		if f := s.focusedInside(); f != nil {
			s.target = s.targetFor(f, size.H)
		}
		s.off.Set(s.target)
	}
	c.Place(s.content, geom.Pt(0, s.headH-s.target))

	return size
}

// Paint clips to the Slide's box and draws the heading and the content, the
// content moved by what is left of the glide.
func (s *Slide) Paint(c *ui.Context, p *ui.Painter) {
	s.c, s.p = c, p
	p.Clip(s.Bounds(), 0, s.inner)
	s.c, s.p = nil, nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// paintInner draws what the clip shows.
func (s *Slide) paintInner() {
	if s.head != nil {
		s.p.Child(s.c, s.head)
	}
	if s.content == nil {
		return
	}
	if d := s.target - s.off.Value(); d != 0 {
		s.p.Transform(scene.Translate(geom.Pt(0, d)), s.drawContent)

		return
	}
	s.p.Child(s.c, s.content)
}

// paintContent is the content, drawn under the glide's translation.
func (s *Slide) paintContent() { s.p.Child(s.c, s.content) }

// focusChanged follows the focus into the content: scrollTV(best).
func (s *Slide) focusChanged(_, now ui.Widget) {
	if now == nil || s.content == nil || !inside(now, s.content) {
		return
	}
	t := s.targetFor(now, s.Frame().H)
	if t == s.target {
		return
	}
	s.target = t
	if v := s.View(); v != nil {
		// the geometry the focus picks by is final at once
		v.Context().Place(s.content, geom.Pt(0, s.headH-t))
	}
	s.off.To(s, t)
	s.Repaint()
}

// targetFor is `Math.max(0, top - lim)` for the focused item f: top is f's top
// relative to the clip with the content unshifted, lim the clip's height
// clipH less f's and 80.
func (s *Slide) targetFor(f ui.Widget, clipH float32) float32 {
	top := f.Node().WindowFrame().Y - s.WindowFrame().Y + s.target
	lim := clipH - f.Node().Frame().H - SlideMargin

	return max(0, top-lim)
}

// focusedInside returns the focused widget when it is inside the content.
func (s *Slide) focusedInside() ui.Widget {
	v := s.View()
	if v == nil || s.content == nil {
		return nil
	}
	if f := v.Focused(); f != nil && inside(f, s.content) {
		return f
	}

	return nil
}

// inside tells whether w is root or one of its descendants.
func inside(w, root ui.Widget) bool {
	for ; w != nil; w = w.Node().Parent() {
		if w == root {
			return true
		}
	}

	return false
}
