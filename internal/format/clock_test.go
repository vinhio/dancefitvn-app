package format

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type clockVector struct {
	Sec  float64 `json:"sec"`
	Want string  `json:"want"`
}

func readVectors[T any](t *testing.T, path string) []T {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	var v []T
	require.NoError(t, json.Unmarshal(b, &v))

	return v
}

func TestClockVectors(t *testing.T) {
	t.Parallel()

	rows := readVectors[clockVector](t, "testdata/fmt.json")
	require.Len(t, rows, 17)
	rows = append(rows,
		clockVector{6000, "100:00"},
		clockVector{6059.9, "100:59"},
		clockVector{36000, "600:00"},
		clockVector{math.NaN(), "00:00"},
		clockVector{math.Inf(-1), "00:00"},
		clockVector{math.Copysign(0, -1), "00:00"},
	)
	for _, tc := range rows {
		t.Run(tc.Want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.Want, Clock(tc.Sec))
			assert.Equal(t, tc.Want, string(AppendClock(nil, tc.Sec)))
		})
	}
}

func TestClockInfDoesNotPanic(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, Clock(math.Inf(1)))
}

func TestClockMatchesLocalVectors(t *testing.T) {
	t.Parallel()

	const path = "../../local/vectors/fmt.json"
	if _, err := os.Stat(path); err != nil {
		t.Skip("local/vectors absent")
	}
	for _, tc := range readVectors[clockVector](t, path) {
		assert.Equal(t, tc.Want, Clock(tc.Sec), "sec %v", tc.Sec)
	}
}

func TestClockTableMatchesAppend(t *testing.T) {
	t.Parallel()

	for s := 0; s < clockTableSeconds; s++ {
		require.Equal(t, string(AppendClock(nil, float64(s))), Clock(float64(s)), "sec %d", s)
	}
}

func TestClockDoesNotAllocate(t *testing.T) {
	// Not parallel: AllocsPerRun must not see other tests' allocations.
	for _, sec := range []float64{0, 72, 5999, -3} {
		allocs := testing.AllocsPerRun(100, func() { _ = Clock(sec) })
		assert.Zero(t, allocs, "Clock(%v)", sec)
	}
	buf := make([]byte, 0, 16)
	allocs := testing.AllocsPerRun(100, func() { buf = AppendClock(buf[:0], 5999) })
	assert.Zero(t, allocs, "AppendClock")
}

func TestTotalMinutes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		sec  int
		want string
	}{{2340, "39"}, {540, "09"}, {0, "00"}, {6000, "100"}} {
		assert.Equal(t, tc.want, TotalMinutes(tc.sec), "sec %d", tc.sec)
	}
}

func BenchmarkClock(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Clock(float64(i % 6000))
	}
}

func BenchmarkAppendClock(b *testing.B) {
	b.ReportAllocs()
	buf := make([]byte, 0, 16)
	for i := 0; i < b.N; i++ {
		buf = AppendClock(buf[:0], float64(i%6000))
	}
}
