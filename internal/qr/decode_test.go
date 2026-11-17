package qr

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

// grid is a bare module matrix the test decoder reads.
type grid struct {
	size int
	dark []bool
}

func gridOf(c *Code) grid { return grid{size: c.size, dark: c.dark} }

// gridFromText parses the '#'/'.' matrices written by testdata/gen.sh.
func gridFromText(s string) (grid, error) {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	g := grid{size: len(lines)}
	for _, ln := range lines {
		if len(ln) != g.size {
			return grid{}, errors.New("matrix is not square")
		}
		for _, r := range ln {
			g.dark = append(g.dark, r == '#')
		}
	}
	return g, nil
}

func (g grid) at(x, y int) bool { return g.dark[y*g.size+x] }

type decoded struct {
	data    []byte
	version int
	level   Level
	mask    int
}

// formatWord returns the 15-bit masked format word for level bits and mask
// (BCH(15,5), generator 0x537, XOR 0x5412), computed bit by bit.
func formatWord(levelBits, mask int) int {
	d := levelBits<<3 | mask
	r := d << 10
	for i := 14; i >= 10; i-- {
		if r>>uint(i)&1 != 0 {
			r ^= 0x537 << uint(i-10)
		}
	}
	return (d<<10 | r) ^ 0x5412
}

// decode is a plain ISO 18004 reader for byte-mode symbols: format
// information, unmask, zig-zag, de-interleave, Reed-Solomon syndromes, parse.
func decode(g grid) (decoded, error) {
	n := g.size
	if n < 21 || n > 177 || (n-17)%4 != 0 {
		return decoded{}, fmt.Errorf("bad size %d", n)
	}
	version := (n - 17) / 4
	// first copy of the format information
	var pos1 [15][2]int
	for i := 0; i <= 5; i++ {
		pos1[i] = [2]int{8, i}
	}
	pos1[6], pos1[7], pos1[8] = [2]int{8, 7}, [2]int{8, 8}, [2]int{7, 8}
	for i := 9; i < 15; i++ {
		pos1[i] = [2]int{14 - i, 8}
	}
	var pos2 [15][2]int
	for i := 0; i < 8; i++ {
		pos2[i] = [2]int{n - 1 - i, 8}
	}
	for i := 8; i < 15; i++ {
		pos2[i] = [2]int{8, n - 15 + i}
	}
	read := func(p [15][2]int) (w int) {
		for i, xy := range p {
			if g.at(xy[0], xy[1]) {
				w |= 1 << uint(i)
			}
		}
		return w
	}
	w1, w2 := read(pos1), read(pos2)
	if w1 != w2 {
		return decoded{}, errors.New("format copies differ")
	}
	lvlOf := map[int]Level{1: LevelL, 0: LevelM, 3: LevelQ, 2: LevelH}
	var lvl Level
	mask := -1
	for lb, l := range lvlOf {
		for m := 0; m < 8; m++ {
			if formatWord(lb, m) == w1 {
				lvl, mask = l, m
			}
		}
	}
	if mask < 0 {
		return decoded{}, errors.New("unknown format information")
	}
	// codeword bits in zig-zag order, unmasked
	b := &builder{size: n, dark: make([]bool, n*n), fixed: make([]bool, n*n)}
	b.drawFunction(version)
	var bits []bool
	for right := n - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for v := 0; v < n; v++ {
			for j := 0; j < 2; j++ {
				x := right - j
				y := v
				if (right+1)/2%2 == 0 {
					y = n - 1 - v
				}
				if b.fixed[y*n+x] {
					continue
				}
				bits = append(bits, g.at(x, y) != maskBit(mask, x, y))
			}
		}
	}
	raw := rawCodewords(version)
	cws := make([]byte, raw)
	for i := range cws {
		for k := 0; k < 8; k++ {
			cws[i] <<= 1
			if bits[i*8+k] {
				cws[i] |= 1
			}
		}
	}
	// de-interleave
	nb, ecc := numBlocks[lvl][version], eccPerBlock[lvl][version]
	short := nb - raw%nb
	blocks := make([][]byte, nb)
	k := 0
	for i := 0; i < raw/nb-ecc+1; i++ {
		for j := 0; j < nb; j++ {
			if i == raw/nb-ecc && j < short {
				continue
			}
			blocks[j] = append(blocks[j], cws[k])
			k++
		}
	}
	for i := 0; i < ecc; i++ {
		for j := 0; j < nb; j++ {
			blocks[j] = append(blocks[j], cws[k])
			k++
		}
	}
	var stream []byte
	for _, blk := range blocks {
		for i := 0; i < ecc; i++ { // syndrome S_i = block(alpha^i) must be 0
			var s byte
			for _, c := range blk {
				s = gfMul(s, gfExp[i]) ^ c
			}
			if s != 0 {
				return decoded{}, fmt.Errorf("nonzero syndrome %d", i)
			}
		}
		stream = append(stream, blk[:len(blk)-ecc]...)
	}
	// parse byte mode
	var acc []bool
	for _, c := range stream {
		for k := 7; k >= 0; k-- {
			acc = append(acc, c>>uint(k)&1 != 0)
		}
	}
	take := func(cnt int) (int, error) {
		if len(acc) < cnt {
			return 0, errors.New("stream too short")
		}
		v := 0
		for _, bit := range acc[:cnt] {
			v <<= 1
			if bit {
				v |= 1
			}
		}
		acc = acc[cnt:]
		return v, nil
	}
	mode, err := take(4)
	if err != nil || mode != 4 {
		return decoded{}, fmt.Errorf("mode %d is not byte mode", mode)
	}
	cnt, err := take(countBits(version))
	if err != nil {
		return decoded{}, err
	}
	data := make([]byte, 0, cnt)
	for i := 0; i < cnt; i++ {
		v, err := take(8)
		if err != nil {
			return decoded{}, err
		}
		data = append(data, byte(v))
	}
	if t, err := take(min(4, len(acc))); err != nil || t != 0 {
		return decoded{}, errors.New("missing terminator")
	}
	// padding: zero bits to the byte boundary, then 0xEC 0x11 alternating
	if len(acc)%8 != 0 {
		if v, _ := take(len(acc) % 8); v != 0 {
			return decoded{}, errors.New("nonzero bit padding")
		}
	}
	for i := 0; len(acc) > 0; i++ {
		v, _ := take(8)
		if want := [2]int{0xEC, 0x11}[i%2]; v != want {
			return decoded{}, errors.New("bad pad codeword")
		}
	}
	return decoded{data: bytes.Clone(data), version: version, level: lvl, mask: mask}, nil
}
