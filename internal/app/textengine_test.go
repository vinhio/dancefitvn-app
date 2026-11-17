//go:build !ios

package app

import (
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/bridge/text/gotext"
	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The texts of the tests, decomposed (NFD) where the API may send them so.
const (
	cowboy      = "\U0001F920"
	grin        = "\U0001F600"
	wordsNFC    = "B\u1ee5ng N\u1eafng Th\u01b0\u01a1ng"
	wordsNFD    = "Bu\u0323ng Na\u0306\u0301ng Thu\u031bo\u031bng"
	latinOnly   = "Set 35 - " + wordsNFC
	withEmoji   = "Cowboy " + cowboy + " " + wordsNFC
	emojiFontTD = "testdata/NotoColorEmoji-Subset.ttf"
)

// newTestColorText returns the Android engine over the app's fonts alone
// (no system font: gotext reads an empty folder) and, unless noEmoji, the
// test emoji font; made counts the color engine's makings.
func newTestColorText(t testing.TB, noEmoji bool, made *atomic.Int32) *colorText {
	t.Helper()
	app, err := embeddedFonts()
	require.NoError(t, err)
	base, err := gotext.New(gotext.WithDirs(t.TempDir()), gotext.WithFonts(app...))
	require.NoError(t, err)
	var emoji [][]byte
	if !noEmoji {
		data, err := os.ReadFile(emojiFontTD)
		require.NoError(t, err)
		emoji = [][]byte{data}
	}
	e := newColorText(base, func() (text.Engine, error) {
		if made != nil {
			made.Add(1)
		}

		return newColorEngine(app, emoji)
	}, nil)
	t.Cleanup(func() { _ = e.Close() })

	return e
}

// body is Inter at the size of the app's body text, as gotext is asked.
func body(weight text.Weight) text.Face {
	return theme.NewFaces(theme.NamingGoText).Face(theme.RoleBody, 15, weight)
}

func TestColorTextLatinStaysOnGotext(t *testing.T) {
	t.Parallel()

	e := newTestColorText(t, false, nil)
	var got text.Layout
	require.NoError(t, e.Shape(latinOnly, body(400), 1, 0, &got))
	for _, g := range got.Glyphs {
		assert.False(t, g.Missing)
		assert.Zero(t, g.Font&colorFont, "gotext lays out text its fonts draw")
	}
	assert.Nil(t, e.color, "no paragraph needed the color engine")
}

func TestColorTextDrawsEmojiInColor(t *testing.T) {
	t.Parallel()

	var made atomic.Int32
	e := newTestColorText(t, false, &made)
	var got text.Layout
	require.NoError(t, e.Shape(withEmoji, body(400), 3, 0, &got))
	emoji := strings.Index(withEmoji, cowboy)
	sawEmoji := false
	for _, g := range got.Glyphs {
		require.False(t, g.Missing, "glyph at %d", g.Index)
		require.NotZero(t, g.Font&colorFont, "the paragraph is typeset's")
		img, _, isColor, err := e.ColorGlyph(g.Font, g.ID, 45)
		require.NoError(t, err)
		if int(g.Index) == emoji {
			sawEmoji = true
			assert.True(t, isColor, "the emoji draws in color")
			require.NotNil(t, img)
			assert.Positive(t, img.Width)

			continue
		}
		assert.False(t, isColor, "a letter at %d is no color glyph", g.Index)
		if g.Index > 0 && withEmoji[g.Index] != ' ' {
			mask, _, err := e.Glyph(g.Font, g.ID, 45)
			require.NoError(t, err)
			assert.NotNil(t, mask, "a letter at %d draws a mask", g.Index)
		}
	}
	assert.True(t, sawEmoji)

	require.NoError(t, e.Shape(grin+" "+wordsNFC, body(700), 1, 0, &got))
	assert.Equal(t, int32(1), made.Load(), "the color engine is made once")
}

// TestColorTextMatchesGotextMetrics checks that typeset, given the same
// fonts, lays the letters out as gotext does: same family, same weight,
// nearly the same advances (typeset applies the kerning of GPOS, gotext
// that of kern), for each family and weight the app asks for.
func TestColorTextMatchesGotextMetrics(t *testing.T) {
	t.Parallel()

	faces := theme.NewFaces(theme.NamingGoText)
	for _, f := range []text.Face{
		faces.Face(theme.RoleBody, 15, 400), faces.Face(theme.RoleBody, 15, 600),
		faces.Face(theme.RoleBody, 15, 800), faces.Face(theme.RoleDisplay, 34, 700),
		faces.Face(theme.RoleDisplay, 34, 800), faces.Face(theme.RoleMono, 12, 500),
	} {
		t.Run(fmt.Sprintf("%s/%d", f.Family, f.Weight), func(t *testing.T) {
			t.Parallel()

			e := newTestColorText(t, false, nil)
			var plain, mixed text.Layout
			require.NoError(t, e.Shape(wordsNFC, f, 1, 0, &plain))
			require.NoError(t, e.Shape(wordsNFC+" "+cowboy, f, 1, 0, &mixed))
			require.NotZero(t, mixed.Glyphs[0].Font&colorFont)
			require.GreaterOrEqual(t, len(mixed.Glyphs), len(plain.Glyphs))
			last := plain.Glyphs[len(plain.Glyphs)-1]
			end := last.X + last.Advance
			mlast := mixed.Glyphs[len(plain.Glyphs)-1]
			assert.InDelta(t, end, mlast.X+mlast.Advance, float64(end)*0.02, "the words are as wide on both engines")
		})
	}
}

func TestColorTextWithoutEmojiFont(t *testing.T) {
	t.Parallel()

	var made atomic.Int32
	app, err := embeddedFonts()
	require.NoError(t, err)
	base, err := gotext.New(gotext.WithDirs(t.TempDir()), gotext.WithFonts(app...))
	require.NoError(t, err)
	var logged atomic.Int32
	e := newColorText(base, func() (text.Engine, error) {
		made.Add(1)

		return nil, core.ErrNotFound
	}, func(string, ...any) { logged.Add(1) })
	t.Cleanup(func() { _ = e.Close() })

	var got text.Layout
	for range 3 {
		require.NoError(t, e.Shape(withEmoji, body(400), 1, 0, &got))
	}
	assert.Equal(t, int32(1), made.Load(), "tried once")
	assert.Equal(t, int32(1), logged.Load(), "told once")
	missing := 0
	for _, g := range got.Glyphs {
		assert.Zero(t, g.Font&colorFont)
		if g.Missing {
			missing++
		}
	}
	assert.Equal(t, 1, missing, "the emoji stays a box, the rest draws")
}

func TestColorTextKeepsGotextOnATie(t *testing.T) {
	t.Parallel()

	// No emoji font: typeset misses the emoji as gotext does.
	e := newTestColorText(t, true, nil)
	var got text.Layout
	require.NoError(t, e.Shape(withEmoji, body(400), 1, 0, &got))
	require.NotNil(t, e.color)
	for _, g := range got.Glyphs {
		assert.Zero(t, g.Font&colorFont, "gotext's layout stands")
	}
}

func TestColorTextRunsAndPolicy(t *testing.T) {
	t.Parallel()

	e := newTestColorText(t, false, nil)
	s := wordsNFC + " " + cowboy + " Set 35"
	split := len(wordsNFC) + 1
	runs := []text.Run{{End: split, Face: body(400)}, {End: len(s), Face: body(700)}}
	var got text.Layout
	require.NoError(t, e.ShapeRunsPolicy(s, runs, 1, 60, text.WrapGraphemes, &got))
	assert.Greater(t, len(got.Lines), 1, "wrapped to 60 DIPs")
	for _, g := range got.Glyphs {
		assert.False(t, g.Missing)
		want := uint16(0)
		if int(g.Index) >= split {
			want = 1
		}
		assert.Equal(t, want, g.Run, "glyph at %d", g.Index)
	}
	require.NoError(t, e.ShapePolicy(s, body(400), 1, 0, text.WrapWords, &got))
	require.NoError(t, e.ShapeRuns(s, runs, 1, 0, &got))
	assert.Len(t, got.Lines, 1)
}

func TestColorTextGlyphOfUnmadeColorFont(t *testing.T) {
	t.Parallel()

	e := newTestColorText(t, false, nil)
	_, _, err := e.Glyph(colorFont|1, 1, 15)
	require.ErrorIs(t, err, core.ErrNotFound)
	_, _, ok, err := e.ColorGlyph(colorFont|1, 1, 15)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestColorTextDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	e := newTestColorText(t, false, nil)
	var l text.Layout
	f := body(400)
	require.NoError(t, e.Shape(latinOnly, f, 3, 300, &l)) // warms the caches
	g := l.Glyphs[0]
	assert.Zero(t, testing.AllocsPerRun(100, func() { _ = e.Shape(latinOnly, f, 3, 300, &l) }), "Shape")
	assert.Zero(t, testing.AllocsPerRun(100, func() { _, _, _, _ = e.ColorGlyph(g.Font, g.ID, 45) }), "ColorGlyph of gotext")
	assert.Zero(t, testing.AllocsPerRun(100, func() { _, _ = e.Metrics(f) }), "Metrics")
}

func BenchmarkColorTextShape(b *testing.B) {
	for _, bc := range []struct{ name, s string }{{"latin", latinOnly}, {"emoji", withEmoji}} {
		b.Run(bc.name, func(b *testing.B) {
			e := newTestColorText(b, false, nil)
			var l text.Layout
			f := body(400)
			b.ReportAllocs()
			for b.Loop() {
				if err := e.Shape(bc.s, f, 3, 300, &l); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkColorEngineMake measures making typeset over the app's fonts and
// the full emoji font at $DFV_EMOJI_FONT (Android's is about 10 MB), what
// the first emoji costs the UI thread once. It skips without the variable.
func BenchmarkColorEngineMake(b *testing.B) {
	p := os.Getenv("DFV_EMOJI_FONT")
	if p == "" {
		b.Skip("DFV_EMOJI_FONT names a full color emoji font")
	}
	emoji, err := os.ReadFile(p) //nolint:gosec // G304: the bench's font
	require.NoError(b, err)
	app, err := embeddedFonts()
	require.NoError(b, err)
	b.ReportAllocs()
	for b.Loop() {
		e, err := newColorEngine(app, [][]byte{emoji})
		if err != nil {
			b.Fatal(err)
		}
		var l text.Layout
		if err := e.Shape(withEmoji, body(400), 3, 0, &l); err != nil {
			b.Fatal(err)
		}
		_ = e.Close()
	}
}

func TestEmojiFileNames(t *testing.T) {
	t.Parallel()

	const fontsXML = `<?xml version="1.0" encoding="utf-8"?>
<familyset version="23">
  <family name="sans-serif">
    <font weight="400" style="normal">Roboto-Regular.ttf
      <axis tag="wght" stylevalue="400" />
    </font>
  </family>
  <family lang="und-Zsye" ignore="true">
    <font weight="400" style="normal">NotoColorEmojiLegacy.ttf</font>
  </family>
  <family lang="und-Zsye">
    <font weight="400" style="normal">NotoColorEmoji.ttf</font>
  </family>
  <family lang="und-Zsye">
    <font weight="400" style="normal"> NotoColorEmojiFlags.ttf </font>
  </family>
  <family lang="und-Zsym">
    <font weight="400" style="normal">NotoSansSymbols-Regular-Subsetted2.ttf</font>
  </family>
  <family lang="ja und-Zsye">
    <font weight="400" style="normal">NotoColorEmoji.ttf</font>
  </family>
</familyset>`
	assert.Equal(t, []string{"NotoColorEmojiLegacy.ttf", "NotoColorEmoji.ttf", "NotoColorEmojiFlags.ttf"},
		emojiFileNames(strings.NewReader(fontsXML)))
	assert.Empty(t, emojiFileNames(strings.NewReader("<familyset><family lang=\"en\"><font>A.ttf</font></family>")))
	assert.Empty(t, emojiFileNames(strings.NewReader("not xml <<")))
}

func TestLoadEmojiFonts(t *testing.T) {
	t.Parallel()

	sys, product := t.TempDir(), t.TempDir()
	write := func(dir, name, data string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600))
	}
	write(sys, "NotoColorEmoji.ttf", "main")
	write(product, "NotoColorEmojiFlags.ttf", "flags")
	write(sys, "SamsungColorEmoji.ttf", "samsung")
	lists := t.TempDir()
	list := func(name string, files ...string) string {
		var b strings.Builder
		b.WriteString("<familyset>")
		for _, f := range files {
			b.WriteString(`<family lang="und-Zsye"><font>` + f + `</font></family>`)
		}
		b.WriteString("</familyset>")
		p := filepath.Join(lists, name)
		require.NoError(t, os.WriteFile(p, []byte(b.String()), 0o600))

		return p
	}
	dirs := []string{sys, product}
	missing := filepath.Join(lists, "none.xml")
	for _, tc := range []struct {
		name  string
		lists []string
		want  []string
	}{
		{"the list's files, in both folders", []string{list("a.xml", "Gone.ttf", "NotoColorEmoji.ttf", "NotoColorEmojiFlags.ttf")}, []string{"main", "flags"}},
		{"the first list naming any", []string{missing, list("b.xml"), list("c.xml", "SamsungColorEmoji.ttf")}, []string{"samsung"}},
		{"at most two", []string{list("d.xml", "SamsungColorEmoji.ttf", "NotoColorEmoji.ttf", "NotoColorEmojiFlags.ttf")}, []string{"samsung", "main"}},
		{"no list: the first guess found", []string{missing}, []string{"main"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := loadEmojiFonts(tc.lists, dirs)
			require.NoError(t, err)
			var s []string
			for _, d := range got {
				s = append(s, string(d))
			}
			assert.Equal(t, tc.want, s)
		})
	}
	_, err := loadEmojiFonts([]string{missing}, []string{t.TempDir()})
	assert.ErrorIs(t, err, core.ErrNotFound)
}

func TestFontDataInterFirst(t *testing.T) {
	t.Parallel()

	data, err := embeddedFonts()
	require.NoError(t, err)
	require.NotEmpty(t, data)
	e, err := gotext.New(gotext.WithDirs(t.TempDir()), gotext.WithFonts(data[0]))
	require.NoError(t, err)
	t.Cleanup(func() { _ = e.Close() })
	var inter, def text.Layout
	require.NoError(t, e.Shape("Wg", text.Face{Family: "Inter", Size: 20}, 1, 0, &inter))
	require.NoError(t, e.Shape("Wg", text.Face{Family: "No Such Family", Size: 20}, 1, 0, &def))
	assert.Equal(t, inter.Glyphs[0].Font, def.Glyphs[0].Font, "the first font is Inter's")
}

func TestUseAppTextEngineOffAndroid(t *testing.T) {
	t.Parallel()

	assert.ErrorIs(t, useAppTextEngine(), core.ErrUnsupported)
}

// TestTextSampleRender draws a sample of the app's text at 412 x 892 on the
// Android engine (gotext, typeset for the emoji), the Vietnamese of the API
// composed by format.NFC as api.Decode composes it: titles in Barlow
// Condensed, body text in Inter, numbers in JetBrains Mono, every letter of
// Vietnamese and emoji. No glyph may be missing. With DFV_TEXT_SAMPLE set
// it writes the picture there (3 pixels to the DIP), to look at.
func TestTextSampleRender(t *testing.T) {
	t.Parallel()

	if kittest.Race {
		t.Skip("the raster at 3x is too slow under the race detector")
	}
	e := newTestColorText(t, false, nil)
	var missing []string
	sh := text.NewShaper(e, text.WithMissingGlyphs(func(m []string) { missing = append(missing, m...) }))
	const (
		lower = "aàáảãạ ăằắẳẵặ âầấẩẫậ " +
			"eèéẻẽẹ êềếểễệ iìíỉĩị " +
			"oòóỏõọ ôồốổỗộ ơờớởỡợ " +
			"uùúủũụ ưừứửữự yỳýỷỹỵ đ"
	)
	upper := strings.ToUpper(lower)
	title := format.NFC(wordsNFD) // as the API's text comes out of api.Decode
	labels := []ui.Widget{
		kit.H1(title + " " + cowboy),
		kit.SecTitle(format.NFC("Bài tập") + " " + grin),
		&kit.Label{Text: format.NFC(wordsNFD) + " " + cowboy, Size: 15, Weight: 600},
		&kit.Label{Text: lower, Size: 15, Wrap: true},
		&kit.Label{Text: upper, Size: 15, Weight: 700, Wrap: true},
		&kit.Label{Text: lower, Role: theme.RoleDisplay, Size: 22, Weight: 800, Wrap: true},
		&kit.Label{Text: upper, Role: theme.RoleDisplay, Size: 22, Weight: 700, Wrap: true},
		&kit.Label{Text: "03:45 · " + wordsNFC, Role: theme.RoleMono, Size: 13, Weight: 500},
		kit.Muted("NFD, not composed: "+wordsNFD, 13),
	}
	root := ui.Column(10, labels...)
	ph := kittest.Phones[1]
	require.Equal(t, "android", ph.Name)
	tt, err := ui.NewTester(root, ph.Size.W, ph.Size.H, ui.WithTheme(theme.New(theme.NamingGoText)), ui.WithShaper(sh))
	require.NoError(t, err)
	t.Cleanup(tt.Close)
	tt.SetSafeArea(ph.Safe)
	tt.SetScale(3)
	tt.Elapse(kittest.Frame)
	tt.RunUntilIdle()
	assert.Empty(t, missing, "every character has a glyph")
	assert.True(t, tt.Shows(title+" "+cowboy) || tt.Shows(strings.ToUpper(title+" "+cowboy)))

	dir := os.Getenv("DFV_TEXT_SAMPLE")
	if dir == "" {
		return
	}
	require.NoError(t, os.MkdirAll(dir, 0o750))
	f, err := os.Create(filepath.Join(dir, "android-412x892.png")) //nolint:gosec // G304: the folder the developer named
	require.NoError(t, err)
	require.NoError(t, errors.Join(png.Encode(f, tt.Snapshot()), f.Close()))
}
