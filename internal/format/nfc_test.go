package format

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNFC(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"ascii", "Set 35 - 100%", "Set 35 - 100%"},
		{"precomposed stays", "B\u1ee5ng N\u1eafng Th\u01b0\u01a1ng", "B\u1ee5ng N\u1eafng Th\u01b0\u01a1ng"},
		{"dot below", "Bu\u0323ng", "B\u1ee5ng"},
		{"breve and acute", "Na\u0306\u0301ng", "N\u1eafng"},
		{"horns", "Thu\u031bo\u031bng", "Th\u01b0\u01a1ng"},
		{"to hop form", "N\u0103\u0301ng Th\u01b0\u01a1ng B\u01b0\u0323ng", "N\u1eafng Th\u01b0\u01a1ng B\u1ef1ng"},
		{"marks out of order", "a\u0306\u0323 a\u0323\u0306", "\u1eb7 \u1eb7"},
		{"precomposed letter, mark under", "\u0103\u0323", "\u1eb7"},
		{"upper case", "BU\u0323NG NA\u0306\u0301NG THU\u031bO\u031bNG", "B\u1ee4NG N\u1eaeNG TH\u01af\u01a0NG"},
		{"d with stroke has no marks", "\u0111a\u0323p", "\u0111\u1ea1p"},
		{"mark without letter", "\u0301a", "\u0301a"},
		{"mark that composes with nothing", "x\u0301", "x\u0301"},
		{"second tone stays", "a\u0301\u0301", "\u00e1\u0301"},
		{"emoji untouched", "Cowboy \U0001F920 Bu\u0323ng", "Cowboy \U0001F920 B\u1ee5ng"},
		{"invalid byte before a mark", "\xff\u0301", "\xff\u0301"},
		{"latin outside vietnamese", "Re\u0301sume\u0301 n\u0303", "R\u00e9sum\u00e9 \u00f1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, NFC(tc.in))
			assert.Equal(t, "x"+tc.want, string(AppendNFC([]byte("x"), tc.in)))
			assert.Equal(t, tc.want, NFC(tc.want), "NFC is idempotent")
		})
	}
}

// TestNFCVectors checks the vectors scripts/gen-nfc.py wrote with Python's
// unicodedata: every Vietnamese letter decomposed, in the "to hop" form, with
// its marks in every order, and every composite of the table decomposed.
func TestNFCVectors(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/nfc.txt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		in, want, ok := strings.Cut(sc.Text(), "\t")
		require.True(t, ok, sc.Text())
		assert.Equal(t, runes(t, want), NFC(runes(t, in)), "line %d: %s", n+1, sc.Text())
		n++
	}
	require.NoError(t, sc.Err())
	assert.Greater(t, n, 1000)
}

// TestNFCTables checks the generated tables: each sorted as its search
// needs, the second a permutation of the first.
func TestNFCTables(t *testing.T) {
	t.Parallel()

	require.Len(t, nfcSplits, len(nfcPairs))
	for i := 1; i < len(nfcPairs); i++ {
		p, q := nfcPairs[i-1], nfcPairs[i]
		assert.True(t, p.base < q.base || (p.base == q.base && p.mark < q.mark), "nfcPairs[%d]", i)
		assert.Less(t, nfcSplits[i-1].composite, nfcSplits[i].composite, "nfcSplits[%d]", i)
	}
	for _, p := range nfcSplits {
		got, ok := composite(p.base, p.mark)
		assert.True(t, ok)
		assert.Equal(t, p.composite, got)
		assert.NotZero(t, class(p.mark), "U+%04X composes with a mark NFC reorders", p.composite)
	}
}

func TestNFCDoesNotAllocate(t *testing.T) {
	const composed = "B\u1ee5ng N\u1eafng Th\u01b0\u01a1ng - Set 35"
	dst := make([]byte, 0, 64)
	assert.Zero(t, testing.AllocsPerRun(100, func() { _ = NFC(composed) }), "NFC of composed text")
	assert.Zero(t, testing.AllocsPerRun(100, func() { dst = AppendNFC(dst[:0], "Bu\u0323ng Na\u0306\u0301ng Thu\u031bo\u031bng") }),
		"AppendNFC of decomposed text into a buffer with room")
}

func BenchmarkNFC(b *testing.B) {
	for _, bc := range []struct{ name, in string }{
		{"composed", "B\u1ee5ng N\u1eafng Th\u01b0\u01a1ng - Set 35 - B\u00e0i 03"},
		{"decomposed", "Bu\u0323ng Na\u0306\u0301ng Thu\u031bo\u031bng - Set 35 - Ba\u0300i 03"},
	} {
		b.Run(bc.name, func(b *testing.B) {
			dst := make([]byte, 0, 128)
			b.ReportAllocs()
			for b.Loop() {
				dst = AppendNFC(dst[:0], bc.in)
			}
		})
	}
}

// runes decodes "0041 0301" into its characters.
func runes(t *testing.T, hexes string) string {
	t.Helper()
	var b []byte
	for h := range strings.FieldsSeq(hexes) {
		v, err := strconv.ParseUint(h, 16, 32)
		require.NoError(t, err, h)
		b = utf8.AppendRune(b, rune(v))
	}

	return string(b)
}
