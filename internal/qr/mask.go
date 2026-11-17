package qr

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// builder holds the matrix while it is drawn.
type builder struct {
	size  int
	dark  []bool
	fixed []bool // function modules: never masked, never carry data
}

// build draws the function patterns, places codewords, picks the mask
// (forced when mask >= 0) and writes format and version information.
func build(codewords []byte, version int, lvl Level, mask int) *Code {
	size := version*4 + 17
	b := &builder{size: size, dark: make([]bool, size*size), fixed: make([]bool, size*size)}
	b.drawFunction(version)
	b.drawCodewords(codewords)
	if mask < 0 {
		best := 1 << 30
		for m := 0; m < 8; m++ {
			b.applyMask(m)
			b.drawFormat(lvl, m)
			if p := b.penalty(); p < best {
				best, mask = p, m
			}
			b.applyMask(m) // undo
		}
	}
	b.applyMask(mask)
	b.drawFormat(lvl, mask)
	return &Code{size: size, version: version, mask: mask, level: lvl, dark: b.dark}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// set sets a function module.
func (b *builder) set(x, y int, dark bool) {
	b.dark[y*b.size+x] = dark
	b.fixed[y*b.size+x] = true
}

// drawFunction draws timing, finder, alignment and version patterns and
// reserves the format areas.
func (b *builder) drawFunction(version int) {
	for i := 0; i < b.size; i++ {
		b.set(6, i, i%2 == 0)
		b.set(i, 6, i%2 == 0)
	}
	b.finder(3, 3)
	b.finder(b.size-4, 3)
	b.finder(3, b.size-4)
	pos := alignPositions(version)
	for i, cy := range pos {
		for j, cx := range pos {
			last := len(pos) - 1
			if (i == 0 && j == 0) || (i == 0 && j == last) || (i == last && j == 0) {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					b.set(cx+dx, cy+dy, max(abs(dx), abs(dy)) != 1)
				}
			}
		}
	}
	b.drawFormat(LevelM, 0) // reserve
	if version >= 7 {
		rem := version
		for i := 0; i < 12; i++ {
			rem = rem<<1 ^ (rem>>11)*0x1F25
		}
		bits := version<<12 | rem
		for i := 0; i < 18; i++ {
			dark := bits>>uint(i)&1 != 0
			a, c := b.size-11+i%3, i/3
			b.set(a, c, dark)
			b.set(c, a, dark)
		}
	}
}

// finder draws a finder pattern with its separator centered at (cx, cy).
func (b *builder) finder(cx, cy int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			x, y := cx+dx, cy+dy
			if x < 0 || y < 0 || x >= b.size || y >= b.size {
				continue
			}
			d := max(abs(dx), abs(dy))
			b.set(x, y, d != 2 && d != 4)
		}
	}
}

// drawFormat writes both copies of the 15-bit format information.
func (b *builder) drawFormat(lvl Level, mask int) {
	data := formatLevelBits[lvl]<<3 | mask
	rem := data
	for i := 0; i < 10; i++ {
		rem = rem<<1 ^ (rem>>9)*0x537
	}
	bits := (data<<10 | rem) ^ 0x5412
	bit := func(i int) bool { return bits>>uint(i)&1 != 0 }
	for i := 0; i <= 5; i++ {
		b.set(8, i, bit(i))
	}
	b.set(8, 7, bit(6))
	b.set(8, 8, bit(7))
	b.set(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		b.set(14-i, 8, bit(i))
	}
	for i := 0; i < 8; i++ {
		b.set(b.size-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		b.set(8, b.size-15+i, bit(i))
	}
	b.set(8, b.size-8, true)
}

// drawCodewords places the bits in the zig-zag order of ISO 18004 7.7.3.
func (b *builder) drawCodewords(cw []byte) {
	i := 0
	for right := b.size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < b.size; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				y := vert
				if (right+1)&2 == 0 {
					y = b.size - 1 - vert
				}
				if !b.fixed[y*b.size+x] && i < len(cw)*8 {
					b.dark[y*b.size+x] = cw[i>>3]>>uint(7-i&7)&1 != 0
					i++
				}
			}
		}
	}
}

// maskBit reports whether pattern m inverts module (x, y).
func maskBit(m, x, y int) bool {
	switch m {
	case 0:
		return (x+y)%2 == 0
	case 1:
		return y%2 == 0
	case 2:
		return x%3 == 0
	case 3:
		return (x+y)%3 == 0
	case 4:
		return (x/3+y/2)%2 == 0
	case 5:
		return x*y%2+x*y%3 == 0
	case 6:
		return (x*y%2+x*y%3)%2 == 0
	default:
		return ((x+y)%2+x*y%3)%2 == 0
	}
}

// applyMask XORs pattern m onto the data modules; applying twice undoes it.
func (b *builder) applyMask(m int) {
	for y := 0; y < b.size; y++ {
		for x := 0; x < b.size; x++ {
			if !b.fixed[y*b.size+x] && maskBit(m, x, y) {
				b.dark[y*b.size+x] = !b.dark[y*b.size+x]
			}
		}
	}
}

// penalty scores the matrix with the four ISO 18004 rules.
func (b *builder) penalty() int {
	n := b.size
	at := func(x, y int) bool { return b.dark[y*n+x] }
	score := 0
	count := 0
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if at(x, y) {
				count++
			}
		}
	}
	for horiz := 0; horiz < 2; horiz++ {
		get := func(i, j int) bool {
			if horiz == 0 {
				return at(j, i)
			}
			return at(i, j)
		}
		for i := 0; i < n; i++ {
			run := 1
			for j := 1; j <= n; j++ {
				if j < n && get(i, j) == get(i, j-1) {
					run++
					continue
				}
				if run >= 5 {
					score += 3 + run - 5
				}
				run = 1
			}
			// N3: 1:1:3:1:1 with four light modules on one side.
			for j := 0; j+7 <= n; j++ {
				if !(get(i, j) && !get(i, j+1) && get(i, j+2) && get(i, j+3) && get(i, j+4) && !get(i, j+5) && get(i, j+6)) {
					continue
				}
				light := func(from, to int) bool {
					if from < 0 || to > n {
						return false
					}
					for k := from; k < to; k++ {
						if get(i, k) {
							return false
						}
					}
					return true
				}
				if light(j-4, j) {
					score += 40
				}
				if light(j+7, j+11) {
					score += 40
				}
			}
		}
	}
	for y := 0; y+1 < n; y++ {
		for x := 0; x+1 < n; x++ {
			c := at(x, y)
			if c == at(x+1, y) && c == at(x, y+1) && c == at(x+1, y+1) {
				score += 3
			}
		}
	}
	total := n * n
	k := (abs(count*20-total*10)+total-1)/total - 1
	return score + k*10
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
