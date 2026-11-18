package nav

import (
	"time"

	"dancefitvn-app/internal/ui/kit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the screen frame (index.html:73-77).
const (
	// StatusBar is where a screen's content starts: `#m .scroll{top:54px}` (:74),
	// the height of the prototype's status bar `.sb{height:54px}` (:75).
	StatusBar float32 = 54
	// statusBarMax is the tallest safe-area top the 54 holds for: the iPhone 16's
	// is 59, an Android phone's 45.3 (measured, local/p2/safearea.md). A taller
	// one (a notch, a Dynamic Island on a bigger phone) pushes the content down to
	// it (Q-P2-1).
	statusBarMax float32 = 59
	// ScrollSide is `#m .scroll{padding:0 20px 120px}` (:74): the side padding.
	ScrollSide float32 = 20
	// ScrollBottom is its bottom padding, 120: the tab bar's 84 and 36 more. With
	// a deeper tab bar than 84 the padding grows by the difference.
	ScrollBottom float32 = 120
	// tabBarPrototype is the prototype's tab bar, `.tabs{height:84px}` (:79).
	tabBarPrototype float32 = 84
	// KeyboardGap is the room kept between the on-screen keyboard and the end of the
	// content while the keyboard shows. The prototype has no keyboard (a web page
	// scrolls itself); this is P6 section 4 decision 4: the bottom padding is the
	// larger of the usual and Keyboard().Bottom + KeyboardGap, so that the last
	// field can rise above it. Nothing changes while the keyboard is down.
	KeyboardGap float32 = 16
	// keyboardReveal is the time the keyboard takes to rise before the focused field
	// is scrolled into view (kit.Sheet's sheetKeyboardReveal, the same 250 ms).
	keyboardReveal = 250 * time.Millisecond
	// revealRoom is the room kept under a field revealed over the keyboard: the
	// text line of an Input is 19 high in a box of 54, so 24 shows the whole box
	// (with its 1.6 ring) above the keyboard.
	revealRoom float32 = 24
)

// Names layout.json gives the parts of a page.
const (
	namePage   = "div.scr"    // `.scr` (:73)
	nameScroll = "div.scroll" // `.scroll` (:74)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Page is the frame most screens stand in: `.scr` with its `.scroll` (:73-74),
// the scroll starting 54 DIPs from the top (more under a taller safe area),
// bottom 0, vertical only, with no bars, padded 0 20 120 (more over a deeper tab
// bar). An optional bar (the offline bar, `top:54px`, :132) sits on the top line
// and pushes the scroll down to 94 (:362). While the on-screen keyboard shows, the bottom
// padding grows to keep the end of the content above it (KeyboardGap) and the field
// that has the focus is scrolled above it 250 ms after the keyboard moved, as kit.Sheet
// does. The page implements Scroller (for Nav.Refresh) and Tabbed (the tab bar shows when Tab is 0..3).
//
// A page lays its content out 20 DIPs from each side, as tall as it wants; the
// scroll shows what does not fit. A flush page (NewPageFlush) lays it out as wide
// as the screen instead, for a content that spans the 20 DIP gutters itself (the
// strip of Home, `margin:0 -20px`): a child cannot reach into its parent's gutter,
// as hit testing clips a child to the frame of its parent.
type Page struct {
	ui.Base

	tab    int
	bar    ui.Widget
	scroll ui.Scroll
	body   pageBody

	kb      float32 // the keyboard's height the last layout saw
	kbTimer ui.Timer
	kbFn    func() // revealFocus as a method value, made once
}

// pageBody is what the scroll scrolls: the content with the padding around it.
type pageBody struct {
	ui.Base

	content ui.Widget
	bottom  float32 // ScrollBottom, or more over a deeper tab bar
	flush   bool    // the content is as wide as the screen, with no side padding
}

var (
	_ Scroller     = (*Page)(nil)
	_ Tabbed       = (*Page)(nil)
	_ ui.Unmounter = (*Page)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// ContentTop returns where a screen's content starts, in DIPs from the top of
// the window: 54, or the safe area's top when that is deeper than 59.
func ContentTop(c *ui.Context) float32 {
	if top := c.SafeArea().Top; top > statusBarMax {
		return top
	}

	return StatusBar
}

// TabBarHeight returns the height of the tab bar for the safe area now:
// `50 + max(34, SafeArea().Bottom)`, 84 on both test phones with the gesture
// bar.
func TabBarHeight(c *ui.Context) float32 { return kit.TabBarHeight(c.SafeArea().Bottom) }

// ScrollBottomPad returns the scroll's bottom padding for the safe area now: 120,
// and the tab bar's excess over 84 when it is deeper.
func ScrollBottomPad(c *ui.Context) float32 {
	return ScrollBottom + max(TabBarHeight(c)-tabBarPrototype, 0)
}

// NewPage returns a page around content. tab is the index of the tab it is the
// root of, which shows the tab bar with that tab current; -1 for a screen
// without the tab bar.
func NewPage(content ui.Widget, tab int) *Page {
	p := &Page{tab: tab}
	p.kbFn = p.revealFocus
	p.Name = namePage
	p.scroll.Name = nameScroll
	kit.PageScroll(&p.scroll)
	p.body.content = content
	p.body.Add(content)
	p.scroll.Add(&p.body)
	p.Add(&p.scroll)

	return p
}

// NewPageFlush returns a page like NewPage whose content is as wide as the screen:
// the side padding `padding:0 20px` is the content's to keep, or to take back
// (`margin:0 -20px`). The bottom padding is still the page's.
func NewPageFlush(content ui.Widget, tab int) *Page {
	p := NewPage(content, tab)
	p.body.flush = true

	return p
}

// SetContent replaces what the scroll scrolls; the scroll keeps its offset (the
// way a screen shows new data in place).
func (p *Page) SetContent(w ui.Widget) {
	if p.body.content == w {
		return
	}
	p.body.Remove(p.body.content)
	p.body.content = w
	p.body.Add(w)
	p.Relayout()
}

// SetBar puts w on the top line, 54 from the top, and the scroll under it; nil
// takes the bar away. The bar is laid out as wide as the page and as tall as it
// wants (the offline bar's 40).
func (p *Page) SetBar(w ui.Widget) {
	if p.bar == w {
		return
	}
	if p.bar != nil {
		p.Remove(p.bar)
	}
	p.bar = w
	if w != nil {
		p.Add(w)
		if p.View() == nil {
			// `.offbar` comes before `.scroll` in the DOM (index.html:362, :461) and
			// layout.json numbers by that order; they never overlap, so the paint
			// order does not matter. Only before the page is mounted: taking the
			// scroll out of a live tree would unmount its content.
			p.Remove(&p.scroll)
			p.Add(&p.scroll)
		}
	}
	p.Relayout()
}

// Scroll returns the page's scroll, for the Nav to keep its offset.
func (p *Page) Scroll() *ui.Scroll { return &p.scroll }

// Tab returns the tab the page is the root of, -1 for none.
func (p *Page) Tab() int { return p.tab }

// Layout fills the room: the bar at the top line, the scroll under it down to the
// bottom.
func (p *Page) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Max
	top := ContentTop(c)
	scrollTop := top
	if p.bar != nil {
		bs := c.Layout(p.bar, ui.Constraints{Min: geom.Sz(size.W, 0), Max: geom.Sz(size.W, ui.Unbounded)})
		c.Place(p.bar, geom.Pt(0, top))
		scrollTop = top + bs.H // `top:94px` (:362): 54 and the offline bar's 40
	}
	kb := c.Keyboard().Bottom
	p.body.bottom = ScrollBottomPad(c)
	p.scroll.RevealMargin = geom.Insets{}
	if kb > 0 {
		// the scroll still reaches the window's bottom, under the keyboard: the
		// content may scroll up by what the keyboard covers, and a field revealed
		// stops above it
		p.body.bottom = max(p.body.bottom, kb+KeyboardGap)
		p.scroll.RevealMargin = geom.Insets{Bottom: kb + revealRoom}
	}
	if kb != p.kb {
		p.kb = kb
		p.kbTimer.Stop()
		if kb > 0 {
			p.kbTimer = c.AfterFunc(keyboardReveal, p.kbFn)
		}
	}
	c.Layout(&p.scroll, ui.Tight(geom.Sz(size.W, max(size.H-scrollTop, 0))))
	c.Place(&p.scroll, geom.Pt(0, scrollTop))

	return size
}

// Unmounted stops the wait for the keyboard.
func (p *Page) Unmounted(*ui.Context) { p.kbTimer.Stop() }

// revealFocus scrolls the field that has the focus into view, once the keyboard
// has risen and the bottom padding has followed it.
func (p *Page) revealFocus() {
	if !p.FocusWithin() {
		return
	}
	if v := p.View(); v != nil {
		if f := v.Focused(); f != nil {
			f.Node().ScrollIntoView()
		}
	}
}

// Layout puts the content 20 from the sides (none when flush), with the bottom
// padding under it.
func (b *pageBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	side := ScrollSide
	if b.flush {
		side = 0
	}
	inner := max(w-2*side, 0)
	cz := c.Layout(b.content, ui.Constraints{Min: geom.Sz(inner, 0), Max: geom.Sz(inner, ui.Unbounded)})
	c.Place(b.content, geom.Pt(side, 0))

	return cs.Constrain(geom.Sz(w, cz.H+b.bottom))
}
