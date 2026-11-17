// Adapted from jplayer-mobile/e2e/engine/avf_ios_test.go at 49de3c2 (wait,
// classOf, mib, percentiles, lumaDiff, luma): the measures both phones'
// engine tests share.

//go:build (ios && !tvos) || (android && cgo)

package engine

import (
	"dancefitvn-app/internal/media"
	"errors"
	"image"
	"slices"
	"time"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// wait lets d of real time pass: the engines play on the wall clock.
func wait(d time.Duration) {
	timer := time.NewTimer(d)
	<-timer.C
}

// classOf names the media sentinel err wraps, "none" when it wraps none.
func classOf(err error) string {
	for _, c := range []struct {
		name string
		err  error
	}{
		{"ErrNetwork", media.ErrNetwork}, {"ErrSource", media.ErrSource}, {"ErrDecode", media.ErrDecode},
		{"ErrMissingFile", media.ErrMissingFile}, {"ErrAborted", media.ErrAborted},
	} {
		if errors.Is(err, c.err) {
			return c.name
		}
	}

	return "none"
}

// mib is n bytes in MiB.
func mib(n int64) float64 { return float64(n) / (1 << 20) }

// percentiles returns the median and the 95th percentile of ds.
func percentiles(ds []time.Duration) (p50, p95 time.Duration) {
	if len(ds) == 0 {
		return 0, 0
	}
	s := slices.Clone(ds)
	slices.Sort(s)

	return s[len(s)/2], s[min(len(s)-1, len(s)*95/100)]
}

// lumaDiff is the mean absolute difference of the grey levels of a and b
// over r (pixels of both), b read mirrored about r's vertical axis when
// flip: 0 for the same picture.
func lumaDiff(a, b image.Image, r image.Rectangle, flip bool) float64 {
	var sum, n float64
	for y := r.Min.Y; y < r.Max.Y; y += 2 {
		for x := r.Min.X; x < r.Max.X; x += 2 {
			bx := x
			if flip {
				bx = r.Max.X - 1 - (x - r.Min.X)
			}
			la, lb := luma(a, x, y), luma(b, bx, y)
			if la > lb {
				sum += la - lb
			} else {
				sum += lb - la
			}
			n++
		}
	}
	if n == 0 {
		return 0
	}

	return sum / n
}

// luma is the grey level of img at x, y, 0 to 255.
func luma(img image.Image, x, y int) float64 {
	r, g, b, _ := img.At(x, y).RGBA()

	return (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 257
}
