//go:build !ios

package app

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"dancefitvn-app/assets"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/bridge/text/gotext"
	"github.com/gdeskdev/desk/bridge/text/typeset"
	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// appTextEngine is the name the Android text engine registers under.
const appTextEngine = "dancefitvn"

// colorFont marks the FontIDs of the color engine's layouts, so that Glyph
// and ColorGlyph know which engine numbered a glyph's font.
const colorFont text.FontID = 1 << 31

// emojiLang is the language fonts.xml gives the system's emoji families
// (Unicode's emoji script, Zsye).
const emojiLang = "und-Zsye"

// emojiFontFiles are the emoji fonts looked for when fonts.xml names none:
// AOSP's, the bitmap one before Android 15, and Samsung's.
var emojiFontFiles = []string{"NotoColorEmoji.ttf", "NotoColorEmojiLegacy.ttf", "SamsungColorEmoji.ttf"}

// maxEmojiFonts bounds the emoji fonts read: the main font and the flags'
// font that Android 15 splits from it.
const maxEmojiFonts = 2

// colorText is the text engine of Android: gotext lays out, allocation-free,
// every paragraph its fonts draw; a paragraph with a character no font of
// gotext has, which on Android is an emoji (gotext reads no color glyph and
// its fallbacks name no emoji font), is laid out again by typeset, given
// the app's fonts and the system's color emoji font, and drawn in color
// (text.ColorEngine). typeset is made at the first such paragraph, from
// makeColor; when it cannot be (no emoji font) gotext's layout stays, its
// missing glyphs drawn as boxes. typeset's layout is kept only when it
// misses fewer characters than gotext's, as for text in a script the
// system has a font for and typeset, reading no system font, does not.
//
// Like every engine it serves one goroutine at a time.
type colorText struct {
	base      text.Engine // gotext
	color     text.Engine // typeset; nil until made, or when it cannot be
	tried     bool        // whether makeColor ran
	makeColor func() (text.Engine, error)
	alt       text.Layout // typeset's layout of the paragraph, before it is chosen
	one       [1]text.Run // the run of a paragraph in one face
	logf      func(string, ...any)
}

var (
	_ text.Engine          = (*colorText)(nil)
	_ text.RunShaper       = (*colorText)(nil)
	_ text.PolicyShaper    = (*colorText)(nil)
	_ text.PolicyRunShaper = (*colorText)(nil)
	_ text.ColorEngine     = (*colorText)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// emojiFonts holds the system's emoji fonts, read once for every engine:
// several engines may serve several goroutines, the bytes are the same.
var emojiFonts struct {
	once sync.Once
	data [][]byte
	err  error
}

// embeddedFonts holds the app's embedded fonts, Inter first, read once.
var embeddedFonts = sync.OnceValues(func() ([][]byte, error) { return fontData(assets.Fonts, "fonts") })

// newColorText returns the engine over base (gotext), making the color
// engine with makeColor the first time a paragraph needs it. logf, nil
// for none, tells why it could not be made.
func newColorText(base text.Engine, makeColor func() (text.Engine, error), logf func(string, ...any)) *colorText {
	return &colorText{base: base, makeColor: makeColor, logf: logf}
}

// newAndroidText makes the engine of the app on Android: gotext with the
// fonts RegisterFonts gave it, and typeset over the app's fonts and the
// emoji fonts the font lists name (in dirs) when a paragraph needs it.
func newAndroidText(lists, dirs []string, logf func(string, ...any)) (text.Engine, error) {
	base, err := gotext.New()
	if err != nil {
		return nil, err
	}

	return newColorText(base, func() (text.Engine, error) {
		emoji, err := readEmojiFonts(lists, dirs)
		if err != nil {
			return nil, err
		}
		app, err := embeddedFonts()
		if err != nil {
			return nil, err
		}

		return newColorEngine(app, emoji)
	}, logf), nil
}

// newColorEngine returns typeset over the given fonts alone, the app's
// first (Inter first: the family of a face naming none), then the emoji
// fonts. It reads no system font: scanning Android's takes seconds.
func newColorEngine(app, emoji [][]byte) (text.Engine, error) {
	return typeset.New(typeset.WithoutSystemFonts(), typeset.WithFonts(slices.Concat(app, emoji)...))
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Shape lays s out as ShapeRuns does, in one face.
func (e *colorText) Shape(s string, face text.Face, scale, maxWidth float32, out *text.Layout) error {
	e.one[0] = text.Run{End: len(s), Face: face}

	return e.ShapeRunsPolicy(s, e.one[:], scale, maxWidth, text.WrapPolicyUnknown, out)
}

// ShapePolicy lays s out as Shape does, under policy.
func (e *colorText) ShapePolicy(s string, face text.Face, scale, maxWidth float32, policy text.WrapPolicy, out *text.Layout) error {
	e.one[0] = text.Run{End: len(s), Face: face}

	return e.ShapeRunsPolicy(s, e.one[:], scale, maxWidth, policy, out)
}

// ShapeRuns lays rich text out as ShapeRunsPolicy does, under the
// engines' own wrapping.
func (e *colorText) ShapeRuns(s string, runs []text.Run, scale, maxWidth float32, out *text.Layout) error {
	return e.ShapeRunsPolicy(s, runs, scale, maxWidth, text.WrapPolicyUnknown, out)
}

// ShapeRunsPolicy lays s out with gotext and, when a character has no
// glyph there, with typeset too, keeping the layout with fewer missing
// characters (gotext's on a tie). It allocates nothing when gotext draws
// every character and its caches are warm.
func (e *colorText) ShapeRunsPolicy(s string, runs []text.Run, scale, maxWidth float32, policy text.WrapPolicy, out *text.Layout) error {
	if err := shapeWith(e.base, s, runs, scale, maxWidth, policy, out); err != nil {
		return err
	}
	missing := countMissing(out)
	if missing == 0 || !e.colorEngine() {
		return nil
	}
	if err := shapeWith(e.color, s, runs, scale, maxWidth, policy, &e.alt); err != nil {
		return nil //nolint:nilerr // gotext's layout stands
	}
	if countMissing(&e.alt) >= missing {
		return nil
	}
	for i := range e.alt.Glyphs {
		e.alt.Glyphs[i].Font |= colorFont
	}
	// The layouts swap their memory: out's goes on as the next scratch.
	*out, e.alt = e.alt, *out

	return nil
}

// Glyph rasterizes a glyph with the engine that numbered its font.
func (e *colorText) Glyph(font text.FontID, id text.GlyphID, px float32) (*scene.Mask, geom.Point, error) {
	if font&colorFont != 0 {
		if e.color == nil {
			return nil, geom.Point{}, core.Wrap("app", "glyph", fmt.Errorf("font %#x: %w", font, core.ErrNotFound))
		}

		return e.color.Glyph(font&^colorFont, id, px)
	}

	return e.base.Glyph(font, id, px)
}

// ColorGlyph rasterizes a glyph of the color engine in its colors; a glyph
// of gotext is never in color.
func (e *colorText) ColorGlyph(font text.FontID, id text.GlyphID, px float32) (*scene.Image, geom.Point, bool, error) {
	if font&colorFont == 0 {
		return nil, geom.Point{}, false, nil
	}
	ce, ok := e.color.(text.ColorEngine)
	if !ok {
		return nil, geom.Point{}, false, nil
	}

	return ce.ColorGlyph(font&^colorFont, id, px)
}

// Metrics returns gotext's metrics of face.
func (e *colorText) Metrics(face text.Face) (text.Metrics, error) { return e.base.Metrics(face) }

// Close closes both engines.
func (e *colorText) Close() error {
	var errs []error
	errs = append(errs, e.base.Close())
	if e.color != nil {
		errs = append(errs, e.color.Close())
	}

	return errors.Join(errs...)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// colorEngine makes the color engine once, and tells whether there is one.
func (e *colorText) colorEngine() bool {
	if !e.tried {
		e.tried = true
		c, err := e.makeColor()
		if err != nil {
			if e.logf != nil {
				e.logf("dancefitvn: text: no color engine, emoji draw as boxes: %v", err)
			}
		} else {
			e.color = c
		}
	}

	return e.color != nil
}

// shapeWith lays s out with eng, under policy when eng takes one.
func shapeWith(eng text.Engine, s string, runs []text.Run, scale, maxWidth float32, policy text.WrapPolicy, out *text.Layout) error {
	if policy != text.WrapPolicyUnknown {
		if ps, ok := eng.(text.PolicyRunShaper); ok {
			return ps.ShapeRunsPolicy(s, runs, scale, maxWidth, policy, out)
		}
	}
	if rs, ok := eng.(text.RunShaper); ok {
		return rs.ShapeRuns(s, runs, scale, maxWidth, out)
	}

	return eng.Shape(s, runs[0].Face, scale, maxWidth, out)
}

// countMissing returns how many glyphs of l no font has.
func countMissing(l *text.Layout) int {
	n := 0
	for i := range l.Glyphs {
		if l.Glyphs[i].Missing {
			n++
		}
	}

	return n
}

// readEmojiFonts reads the system's emoji fonts once: the files the first
// of lists (font_fallback.xml, fonts.xml) that names any gives the
// und-Zsye families, in order, found in dirs, at most maxEmojiFonts; else
// the first of emojiFontFiles found there. None is core.ErrNotFound.
func readEmojiFonts(lists, dirs []string) ([][]byte, error) {
	emojiFonts.once.Do(func() { emojiFonts.data, emojiFonts.err = loadEmojiFonts(lists, dirs) })

	return emojiFonts.data, emojiFonts.err
}

// loadEmojiFonts is readEmojiFonts without the cache.
func loadEmojiFonts(lists, dirs []string) ([][]byte, error) {
	var names []string
	for _, list := range lists {
		f, err := os.Open(list) //nolint:gosec // G304: the system's font list
		if err != nil {
			continue
		}
		names = emojiFileNames(f)
		_ = f.Close() // read-only
		if len(names) > 0 {
			break
		}
	}
	// The files of the lists are parts of one emoji set (the flags apart
	// since Android 15); the guesses are alternatives, one is enough.
	parts := len(names) > 0
	if !parts {
		names = emojiFontFiles
	}
	var out [][]byte
	for _, name := range names {
		data, ok := readFontIn(dirs, name)
		if !ok {
			continue
		}
		out = append(out, data)
		if !parts || len(out) == maxEmojiFonts {
			break
		}
	}
	if len(out) == 0 {
		return nil, core.Wrap("app", "emoji fonts", fmt.Errorf("%s in %s: %w", strings.Join(names, ", "), strings.Join(dirs, ", "), core.ErrNotFound))
	}

	return out, nil
}

// readFontIn reads the font file name in the first of dirs that has it.
func readFontIn(dirs []string, name string) ([]byte, bool) {
	for _, dir := range dirs {
		data, err := os.ReadFile(filepath.Join(dir, filepath.Base(name))) //nolint:gosec // G304: a system font
		if err == nil {
			return data, true
		}
	}

	return nil, false
}

// emojiFileNames returns the font files of the families fonts.xml (AOSP's
// format: <familyset><family lang="..."><font>File.ttf</font>) gives the
// emoji language, in order. A file it cannot read gives what it read.
func emojiFileNames(r io.Reader) []string {
	dec := xml.NewDecoder(r)
	var names []string
	inEmoji, inFont := false, false
	var buf bytes.Buffer
	for {
		tok, err := dec.Token()
		if err != nil {
			return names
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "family":
				inEmoji = hasLang(t.Attr, emojiLang)
			case "font":
				inFont = inEmoji
				buf.Reset()
			default:
			}
		case xml.CharData:
			if inFont {
				buf.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "font":
				if name := strings.TrimSpace(buf.String()); inFont && name != "" && !slices.Contains(names, name) {
					names = append(names, name)
				}
				inFont = false
			case "family":
				inEmoji = false
			default:
			}
		default:
		}
	}
}

// hasLang tells whether attrs hold a lang attribute listing lang (it may
// list several, separated by spaces).
func hasLang(attrs []xml.Attr, lang string) bool {
	for _, a := range attrs {
		if a.Name.Local == "lang" && slices.Contains(strings.Fields(a.Value), lang) {
			return true
		}
	}

	return false
}

// fontData returns the bytes of the .ttf files of dir in fsys, Inter's
// first (firstRank), as RegisterFonts orders them.
func fontData(fsys fs.FS, dir string) ([][]byte, error) {
	names, err := fs.Glob(fsys, path.Join(dir, "*.ttf"))
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, core.Wrap("app", "font data", fmt.Errorf("%s: no .ttf file: %w", dir, core.ErrNotFound))
	}
	slices.SortStableFunc(names, func(a, b string) int { return firstRank(a) - firstRank(b) })
	out := make([][]byte, 0, len(names))
	for _, n := range names {
		data, err := fs.ReadFile(fsys, n)
		if err != nil {
			return nil, err
		}
		out = append(out, data)
	}

	return out, nil
}
