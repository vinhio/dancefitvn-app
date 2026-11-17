// Adapted from jplayer-mobile/internal/media/enginekit/spectrum_test.go at 49de3c2.

package enginekit

import (
	"dancefitvn-app/internal/media"
	"math"
	"math/cmplx"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refSpectrum is the Web Audio formula written straight from its
// specification: a float64 DFT of the Blackman-windowed samples, magnitudes
// over fftSize, smoothed by tau, then to dB and to a byte.
type refSpectrum struct{ smooth [media.FreqBins]float64 }

func (r *refSpectrum) bytes(x *[SpectrumSize]float32, tau float64) (out [media.FreqBins]uint8) {
	const n = SpectrumSize
	for k := range r.smooth {
		var sum complex128
		for i := 0; i < n; i++ {
			var v float64
			if x != nil {
				v = float64(x[i])
			}
			w := 0.42 - 0.5*math.Cos(2*math.Pi*float64(i)/n) + 0.08*math.Cos(4*math.Pi*float64(i)/n)
			sum += complex(v*w, 0) * cmplx.Exp(complex(0, -2*math.Pi*float64(k*i)/n))
		}
		r.smooth[k] = tau*r.smooth[k] + (1-tau)*cmplx.Abs(sum)/n
		db := 20 * math.Log10(r.smooth[k])
		b := math.Floor(255.0 / 70.0 * (db + 100))
		out[k] = uint8(math.Max(0, math.Min(255, b)))
	}

	return out
}

// sine returns amp*sin at the bin's centre frequency plus frac bins.
func sine(bin, frac, amp float64) *[SpectrumSize]float32 {
	var s [SpectrumSize]float32
	for i := range s {
		s[i] = float32(amp * math.Sin(2*math.Pi*(bin+frac)*float64(i)/SpectrumSize))
	}

	return &s
}

func peak(b *[media.FreqBins]uint8) int {
	p := 0
	for i, v := range b {
		if v > b[p] {
			p = i
		}
	}

	return p
}

func TestSpectrumMatchesReference(t *testing.T) {
	t.Parallel()

	frame := time.Second / 60
	tests := []struct {
		name  string
		in    *[SpectrumSize]float32
		dt    time.Duration
		calls int
	}{
		{"sine bin 2", sine(2, 0, 0.5), frame, 1},
		{"sine bin 10", sine(10, 0, 0.1), frame, 1},
		{"sine bin 40", sine(40, 0, 0.9), frame, 1},
		{"between bins", sine(17, 0.5, 0.05), frame, 1},
		{"quiet", sine(30, 0, 0.0005), frame, 1},
		{"smoothed over 12 frames", sine(10, 0, 0.3), frame, 12},
		{"120 Hz", sine(10, 0, 0.3), time.Second / 120, 12},
		{"30 Hz", sine(5, 0, 0.3), time.Second / 30, 5},
		{"silence", nil, frame, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s, ref := NewSpectrum(), &refSpectrum{}
			tau := math.Pow(0.82, tt.dt.Seconds()*60)
			var got [media.FreqBins]uint8
			for i := 0; i < tt.calls; i++ {
				s.Bytes(tt.in, tt.dt, &got)
				want := ref.bytes(tt.in, tau)
				for k := range got {
					require.InDelta(t, int(want[k]), int(got[k]), 1, "call %d bin %d", i, k)
				}
			}
		})
	}
}

func TestSpectrumPeakBin(t *testing.T) {
	t.Parallel()

	for _, bin := range []int{1, 2, 3, 10, 40, 100, 126} {
		s := NewSpectrum()
		var got [media.FreqBins]uint8
		for i := 0; i < 8; i++ {
			s.Bytes(sine(float64(bin), 0, 0.01), time.Second/60, &got)
		}
		assert.Equal(t, bin, peak(&got), "bin %d", bin)
		assert.Greater(t, got[bin], uint8(150), "bin %d", bin)
	}
}

func TestSpectrumLoudSineSaturates(t *testing.T) {
	t.Parallel()

	s := NewSpectrum()
	var got [media.FreqBins]uint8
	for i := 0; i < 30; i++ {
		s.Bytes(sine(20, 0, 0.5), time.Second/60, &got)
	}
	assert.Equal(t, uint8(255), got[20])
	assert.Zero(t, got[60])
}

func TestSpectrumSilence(t *testing.T) {
	t.Parallel()

	s := NewSpectrum()
	got := [media.FreqBins]uint8{1, 2, 3}
	s.Bytes(nil, time.Second/60, &got)
	assert.Equal(t, [media.FreqBins]uint8{}, got)

	var zeros [SpectrumSize]float32
	s.Bytes(&zeros, time.Second/60, &got)
	assert.Equal(t, [media.FreqBins]uint8{}, got)

	bad := sine(5, 0, 1)
	bad[3] = float32(math.NaN())
	bad[9] = float32(math.Inf(1))
	s.Bytes(bad, time.Second/60, &got)
	for _, v := range got {
		assert.LessOrEqual(t, v, uint8(255))
	}
}

func TestSpectrumSmoothing(t *testing.T) {
	t.Parallel()

	loud := sine(10, 0, 0.5)
	frame := time.Second / 60
	bytesAfter := func(dt time.Duration, n int) (b [media.FreqBins]uint8) {
		s := NewSpectrum()
		for i := 0; i < n; i++ {
			s.Bytes(loud, dt, &b)
		}

		return b
	}
	t.Run("rises over frames", func(t *testing.T) {
		t.Parallel()
		one, five := bytesAfter(frame, 1), bytesAfter(frame, 5)
		assert.Less(t, one[10], five[10])
		assert.Equal(t, uint8(255), bytesAfter(frame, 30)[10])
	})
	t.Run("one 60 Hz frame equals two 120 Hz frames", func(t *testing.T) {
		t.Parallel()
		a, b := bytesAfter(frame, 4), bytesAfter(time.Second/120, 8)
		for k := range a {
			assert.InDelta(t, int(a[k]), int(b[k]), 1, "bin %d", k)
		}
	})
	t.Run("decays in silence to zero", func(t *testing.T) {
		t.Parallel()
		s := NewSpectrum()
		var b [media.FreqBins]uint8
		s.Bytes(loud, frame, &b)
		s.Bytes(loud, frame, &b)
		start := b[10]
		s.Bytes(nil, frame, &b)
		assert.Less(t, b[10], start)
		n := 0
		for ; n < 600 && b != [media.FreqBins]uint8{}; n++ {
			s.Bytes(nil, frame, &b)
		}
		assert.Less(t, n, 600)
		assert.Equal(t, [media.FreqBins]uint8{}, b)
	})
	t.Run("zero dt adds no analysis", func(t *testing.T) {
		t.Parallel()
		s := NewSpectrum()
		var a, b [media.FreqBins]uint8
		s.Bytes(loud, frame, &a)
		s.Bytes(nil, 0, &b)
		assert.Equal(t, a, b)
	})
	t.Run("reset forgets", func(t *testing.T) {
		t.Parallel()
		s := NewSpectrum()
		var b [media.FreqBins]uint8
		s.Bytes(loud, frame, &b)
		s.Reset()
		s.Bytes(nil, frame, &b)
		assert.Equal(t, [media.FreqBins]uint8{}, b)
	})
}

func TestSpectrumDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	s := NewSpectrum()
	in := sine(10, 0.3, 0.3)
	var b [media.FreqBins]uint8
	s.Bytes(in, time.Second/60, &b)
	assert.Zero(t, testing.AllocsPerRun(100, func() { s.Bytes(in, time.Second/60, &b) }))
	assert.Zero(t, testing.AllocsPerRun(100, func() { s.Bytes(nil, time.Second/120, &b) }))
}

func BenchmarkSpectrum(b *testing.B) {
	s := NewSpectrum()
	in := sine(10, 0.3, 0.3)
	var out [media.FreqBins]uint8
	b.ReportAllocs()
	for b.Loop() {
		s.Bytes(in, time.Second/60, &out)
	}
}
