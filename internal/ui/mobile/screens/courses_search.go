//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/views/iptv_list.go at 49de3c2 (filter:
// the indexes of the rows that pass, written into a reused slice, so that a key of
// the search allocates nothing; the text each row is searched in is normalised
// once, when the list is read) and from jplayer-mobile/internal/l10n/text.go at
// 49de3c2 (AppendNorm: the query normalised into a buffer, trimmed of JavaScript
// white space, `String.trim`). The rule is myCourses (index.html:312-315):
// `q=S.m.q.trim().toLowerCase()`, a course passes when `name` or `quote` includes q.

package screens

import (
	"bytes"
	"unicode"
	"unicode/utf8"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// SearchFoldAccents is the one switch of the search's accents (Q-P3-11, K18). The
// prototype's search is `toLowerCase().includes()`, which tells "Cùng" from "cung";
// 03 section 4 asks for a search that ignores the accents ("khoa" finds "Khoá").
// The prototype's code wins until the owner decides on K18: false is the
// prototype's, true folds the Vietnamese marks and đ with format.Fold. Flipping it
// is this one line.
const SearchFoldAccents = false

// courseHay is what a course is searched in: its name and tagline, normalised
// once, when the list is read.
type courseHay struct {
	name, tagline []byte
}

// courseFilter is the filter of the Courses screen: the courses, what they are
// searched in, and the indexes of those that pass, in the order of the list.
type courseFilter struct {
	courses []model.Course
	hay     []courseHay
	shown   []int  // indexes into courses, reused by every call of apply
	query   []byte // the normalised query, reused
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newCourseFilter returns the filter over courses, the text of each made once.
func newCourseFilter(courses []model.Course) *courseFilter {
	f := &courseFilter{
		courses: courses,
		hay:     make([]courseHay, len(courses)),
		shown:   make([]int, 0, len(courses)),
		query:   make([]byte, 0, 64),
	}
	for i := range courses {
		f.hay[i] = courseHay{
			name:    appendHay(nil, courses[i].Name, SearchFoldAccents),
			tagline: appendHay(nil, courses[i].Tagline, SearchFoldAccents),
		}
	}

	return f
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// apply puts into shown the indexes of the courses of category cat (CategoryUnknown
// for all of them) that hold the search text q (the raw text of the field), and
// returns them. It allocates nothing once the buffers have grown to the longest
// query.
func (f *courseFilter) apply(cat model.Category, q string) []int {
	f.query = appendQuery(f.query[:0], q, SearchFoldAccents)
	f.shown = f.shown[:0]
	for i := range f.courses {
		if cat != model.CategoryUnknown && f.courses[i].Category != cat {
			continue
		}
		if len(f.query) > 0 && !bytes.Contains(f.hay[i].name, f.query) && !bytes.Contains(f.hay[i].tagline, f.query) {
			continue
		}
		f.shown = append(f.shown, i)
	}

	return f.shown
}

// appendQuery appends the normalised form of the field's text: trimmed of white
// space as JavaScript's `trim()` does, then lower-cased (and folded when fold).
func appendQuery(dst []byte, raw string, fold bool) []byte {
	return appendHay(dst, trimJS(raw), fold)
}

// appendHay appends s lower-cased (and, when fold, without the Vietnamese marks).
func appendHay(dst []byte, s string, fold bool) []byte {
	if fold {
		return format.AppendFold(dst, s)
	}
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			dst = append(dst, c)
			i++

			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		dst = utf8.AppendRune(dst, unicode.ToLower(r))
	}

	return dst
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// trimJS cuts the white space JavaScript's `String.prototype.trim` cuts from both
// ends: Unicode space separators, the line terminators and U+FEFF, and not U+0085.
func trimJS(s string) string {
	start, end := 0, len(s)
	for start < end {
		r, n := utf8.DecodeRuneInString(s[start:end])
		if !isJSSpace(r) {
			break
		}
		start += n
	}
	for end > start {
		r, n := utf8.DecodeLastRuneInString(s[start:end])
		if !isJSSpace(r) {
			break
		}
		end -= n
	}

	return s[start:end]
}

// isJSSpace tells whether r is JavaScript white space: what unicode.IsSpace
// takes, but for U+0085 (NEL), and U+FEFF.
func isJSSpace(r rune) bool {
	if r == 0xFEFF {
		return true
	}

	return r != 0x85 && unicode.IsSpace(r)
}
