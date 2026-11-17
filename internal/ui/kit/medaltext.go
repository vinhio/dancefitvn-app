package kit

import (
	"strings"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

const (
	// medalShrinkStep is the factor a text that does not fit loses of its size at
	// each try; the sizes tried are few and each is a layout of a short text.
	medalShrinkStep float32 = .92
	// MedalCaptionFloor is the smallest share of its size a badge's caption is
	// shrunk to so that its longest word fits on a line.
	MedalCaptionFloor float32 = .7
	// MedalSubFloor is the smallest share of its size the text on a medal's
	// hexagon is shrunk to; what does not fit then ends in an ellipsis.
	MedalSubFloor float32 = .6
	// MedalSubWidth is the room the hexagon of medal() leaves for a line at the
	// baseline of its sub text (y=77): the inner hexagon of radius 28 about
	// (50, 62) spans about 44 of the 100 units there, less a unit on each side.
	MedalSubWidth float32 = 42
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// MedalText is the text a medal or a badge shows for a name or a code: the
// underscores of a code ("FOUNDER_CEO") are the spaces of a name ("HDV
// DanceFIT", `BADGES`, index.html:311), so that its words wrap and fit apart.
func MedalText(s string) string { return strings.ReplaceAll(s, "_", " ") }

// BreaksWord tells whether the text, as the last layout wrapped it, has a line
// that starts inside a word: the word is wider than the room, and the shaper
// broke it ("FOUNDE" over "R").
func (l *Label) BreaksWord() bool {
	ly := l.layout
	if ly == nil || !l.multi {
		return false
	}
	for i := 1; i < len(ly.Lines); i++ {
		ts := int(ly.Lines[i].TextStart)
		if ts <= 0 || ts > len(ly.Text) {
			continue
		}
		if p := ly.Text[ts-1]; p != ' ' && p != '\n' {
			return true
		}
	}

	return false
}

// WordSize returns the size, from base down to base*floor, at which the
// longest word of probe's text fits in a line of room: the largest of the sizes
// base, base*.92, base*.92^2 ... that breaks no word, else base*floor. probe is
// a label that is in no tree, styled as the text that is to be fitted (it is
// set to wrap, and its size is left at the last size tried).
func WordSize(c *ui.Context, probe *Label, base, floor, room float32) float32 {
	probe.Wrap = true
	least := base * floor
	size := base
	for {
		probe.Size = size
		probe.Relayout()
		c.Layout(probe, ui.Loose(geom.Sz(room, ui.Unbounded)))
		if !probe.BreaksWord() || size <= least {
			return size
		}
		size = max(size*medalShrinkStep, least)
	}
}

// LineSize returns the size, from base down to base*floor, at which the one
// line of probe's text is no wider than maxW (shrunk in proportion: the width
// of a line grows with its size); base*floor when it is still wider, where the
// caller cuts it with an ellipsis. probe is a label in no tree, styled as the
// text (its size is left at base).
func LineSize(c *ui.Context, probe *Label, base, floor, maxW float32) float32 {
	probe.Wrap, probe.Ellipsis = false, false
	probe.Size = base
	probe.Relayout()
	w := c.Layout(probe, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))).W
	if w <= maxW || w <= 0 {
		return base
	}

	return max(base*maxW/w, base*floor)
}
