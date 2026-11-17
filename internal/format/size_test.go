package format

import (
	"math"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type sizeVector struct {
	Bytes int64  `json:"bytes"`
	Want  string `json:"want"`
}

func TestSizeVectors(t *testing.T) {
	t.Parallel()

	rows := readVectors[sizeVector](t, "testdata/size.json")
	assert.Len(t, rows, 10)
	for _, tc := range rows {
		assert.Equal(t, tc.Want, Size(tc.Bytes), "bytes %d", tc.Bytes)
	}
}

func TestSizeJSTies(t *testing.T) {
	t.Parallel()

	rows := readVectors[sizeVector](t, "testdata/size_ties.json")
	assert.Len(t, rows, 9)
	for _, tc := range rows {
		assert.Equal(t, tc.Want+" MB", Size(tc.Bytes), "bytes %d", tc.Bytes)
		assert.Equal(t, tc.Want, MB(tc.Bytes), "bytes %d", tc.Bytes)
	}
}

func TestSizeMatchesLocalVectors(t *testing.T) {
	t.Parallel()

	const path = "../../local/vectors/size.json"
	if _, err := os.Stat(path); err != nil {
		t.Skip("local/vectors absent")
	}
	for _, tc := range readVectors[sizeVector](t, path) {
		assert.Equal(t, tc.Want, Size(tc.Bytes), "bytes %d", tc.Bytes)
	}
}

func TestMB(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0,0"},
		{"negative", -5, "0,0"},
		{"small", 49999, "0,0"},
		{"tenth", 50000, "0,1"},
		{"typical", 8800000, "8,8"},
		{"large", 123456789012, "123456,8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, MB(tc.bytes))
		})
	}
}

// TestFixed1MatchesBigFloat compares the fast path with exact arithmetic on
// many sizes, ties included.
func TestFixed1MatchesBigFloat(t *testing.T) {
	t.Parallel()

	for b := int64(0); b < 3_000_000; b += 5_000 {
		x := float64(b) / 1e6
		assert.Equal(t, tenthsBig(x), tenths(x), "bytes %d", b)
	}
	for _, x := range []float64{0.05, 0.15, 0.25, 0.35, 0.45, 2.675, 1.005, 0.49999999999999994} {
		assert.Equal(t, tenthsBig(x), tenths(x), "x %v", x)
	}
}

func TestAppendMBDoesNotAllocate(t *testing.T) {
	buf := make([]byte, 0, 32)
	for _, b := range []int64{8800000, 7400000, 12800000} {
		allocs := testing.AllocsPerRun(100, func() { buf = AppendMB(buf[:0], b) })
		assert.Zero(t, allocs, "bytes %d", b)
	}
}

func TestTenthsHuge(t *testing.T) {
	t.Parallel()

	assert.NotPanics(t, func() { _ = tenths(math.MaxFloat64) })
	assert.NotPanics(t, func() { _ = tenths(math.Inf(1)) })
	assert.Zero(t, tenths(math.NaN()))
}

func TestGBVectors(t *testing.T) {
	t.Parallel()

	rows := readVectors[sizeVector](t, "testdata/gb.json")
	assert.Len(t, rows, 9)
	for _, tc := range rows {
		assert.Equal(t, tc.Want, GB(tc.Bytes), "bytes %d", tc.Bytes)
		assert.Equal(t, tc.Want, string(AppendGB([]byte("x"), tc.Bytes))[1:], "append, bytes %d", tc.Bytes)
	}
}

func TestAppendGBDoesNotAllocate(t *testing.T) {
	buf := make([]byte, 0, 32)
	allocs := testing.AllocsPerRun(1000, func() { buf = AppendGB(buf[:0], 18_200_000_000) })
	assert.Zero(t, allocs)
	assert.Equal(t, "18,2 GB", string(buf))
}
