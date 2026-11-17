// Adapted from jplayer-mobile/internal/ui/icon/icon.go at 49de3c2.

package icon

import (
	"strconv"
	"sync"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene/svg"
	"github.com/gdeskdev/desk/framework/ui"
)

//go:generate go run ./gen -in ../../../reference/prototype/index.html -out icons_gen.go

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// StrokeDefault is the stroke width of every icon the prototype draws without
// asking for another: `svg.i{stroke-width:1.9}` (index.html:64). Callers pass 0
// for it; ic(n,s,sw) (index.html:239) overrides it per use (1.8, 2, 2.2 ...).
const StrokeDefault float32 = 1.9

// def is one icon of the generated table.
type def struct {
	// path is the path data in the 24-unit box.
	path string
	// filled is a glyph with a fill and no stroke: PLAY and PAUSE.
	filled bool
	// fillAlso fills a stroked glyph too (the triangle of the current lesson).
	fillAlso bool
	// stroke is the width the glyph is drawn with when the caller asks for the
	// default, 0 for StrokeDefault.
	stroke float32
}

// key identifies a parsed icon: the icon and its stroke width.
type key struct {
	n      Name
	stroke float32
}

// entry is a parsed document. Draw reuses scratch memory inside a
// svg.Document, so drawing locks it; the UI thread is its only user in the
// app and the lock never waits there.
type entry struct {
	mu  sync.Mutex
	doc *svg.Document
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// cache holds the documents parsed so far, one per icon and stroke width: the
// stroke of a document is fixed when it is parsed (GT19 of jPlayer TV).
var cache struct {
	mu    sync.RWMutex
	icons map[key]*entry
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// String returns the prototype's key of the icon ("dl"); "" for NameUnknown
// and values past the last icon.
func (n Name) String() string {
	if n == NameUnknown || n >= nameEnd {
		return ""
	}

	return ids[n]
}

// Parse returns the icon with that prototype key; false for a key that is not
// one.
func Parse(id string) (Name, bool) {
	for n := NameUnknown + 1; n < nameEnd; n++ {
		if ids[n] == id {
			return n, true
		}
	}

	return NameUnknown, false
}

// Names returns every icon, in the order of the prototype's table.
func Names() []Name {
	out := make([]Name, 0, nameEnd-1)
	for n := NameUnknown + 1; n < nameEnd; n++ {
		out = append(out, n)
	}

	return out
}

// Draw paints the icon in r, in color c: the 24-unit box fitted into r,
// centered, with stroke the stroke width in that box (0: StrokeDefault, or the
// glyph's own). Filled glyphs ignore it. Unknown icons draw nothing. After the
// first draw of an icon at a stroke it does not allocate.
func Draw(p *ui.Painter, n Name, r geom.Rect, c color.Color, stroke float32) {
	e := lookup(n, stroke)
	if e == nil || e.doc == nil {
		return
	}
	e.mu.Lock()
	e.doc.Draw(p.Scene(), r.Translate(p.Offset()), c)
	e.mu.Unlock()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// lookup returns the parsed icon, parsing it on the first use; nil for an
// unknown icon.
func lookup(n Name, stroke float32) *entry {
	if n == NameUnknown || n >= nameEnd {
		return nil
	}
	d := &defs[n]
	if d.filled {
		stroke = 0 // one document, whatever the caller asked
	} else if stroke <= 0 {
		stroke = d.stroke
		if stroke <= 0 {
			stroke = StrokeDefault
		}
	}
	k := key{n: n, stroke: stroke}
	cache.mu.RLock()
	e := cache.icons[k]
	cache.mu.RUnlock()
	if e != nil {
		return e
	}
	e = &entry{doc: parse(source(d, stroke))}
	cache.mu.Lock()
	if cache.icons == nil {
		cache.icons = make(map[key]*entry)
	}
	if old := cache.icons[k]; old != nil {
		e = old
	} else {
		cache.icons[k] = e
	}
	cache.mu.Unlock()

	return e
}

// source returns the SVG document of d: for a stroked icon the classes of
// `svg.i` (index.html:64: no fill, a currentColor stroke, round caps and
// joins), for a filled one the `fill="currentColor"` of PLAY and PAUSE, and for
// the lesson row's triangle a stroke with the SVG defaults (miter joins, butt
// caps: that SVG has no class).
func source(d *def, stroke float32) string {
	const head = `<svg viewBox="0 0 24 24"`
	width := strconv.FormatFloat(float64(stroke), 'f', -1, 32)
	switch {
	case d.filled:
		return head + `><path d="` + d.path + `" fill="currentColor"/></svg>`
	case d.stroke > 0:
		fill := "none"
		if d.fillAlso {
			fill = "currentColor"
		}

		return head + ` fill="` + fill + `" stroke="currentColor" stroke-width="` + width + `"><path d="` + d.path + `"/></svg>`
	default:
		return head + ` fill="none" stroke="currentColor" stroke-width="` + width +
			`" stroke-linecap="round" stroke-linejoin="round"><path d="` + d.path + `"/></svg>`
	}
}

// parse reads a generated source; it cannot fail on the paths of the
// prototype, and a failure leaves a nil document that draws nothing.
func parse(src string) *svg.Document {
	d, err := svg.Parse([]byte(src))
	if err != nil {
		return nil
	}

	return d
}
