// Adapted from jplayer-mobile/internal/media/enginekit/spectrum.go at 49de3c2.

package enginekit

import (
	"dancefitvn-app/internal/media"
	"math"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// SpectrumSize is the analyser's fftSize (player.js:281).
const SpectrumSize = 256

// The settings of the prototype's AnalyserNode: smoothingTimeConstant .82
// per 60 Hz frame (player.js:281); the others are Web Audio's defaults.
const (
	spectrumSmoothing = 0.82
	spectrumFrame     = 60.0 // frames per second the smoothing is meant for
	spectrumMinDB     = -100.0
	spectrumMaxDB     = -30.0
	blackmanAlpha     = 0.16
)

// Spectrum is Web Audio's AnalyserNode as the prototype sets it: fftSize
// 256, smoothingTimeConstant .82, Blackman window, minDecibels -100,
// maxDecibels -30. It allocates nothing.
type Spectrum struct {
	window [SpectrumSize]float64
	cos    [SpectrumSize / 2]float64
	sin    [SpectrumSize / 2]float64
	rev    [SpectrumSize]uint8
	re, im [SpectrumSize]float64
	smooth [media.FreqBins]float64
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewSpectrum returns a Spectrum with its tables made.
func NewSpectrum() *Spectrum {
	s := &Spectrum{}
	a0, a1, a2 := (1-blackmanAlpha)/2, 0.5, blackmanAlpha/2
	for i := range s.window {
		x := 2 * math.Pi * float64(i) / SpectrumSize
		s.window[i] = a0 - a1*math.Cos(x) + a2*math.Cos(2*x)
		r := 0
		for b := 0; b < 8; b++ {
			r |= (i >> b & 1) << (7 - b)
		}
		s.rev[i] = uint8(r) //nolint:gosec // r < 256
	}
	for k := range s.cos {
		x := 2 * math.Pi * float64(k) / SpectrumSize
		s.cos[k], s.sin[k] = math.Cos(x), -math.Sin(x)
	}

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Reset forgets the smoothed magnitudes (a new media).
func (s *Spectrum) Reset() {
	s.smooth = [media.FreqBins]float64{}
}

// Bytes takes the latest SpectrumSize samples (mono, -1..1; nil is
// silence), dt after the last call, and fills dst with the byte
// frequency data. The smoothing constant is .82 per 60 Hz frame, so one
// call of dt scales it to .82^(dt*60); a dt that is not positive adds no
// new analysis and only reads the bytes again.
func (s *Spectrum) Bytes(samples *[SpectrumSize]float32, dt time.Duration, dst *[media.FreqBins]uint8) {
	if dt > 0 {
		s.analyse(samples, math.Pow(spectrumSmoothing, dt.Seconds()*spectrumFrame))
	}
	const scale = 255 / (spectrumMaxDB - spectrumMinDB)
	for i, m := range s.smooth {
		if m <= 0 {
			dst[i] = 0

			continue
		}
		v := scale * (20*math.Log10(m) - spectrumMinDB)
		switch {
		case !(v > 0): // also NaN
			dst[i] = 0
		case v >= 255:
			dst[i] = 255
		default:
			dst[i] = uint8(v)
		}
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// analyse windows the samples, takes their FFT and smooths the magnitudes
// (each divided by fftSize) with tau.
func (s *Spectrum) analyse(samples *[SpectrumSize]float32, tau float64) {
	for i := range s.window {
		var x float64
		if samples != nil {
			x = float64(samples[i])
			if x != x || math.IsInf(x, 0) { // a broken sample is silence
				x = 0
			}
		}
		j := s.rev[i]
		s.re[j], s.im[j] = x*s.window[i], 0
	}
	for size := 2; size <= SpectrumSize; size <<= 1 {
		half, step := size/2, SpectrumSize/size
		for start := 0; start < SpectrumSize; start += size {
			for k := 0; k < half; k++ {
				wr, wi := s.cos[k*step], s.sin[k*step]
				a, b := start+k, start+k+half
				tr := s.re[b]*wr - s.im[b]*wi
				ti := s.re[b]*wi + s.im[b]*wr
				s.re[b], s.im[b] = s.re[a]-tr, s.im[a]-ti
				s.re[a], s.im[a] = s.re[a]+tr, s.im[a]+ti
			}
		}
	}
	for k := range s.smooth {
		mag := math.Hypot(s.re[k], s.im[k]) / SpectrumSize
		s.smooth[k] = tau*s.smooth[k] + (1-tau)*mag
	}
}
