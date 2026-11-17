package kit

import (
	"dancefitvn-app/internal/ui/theme"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The names layout.json gives the text atoms: tag and classes.
const (
	nameEyebrow = "div.eb"
	nameDisplay = "div.d"
	nameH1      = "div.h1"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Eyebrow returns `.eb` (index.html:46): 11/700 upper case with 1.4 px spacing
// in --tx2.
func Eyebrow(s string) *Label {
	l := &Label{Text: s, Size: 11, Weight: 700, Spacing: 1.4, Upper: true, Ink: theme.InkText2}
	l.Name = nameEyebrow

	return l
}

// Display returns `.d` (index.html:45) at size px: Barlow Condensed 700 upper
// case, 0.6 px spacing, line-height 1.
func Display(s string, size float32) *Label {
	l := &Label{Text: s, Role: theme.RoleDisplay, Size: size, Weight: 700, Spacing: .6, LineHeight: 1, Upper: true}
	l.Name = nameDisplay

	return l
}

// H1 returns `.h1` (index.html:87): Barlow Condensed 700 34 upper case, 1 px
// spacing, line-height 1.
func H1(s string) *Label {
	l := &Label{Text: s, Role: theme.RoleDisplay, Size: 34, Weight: 700, Spacing: 1, LineHeight: 1, Upper: true}
	l.Name = nameH1
	l.Heading = 1 // `.h1` is the page's title

	return l
}

// SecTitle returns the `.d` of a section head `.sec .d` (index.html:88-89): 18
// px with 1 px spacing.
func SecTitle(s string) *Label {
	l := Display(s, 18)
	l.Spacing = 1
	l.Heading = 2 // the title of a section of the page
	l.Name = "span.d"

	return l
}

// SecLink returns the `a` of a section head `.sec a` (index.html:90): 13/600 in
// --pr.
func SecLink(s string) *Label {
	return &Label{Text: s, Size: 13, Weight: 600, Ink: theme.InkPrimary}
}

// Muted returns `.muted` (index.html:49) at size px: regular --tx2.
func Muted(s string, size float32) *Label {
	return &Label{Text: s, Size: size, Ink: theme.InkText2}
}
