package format

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// nfcPair is a canonical composition: base followed by mark composes to
// composite.
type nfcPair struct {
	base, mark, composite rune
}

// maxSegment is the most characters a letter and its marks may hold for
// NFC to reorder and compose them; a longer run of marks is copied as it
// is.
const maxSegment = 32

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NFC returns s with its letters and their combining marks (U+0300-036F)
// in Unicode normal form C: "Bu\u0323ng" (NFD, as some editors store
// Vietnamese) is "B\u1ee5ng". The marks after a letter are put in
// canonical order, then composed with it as far as Unicode has precomposed
// characters (nfc_table.go, written by scripts/gen-nfc.py): the composites
// of Latin, Greek and Cyrillic, every Vietnamese letter among them. A mark
// that composes with nothing stays after its letter. A string without such
// marks is returned as it is, without allocation.
//
// It exists because gotext, the text engine of Android, does no shaping:
// it draws a combining mark as a glyph of its own beside its letter, or
// as a missing glyph's box, where Core Text composes it (golang.org/x/text
// is barred from the core, Q-P2-2).
func NFC(s string) string {
	if !hasMarks(s) {
		return s
	}

	return string(AppendNFC(make([]byte, 0, len(s)), s))
}

// AppendNFC appends NFC(s) to dst. It allocates only when dst has not the
// room.
func AppendNFC(dst []byte, s string) []byte {
	if !hasMarks(s) {
		return append(dst, s...)
	}
	var seg [maxSegment]rune
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		// The marks that follow r run to j.
		j := i + n
		for j < len(s) {
			m, k := utf8.DecodeRuneInString(s[j:])
			if !isMark(m) {
				break
			}
			j += k
		}
		if j == i+n || isMark(r) || (r == utf8.RuneError && n == 1) {
			// Nothing follows r, r is a mark no letter carries, or a byte
			// that is no UTF-8: copied as it is.
			dst = append(dst, s[i:j]...)
			i = j

			continue
		}
		out, ok := compose(seg[:0], r, s[i+n:j])
		if !ok {
			dst = append(dst, s[i:j]...)
		} else {
			for _, c := range out {
				dst = utf8.AppendRune(dst, c)
			}
		}
		i = j
	}

	return dst
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// hasMarks tells whether s holds a character of U+0300-037F: the bytes
// 0xCC and 0xCD lead their encodings and nothing else's.
func hasMarks(s string) bool {
	return strings.IndexByte(s, 0xCC) >= 0 || strings.IndexByte(s, 0xCD) >= 0
}

// isMark tells whether r is a combining mark of U+0300-036F that NFC
// reorders and composes: one of a combining class above 0.
func isMark(r rune) bool { return r >= 0x300 && r < 0x370 && markClass[r-0x300] != 0 }

// class returns the canonical combining class of a mark of isMark.
func class(r rune) uint8 { return markClass[r-0x300] }

// compose returns in seg the letter r and its marks in NFC: r decomposed,
// the marks appended, put in canonical order (stable, by combining class),
// then each composed with the letter unless an earlier mark left standing
// blocks it (one of the same class or higher). It reports false when the
// segment is longer than seg can hold.
func compose(seg []rune, r rune, marks string) ([]rune, bool) {
	seg = decompose(seg, r)
	for _, m := range marks {
		if len(seg) == cap(seg) {
			return seg, false
		}
		seg = append(seg, m)
	}
	// Insertion sort: a handful of marks.
	for i := 2; i < len(seg); i++ {
		for k := i; k > 1 && class(seg[k-1]) > class(seg[k]); k-- {
			seg[k-1], seg[k] = seg[k], seg[k-1]
		}
	}
	letter, w := seg[0], 1
	var last uint8 // the class of the last mark left standing, 0 for none
	for _, m := range seg[1:] {
		c := class(m)
		if last < c {
			if p, ok := composite(letter, m); ok {
				letter = p

				continue
			}
		}
		seg[w] = m
		w++
		last = c
	}
	seg[0] = letter

	return seg[:w], true
}

// decompose appends r's canonical decomposition through the table's pairs
// to seg: the letter, then its marks.
func decompose(seg []rune, r rune) []rune {
	i, ok := slices.BinarySearchFunc(nfcSplits[:], r, func(p nfcPair, c rune) int { return int(p.composite - c) })
	if !ok || len(seg)+2 > cap(seg) {
		return append(seg, r)
	}
	seg = decompose(seg, nfcSplits[i].base)

	return append(seg, nfcSplits[i].mark)
}

// composite returns the precomposed character of base and mark.
func composite(base, mark rune) (rune, bool) {
	i, ok := slices.BinarySearchFunc(nfcPairs[:], nfcPair{base: base, mark: mark}, func(p, q nfcPair) int {
		if p.base != q.base {
			return int(p.base - q.base)
		}

		return int(p.mark - q.mark)
	})
	if !ok {
		return 0, false
	}

	return nfcPairs[i].composite, true
}
