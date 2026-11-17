// Adapted from jplayer-mobile/internal/media/enginekit/size.go at 49de3c2.

package enginekit

import "math"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The sizes of the probe's picture and the limit of a frame.
const (
	// MaxEdge is the longest side of a frame, in pixels.
	MaxEdge = 1280
	// ThumbWidth is the width of the picture a probe takes.
	ThumbWidth = 320
	// ThumbFallbackHeight is its height when the video's size is unknown.
	ThumbFallbackHeight = 180
	// ThumbQuality is the quality of its JPEG (toDataURL's 0.72).
	ThumbQuality = 0.72
	// ProbeFraction, ProbeFallback and ProbeMax place the frame the probe takes.
	ProbeFraction = 0.15
	ProbeFallback = 1.0
	ProbeMax      = 12.0
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Fit returns the size of a w by h frame that has no side over max: the same
// proportions, never larger, each side even (the video codecs' way). A max
// that is not positive sets no limit. A size that is not positive gives 0, 0.
func Fit(w, h, limit int) (int, int) {
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	if limit > 0 && max(w, h) > limit {
		if w >= h {
			w, h = limit, int(math.Round(float64(h)*float64(limit)/float64(w)))
		} else {
			w, h = int(math.Round(float64(w)*float64(limit)/float64(h))), limit
		}
	}

	return even(w), even(h)
}

// ProbeAt returns where in a media of dur seconds the probe takes its
// picture: Math.min(duration * 0.15 || 1, 12) of sources.js:78, so 1 s for a
// length that is 0, unknown or negative.
func ProbeAt(dur float64) float64 {
	at := dur * ProbeFraction
	if math.IsNaN(at) || at <= 0 {
		at = ProbeFallback
	}

	return min(at, ProbeMax)
}

// ThumbSize returns the size of the probe's picture of a video vw by vh:
// 320 wide, round(vh/vw·320) high, 180 when that is not known (sources.js:73).
func ThumbSize(vw, vh int) (w, h int) {
	if vw <= 0 || vh <= 0 {
		return ThumbWidth, ThumbFallbackHeight
	}
	h = int(math.Floor(float64(vh)/float64(vw)*ThumbWidth + 0.5)) // Math.round
	if h == 0 {
		h = ThumbFallbackHeight
	}

	return ThumbWidth, h
}

// RotationOf returns how many degrees clockwise, 0, 90, 180 or 270, the
// transform a, b, c, d of a CGAffineTransform (the track's preferredTransform)
// turns the picture. Only the turn counts; a flip is not one.
func RotationOf(a, b, _, _ float64) int {
	if a == 0 && b == 0 {
		return 0
	}
	deg := math.Atan2(b, a) * 180 / math.Pi
	q := int(math.Round(deg / 90))

	return ((q%4 + 4) % 4) * 90
}

// Turned returns the size of a w by h picture turned by deg degrees: the
// sides change places for 90 and 270.
func Turned(w, h, deg int) (int, int) {
	d := (deg%360 + 360) % 360
	if d == 90 || d == 270 {
		return h, w
	}

	return w, h
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// even rounds n down to an even number, at least 2.
func even(n int) int { return max(n&^1, 2) }
