//go:build !tvos

package screens

import (
	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/ui/kit"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// What a screen reader gets of the screens that the kit's widgets do not say by
// themselves (P8.3a, docs/plan P8 §4): the prototype has no ARIA, so the labels are
// the visible text, a prototype string for the same control, an existing a11y.*
// key or a new one (locales/new-keys.txt). The values of controls that hold one:
// the speed ("0.75x", the button's own text), the A-B loop ("01:12–01:40"), the
// position of a bar ("01:12").

// loopSep is the en dash of "01:12–01:40", the A-B range as a screen reader
// reads it.
const loopSep = "–"

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// loopValue returns the range of the A-B loop as "mm:ss–mm:ss" while it is on,
// else "".
func loopValue(on bool, a, b float64) string {
	if !on {
		return ""
	}

	return format.Clock(a) + loopSep + format.Clock(b)
}

// heading makes l a heading of the level, 1 for the title of a screen or of a sheet
// (a sheet is a screen of its own), 2 for the title of a section, and returns it.
func heading(l *kit.Label, level int) *kit.Label {
	l.Heading = level

	return l
}
