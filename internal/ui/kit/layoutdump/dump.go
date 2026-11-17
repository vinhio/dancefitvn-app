// Adapted from jplayer-mobile/internal/app/shots_test.go at 49de3c2 (shotLayout,
// shotElement, writeShot: the walk of the widget tree that records every named
// widget's box, numbering of repeated names, and the picture and layout files
// written side by side). Kept: the walk, the PNG + JSON pair, the box taken
// with WindowFrame. Changed: the JSON is the DanceFITVN schema of
// cmd/shotcmp/schema.go (a key per widget, style as CSS strings) instead of
// jPlayer's `name/index/x/y/w/h`; every named widget is recorded (jPlayer
// filtered by a table of names); the `posedFrame` correction for the cover and
// the mini player is left out (nothing here is drawn under a transform at rest).

// Package layoutdump writes what an app state looks like in the form the
// prototype's capture (local/tools/capture_kit.py) writes it: a PNG and a
// layout.json holding the box of every named widget, so that cmd/shotcmp can
// compare the two. The schema is the one of cmd/shotcmp/schema.go; the types
// are repeated here because a command's package cannot be imported.
package layoutdump

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// iconName is the key of an icon, `svg.i` (index.html:64) as the capture names it.
const iconName = "svg.i"

// progFillName is the key of the fill of a bar, `.prog i`; fieldLabelName the
// key of the label of a Field.
const (
	progFillName   = "i"
	fieldLabelName = "label"
)

// Unit is the layout's unit: DIPs, the same number as the prototype's CSS px at
// scale 1.
const Unit = "dip"

// Layout is a layout.json (cmd/shotcmp/schema.go Layout).
type Layout struct {
	ID       string    `json:"id"`
	Platform string    `json:"platform"` // "mobile" or "tv"
	Unit     string    `json:"unit"`
	Screen   Screen    `json:"screen"`
	Elements []Element `json:"elements"`
}

// Screen is the size of the window in DIPs.
type Screen struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Element is one named widget: Key is its Name, or Name#Index from the second
// widget of that name, in tree order (the prototype's capture numbers its
// elements the same way).
type Element struct {
	Key    string  `json:"key"`
	Name   string  `json:"name"`
	Index  int     `json:"index"`
	Path   string  `json:"path"` // the Go type of the widget, informational
	Parent string  `json:"parent"`
	Tag    string  `json:"tag"`
	Text   string  `json:"text"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	W      float64 `json:"w"`
	H      float64 `json:"h"`
	Style  Style   `json:"style"`
}

// Style is the computed style of an element as CSS strings; a field left empty
// is "not recorded" and cmd/shotcmp does not compare it. It is kit.CSS, which
// the widgets answer.
type Style = kit.CSS

// Styler is implemented by a widget that knows its own computed style; the
// dump asks it for the element's style and text (every kit.Label and the kit
// widgets do). Without it the dump records the box only.
type Styler interface {
	// LayoutStyle returns the style and the visible text of the widget.
	LayoutStyle(c *ui.Context) (s Style, text string)
}

// Transformer is implemented by a widget that paints itself under a transform
// about its own box (a focused TV control scaled to 1.06 about its centre). The
// prototype's capture records the box such an element has after its CSS
// transform (`getBoundingClientRect`: the focused `h1` of TV04 is 339.76 wide,
// 1.06 times its 320.57), and so for everything inside it; Dump does the same.
type Transformer interface {
	// PaintTransform returns the map, in the widget's own coordinates (its
	// top-left corner is the origin), that its painting is under now, and
	// false when there is none.
	PaintTransform() (m scene.Affine, ok bool)
}

// xform is a Transformer's map with the window position of its widget's
// top-left corner, where its coordinates start.
type xform struct {
	origin geom.Point
	m      scene.Affine
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Dump records every visible named widget under root, in tree order, with the
// boxes in DIPs from the window's top left. A hidden widget and what is under
// it are left out, as a `display:none` element is in the capture.
//
// The capture numbers the elements of a name from 1 in document order, and
// that includes what a kitdemo page does not draw. Two ways to line the
// numbers up with it: a Name ending in "#n" ("playVideo:set35-3#2") is the
// n-th element of that name, and skip[name] elements of the name are counted
// as already seen before the first one of the page.
func Dump(c *ui.Context, root ui.Widget, id, platform string, size geom.Size, skip map[string]int) Layout {
	lay := Layout{ID: id, Platform: platform, Unit: Unit, Screen: Screen{W: float64(size.W), H: float64(size.H)}}
	counts := map[string]int{}
	for k, n := range skip {
		counts[k] = n
	}
	add := func(name string, r geom.Rect, tag string) *Element {
		n := 0
		if base, num, ok := pinned(name); ok {
			name, n = base, num
			counts[name] = n
		} else {
			counts[name]++
			n = counts[name]
		}
		e := Element{
			Key: name, Name: name, Index: n, Tag: tag,
			X: round2(r.X), Y: round2(r.Y), W: round2(r.W), H: round2(r.H),
		}
		if n > 1 {
			e.Key = name + "#" + strconv.Itoa(n)
		}
		lay.Elements = append(lay.Elements, e)

		return &lay.Elements[len(lay.Elements)-1]
	}
	var walk func(w, parent ui.Widget, xs []xform)
	walk = func(w, parent ui.Widget, xs []xform) {
		b := w.Node()
		if b.Hidden {
			return
		}
		if t, ok := w.(Transformer); ok {
			if m, on := t.PaintTransform(); on {
				xs = append(xs[:len(xs):len(xs)], xform{origin: b.WindowFrame().Min(), m: m})
			}
		}
		if b.Name != "" {
			r := transformed(b.WindowFrame(), xs)
			e := add(b.Name, r, fmt.Sprintf("%T", w))
			e.Style, e.Text = styleOf(c, w)
			if f, ok := parent.(*kit.Field); ok && f.Required && b.Name == fieldLabelName {
				// the `<em>*</em>` of the label is part of its text
				e.Text += "*"
			}
			// A Button draws its icon itself; the prototype's button holds an
			// svg.i, so the icon is recorded as one, right after the button.
			// The play triangle is the class-less `PLAY` svg (index.html), which the
			// capture does not record.
			if btn, ok := w.(*kit.Button); ok && btn.Icon != icon.Play {
				if ir := btn.IconRect(); ir.W > 0 {
					add(iconName, geom.XYWH(r.X+ir.X, r.Y+ir.Y, ir.W, ir.H), "icon of kit.Button")
				}
			}
			// The same for a Chip.
			if chip, ok := w.(*kit.Chip); ok {
				if ir := chip.IconRect(); ir.W > 0 {
					add(iconName, geom.XYWH(r.X+ir.X, r.Y+ir.Y, ir.W, ir.H), "icon of kit.Chip")
				}
			}
			// The fill of a Progress is the `i` of `.prog i`.
			if pr, ok := w.(*kit.Progress); ok {
				if x, y, fw, fh := pr.FillRect(); fw > 0 {
					fe := add(progFillName, geom.XYWH(r.X+x, r.Y+y, fw, fh), "fill of kit.Progress")
					fe.Style = pr.FillStyle(c)
				}
			}
		}
		for ch := range b.Children() {
			walk(ch, w, xs)
		}
	}
	walk(root, nil, nil)

	return lay
}

// Shot dumps the tester's window (skip as for Dump) and writes <dir>/<id>.png and
// <dir>/<id>.layout.json: the picture at the tester's scale, the layout in
// DIPs. Draw the state before (RunUntilIdle).
func Shot(tt *ui.Tester, id, platform string, size geom.Size, dir string, skip map[string]int) error {
	lay := Dump(tt.View().Context(), tt.View().Root(), id, platform, size, skip)

	return lay.Write(dir, tt.Snapshot())
}

// Write writes the layout as <dir>/<ID>.layout.json and, when img is not nil,
// the picture as <dir>/<ID>.png; dir is made when missing.
func (l Layout) Write(dir string, img image.Image) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("layoutdump: %w", err)
	}
	if img != nil {
		f, err := os.Create(filepath.Join(dir, l.ID+".png"))
		if err != nil {
			return fmt.Errorf("layoutdump: %w", err)
		}
		werr := png.Encode(f, img)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return fmt.Errorf("layoutdump: png %s: %w", l.ID, werr)
		}
	}
	data, err := json.MarshalIndent(l, "", " ")
	if err != nil {
		return fmt.Errorf("layoutdump: json %s: %w", l.ID, err)
	}
	if err := os.WriteFile(filepath.Join(dir, l.ID+".layout.json"), data, 0o600); err != nil {
		return fmt.Errorf("layoutdump: %w", err)
	}

	return nil
}

// Find returns the element with the key, and whether there is one.
func (l Layout) Find(key string) (Element, bool) {
	for _, e := range l.Elements {
		if e.Key == key {
			return e, true
		}
	}

	return Element{}, false
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// transformed returns r, a window frame, as it is painted under the transforms
// of the widget and of its ancestors, the innermost first.
func transformed(r geom.Rect, xs []xform) geom.Rect {
	for i := len(xs) - 1; i >= 0; i-- {
		x := xs[i]
		r = x.m.Bounds(r.Translate(geom.Pt(-x.origin.X, -x.origin.Y))).Translate(x.origin)
	}

	return r
}

// pinned splits a name that ends in "#n": the base name and n.
func pinned(name string) (base string, n int, ok bool) {
	i := strings.LastIndexByte(name, '#')
	if i <= 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(name[i+1:])
	if err != nil || n < 1 {
		return "", 0, false
	}

	return name[:i], n, true
}

// styleOf returns the style and visible text of w: its own, when it is a
// Styler; nothing otherwise.
func styleOf(c *ui.Context, w ui.Widget) (Style, string) {
	if s, ok := w.(Styler); ok {
		return s.LayoutStyle(c)
	}

	return Style{}, ""
}

// CSSColor writes c as Chrome's computed color (see kit.CSSColor).
func CSSColor(c color.Color) string { return kit.CSSColor(c) }

// round2 rounds v to two decimals, as the capture does.
func round2(v float32) float64 { return math.Round(float64(v)*100) / 100 }
