package poster

import (
	"dancefitvn-app/internal/tokens"

	"github.com/gdeskdev/desk/foundation/core/color"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Colors is what a course's poster is painted in: the two ends of the
// gradient, the ink of the figure and the title, and the figure.
type Colors struct {
	// From and To are `g[0]` and `g[1]` of addC (index.html:248-255), the
	// ends of `linear-gradient(135deg, From, To)`.
	From, To color.Color
	// Ink is `c.ink` (index.html:250, default '#fff' at :247): the color of
	// the dancers, the title and its shadow-less text.
	Ink color.Color
	// Vinyl is `c.vinyl` (index.html:250, :255): a record in place of the
	// dancers.
	Vinyl bool
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// The two inks of addC: the default `ink:'#fff'` (index.html:247) and the
// dark red of the Hot Extra posters (`ink:'#8A1424'`, index.html:250, :255).
var (
	inkWhite = color.RGB(0xff, 0xff, 0xff)
	inkHot   = color.RGB(0x8a, 0x14, 0x24)
)

// row is a line of the palette, 02 section 5.
type row struct {
	id string // the API course id
	c  Colors
}

// palette is the table of 02 section 5, in its order: the sixteen courses of
// addC (index.html:244-255) under their API ids. Set 24..30 take PAL[(35-n)
// % 12] (index.html:254): 30 -> PAL[5] ... 24 -> PAL[11].
var palette = [...]row{
	{"set-35", Colors{From: rgb(0xFF4FB8), To: rgb(0x9E1470), Ink: inkWhite}},                 // PAL[0], :248
	{"set-34", Colors{From: rgb(0x3CC3D6), To: rgb(0x0B5466), Ink: inkWhite}},                 // PAL[1], :249
	{"hot-extra", Colors{From: rgb(0xFFF1EE), To: rgb(0xF8C9CC), Ink: inkHot, Vinyl: true}},   // :250
	{"set-33", Colors{From: rgb(0x2FB8C8), To: rgb(0x0E6E80), Ink: inkWhite}},                 // PAL[2], :251
	{"set-32", Colors{From: rgb(0xF23D6E), To: rgb(0xA30E3A), Ink: inkWhite}},                 // PAL[3], :252
	{"set-31", Colors{From: rgb(0x4B2A86), To: rgb(0xE8A23A), Ink: inkWhite}},                 // PAL[4], :253
	{"set-30", Colors{From: rgb(0xFF7A2F), To: rgb(0xB3261E), Ink: inkWhite}},                 // PAL[5], :254
	{"set-29", Colors{From: rgb(0x1E6FE0), To: rgb(0x22B8CF), Ink: inkWhite}},                 // PAL[6]
	{"set-28", Colors{From: rgb(0x7A3FD1), To: rgb(0xE5383B), Ink: inkWhite}},                 // PAL[7]
	{"set-27", Colors{From: rgb(0x1F7A4D), To: rgb(0x5BC489), Ink: inkWhite}},                 // PAL[8]
	{"set-26", Colors{From: rgb(0xC2185B), To: rgb(0xFF8A65), Ink: inkWhite}},                 // PAL[9]
	{"set-25", Colors{From: rgb(0x0D47A1), To: rgb(0x26C6DA), Ink: inkWhite}},                 // PAL[10]
	{"set-24", Colors{From: rgb(0xAD1457), To: rgb(0xF06292), Ink: inkWhite}},                 // PAL[11]
	{"hot-extra-2", Colors{From: rgb(0xFFF1EE), To: rgb(0xF8C9CC), Ink: inkHot, Vinyl: true}}, // :255
	{"intro", Colors{From: rgb(0x2E2E35), To: rgb(0xE5383B), Ink: inkWhite}},                  // 02 section 5
	{"set-36", Colors{From: rgb(0x3A3D44), To: rgb(0x16171A), Ink: inkWhite}},                 // 02 section 5
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Lookup returns the colors of the course with the API id id. A course the
// table does not hold (02 section 5, live rule) gets `--sf2` to `--bg` and
// no figure; ok tells whether the id was known. It does not allocate.
func Lookup(id string) (c Colors, ok bool) {
	for i := range palette {
		if palette[i].id == id {
			return palette[i].c, true
		}
	}

	return Colors{From: tokens.Dark.Surface2, To: tokens.Dark.Bg, Ink: inkWhite}, false
}

// IDs returns the API ids of the palette, in the order of 02 section 5.
func IDs() []string {
	ids := make([]string, len(palette))
	for i := range palette {
		ids[i] = palette[i].id
	}

	return ids
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// rgb returns the opaque color of a 0xRRGGBB literal.
func rgb(v uint32) color.Color {
	return color.RGB(uint8(v>>16), uint8(v>>8), uint8(v))
}
