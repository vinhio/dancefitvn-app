// Adapted from jplayer-mobile/internal/app/fonts_test.go at 49de3c2.

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"maps"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"
	"unicode"
	"unicode/utf8"

	"dancefitvn-app/assets"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/bridge/text/coretext"
	"github.com/gdeskdev/desk/bridge/text/gotext"
	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstallFontsWritesOnce checks that fonts are written to the cache
// once, under their hash, Inter first; that a launch with the cache
// filled writes nothing; and that a truncated file is written again and
// the files of another build removed.
func TestInstallFontsWritesOnce(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"BarlowCondensed-ExtraBold.ttf": {Data: []byte("barlow 800")},
		"BarlowCondensed-Bold.ttf":      {Data: []byte("barlow 700")},
		"Inter-Regular.ttf":             {Data: []byte("inter 400")},
		"JetBrainsMono-Medium.ttf":      {Data: []byte("mono 500")},
		"OFL.txt":                       {Data: []byte("licence")},
	}
	names, err := fs.Glob(fsys, "*.ttf")
	require.NoError(t, err)
	dir := t.TempDir()

	first, err := installFonts(fsys, dir)
	require.NoError(t, err)
	assert.Len(t, first.paths, len(names))
	assert.Equal(t, len(names), first.written)
	assert.Zero(t, first.removed)
	assert.Contains(t, filepath.Base(first.paths[0]), "-Inter-")
	for _, p := range first.paths {
		got, err := os.ReadFile(p)
		require.NoError(t, err)
		base := filepath.Base(p)
		name := base[strings.IndexByte(base, '-')+1:]
		want, err := fs.ReadFile(fsys, name)
		require.NoError(t, err)
		assert.Equal(t, want, got, name)
		sum := sha256.Sum256(want)
		assert.Equal(t, hex.EncodeToString(sum[:4])+"-"+name, base)
	}

	// A launch with the cache filled writes and removes nothing.
	before := modTimes(t, first.paths)
	second, err := installFonts(fsys, dir)
	require.NoError(t, err)
	assert.Equal(t, first.paths, second.paths)
	assert.Zero(t, second.written)
	assert.Zero(t, second.removed)
	assert.Equal(t, before, modTimes(t, first.paths))

	// A truncated font is written again; another build's font and a
	// write cut short are removed; other files are left.
	require.NoError(t, os.Truncate(first.paths[3], 1))
	for _, name := range []string{"00000000-Old.ttf", ".font-123", "index.json"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600))
	}
	third, err := installFonts(fsys, dir)
	require.NoError(t, err)
	assert.Equal(t, 1, third.written)
	assert.Equal(t, 2, third.removed)
	assert.FileExists(t, filepath.Join(dir, "index.json"))
	assert.NoFileExists(t, filepath.Join(dir, "00000000-Old.ttf"))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, len(names)+1)
}

// TestFontSums checks that SHA256SUMS.txt lists every embedded font with
// its sha256 (else a launch hashes the font), and that a folder without
// the list, or with a malformed line, is hashed instead.
func TestFontSums(t *testing.T) {
	t.Parallel()

	fsys := appFonts(t)
	names, err := fs.Glob(fsys, "*.ttf")
	require.NoError(t, err)
	require.NotEmpty(t, names)
	sums := readSums(fsys)
	assert.Len(t, sums, len(names))
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		require.NoError(t, err)
		sum := sha256.Sum256(data)
		assert.Equal(t, hex.EncodeToString(sum[:]), sums[name], name)
	}

	font := []byte("font data")
	whole := sha256.Sum256(font)
	want := hex.EncodeToString(whole[:4]) + "-A.ttf"
	tests := []struct {
		name string
		sums string
	}{
		{"no list", ""},
		{"malformed line", "xyz  A.ttf\n"},
		{"short sum", "abcd  A.ttf\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := fstest.MapFS{"A.ttf": {Data: font}}
			if tc.sums != "" {
				m[sumsFile] = &fstest.MapFile{Data: []byte(tc.sums)}
			}
			in, err := installFonts(m, t.TempDir())
			require.NoError(t, err)
			require.Len(t, in.paths, 1)
			assert.Equal(t, want, filepath.Base(in.paths[0]))
		})
	}
}

// TestInstallFontsFails checks the errors of a folder without fonts and of
// a cache that cannot be written.
func TestInstallFontsFails(t *testing.T) {
	t.Parallel()

	blocked := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(blocked, nil, 0o600))
	tests := []struct {
		name string
		fsys fs.FS
		dir  string
		want error
	}{
		{"no fonts", fstest.MapFS{"OFL.txt": {Data: []byte("x")}}, t.TempDir(), core.ErrNotFound},
		{"cache is a file", fstest.MapFS{"A.ttf": {Data: []byte("x")}}, filepath.Join(blocked, "fonts"), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := installFonts(tc.fsys, tc.dir)
			require.Error(t, err)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			}
		})
	}
}

// TestRegisterFontsRejectsNonFont checks that a file Core Text finds is no
// font fails the registration (gotext skips such data silently).
func TestRegisterFontsRejectsNonFont(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" {
		t.Skip("only Core Text tells a file is no font")
	}

	err := RegisterFonts(fstest.MapFS{"Bad.ttf": {Data: []byte("not a font")}}, t.TempDir())
	require.ErrorIs(t, err, core.ErrInvalid)
}

// TestFacesResolve checks that every face the theme names draws from the
// file it names, on both engines, as the app registers the fonts: gotext
// (Android) given the files' data in RegisterFonts' order, as
// auto.RegisterFontFile gives it; Core Text (iOS, tvOS, and the Mac this
// runs on) once RegisterFonts registered the files. A face is told from
// the others by its width (the advances and kerning of its own file) and
// its ink, which tells the weights of the monospaced family apart; Inter
// 600, 700 and 800, which Core Text would collapse into Inter Bold by
// family name (GT15), and Barlow Condensed 700 and 800 are told apart by
// width alone. gotext's default face, a face naming no family, is Inter
// Regular: the first family registered.
func TestFacesResolve(t *testing.T) {
	t.Parallel()

	fsys := appFonts(t)
	in, err := installFonts(fsys, t.TempDir())
	require.NoError(t, err)
	sample := sampleText()
	refs := references(t, fsys, sample)

	// The theme names exactly the files of assets/fonts, and the Inter
	// and Barlow weights differ in width.
	files := slices.Sorted(maps.Keys(refs))
	var named []string
	for _, n := range theme.FaceNames() {
		named = append(named, n+".ttf")
	}
	slices.Sort(named)
	assert.Equal(t, files, named, "theme.FaceNames against assets/fonts")
	for _, set := range [][]string{
		{"Inter-Regular.ttf", "Inter-Medium.ttf", "Inter-SemiBold.ttf", "Inter-Bold.ttf", "Inter-ExtraBold.ttf"},
		{"BarlowCondensed-Bold.ttf", "BarlowCondensed-ExtraBold.ttf"},
	} {
		for i := 1; i < len(set); i++ {
			assert.Greater(t, refs[set[i]].width, refs[set[i-1]].width*1.005, "%s is wider than %s", set[i], set[i-1])
		}
	}
	t.Logf("widths of the sample at %v DIPs: %v", sampleSize, widths(refs))

	data := make([][]byte, 0, len(in.paths))
	for _, p := range in.paths {
		b, err := os.ReadFile(p)
		require.NoError(t, err)
		data = append(data, b)
	}
	engines := []struct {
		name   string
		naming theme.Naming
		make   func(t *testing.T) (text.Engine, error)
	}{
		{"gotext", theme.NamingGoText, func(*testing.T) (text.Engine, error) {
			return gotext.New(gotext.WithDirs(), gotext.WithFonts(data...))
		}},
		{"coretext", theme.NamingCoreText, func(t *testing.T) (text.Engine, error) {
			start := time.Now()
			require.NoError(t, RegisterFonts(fsys, t.TempDir()))
			t.Logf("RegisterFonts: %.1f ms with an empty cache", ms(time.Since(start)))

			return coretext.New()
		}},
	}
	for _, eng := range engines {
		t.Run(eng.name, func(t *testing.T) {
			t.Parallel()
			if eng.naming == theme.NamingCoreText && runtime.GOOS != "darwin" {
				t.Skip("Core Text runs on Apple systems only")
			}

			e, err := eng.make(t)
			require.NoError(t, err)
			t.Cleanup(func() { _ = e.Close() })
			faces := theme.NewFaces(eng.naming)
			names := theme.NewFaces(theme.NamingCoreText)
			for _, r := range []theme.Role{theme.RoleDisplay, theme.RoleBody, theme.RoleMono} {
				for w := text.Weight(100); w <= 900; w += 100 {
					face := faces.Face(r, sampleSize, w)
					file := names.Face(r, sampleSize, w).Family + ".ttf"
					got := measureFace(t, e, face, sample, file)
					want := refs[file]
					assert.InEpsilon(t, want.width, got.width, 0.002, "%s asked as %+v: width", file, face)
					assert.InEpsilon(t, want.ink, got.ink, 0.02, "%s asked as %+v: ink", file, face)
					assert.Equal(t, file, nearest(refs, got), "%s asked as %+v draws like another file", file, face)
				}
			}
			if eng.naming == theme.NamingGoText {
				got := measureFace(t, e, text.Face{Size: sampleSize}, sample, "default face")
				assert.Equal(t, "Inter-Regular.ttf", nearest(refs, got), "gotext's face naming no family")
			}
		})
	}
}

// TestFontsCoverStrings checks that every character of the app's strings
// (locales/vi.json, and the prototype's, local/strings.json, when the
// checkout has it) has a glyph in Inter and Barlow Condensed, and the
// characters JetBrains Mono sets (times, codes: ASCII) in it, each file
// alone: no .notdef, no other font. Barlow Condensed lacks two characters
// of body strings only, which Inter draws.
func TestFontsCoverStrings(t *testing.T) {
	t.Parallel()

	texts := slices.Collect(maps.Values(locales.MustTexts()))
	if data, err := os.ReadFile(filepath.Join("..", "..", "local", "strings.json")); err == nil {
		var more []string
		require.NoError(t, json.Unmarshal(data, &more))
		texts = append(texts, more...)
	}
	var b strings.Builder
	for _, s := range texts {
		for _, r := range s {
			if !strings.ContainsRune(b.String(), r) && !unicode.IsControl(r) {
				b.WriteRune(r)
			}
		}
	}
	all := b.String()
	var ascii strings.Builder
	for r := rune(0x20); r <= 0x7e; r++ {
		ascii.WriteRune(r)
	}
	// U+2011 (Wi\u2011Fi), U+2192 (the loop's "a → b") and U+2197 (a link's arrow) are in
	// body strings.
	barlowGaps := []rune{0x2011, 0x2192, 0x2197}

	fsys := appFonts(t)
	tests := []struct {
		file   string
		sample string
		gaps   []rune
	}{
		{"BarlowCondensed-Bold.ttf", all, barlowGaps},
		{"BarlowCondensed-ExtraBold.ttf", all, barlowGaps},
		{"Inter-Regular.ttf", all, nil},
		{"Inter-Medium.ttf", all, nil},
		{"Inter-SemiBold.ttf", all, nil},
		{"Inter-Bold.ttf", all, nil},
		{"Inter-ExtraBold.ttf", all, nil},
		{"JetBrainsMono-Medium.ttf", ascii.String(), nil},
		{"JetBrainsMono-Bold.ttf", ascii.String(), nil},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()

			data, err := fs.ReadFile(fsys, tc.file)
			require.NoError(t, err)
			e, err := gotext.New(gotext.WithDirs(), gotext.WithFonts(data))
			require.NoError(t, err)
			t.Cleanup(func() { _ = e.Close() })
			var l text.Layout
			require.NoError(t, e.Shape(tc.sample, text.Face{Size: sampleSize}, 1, 0, &l))
			require.NotEmpty(t, l.Glyphs)
			var missing []rune
			for _, g := range l.Glyphs {
				if g.Missing || g.Font != l.Glyphs[0].Font {
					r, _ := utf8.DecodeRuneInString(tc.sample[g.Index:])
					if r != ' ' {
						missing = append(missing, r)
					}
				}
			}
			assert.ElementsMatch(t, tc.gaps, missing, "%s lacks %q", tc.file, string(missing))
		})
	}
}

// appFonts returns the embedded fonts' folder.
func appFonts(t *testing.T) fs.FS {
	t.Helper()
	fsys, err := fs.Sub(assets.Fonts, "fonts")
	require.NoError(t, err)

	return fsys
}

// modTimes returns the modification time of each file.
func modTimes(t *testing.T, paths []string) []time.Time {
	t.Helper()
	out := make([]time.Time, 0, len(paths))
	for _, p := range paths {
		st, err := os.Stat(p)
		require.NoError(t, err)
		out = append(out, st.ModTime())
	}

	return out
}

// sampleSize is the size the faces are measured at, large enough for a
// width to tell the weights apart.
const sampleSize = 40

// sampleP0 is the P0 sample: a few phrases of the app in Vietnamese. It
// was the locales key p0.fonts.sample; only this test reads it, so it
// lives here (tests may hold Vietnamese text).
const sampleP0 = "Đăng nhập huấn luyện viên · Khoá học · Đùi Mông"

// sampleText returns what the faces are measured on: the P0 sample, the 134
// marked letters of Vietnamese, and Latin letters and digits.
func sampleText() string {
	return sampleP0 + " " + vietnameseLetters + " Hamburgefonstiv 0123456789"
}

// vietnameseLetters are the 134 letters of Vietnamese with a mark,
// precomposed (NFC): the 12 vowels with each of the 5 tone marks, the
// marked vowels alone, and d with stroke, in both cases.
const vietnameseLetters = "àáảãạăằắẳẵặâầấẩẫậèéẻẽẹêềếểễệìíỉĩịòóỏõọôồốổỗộơờớởỡợùúủũụưừứửữựỳýỷỹỵđ" +
	"ÀÁẢÃẠĂẰẮẲẴẶÂẦẤẨẪẬÈÉẺẼẸÊỀẾỂỄỆÌÍỈĨỊÒÓỎÕỌÔỒỐỔỖỘƠỜỚỞỠỢÙÚỦŨỤƯỪỨỬỮỰỲÝỶỸỴĐ"

// measure is how a text draws in a face: its width, and its ink (the sum of
// its glyphs' coverage), which tells the weights of a monospaced family
// apart where the width cannot.
type measure struct{ width, ink float32 }

// references returns, for each font file of fsys, how sample draws in it
// alone (a gotext engine with no other font).
func references(t *testing.T, fsys fs.FS, sample string) map[string]measure {
	t.Helper()
	names, err := fs.Glob(fsys, "*.ttf")
	require.NoError(t, err)
	out := make(map[string]measure, len(names))
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		require.NoError(t, err)
		e, err := gotext.New(gotext.WithDirs(), gotext.WithFonts(data))
		require.NoError(t, err)
		out[name] = measureFace(t, e, text.Face{Size: sampleSize}, sample, name)
		require.NoError(t, e.Close())
	}

	return out
}

// measureFace shapes sample in face with e and measures it, failing the
// test if a glyph is missing or comes from another font than the first.
func measureFace(t *testing.T, e text.Engine, face text.Face, sample, file string) measure {
	t.Helper()
	var l text.Layout
	require.NoError(t, e.Shape(sample, face, 1, 0, &l), file)
	require.NotEmpty(t, l.Glyphs, file)
	m := measure{width: l.Size.W}
	for _, g := range l.Glyphs {
		require.False(t, g.Missing, "%s: glyph of %q missing", file, sample[g.Index:])
		require.Equal(t, l.Glyphs[0].Font, g.Font, "%s: %q from another font", file, sample[g.Index:])
		mask, _, err := e.Glyph(g.Font, g.ID, face.Size)
		require.NoError(t, err, file)
		if mask == nil {
			continue
		}
		for y := range mask.Height {
			for _, a := range mask.Pix[y*mask.Stride : y*mask.Stride+mask.Width] {
				m.ink += float32(a) / 255
			}
		}
	}

	return m
}

// nearest returns the file of refs whose measure is nearest to m, by the
// sum of the relative differences of width and ink.
func nearest(refs map[string]measure, m measure) string {
	best, bestD := "", math.Inf(1)
	for name, r := range refs {
		d := math.Abs(float64(m.width/r.width-1)) + math.Abs(float64(m.ink/r.ink-1))
		if d < bestD {
			best, bestD = name, d
		}
	}

	return best
}

// widths returns the widths of refs by file, rounded, for the log.
func widths(refs map[string]measure) map[string]float32 {
	out := make(map[string]float32, len(refs))
	for name, r := range refs {
		out[name] = float32(math.Round(float64(r.width)*10) / 10)
	}

	return out
}
