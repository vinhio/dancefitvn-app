// Adapted from jplayer-mobile/internal/media/enginekit/cmtime_test.go at 49de3c2.

package enginekit

import (
	"math"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func TestCMTimeLayout(t *testing.T) {
	t.Parallel()

	assert.EqualValues(t, 24, unsafe.Sizeof(CMTime{}))
	assert.EqualValues(t, 48, unsafe.Sizeof(CMTimeRange{}))
	assert.EqualValues(t, 8, unsafe.Offsetof(CMTime{}.Timescale))
	assert.EqualValues(t, 12, unsafe.Offsetof(CMTime{}.Flags))
	assert.EqualValues(t, 16, unsafe.Offsetof(CMTime{}.Epoch))
}

func TestCMTimeSeconds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		t    CMTime
		sec  float64
		ok   bool
	}{
		{"valid", CMTime{Value: 1500, Timescale: 600, Flags: CMTimeFlagValid}, 2.5, true},
		{"zero valid", CMTime{Timescale: 600, Flags: CMTimeFlagValid}, 0, true},
		{"rounded still counts", CMTime{Value: 30, Timescale: 30, Flags: CMTimeFlagValid | CMTimeFlagHasBeenRounded}, 1, true},
		{"not valid", CMTime{Value: 1, Timescale: 1}, 0, false},
		{"the zero value", CMTime{}, 0, false},
		{"indefinite", CMTime{Flags: CMTimeFlagValid | CMTimeFlagIndefinite, Timescale: 1}, 0, false},
		{"positive infinity", CMTime{Flags: CMTimeFlagValid | CMTimeFlagPositiveInfinity}, 0, false},
		{"negative infinity", CMTime{Flags: CMTimeFlagValid | CMTimeFlagNegativeInfinity}, 0, false},
		{"timescale 0", CMTime{Value: 5, Flags: CMTimeFlagValid}, 0, false},
		{"timescale negative", CMTime{Value: 5, Timescale: -1, Flags: CMTimeFlagValid}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sec, ok := tt.t.Seconds()
			assert.Equal(t, tt.ok, ok)
			assert.InDelta(t, tt.sec, sec, 1e-12)
		})
	}
}

func TestCMTimeOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		sec   float64
		value int64
		flags uint32
	}{
		{"zero", 0, 0, CMTimeFlagValid},
		{"one", 1, 600_000, CMTimeFlagValid},
		{"microsecond", 1.000001, 600_001, CMTimeFlagValid},
		{"negative", -2.5, -1_500_000, CMTimeFlagValid},
		{"nan", math.NaN(), 0, CMTimeFlagValid | CMTimeFlagIndefinite},
		{"plus inf", math.Inf(1), 0, CMTimeFlagValid | CMTimeFlagPositiveInfinity},
		{"minus inf", math.Inf(-1), 0, CMTimeFlagValid | CMTimeFlagNegativeInfinity},
		{"too long", 1e15, 0, CMTimeFlagValid | CMTimeFlagPositiveInfinity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CMTimeOf(tt.sec)
			assert.Equal(t, tt.flags, got.Flags)
			assert.Equal(t, tt.value, got.Value)
			if got.Flags == CMTimeFlagValid {
				assert.Equal(t, CMTimescale, got.Timescale)
				back, ok := got.Seconds()
				assert.True(t, ok)
				assert.InDelta(t, tt.sec, back, 2e-6, "a seek is exact within 2 µs")
			}
		})
	}
}

func TestBufferedEnd(t *testing.T) {
	t.Parallel()

	at := func(s, d float64) CMTimeRange { return CMTimeRange{Start: CMTimeOf(s), Duration: CMTimeOf(d)} }
	tests := []struct {
		name string
		rs   []CMTimeRange
		want float64
	}{
		{"none", nil, 0},
		{"one", []CMTimeRange{at(0, 12.5)}, 12.5},
		{"one from the middle", []CMTimeRange{at(30, 10)}, 40},
		{"several take the last, as player.js does", []CMTimeRange{at(0, 5), at(50, 10), at(20, 3)}, 23},
		{"invalid last", []CMTimeRange{at(0, 5), {}}, 0},
		{"indefinite duration", []CMTimeRange{{Start: CMTimeOf(1), Duration: CMTimeOf(math.NaN())}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.InDelta(t, tt.want, BufferedEnd(tt.rs), 1e-6)
		})
	}
}
