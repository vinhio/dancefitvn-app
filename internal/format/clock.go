// Adapted from jplayer-mobile/internal/l10n/format.go at 49de3c2 (AppendTime, pad2).

package format

import (
	"math"
	"strconv"
	"sync"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// clockTableSeconds is how many seconds clockTable covers: "00:00" to "99:59".
const clockTableSeconds = 6000

// clockWidth is the length of "mm:ss".
const clockWidth = 5

var (
	clockOnce  sync.Once
	clockTable string
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Clock formats a position as the prototype's fmt() does: "mm:ss" with the
// minutes padded to two digits and no cap (6000 s is "100:00"). A negative
// or NaN value is "00:00", the fraction of a second is dropped, +Inf is
// the largest value.
//
// Below 6000 s the result is a slice of one string built once, so it does
// not allocate; from there on it allocates.
func Clock(sec float64) string {
	t := clockSeconds(sec)
	if t < clockTableSeconds {
		clockOnce.Do(buildClockTable)
		i := int(t) * clockWidth

		return clockTable[i : i+clockWidth]
	}

	return string(AppendClock(make([]byte, 0, 24), sec))
}

// AppendClock appends Clock(sec) to dst. It allocates only when dst has not
// the room.
func AppendClock(dst []byte, sec float64) []byte {
	t := clockSeconds(sec)
	dst = pad2(dst, t/60)
	dst = append(dst, ':')

	return pad2(dst, t%60)
}

// TotalMinutes formats the length of a course the prototype's way: the
// minutes of Clock(sec) with their padding, "09" for 540 s, "39" for 2340 s.
func TotalMinutes(sec int) string {
	c := Clock(float64(sec))
	for i := 0; i < len(c); i++ {
		if c[i] == ':' {
			return c[:i]
		}
	}

	return c
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// clockSeconds returns the whole seconds of sec, never negative.
func clockSeconds(sec float64) int64 {
	if !(sec > 0) { // also NaN
		return 0
	}

	return int64(math.Min(math.Floor(sec), 1<<62))
}

// buildClockTable fills clockTable with "00:00", "00:01" ... "99:59".
func buildClockTable() {
	b := make([]byte, 0, clockTableSeconds*clockWidth)
	for s := int64(0); s < clockTableSeconds; s++ {
		b = pad2(b, s/60)
		b = append(b, ':')
		b = pad2(b, s%60)
	}
	clockTable = string(b)
}

// pad2 appends v as at least two digits.
func pad2(dst []byte, v int64) []byte {
	if v < 10 {
		dst = append(dst, '0')
	}

	return strconv.AppendInt(dst, v, 10)
}
