// Adapted from jplayer-mobile/internal/media/enginekit/cmtime.go at 49de3c2.

package enginekit

import "math"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The flags of a CMTime (CMTime.h).
const (
	CMTimeFlagValid            uint32 = 1
	CMTimeFlagHasBeenRounded   uint32 = 2
	CMTimeFlagPositiveInfinity uint32 = 4
	CMTimeFlagNegativeInfinity uint32 = 8
	CMTimeFlagIndefinite       uint32 = 16
)

// CMTimescale is the timescale CMTimeOf makes times with: 600 000 counts
// 24, 25, 30, 50 and 60 fps and 44.1 and 48 kHz exactly, and a seek to the
// microsecond within 2 µs.
const CMTimescale int32 = 600_000

// CMTime is a CoreMedia time, laid out as in CMTime.h (24 bytes), so that an
// objc invocation can pass it by value.
type CMTime struct {
	Value     int64
	Timescale int32
	Flags     uint32
	Epoch     int64
}

// CMTimeRange is a CoreMedia time range (48 bytes), as the item's
// loadedTimeRanges hold.
type CMTimeRange struct {
	Start, Duration CMTime
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Seconds returns the time in seconds. It reports false for a time that is
// not valid, infinite or indefinite, which is what a live stream's duration
// is, and for a timescale that is not positive.
func (t CMTime) Seconds() (float64, bool) {
	const unusable = CMTimeFlagPositiveInfinity | CMTimeFlagNegativeInfinity | CMTimeFlagIndefinite
	if t.Flags&CMTimeFlagValid == 0 || t.Flags&unusable != 0 || t.Timescale <= 0 {
		return 0, false
	}

	return float64(t.Value) / float64(t.Timescale), true
}

// CMTimeOf returns sec as a CMTime on CMTimescale. NaN is indefinite and an
// infinity, or a time too long for the count, is infinite.
func CMTimeOf(sec float64) CMTime {
	switch {
	case math.IsNaN(sec):
		return CMTime{Flags: CMTimeFlagValid | CMTimeFlagIndefinite}
	case sec >= math.MaxInt64/float64(CMTimescale):
		return CMTime{Flags: CMTimeFlagValid | CMTimeFlagPositiveInfinity}
	case sec <= math.MinInt64/float64(CMTimescale):
		return CMTime{Flags: CMTimeFlagValid | CMTimeFlagNegativeInfinity}
	}

	return CMTime{Value: int64(math.Round(sec * float64(CMTimescale))), Timescale: CMTimescale, Flags: CMTimeFlagValid}
}

// BufferedEnd returns where the loaded part of the media ends, in seconds:
// the end of the last range, as player.js:56 does with buffered.end(length −
// 1), not the range around the playing position. It is 0 for no range, or
// for a last one that is not valid.
func BufferedEnd(rs []CMTimeRange) float64 {
	if len(rs) == 0 {
		return 0
	}
	r := rs[len(rs)-1]
	start, ok := r.Start.Seconds()
	if !ok {
		return 0
	}
	dur, ok := r.Duration.Seconds()
	if !ok {
		return 0
	}

	return start + dur
}
