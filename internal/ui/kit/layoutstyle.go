package kit

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// CSS is the computed style of an element as CSS strings, the way the
// prototype's capture writes it (cmd/shotcmp/schema.go Style); a field left
// empty is "not recorded" and cmd/shotcmp does not compare it. It lives here
// and not in layoutdump because the widgets answer it (LayoutStyle) and
// layoutdump imports kit.
type CSS struct {
	FontFamily    string `json:"fontFamily,omitempty"`
	FontSize      string `json:"fontSize,omitempty"`
	FontWeight    string `json:"fontWeight,omitempty"`
	LineHeight    string `json:"lineHeight,omitempty"`
	LetterSpacing string `json:"letterSpacing,omitempty"`
	Color         string `json:"color,omitempty"`
	Background    string `json:"background,omitempty"`
	Radius        string `json:"radius,omitempty"`
	Border        string `json:"border,omitempty"`
	Opacity       string `json:"opacity,omitempty"`
}

// radiusRound is Chrome's computed `border-radius:50%`.
const radiusRound = "50%"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// LayoutStyle returns the label's computed style and its visible text.
func (l *Label) LayoutStyle(c *ui.Context) (CSS, string) { return labelCSS(c, l) }

// CSSColor writes c as Chrome's computed color: rgb(r, g, b) when opaque,
// rgba(r, g, b, a) otherwise.
func CSSColor(c color.Color) string {
	if c.A == 255 {
		return fmt.Sprintf("rgb(%d, %d, %d)", c.R, c.G, c.B)
	}
	a := math.Round(float64(c.A)/255*100) / 100

	return fmt.Sprintf("rgba(%d, %d, %d, %s)", c.R, c.G, c.B, strconv.FormatFloat(a, 'f', -1, 64))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// cssPx writes v as a CSS length: "14px", "8.4px".
func cssPx(v float32) string {
	return strconv.FormatFloat(math.Round(float64(v)*100)/100, 'f', -1, 64) + "px"
}

// cssBorder writes a border as the capture does: its width, snapped down to a
// whole pixel as Chrome does at a device ratio of 1 (1.2px and 1.6px read
// 1px), and its color.
func cssBorder(width float32, col color.Color) string {
	if width <= 0 {
		return ""
	}

	return cssPx(float32(math.Max(1, math.Floor(float64(width))))) + " " + CSSColor(col)
}

// labelCSS is the computed style of a Label the way Chrome reports it: px
// sizes, `normal` for a line height of 0 and a spacing of 0, the color of the
// ink, and the text upper-cased when the label is (innerText applies
// text-transform).
func labelCSS(c *ui.Context, l *Label) (CSS, string) {
	look := theme.From(c)
	size := l.Size
	if size == 0 {
		size = 16
	}
	weight := int(l.Weight)
	if weight == 0 {
		weight = 400
	}
	col := l.Color
	if col.A == 0 {
		col = l.Ink.Of(look)
	}
	s := CSS{
		FontFamily:    theme.Family(l.Role),
		FontSize:      cssPx(size),
		FontWeight:    strconv.Itoa(weight),
		LineHeight:    "normal",
		LetterSpacing: "normal",
		Color:         CSSColor(col),
	}
	if l.LineHeight != 0 {
		s.LineHeight = cssPx(l.LineHeight * size)
	}
	if l.Spacing != 0 {
		s.LetterSpacing = cssPx(l.Spacing)
	}
	text := l.Text
	if l.Upper {
		text = strings.ToUpper(text)
	}

	return s, text
}

// withBox returns the text style s with the box fields: background (transparent
// when bg is zero), radius, and a border (none when width is 0).
func withBox(s CSS, bg color.Color, radius string, width float32, border color.Color) CSS {
	s.Background = CSSColor(bg)
	if bg.A == 0 {
		s.Background = "rgba(0, 0, 0, 0)"
	}
	s.Radius = radius
	s.Border = cssBorder(width, border)

	return s
}

// boxCSS is the style of an element with no text of its own: the box fields.
func boxCSS(bg color.Color, radius string, width float32, border color.Color) CSS {
	return withBox(CSS{}, bg, radius, width, border)
}

// The LayoutStyle methods of the widgets. Each method returns what the prototype's capture records for the element the
// widget stands for (the CSS selector is in the comment): the text style for a
// widget that holds its own text, always the background, radius and border.
// They are read by layoutdump after a frame; they allocate and are not on any
// hot path.

// LayoutStyle is `button.btn.*` (index.html:57-63).
func (b *Button) LayoutStyle(c *ui.Context) (CSS, string) {
	look := theme.From(c)
	s, txt := labelCSS(c, &b.label)
	width, border := float32(0), look.T.Border
	if b.Kind == ButtonG {
		width = btnOutline
	}

	return withBox(s, b.fill(look), cssPx(btnRadius), width, border), txt
}

// LayoutStyle is `span.chip.*` (:50-56).
func (b *Chip) LayoutStyle(c *ui.Context) (CSS, string) {
	look := theme.From(c)
	bg, fg := b.colors(look)
	s, txt := labelCSS(c, &b.label)
	s.Color = CSSColor(fg)
	radius := b.Radius
	if radius <= 0 {
		radius = chipRadius
	}

	return withBox(s, bg, cssPx(radius), 0, color.Color{}), txt
}

// LayoutStyle is `div.inp` and `div.inp.err` (:107-111).
func (in *Input) LayoutStyle(c *ui.Context) (CSS, string) {
	t := &theme.From(c).T
	radius := inputRadius
	if in.Search {
		radius = searchRadius
	}
	border, width := t.Border, inputBorder
	switch {
	case in.Error:
		border, width = t.Danger, inputRing
	case in.FocusWithin():
		border, width = t.Primary, inputRing
	default: // `.inp`: the plain 1 px --bd border
	}

	return boxCSS(t.Surface, cssPx(radius), width, border), ""
}

// LayoutStyle is `input` (:110): the text the field holds, 15/400 --tx.
func (f *inputField) LayoutStyle(c *ui.Context) (CSS, string) {
	t := &theme.From(c).T
	s := CSS{
		FontFamily: theme.Family(theme.RoleBody), FontSize: cssPx(inputSize), FontWeight: "400",
		LineHeight: "normal", LetterSpacing: "normal", Color: CSSColor(t.Text),
	}

	return s, f.Text()
}

// LayoutStyle is `span.sw` and `span.sw.on` (:118): the colors of the state the
// switch is going to.
func (s *Switch) LayoutStyle(c *ui.Context) (CSS, string) {
	t := &theme.From(c).T
	k := flag(s.On)

	return boxCSS(t.Surface2.Mix(t.Primary, k), cssPx(switchRadius), switchBorder, t.Border.Mix(t.Primary, k)), ""
}

// LayoutStyle is `div.segc` (:121).
func (s *Segmented) LayoutStyle(c *ui.Context) (CSS, string) {
	t := &theme.From(c).T

	return boxCSS(t.Surface, cssPx(segcRadius), segcBorder, t.Border), ""
}

// LayoutStyle is `.segc button` (:122): the text style of the label, a --pr
// pill when chosen.
func (b *segItem) LayoutStyle(c *ui.Context) (CSS, string) {
	t := &theme.From(c).T
	s, txt := labelCSS(c, &b.label)
	var bg color.Color
	if b.seg.Selected == b.index {
		bg = t.Primary
	}

	return withBox(s, bg, cssPx(segButtonRadius), 0, color.Color{}), txt
}

// LayoutStyle is `div.prog` (:114): the track.
func (p *Progress) LayoutStyle(c *ui.Context) (CSS, string) {
	t := &theme.From(c).T
	r := p.Bounds()

	return boxCSS(orColor(p.Track, t.Surface2), cssPx(capRadius(r, progRadius)), 0, color.Color{}), ""
}

// FillStyle is `.prog i`, the fill of the bar.
func (p *Progress) FillStyle(c *ui.Context) CSS {
	t := &theme.From(c).T

	return boxCSS(orColor(p.Fill, t.Primary), cssPx(progRadius), 0, color.Color{})
}

// FillRect returns the fill in the bar's coordinates, as of the last layout.
func (p *Progress) FillRect() (x, y, w, h float32) {
	r := p.Bounds()

	return 0, 0, r.W * min(max(p.Value, 0), 1), r.H
}

// LayoutStyle is `.radio` (:133).
func (r *Radio) LayoutStyle(c *ui.Context) (CSS, string) {
	t := &theme.From(c).T
	border, width := t.Border, borderInset
	if r.On {
		border, width = t.Primary, inputRing
	}

	return boxCSS(t.Bg, cssPx(radioRadius), width, border), ""
}

// LayoutStyle is `.radio .dot`: a ring, --pr when chosen.
func (d *radioDotBox) LayoutStyle(c *ui.Context) (CSS, string) {
	t := &theme.From(c).T
	ring := t.Text3
	if d.r.On {
		ring = t.Primary
	}

	return boxCSS(color.Color{}, radiusRound, radioDotBorder, ring), ""
}

// LayoutStyle is `.row` (:116): transparent, with a line under it.
func (r *Row) LayoutStyle(*ui.Context) (CSS, string) {
	return boxCSS(color.Color{}, "0px", 0, color.Color{}), ""
}

// LayoutStyle is `.row .ico` (:117): a rounded --sf2 square.
func (b *rowIcoBox) LayoutStyle(c *ui.Context) (CSS, string) {
	return boxCSS(theme.From(c).T.Surface2, cssPx(rowIcoRadius), 0, color.Color{}), ""
}

// LayoutStyle is `.les` (:92-93): the tint and rounding of the current lesson.
func (r *LessonRow) LayoutStyle(c *ui.Context) (CSS, string) {
	look := theme.From(c)
	if r.State == LessonCurrent {
		return boxCSS(look.C.LessonCur, cssPx(lesCurRadius), 0, color.Color{}), ""
	}

	return boxCSS(color.Color{}, "0px", 0, color.Color{}), ""
}

// LayoutStyle is `.les .no` (:94): a circle with the number, filled --pr on the
// current lesson.
func (n *lessonNo) LayoutStyle(c *ui.Context) (CSS, string) {
	look := theme.From(c)
	s, txt := labelCSS(c, &n.label)
	col := look.T.Border
	var bg color.Color
	if n.row.State == LessonCurrent {
		col, bg = look.T.Primary, look.T.Primary
	}

	return withBox(s, bg, radiusRound, lesNoBorder, col), txt
}

// LayoutStyle is `.dlb` (:95): a bare circle.
func (d *lessonDl) LayoutStyle(*ui.Context) (CSS, string) {
	return boxCSS(color.Color{}, radiusRound, 0, color.Color{}), ""
}

// LayoutStyle is the offline play disc: a --pr circle.
func (l *lessonPlay) LayoutStyle(c *ui.Context) (CSS, string) {
	return boxCSS(theme.From(c).T.Primary, radiusRound, 0, color.Color{}), ""
}

// LayoutStyle is `.card` (:64) as a course card: a --sf box of 16 radius.
func (cc *CourseCard) LayoutStyle(c *ui.Context) (CSS, string) {
	return boxCSS(theme.From(c).T.Surface, cssPx(cardRadius), 0, color.Color{}), ""
}

// LayoutStyle is `.badge` (:174).
func (b *Badge) LayoutStyle(c *ui.Context) (CSS, string) {
	return boxCSS(theme.From(c).T.Surface, cssPx(badgeRadius), 0, color.Color{}), ""
}

// LayoutStyle is `.codebox` (:176), --pr border while focused. The text style
// is the character's.
func (b *codeBox) LayoutStyle(c *ui.Context) (CSS, string) {
	look := theme.From(c)
	s, txt := labelCSS(c, &b.label)
	col, width := look.T.Border, codeBorder
	if b.focused {
		col, width = look.T.Primary, codeFocusBorder
	}

	return withBox(s, look.T.Surface, cssPx(codeRadius), width, col), txt
}

// LayoutStyle is `i` of the code row: a --tx3 dash.
func (d *codeDash) LayoutStyle(c *ui.Context) (CSS, string) {
	return boxCSS(theme.From(c).T.Text3, "0px", 0, color.Color{}), ""
}

// LayoutStyle is `.tabs` (:79): the bar color over a 1 px --bd line.
func (t *TabBar) LayoutStyle(c *ui.Context) (CSS, string) {
	look := theme.From(c)

	return boxCSS(look.C.TabBar, "0px", tabBorder, look.T.Border), ""
}

// LayoutStyle is a tab button (:80-81): the label's style.
func (b *tabButton) LayoutStyle(c *ui.Context) (CSS, string) {
	s, txt := labelCSS(c, &b.label)

	return withBox(s, color.Color{}, "0px", 0, color.Color{}), txt
}

// LayoutStyle is `.pill` (:82): the tint of the current tab.
func (p *tabPill) LayoutStyle(c *ui.Context) (CSS, string) {
	var bg color.Color
	if p.on {
		bg = theme.From(c).C.PillOn
	}

	return boxCSS(bg, cssPx(tabPillRadius), 0, color.Color{}), ""
}

// LayoutStyle is `.dim-bg` (:124): the scrim.
func (d *sheetDim) LayoutStyle(c *ui.Context) (CSS, string) {
	return boxCSS(theme.From(c).T.Scrim, "0px", 0, color.Color{}), ""
}

// LayoutStyle is `.sheet` (:125): --sf, rounded at the top.
func (p *sheetPanel) LayoutStyle(c *ui.Context) (CSS, string) {
	return boxCSS(theme.From(c).T.Surface, cssPx(sheetRadius), 0, color.Color{}), ""
}

// LayoutStyle is `.sheet .grab` (:126): the 36 x 5 bar.
func (g *sheetGrab) LayoutStyle(c *ui.Context) (CSS, string) {
	return boxCSS(theme.From(c).T.Text3, cssPx(grabRadius), 0, color.Color{}), ""
}

// LayoutStyle is `div.toast` (:131): the message's style on the toast color.
func (b *toastBubble) LayoutStyle(c *ui.Context) (CSS, string) {
	look := theme.From(c)
	s, txt := labelCSS(c, &b.msg)

	return withBox(s, look.C.ToastBg, cssPx(toastRadius), 0, color.Color{}), txt
}

// LayoutStyle is `.rb` and `.rb.dk` (:84-85): a disc.
func (b *RoundButton) LayoutStyle(c *ui.Context) (CSS, string) {
	look := theme.From(c)
	bg := look.T.Surface
	if b.Dark {
		bg = look.C.RoundDark
	}

	return boxCSS(orColor(b.Bg, bg), radiusRound, 0, color.Color{}), ""
}

// LayoutStyle is an icon-only button (`pwEye`): bare.
func (b *IconButton) LayoutStyle(*ui.Context) (CSS, string) {
	return boxCSS(color.Color{}, "0px", 0, color.Color{}), ""
}

// LayoutStyle is `.offbar` (:132): the tint, the text 13/700 --warn.
func (o *OfflineBar) LayoutStyle(c *ui.Context) (CSS, string) {
	s, txt := labelCSS(c, &o.label)

	return withBox(s, theme.From(c).C.OfflineBg, "0px", 0, color.Color{}), txt
}

// LayoutStyle is a button that holds the avatar (`go.account`): bare.
func (b *AvatarButton) LayoutStyle(*ui.Context) (CSS, string) {
	return boxCSS(color.Color{}, "0px", 0, color.Color{}), ""
}
