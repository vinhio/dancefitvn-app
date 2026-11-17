package theme_test

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"unicode"

	"dancefitvn-app/assets"
	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/bridge/text/gotext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// vietnameseLetters returns every letter Vietnamese writes, lower and upper
// case, precomposed: those of Latin-1 (à á â ã è é ê ì í ò ó ô õ ù ú ý),
// of Latin Extended-A and -B (ă đ ĩ ũ ơ ư) and U+1EA0-1EF9.
func vietnameseLetters() []rune {
	var out []rune
	for _, r := range []rune{
		0xC0, 0xC1, 0xC2, 0xC3, 0xC8, 0xC9, 0xCA, 0xCC, 0xCD, 0xD2, 0xD3, 0xD4, 0xD5, 0xD9, 0xDA, 0xDD,
		0x102, 0x110, 0x128, 0x168, 0x1A0, 0x1AF,
	} {
		out = append(out, r, unicode.ToLower(r))
	}
	for r := rune(0x1EA0); r <= 0x1EF9; r++ {
		out = append(out, r)
	}

	return out
}

// vietnameseMarks are the combining marks of decomposed (NFD) Vietnamese:
// grave, acute, circumflex, tilde, breve, hook above, horn, dot below.
var vietnameseMarks = []rune{0x300, 0x301, 0x302, 0x303, 0x306, 0x309, 0x31B, 0x323}

// androidEngine returns gotext over the embedded fonts alone, as Android
// draws them (the system's fonts left out: the app's must suffice).
func androidEngine(t *testing.T) text.Engine {
	t.Helper()
	names, err := fs.Glob(assets.Fonts, "fonts/*.ttf")
	require.NoError(t, err)
	var data [][]byte
	for _, n := range names {
		d, err := fs.ReadFile(assets.Fonts, n)
		require.NoError(t, err)
		data = append(data, d)
	}
	e, err := gotext.New(gotext.WithDirs(t.TempDir()), gotext.WithFonts(data...))
	require.NoError(t, err)
	t.Cleanup(func() { _ = e.Close() })

	return e
}

// TestFontsCoverVietnamese checks that every face the app draws in on
// Android has a glyph of its own (not a fallback's, not the missing box)
// for every Vietnamese letter, upper and lower case, precomposed, and for
// every combining mark of decomposed Vietnamese: the subset of
// scripts/fonts/charset.py.
func TestFontsCoverVietnamese(t *testing.T) {
	t.Parallel()

	e := androidEngine(t)
	gt := theme.NewFaces(theme.NamingGoText)
	letters, marks := string(vietnameseLetters()), string(vietnameseMarks)
	for _, f := range []text.Face{
		gt.Face(theme.RoleBody, 15, 400), gt.Face(theme.RoleBody, 15, 500), gt.Face(theme.RoleBody, 15, 600),
		gt.Face(theme.RoleBody, 15, 700), gt.Face(theme.RoleBody, 15, 800),
		gt.Face(theme.RoleDisplay, 30, 700), gt.Face(theme.RoleDisplay, 30, 800),
		gt.Face(theme.RoleMono, 13, 500), gt.Face(theme.RoleMono, 13, 700),
	} {
		var probe, l text.Layout
		require.NoError(t, e.Shape("a", f, 1, 0, &probe))
		own := probe.Glyphs[0].Font
		for _, s := range []string{letters, marks} {
			require.NoError(t, e.Shape(s, f, 1, 0, &l))
			for _, g := range l.Glyphs {
				r := []rune(s[g.Index:])[0]
				assert.False(t, g.Missing, "%s %d: U+%04X", f.Family, f.Weight, r)
				assert.Equal(t, own, g.Font, "%s %d: U+%04X is the face's own", f.Family, f.Weight, r)
			}
		}
	}
}

// TestFontsCoverAppText checks that every character of locales/vi.json, and
// of a Vietnamese corpus as the API may send it (NFD) and as api.Decode
// gives it (NFC), has a glyph among the embedded fonts on Android, in the
// body face and the display face.
func TestFontsCoverAppText(t *testing.T) {
	t.Parallel()

	texts, err := locales.Texts()
	require.NoError(t, err)
	corpus := map[string]string{}
	for k, v := range texts {
		corpus["vi.json "+k] = v
	}
	// Each vowel letter of Vietnamese with each tone, decomposed (NFD): the
	// base, its vowel mark (breve, circumflex, horn), the tone mark; lower
	// and upper case.
	vowels := [][]rune{{'a'}, {'a', 0x306}, {'a', 0x302}, {'e'}, {'e', 0x302}, {'i'}, {'o'}, {'o', 0x302}, {'o', 0x31B}, {'u'}, {'u', 0x31B}, {'y'}}
	for _, v := range vowels {
		for _, tone := range []rune{0, 0x300, 0x301, 0x303, 0x309, 0x323} {
			for _, upper := range []bool{false, true} {
				letter := append([]rune{}, v...)
				if upper {
					letter[0] = unicode.ToUpper(letter[0])
				}
				if tone != 0 {
					letter = append(letter, tone)
				}
				s := "T" + string(letter) + "ng"
				corpus[fmt.Sprintf("nfd %U", letter)] = s
				corpus[fmt.Sprintf("nfc %U", letter)] = format.NFC(s)
			}
		}
	}
	e := androidEngine(t)
	gt := theme.NewFaces(theme.NamingGoText)
	for _, f := range []text.Face{gt.Face(theme.RoleBody, 15, 400), gt.Face(theme.RoleDisplay, 30, 700)} {
		var l text.Layout
		for name, s := range corpus {
			require.NoError(t, e.Shape(s, f, 1, 0, &l))
			for _, g := range l.Glyphs {
				if g.Missing {
					assert.Fail(t, "missing glyph", "%s %s: U+%04X in %q", f.Family, name, []rune(s[g.Index:])[0], s)
				}
			}
		}
	}
}

// TestCorpusComposes checks that the NFD corpus composes to precomposed
// letters only: no Vietnamese letter needs a mark drawn apart.
func TestCorpusComposes(t *testing.T) {
	t.Parallel()

	for _, base := range "aeiouy" {
		for _, mark := range []rune{0x300, 0x301, 0x303, 0x309, 0x323} {
			s := format.NFC(string([]rune{base, mark}))
			assert.Equal(t, 1, len([]rune(s)), "%q", s)
			assert.False(t, strings.ContainsFunc(s, func(r rune) bool { return r >= 0x300 && r < 0x370 }))
		}
	}
}
