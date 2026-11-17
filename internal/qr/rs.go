package qr

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// eccPerBlock is the ECC codewords per block, indexed [level][version]
// (ISO 18004 table 9).
var eccPerBlock = [5][41]int{
	LevelL: {0, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30, 28, 28, 28, 28, 30, 30, 26, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
	LevelM: {0, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28},
	LevelQ: {0, 13, 22, 18, 26, 18, 24, 18, 22, 20, 24, 28, 26, 24, 20, 30, 24, 28, 28, 26, 30, 28, 30, 30, 30, 30, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
	LevelH: {0, 17, 28, 22, 16, 22, 28, 26, 26, 24, 28, 24, 28, 22, 24, 24, 30, 28, 28, 26, 28, 30, 24, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
}

// numBlocks is the number of error correction blocks, indexed [level][version].
var numBlocks = [5][41]int{
	LevelL: {0, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8, 8, 9, 9, 10, 12, 12, 12, 13, 14, 15, 16, 17, 18, 19, 19, 20, 21, 22, 24, 25},
	LevelM: {0, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49},
	LevelQ: {0, 1, 1, 2, 2, 4, 4, 6, 6, 8, 8, 8, 10, 12, 16, 12, 17, 16, 18, 21, 20, 23, 23, 25, 27, 29, 34, 34, 35, 38, 40, 43, 45, 48, 51, 53, 56, 59, 62, 65, 68},
	LevelH: {0, 1, 1, 2, 4, 4, 4, 5, 6, 8, 8, 11, 11, 16, 16, 18, 16, 19, 21, 25, 25, 25, 34, 30, 32, 35, 37, 40, 42, 45, 48, 51, 54, 57, 60, 63, 66, 70, 74, 77, 81},
}

// formatLevelBits is the two-bit level field of the format information.
var formatLevelBits = [5]int{LevelL: 1, LevelM: 0, LevelQ: 3, LevelH: 2}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// gfExp and gfLog are the GF(256) tables for x^8+x^4+x^3+x^2+1 (0x11D).
var gfExp, gfLog = func() (e [510]byte, l [256]byte) {
	x := 1
	for i := 0; i < 255; i++ {
		e[i] = byte(x)
		l[x] = byte(i)
		x <<= 1
		if x >= 256 {
			x ^= 0x11D
		}
	}
	for i := 255; i < 510; i++ {
		e[i] = e[i-255]
	}
	return e, l
}()

// gfMul multiplies in GF(256).
func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

// rsGenerator returns the generator polynomial of degree n without the
// leading coefficient, highest power first.
func rsGenerator(n int) []byte {
	g := make([]byte, n)
	g[n-1] = 1
	root := byte(1)
	for i := 0; i < n; i++ {
		for j := range g {
			g[j] = gfMul(g[j], root)
			if j+1 < n {
				g[j] ^= g[j+1]
			}
		}
		root = gfMul(root, 2)
	}
	return g
}

// rsRemainder returns the Reed-Solomon ECC codewords of data.
func rsRemainder(data, gen []byte) []byte {
	rem := make([]byte, len(gen))
	for _, b := range data {
		f := b ^ rem[0]
		copy(rem, rem[1:])
		rem[len(rem)-1] = 0
		for i, c := range gen {
			rem[i] ^= gfMul(c, f)
		}
	}
	return rem
}
