package format

import (
	"unicode"
	"unicode/utf8"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// latin1Base maps U+00C0..U+00FF to the lower-case ASCII letter without
// its mark; '_' marks a character that has none (Æ, ×, Þ, ß, ÷, æ, þ).
const latin1Base = "aaaaaa_ceeeeiiiidnooooo_ouuuuy__" + "aaaaaa_ceeeeiiiidnooooo_ouuuuy_y"

// vietnameseBase maps U+1EA0..U+1EF9 (Latin Extended Additional, the
// precomposed Vietnamese letters) to the lower-case ASCII letter.
const vietnameseBase = "" +
	"aaaaaaaaaaaaaaaaaaaaaaaa" + // U+1EA0..U+1EB7
	"eeeeeeeeeeeeeeee" + // U+1EB8..U+1EC7
	"iiii" + // U+1EC8..U+1ECB
	"oooooooooooooooooooooooo" + // U+1ECC..U+1EE3
	"uuuuuuuuuuuuuu" + // U+1EE4..U+1EF1
	"yyyyyyyy" // U+1EF2..U+1EF9

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Fold lower-cases s and removes the Vietnamese marks, so that a search
// ignores case and accents (03 §4): "Đùi Mông" is "dui mong". đ and Đ are d,
// and combining marks U+0300..U+036F are dropped, so decomposed text folds
// like composed text.
func Fold(s string) string { return string(AppendFold(make([]byte, 0, len(s)), s)) }

// AppendFold appends Fold(s) to dst. It allocates only when dst has not the
// room.
func AppendFold(dst []byte, s string) []byte {
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf { // ASCII fast path
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			dst = append(dst, c)
			i++

			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r >= 0x300 && r <= 0x36F {
			continue
		}
		if b := baseLetter(r); b != 0 {
			dst = append(dst, b)

			continue
		}
		dst = utf8.AppendRune(dst, unicode.ToLower(r))
	}

	return dst
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// baseLetter returns the ASCII letter r folds to, or 0 when r stays.
func baseLetter(r rune) byte {
	switch {
	case r >= 0xC0 && r <= 0xFF:
		if b := latin1Base[r-0xC0]; b != '_' {
			return b
		}
	case r >= 0x1EA0 && r <= 0x1EF9:
		return vietnameseBase[r-0x1EA0]
	case r == 0x102 || r == 0x103:
		return 'a'
	case r == 0x110 || r == 0x111:
		return 'd'
	case r == 0x128 || r == 0x129:
		return 'i'
	case r == 0x168 || r == 0x169:
		return 'u'
	case r == 0x1A0 || r == 0x1A1:
		return 'o'
	case r == 0x1AF || r == 0x1B0:
		return 'u'
	}

	return 0
}
