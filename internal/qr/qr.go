package qr

import (
	"errors"
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Level is the error correction level.
type Level uint8

// Error correction levels, recovering about 7, 15, 25 and 30 percent.
const (
	LevelUnknown Level = iota
	LevelL
	LevelM
	LevelQ
	LevelH
)

// MaxVersion is the largest symbol version (177 x 177 modules).
const MaxVersion = 40

// ErrTooLong reports a payload that does not fit version 40 at the level.
// It wraps core.ErrInvalid.
var ErrTooLong = fmt.Errorf("qr: data too long: %w", core.ErrInvalid)

// Code is an encoded QR symbol; it holds the dark modules, no quiet zone.
type Code struct {
	size    int
	version int
	mask    int
	level   Level
	dark    []bool // row-major, size*size
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Encode encodes data in byte mode at the smallest version that fits and
// picks the mask with the lowest ISO 18004 penalty. A Level other than
// L, M, Q or H gives core.ErrInvalid, as does a payload too long for
// version 40 (ErrTooLong).
func Encode(data []byte, lvl Level) (*Code, error) {
	return encode(data, lvl, 1, -1)
}

// encode is Encode with a minimum version (1..40) and a forced mask
// (0..7, -1 = automatic).
func encode(data []byte, lvl Level, minVersion, mask int) (*Code, error) {
	if lvl < LevelL || lvl > LevelH {
		return nil, fmt.Errorf("qr: level %d: %w", lvl, core.ErrInvalid)
	}
	if minVersion < 1 || minVersion > MaxVersion || mask < -1 || mask > 7 {
		return nil, fmt.Errorf("qr: version %d mask %d: %w", minVersion, mask, core.ErrInvalid)
	}
	version := minVersion
	for ; ; version++ {
		if version > MaxVersion {
			return nil, errors.Join(ErrTooLong, fmt.Errorf("qr: %d bytes at level %d", len(data), lvl))
		}
		if len(data) <= byteCapacity(version, lvl) {
			break
		}
	}
	cw := dataCodewords(data, version, lvl)
	all := interleave(cw, version, lvl)
	return build(all, version, lvl, mask), nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Size returns the side length in modules.
func (c *Code) Size() int { return c.size }

// Version returns the symbol version, 1..40.
func (c *Code) Version() int { return c.version }

// Mask returns the mask pattern index, 0..7.
func (c *Code) Mask() int { return c.mask }

// Level returns the error correction level.
func (c *Code) Level() Level { return c.level }

// Black reports whether the module at column x, row y is dark. Positions
// outside the symbol are light (the quiet zone).
func (c *Code) Black(x, y int) bool {
	if x < 0 || y < 0 || x >= c.size || y >= c.size {
		return false
	}
	return c.dark[y*c.size+x]
}

// String returns the symbol as text, one line per row, '#' dark and ' ' light.
func (c *Code) String() string {
	out := make([]byte, 0, (c.size+1)*c.size)
	for y := 0; y < c.size; y++ {
		for x := 0; x < c.size; x++ {
			if c.dark[y*c.size+x] {
				out = append(out, '#')
			} else {
				out = append(out, ' ')
			}
		}
		out = append(out, '\n')
	}
	return string(out)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// byteCapacity returns how many bytes fit in version at lvl in byte mode.
func byteCapacity(version int, lvl Level) int {
	bits := dataCapacityBits(version, lvl)
	head := 4 + countBits(version)
	if bits < head {
		return 0
	}
	return (bits - head) / 8
}

// countBits is the width of the byte-mode character count field.
func countBits(version int) int {
	if version <= 9 {
		return 8
	}
	return 16
}

// dataCapacityBits returns the number of data bits (not codewords' ECC).
func dataCapacityBits(version int, lvl Level) int {
	return (rawCodewords(version) - eccPerBlock[lvl][version]*numBlocks[lvl][version]) * 8
}

// dataCodewords builds the padded data codeword sequence for data.
func dataCodewords(data []byte, version int, lvl Level) []byte {
	capBytes := dataCapacityBits(version, lvl) / 8
	out := make([]byte, 0, capBytes)
	var acc uint32
	var nbits int
	put := func(v uint32, n int) {
		acc = acc<<uint(n) | v
		nbits += n
		for nbits >= 8 {
			out = append(out, byte(acc>>uint(nbits-8)))
			nbits -= 8
		}
		acc &= 1<<uint(nbits) - 1
	}
	put(0x4, 4) // byte mode
	put(uint32(len(data)), countBits(version))
	for _, b := range data {
		put(uint32(b), 8)
	}
	free := capBytes*8 - (len(out)*8 + nbits)
	if free > 4 {
		free = 4
	}
	put(0, free) // terminator
	if nbits > 0 {
		put(0, 8-nbits)
	}
	for pad := byte(0xEC); len(out) < capBytes; pad ^= 0xEC ^ 0x11 {
		out = append(out, pad)
	}
	return out
}

// interleave splits cw into blocks, appends Reed-Solomon codewords, and
// interleaves data then ECC codewords as ISO 18004 section 7.6 says.
func interleave(cw []byte, version int, lvl Level) []byte {
	nb := numBlocks[lvl][version]
	ecc := eccPerBlock[lvl][version]
	raw := rawCodewords(version)
	short := nb - raw%nb
	shortLen := raw / nb
	gen := rsGenerator(ecc)
	blocks := make([][]byte, nb)
	eccs := make([][]byte, nb)
	k := 0
	for i := range blocks {
		n := shortLen - ecc
		if i >= short {
			n++
		}
		blocks[i] = cw[k : k+n]
		eccs[i] = rsRemainder(blocks[i], gen)
		k += n
	}
	out := make([]byte, 0, raw)
	for i := 0; i <= shortLen-ecc; i++ {
		for j, b := range blocks {
			if i != shortLen-ecc || j >= short {
				out = append(out, b[i])
			}
		}
	}
	for i := 0; i < ecc; i++ {
		for _, e := range eccs {
			out = append(out, e[i])
		}
	}
	return out
}

// rawDataModules returns the modules available for data and ECC bits.
func rawDataModules(version int) int {
	n := (16*version+128)*version + 64
	if version >= 2 {
		na := version/7 + 2
		n -= (25*na-10)*na - 55
		if version >= 7 {
			n -= 36
		}
	}
	return n
}

// rawCodewords returns the total codeword count (data + ECC).
func rawCodewords(version int) int { return rawDataModules(version) / 8 }

// alignPositions returns the alignment pattern center coordinates.
func alignPositions(version int) []int {
	if version == 1 {
		return nil
	}
	na := version/7 + 2
	step := 26
	if version != 32 {
		step = (version*8 + na*3 + 5) / (na*4 - 4) * 2
	}
	size := version*4 + 17
	pos := make([]int, na)
	pos[0] = 6
	for i, p := na-1, size-7; i >= 1; i, p = i-1, p-step {
		pos[i] = p
	}
	return pos
}
