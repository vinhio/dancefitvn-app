package format

import (
	"math"
	"math/big"
	"strconv"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// MB formats a size in megabytes (10^6 bytes) the way the prototype's mb()
// does: one decimal and a comma, "8,8" for 8 800 000 bytes.
func MB(bytes int64) string { return string(AppendMB(make([]byte, 0, 16), bytes)) }

// Size is MB with the unit: "8,8 MB".
func Size(bytes int64) string { return string(AppendSize(make([]byte, 0, 16), bytes)) }

// GB formats a size in gigabytes (10^9 bytes) with the rules of MB and the
// unit, for the free-space line ("Trống 18,2 GB"): "18,2 GB" for
// 18 200 000 000 bytes.
func GB(bytes int64) string { return string(AppendGB(make([]byte, 0, 16), bytes)) }

// AppendMB appends MB(bytes) to dst.
func AppendMB(dst []byte, bytes int64) []byte { return appendFixed1(dst, megabytes(bytes)) }

// AppendSize appends Size(bytes) to dst.
func AppendSize(dst []byte, bytes int64) []byte {
	return append(AppendMB(dst, bytes), " MB"...)
}

// AppendGB appends GB(bytes) with its unit, "18,2 GB", to dst.
func AppendGB(dst []byte, bytes int64) []byte {
	if bytes < 0 {
		bytes = 0
	}

	return append(appendFixed1(dst, float64(bytes)/1e9), " GB"...)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// megabytes is the double the prototype computes for a size.
func megabytes(bytes int64) float64 {
	if bytes < 0 {
		bytes = 0
	}

	return float64(bytes) / 1e6
}

// appendFixed1 appends x as x.toFixed(1) does in JavaScript, with a comma as
// the separator: it rounds the exact binary value of x and takes the larger
// of two equally near results. strconv would round an exact tie to even.
func appendFixed1(dst []byte, x float64) []byte {
	n := tenths(x)
	dst = strconv.AppendUint(dst, n/10, 10)
	dst = append(dst, ',')

	return append(dst, byte('0'+n%10))
}

// tenths returns round(x*10) for x >= 0, ties up, on the exact value of x.
func tenths(x float64) uint64 {
	if !(x > 0) {
		return 0
	}
	if x >= 1e15 { // beyond any size; keeps the conversions below in range
		return uint64(math.Min(x*10, 1<<63))
	}
	p := x * 10
	f := math.Floor(p)
	d := p - f
	// x*10 in floating point is off by at most half an ulp; far from a tie
	// the float answer is the exact one.
	margin := p*1e-15 + 1e-9
	if math.Abs(d-0.5) > margin {
		if d > 0.5 {
			return uint64(f) + 1
		}

		return uint64(f)
	}

	return tenthsBig(x)
}

// tenthsBig is tenths on exact arithmetic: x*10 and the 0.5 are exact in
// 256 bits, and Int truncates toward zero.
func tenthsBig(x float64) uint64 {
	t := new(big.Float).SetPrec(256).SetFloat64(x)
	t.Mul(t, big.NewFloat(10))
	t.Add(t, big.NewFloat(0.5))
	n, _ := t.Int(nil)

	return n.Uint64()
}
