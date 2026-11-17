package format

import (
	"math"
	"strconv"
	"strings"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Progress formats the text of a running download, "3,1 / 8,8 MB · 35%":
// the part done of totalBytes, the total and the percentage.
func Progress(doneFrac float64, totalBytes int64) string {
	return string(AppendProgress(make([]byte, 0, 32), doneFrac, totalBytes))
}

// AppendProgress appends Progress(doneFrac, totalBytes) to dst.
func AppendProgress(dst []byte, doneFrac float64, totalBytes int64) []byte {
	if !(doneFrac > 0) {
		doneFrac = 0
	}
	dst = appendFixed1(dst, doneFrac*megabytes(totalBytes))
	dst = append(dst, " / "...)
	dst = AppendSize(dst, totalBytes)
	dst = append(dst, " · "...)
	dst = strconv.AppendInt(dst, int64(Percent(doneFrac)), 10)

	return append(dst, '%')
}

// dot is the separator of the prototype's meta lines, `a · b · c`.
const dot = " · "

// Join returns the non-empty parts joined by " · ": a lesson that has no tag
// is "04:32", not " · 04:32". With every part given it is the plain join.
func Join(parts ...string) string {
	var n int
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			n++
		}
	}
	if n == len(parts) {
		return strings.Join(parts, dot)
	}
	kept := make([]string, 0, n)
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}

	return strings.Join(kept, dot)
}

// DropBlank removes the blank parts of a " · " line a template left where a
// value was empty: "Bài 04 ·  · còn 3 phút" is "Bài 04 · còn 3 phút", and
// "A · BÀI 01 / 10 · " is "A · BÀI 01 / 10". A line with no blank part is
// returned as it is.
func DropBlank(s string) string {
	if !strings.Contains(s, "·") {
		return s
	}
	parts := strings.Split(s, dot)
	blank := false
	for _, p := range parts {
		if strings.TrimSpace(p) == "" || strings.TrimSpace(p) == "·" {
			blank = true

			break
		}
	}
	if !blank {
		return s
	}

	return Join(parts...)
}

// Percent returns the percentage of a fraction as Math.round(p*100) does.
func Percent(p float64) int {
	if !(p > 0) {
		return 0
	}

	return int(math.Round(p * 100))
}

// No formats a lesson number padded to two digits, "03".
func No(n int) string {
	if n >= 0 && n < 10 {
		return string([]byte{'0', byte('0' + n)})
	}

	return strconv.Itoa(n)
}

// RemainingMinutes returns the minutes left in a lesson as the prototype
// shows them: max(1, round((dur-pos)/60)).
func RemainingMinutes(durSec, posSec float64) int {
	m := math.Round((durSec - posSec) / 60)
	if !(m >= 1) {
		return 1
	}

	return int(m)
}
