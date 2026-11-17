// Adapted from jplayer-mobile/internal/ui/kit/label.go at 49de3c2 (the CSS
// line box, letter-spacing on a copied layout, wrapping, the ellipsis), with
// Chrome's rounded line box of jplayer-tv/internal/ui/theme/lineheight.go at
// 781bee5 and Space of jplayer-tv/internal/ui/kit/label.go at 781bee5.

package kit

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The defaults of a Label, from the page's own text.
const (
	// defaultLabelSize is the browser's 16 px: `html,body` set no size
	// (index.html:12-15) and layout.json reports fs=16px on every unstyled box.
	defaultLabelSize float32 = 16
	// ellipsis is what text-overflow: ellipsis ends a cut line with.
	ellipsis = "…"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Label is text with the metrics CSS gives it. Its line box is the one Chrome
// makes: LineHeight times Size tall, or, with LineHeight 0 (`line-height:
// normal`), the font's ascent and descent each rounded to whole pixels
// ([theme.Chrome]); the baseline sits where Chrome puts it; and Spacing (in
// px, as the prototype writes it) is added after every character, the last one
// too, as Chrome does. Its color is an Ink read when it paints.
//
// Letter-spacing is done here, not in gDesk: the glyphs of the shaper's layout
// are copied once per layout, each cluster moved by its share, and drawn from
// the copy; the copy's memory is reused, so a steady frame allocates nothing.
type Label struct {
	ui.Base
	// Text is the string shown. Call SetText to change it.
	Text string
	// Role picks the font set: theme.RoleBody, RoleDisplay, RoleMono. The
	// zero value reads as RoleBody.
	Role theme.Role
	// Size is the font size in CSS px (= DIPs); 0 reads as 16.
	Size float32
	// Weight is the CSS font-weight; 0 reads as 400.
	Weight text.Weight
	// LineHeight is the CSS line-height as a multiple of Size (`line-height:1`
	// is 1, `.95` is .95); 0 is `line-height: normal`.
	LineHeight float32
	// Spacing is the CSS letter-spacing in px (index.html writes px: 1.4px,
	// 8px).
	Spacing float32
	// Upper is `text-transform: uppercase`; the text is upper-cased once per
	// change, not per frame.
	Upper bool
	// Ink is the token color of the text; InkUnknown reads as --tx.
	Ink theme.Ink
	// Color, when not zero, is used in place of Ink.
	Color color.Color
	// Align places the line in a box wider than it.
	Align ui.TextAlign
	// Ellipsis cuts a line wider than the space it gets and ends it with
	// "…" (white-space: nowrap; overflow: hidden; text-overflow: ellipsis).
	// With MaxLines or Wrap it ends the last line allowed, when the text
	// needs more (line-clamp).
	Ellipsis bool
	// Wrap breaks the text into lines no wider than the room it gets,
	// between words (a word too long for a line of its own is broken
	// inside), each line a box as tall as the first, as a block in CSS is.
	// Without it, and without a MaxLines above 1, the text is one line. A
	// label given no width limit is one line either way. A "\n" in the text
	// always breaks the line (the prototype's <br>).
	Wrap bool
	// MaxLines is the most lines a wrapped text takes; 0 is as many as it
	// needs. A value above 1 wraps the text, with Wrap or without.
	MaxLines int
	// Heading is the level of the h1, h2 or h3 the label is, 1 the highest:
	// assistive technologies read it as a heading. 0 is plain text.
	Heading int

	layout   *text.Layout // what Paint draws: the shaper's, or own
	own      text.Layout  // the copy letter-spacing or wrapping builds, its memory reused
	scratch  []text.Line  // the lines of a text before wrapping, reused
	src      string       // the Text that upper was made from
	upper    string       // Text upper-cased, when Upper
	baseline float32      // of the first line, from the top of the box
	boxH     float32      // of one line
	lines    int          // how many lines the layout has
	multi    bool         // the layout was broken in lines: own holds it, lines aligned
}

var (
	_ ui.Describer = (*Label)(nil)
	_ ui.Labeler   = (*Label)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetText changes the text shown, laying it out again; the same text
// changes nothing.
func (l *Label) SetText(s string) {
	if s != l.Text {
		l.Text = s
		l.Relayout()
	}
}

// Baseline returns where the baseline of the line is, in DIPs from the top
// of the label, as of the last layout.
func (l *Label) Baseline() float32 { return l.baseline }

// Lines returns how many lines the text takes, as of the last layout: 1 for
// a label that does not wrap.
func (l *Label) Lines() int { return l.lines }

// TextLayout returns the layout Paint draws, nil before the first layout.
func (l *Label) TextLayout() *text.Layout { return l.layout }

// AccessibleLabel returns the text shown.
func (l *Label) AccessibleLabel() string { return l.Text }

// Describe describes the label as text, or as a heading of its level.
func (l *Label) Describe(_ *ui.Context, n *platform.SemanticNode) {
	n.Role, n.Label = platform.RoleText, l.Text
	if l.Heading > 0 {
		n.Role, n.Level = platform.RoleHeading, int32(l.Heading)
	}
}

// Layout shapes the text. The width is the line's, ending with the spacing
// after its last character, or the room it gets when it must be cut; the
// height is the box CSS gives the line.
func (l *Label) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := l.Size
	if size <= 0 {
		size = defaultLabelSize
	}
	role := l.Role
	if role == theme.RoleUnknown {
		role = theme.RoleBody
	}
	face := theme.From(c).Faces.Face(role, size, l.Weight)
	body := l.shown()
	bounded := cs.Max.W <= math.MaxFloat32
	cut := l.Ellipsis && bounded
	px := l.Spacing
	l.multi = bounded && (l.Wrap || l.MaxLines > 1) || strings.IndexByte(body, '\n') >= 0
	room := float32(math.MaxFloat32)
	if bounded {
		room = cs.Max.W
	}

	switch {
	case l.multi:
		l.wrapped(c, body, face, room, px, max(l.MaxLines, 0))
		l.layout = &l.own
	case px == 0 && cut:
		// text-overflow: ellipsis cuts a line after its last character that
		// fits, not at a word (white-space: nowrap), on every engine.
		l.layout = c.TextLayoutPolicy(body, face, room, 1, ellipsis, text.WrapGraphemes)
	case px == 0:
		l.layout = c.TextLayout(body, face, 0)
	default:
		l.space(c.TextLayout(body, face, 0), px)
		if cut && l.own.Size.W > room {
			if len(l.own.Lines) > 1 {
				l.own.Lines = l.own.Lines[:1]
			}
			l.truncate(c.TextLayout(ellipsis, face, 0), room, px)
		}
		l.layout = &l.own
	}

	// The line box is Chrome's: the font's rounded ascent and descent, the
	// leading split in two around them.
	var m text.Metrics
	if fm, err := c.Shaper().Metrics(face); err == nil {
		m = fm
	} else if len(l.layout.Lines) > 0 {
		m = text.Metrics{Ascent: l.layout.Lines[0].Ascent, Descent: l.layout.Lines[0].Descent}
	}
	box := theme.Chrome(m, l.LineHeight*size)
	l.boxH, l.baseline = box.Height, box.Baseline

	l.lines = 1
	if l.multi {
		l.lines = max(len(l.own.Lines), 1)
		out := cs.Constrain(geom.Size{W: l.own.Size.W, H: l.boxH * float32(l.lines)})
		l.boxLines(out.W, c.Direction().RTL())

		return out
	}

	return cs.Constrain(geom.Size{W: l.layout.Size.W, H: l.boxH})
}

// Paint draws the line with its baseline where CSS puts it.
func (l *Label) Paint(c *ui.Context, p *ui.Painter) {
	if l.layout == nil || len(l.layout.Lines) == 0 {
		return
	}
	col := l.Color
	if col.A == 0 {
		col = l.Ink.Of(theme.From(c))
	}
	if col.A == 0 {
		return
	}
	at := l.textAt(c.Direction().RTL())
	if !l.multi {
		// the baseline of the 1x line box falls on a whole CSS px in Chrome
		off, top := p.Offset(), l.layout.Lines[0].Baseline
		at.Y = snapAxis(off.Y+at.Y+top, iconSnapUnit) - top - off.Y
	}
	p.Text(l.layout, at, col)
}

// Space returns a copy of l with spacing added after every cluster of
// characters (CSS letter-spacing, which Chrome puts after each character, the
// last one too): glyphs move by spacing times the clusters before them, and
// each line is as much wider as it has drawn clusters. A cluster is one
// character with its combining marks, so a Vietnamese letter in NFC or NFD
// counts once. The copy shares nothing with l, which may be the shaper's cached
// layout; a spacing of 0 returns l.
func Space(l *text.Layout, spacing float32) *text.Layout {
	if spacing == 0 || l == nil || len(l.Glyphs) == 0 {
		return l
	}
	out := new(text.Layout)
	spaceInto(out, l, spacing)

	return out
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// shown returns the text as it is laid out: upper-cased when Upper, which is
// done once per text, not per layout.
func (l *Label) shown() string {
	if !l.Upper {
		return l.Text
	}
	if l.upper == "" || l.src != l.Text {
		l.src, l.upper = l.Text, strings.ToUpper(l.Text)
	}

	return l.upper
}

// textAt returns where the layout's top-left lies in the label's frame, as
// Paint puts it. The layout must have lines.
func (l *Label) textAt(rtl bool) geom.Point {
	x := float32(0)
	if !l.multi {
		x = l.lineX(rtl)
	}

	return geom.Pt(x+textShiftX(), l.baseline-l.layout.Lines[0].Baseline)
}

// lineX is where the line starts in the label's frame, as Align places it;
// rtl swaps the start and the end.
func (l *Label) lineX(rtl bool) float32 {
	free := l.Frame().W - l.layout.Size.W
	end := l.Align == ui.TextAlignEnd
	if rtl && l.Align != ui.TextAlignCenter {
		end = !end
	}
	switch {
	case l.Align == ui.TextAlignCenter:
		return free / 2
	case end:
		return free
	default:
		return 0
	}
}

// copyFrom makes l.own a copy of src, which the shaper owns, keeping the
// memory l.own had.
func (l *Label) copyFrom(src *text.Layout) { copyLayout(&l.own, src) }

// copyLayout makes d a copy of src, keeping the memory d had.
func copyLayout(d, src *text.Layout) {
	d.Reset(src.Text, src.Face, src.Scale)
	d.Runs = src.Runs
	d.Lines = append(d.Lines, src.Lines...)
	d.Glyphs = append(d.Glyphs, src.Glyphs...)
	d.Size = src.Size
}

// space makes l.own a copy of src with letter-spacing px applied.
func (l *Label) space(src *text.Layout, px float32) { spaceInto(&l.own, src, px) }

// spaceInto makes d a copy of src with every cluster of every line moved px
// to the right of the one before it, and px added to the advance of the
// cluster's last glyph and to the width of the line: the letter-spacing of
// CSS, which Chromium puts after each character, the last one too. A cluster
// is a run of glyphs of one character plus its marks; a glyph starts one
// when its text index changes, unless the character there is a mark, which
// stays with the glyph before it. Right-to-left lines are spaced in visual
// order, which is the same for spacing.
func spaceInto(d, src *text.Layout, px float32) {
	copyLayout(d, src)
	d.Size.W = 0
	for i := range d.Lines {
		ln := &d.Lines[i]
		k, drawn := -1, -1
		prev := int32(-1)
		for j := ln.Start; j < ln.End; j++ {
			g := &d.Glyphs[j]
			if k < 0 || g.Index != prev && !continues(src.Text, g.Index) {
				if k >= 0 {
					d.Glyphs[j-1].Advance += px
				}
				k++
			}
			prev = g.Index
			g.X += float32(k) * px
			if text.Drawn(runeAt(src.Text, g.Index)) {
				drawn = k
			}
		}
		if k >= 0 {
			d.Glyphs[ln.End-1].Advance += px
		}
		if drawn >= 0 {
			ln.Width += float32(drawn+1) * px
		}
		d.Size.W = max(d.Size.W, ln.Width)
	}
}

// truncate cuts the last line of l.own so that its prefix and the ellipsis
// (dots, shaped alone) fit in maxW: the longest prefix of whole clusters
// does. The glyphs after it go. Text becomes the text up to the cut and "…",
// so that the glyph indices stay inside it.
func (l *Label) truncate(dots *text.Layout, maxW, px float32) {
	d := &l.own
	if len(d.Lines) == 0 {
		return
	}
	ln := &d.Lines[len(d.Lines)-1]
	var dotsW float32
	var dotGlyphs []text.Glyph
	if len(dots.Lines) > 0 {
		dl := dots.Lines[0]
		dotGlyphs = dots.Glyphs[dl.Start:dl.End]
		for _, g := range dotGlyphs {
			dotsW += g.Advance
		}
		dotsW += px
	}
	// The longest prefix of whole clusters that leaves room for the dots.
	cut := ln.Start
	for j := ln.Start + 1; j <= ln.End; j++ {
		if j < ln.End && !startsCluster(d.Text, d.Glyphs, j) {
			continue
		}
		end := ln.Width
		if j < ln.End {
			end = d.Glyphs[j].X
		}
		if end+dotsW > maxW {
			break
		}
		cut = j
	}
	byteCut, xCut := len(d.Text), ln.Width
	if cut < ln.End {
		byteCut, xCut = int(d.Glyphs[cut].Index), d.Glyphs[cut].X
	}
	d.Glyphs = d.Glyphs[:cut]
	for _, g := range dotGlyphs {
		g.X += xCut
		g.Index = int32(byteCut)
		g.Run = 0
		d.Glyphs = append(d.Glyphs, g)
	}
	if n := len(dotGlyphs); n > 0 {
		d.Glyphs[len(d.Glyphs)-1].Advance += px
	}
	d.Text = d.Text[:byteCut] + ellipsis
	d.Runs = nil
	ln.End = int32(len(d.Glyphs))
	ln.TextEnd = int32(len(d.Text))
	ln.Width = xCut + dotsW
	d.Size.W = ln.Width
}

// wrapEpsilon is the slack, in DIPs, a line gets before a word that overflows
// it by less goes to the next: float rounding, not a tolerance of the design.
const wrapEpsilon float32 = 1e-3

// wrapped makes l.own the text in lines no wider than room, at most limit of
// them (0: any number). Without letter-spacing the shaper breaks the lines;
// with it the breaks are found here, on the spaced glyphs, since a word that
// fits once the characters are drawn closer is not one for the shaper, which
// knows nothing of the spacing.
func (l *Label) wrapped(c *ui.Context, body string, face text.Face, room, px float32, limit int) {
	ell := ""
	if l.Ellipsis {
		ell = ellipsis
	}
	if px == 0 {
		w := room
		if w >= math.MaxFloat32 {
			w = 0 // no limit
		}
		l.copyFrom(c.TextLayoutClamped(body, face, w, limit, ell))

		return
	}
	l.space(c.TextLayout(body, face, 0), px)
	var dots *text.Layout
	if l.Ellipsis {
		dots = c.TextLayout(ellipsis, face, 0)
	}
	l.wrap(dots, room, px, limit)
}

// wrap breaks the lines of l.own, which space made from text laid out
// without a width, so that none is wider than room: between words, and
// inside a word that cannot fit a line. The glyphs of each new line start
// at x 0. With a limit, the last line allowed takes the rest of its text
// and, when that is too long, ends with the ellipsis (dots) if the label
// has one, or is cut there if not. Lines of Text that the shaper broke
// stay broken.
func (l *Label) wrap(dots *text.Layout, room, px float32, limit int) {
	d := &l.own
	src := append(l.scratch[:0], d.Lines...)
	l.scratch = src
	d.Lines = d.Lines[:0]
	for si, ln := range src {
		a := ln.Start
		for {
			atCap := limit > 0 && len(d.Lines) == limit-1
			b := ln.End
			if !atCap || !l.Ellipsis {
				b = l.fit(a, ln.End, room)
			}
			l.emit(ln, a, b, a == ln.Start)
			if limit > 0 && len(d.Lines) == limit {
				last := &d.Lines[len(d.Lines)-1]
				switch {
				case !l.Ellipsis:
					d.Glyphs = d.Glyphs[:b]
				case last.Width > room || si < len(src)-1:
					l.truncate(dots, room, px)
				default: // the last line fits: nothing to cut
				}
				l.widest()

				return
			}
			if a = b; a >= ln.End {
				break
			}
		}
	}
	l.widest()
}

// emit appends the line made of the glyphs [a, b) of the source line ln to
// l.own, its glyphs moved to start at 0; first tells that a is the start of
// ln. Spaces at its end hang outside its width.
func (l *Label) emit(ln text.Line, a, b int32, first bool) {
	d := &l.own
	var x0, w float32
	if b > a {
		x0 = d.Glyphs[a].X
	}
	for j := b - 1; j >= a; j-- {
		if !l.blank(j) {
			w = d.Glyphs[j].X + d.Glyphs[j].Advance - x0

			break
		}
	}
	for j := a; j < b; j++ {
		d.Glyphs[j].X -= x0
	}
	ts, te := ln.TextStart, ln.TextEnd
	if !first && b > a {
		ts = d.Glyphs[a].Index
	}
	if b < ln.End {
		te = d.Glyphs[b].Index
	}
	d.Lines = append(d.Lines, text.Line{
		Start: a, End: b, TextStart: ts, TextEnd: te,
		Ascent: ln.Ascent, Descent: ln.Descent, Width: w,
	})
}

// widest sets the width of l.own to that of its widest line.
func (l *Label) widest() {
	d := &l.own
	d.Size.W = 0
	for i := range d.Lines {
		d.Size.W = max(d.Size.W, d.Lines[i].Width)
	}
}

// fit returns where the line that starts at glyph a of l.own ends, the
// source line ending at end: the start of the first word that does not fit
// in room, or end when all do. A first word that does not fit is broken
// inside.
func (l *Label) fit(a, end int32, room float32) int32 {
	if a >= end {
		return end
	}
	d := &l.own
	x0 := d.Glyphs[a].X
	next := int32(-1) // where the line after the words that fit starts
	for j := a; j < end; {
		w := j
		for w < end && !l.blank(w) {
			w++
		}
		s := w
		for s < end && l.blank(s) {
			s++
		}
		if w > j {
			if g := &d.Glyphs[w-1]; g.X+g.Advance-x0 > room+wrapEpsilon {
				if next >= 0 {
					return next
				}

				return l.breakWord(a, w, room)
			}
			next = s
		}
		j = s
	}

	return end
}

// breakWord returns where to cut the word [a, w) of l.own, which does not
// fit room: after the longest prefix of whole clusters that does, or the
// first cluster when not even that does.
func (l *Label) breakWord(a, w int32, room float32) int32 {
	d := &l.own
	x0 := d.Glyphs[a].X
	cut := a
	for j := a + 1; j <= w; j++ {
		if j < w && !startsCluster(d.Text, d.Glyphs, j) {
			continue
		}
		end := d.Glyphs[w-1].X + d.Glyphs[w-1].Advance
		if j < w {
			end = d.Glyphs[j].X
		}
		if end-x0 > room+wrapEpsilon {
			break
		}
		cut = j
	}
	if cut == a {
		cut = a + 1
		for cut < w && !startsCluster(d.Text, d.Glyphs, cut) {
			cut++
		}
	}

	return cut
}

// blank tells whether glyph j of l.own is a space the text may break at: the
// no-break spaces (U+00A0, U+2007, U+202F) are spaces to the eye and glue the words
// they stand between, as in CSS.
func (l *Label) blank(j int32) bool {
	r := runeAt(l.own.Text, l.own.Glyphs[j].Index)
	if r == '\u00a0' || r == '\u2007' || r == '\u202f' {
		return false
	}

	return unicode.IsSpace(r)
}

// boxLines puts the lines of l.own in their boxes: the baseline of line i is
// the first one's plus i boxes, and the line is aligned in a label w wide
// as Align says. The text is drawn at the label's origin afterwards.
func (l *Label) boxLines(w float32, rtl bool) {
	d := &l.own
	end := l.Align == ui.TextAlignEnd
	if rtl && l.Align != ui.TextAlignCenter {
		end = !end
	}
	var widest float32
	for i := range d.Lines {
		ln := &d.Lines[i]
		ln.Baseline = l.baseline + float32(i)*l.boxH
		widest = max(widest, ln.Width)
		free := max(w-ln.Width, 0)
		var shift float32
		switch {
		case l.Align == ui.TextAlignCenter:
			shift = free / 2
		case end:
			shift = free
		default:
		}
		if shift != 0 {
			for j := ln.Start; j < ln.End; j++ {
				d.Glyphs[j].X += shift
			}
		}
	}
	d.Size = geom.Size{W: widest, H: l.boxH * float32(max(len(d.Lines), 1))}
}

// startsCluster tells whether glyph j (not the first of its line) starts a
// cluster: its text index differs from the glyph before it and the character
// there is not a mark.
func startsCluster(s string, glyphs []text.Glyph, j int32) bool {
	return glyphs[j].Index != glyphs[j-1].Index && !continues(s, glyphs[j].Index)
}

// continues tells whether the character at byte i of s joins the cluster
// before it: a combining mark, a zero-width joiner or a variation selector.
func continues(s string, i int32) bool {
	r := runeAt(s, i)

	return unicode.Is(unicode.M, r) || r == '‍' || r >= '︀' && r <= '️'
}

// runeAt returns the character at byte i of s; U+FFFD past its end.
func runeAt(s string, i int32) rune {
	if i < 0 || int(i) >= len(s) {
		return utf8.RuneError
	}
	r, _ := utf8.DecodeRuneInString(s[i:])

	return r
}
