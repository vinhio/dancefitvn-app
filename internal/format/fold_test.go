package format

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFold(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"ascii unchanged", "hello world 35", "hello world 35"},
		{"ascii upper", "SET 35", "set 35"},
		{"d stroke", "Đùi Mông", "dui mong"},
		{"sentence", "Cùng nhảy cùng hát", "cung nhay cung hat"},
		{"title", "Giới thiệu DanceFIT", "gioi thieu dancefit"},
		{"horn letters", "Ơ ơ Ư ư", "o o u u"},
		{"all tones of a", "ạảấầẩẫậắằẳẵặ ĂăÂâ", "aaaaaaaaaaaa aaaa"},
		{"e", "ẹẻẽếềểễệ ÊêÉé", "eeeeeeee eeee"},
		{"i", "ỉịĩ ÌÍì", "iii iii"},
		{"o", "ọỏốồổỗộớờởỡợ ÔôÕõ", "oooooooooooo oooo"},
		{"u", "ụủứừửữự ŨũÙú", "uuuuuuu uuuu"},
		{"y", "ỳỵỷỹ Ýý", "yyyy yy"},
		{"nfd input", "Đùi Mông cảm", "dui mong cam"},
		{"kept symbols", "Hãy Hát Lên - 100%!", "hay hat len - 100%!"},
		{"non vietnamese", "Straße Æ", "straße æ"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, Fold(tc.in))
			assert.Equal(t, tc.want, string(AppendFold(nil, tc.in)))
		})
	}
}

func TestFoldEveryVietnameseLetter(t *testing.T) {
	t.Parallel()

	const lower = "aàáảãạăằắẳẵặâầấẩẫậeèéẻẽẹêềếểễệiìíỉĩịoòóỏõọôồốổỗộơờớởỡợuùúủũụưừứửữựyỳýỷỹỵđ"
	base := strings.Repeat("a", 18) + strings.Repeat("e", 12) + strings.Repeat("i", 6) +
		strings.Repeat("o", 18) + strings.Repeat("u", 12) + strings.Repeat("y", 6) + "d"
	var got []rune
	for _, r := range Fold(lower) {
		got = append(got, r)
	}
	assert.Equal(t, base, string(got))
	assert.Equal(t, Fold(lower), Fold(upperVN(lower)))
}

// upperVN upper-cases the Vietnamese letters of s for the test.
func upperVN(s string) string {
	out := []rune(s)
	for i, r := range out {
		switch {
		case r == 'đ':
			out[i] = 'Đ'
		case r >= 0x1EA0 && r <= 0x1EF9:
			out[i] = r &^ 1
		case r >= 'a' && r <= 'z':
			out[i] = r - 32
		case r >= 0xE0 && r <= 0xFD:
			out[i] = r - 32
		case r == 0x103 || r == 0x129 || r == 0x169 || r == 0x1A1 || r == 0x1B0:
			out[i] = r - 1
		}
	}

	return string(out)
}

func TestFoldDoesNotAllocate(t *testing.T) {
	buf := make([]byte, 0, 64)
	for _, in := range []string{"Đùi Mông", "SET 35", "Cùng nhảy cùng hát"} {
		allocs := testing.AllocsPerRun(100, func() { buf = AppendFold(buf[:0], in) })
		assert.Zero(t, allocs, in)
	}
}

func BenchmarkAppendFold(b *testing.B) {
	b.ReportAllocs()
	buf := make([]byte, 0, 64)
	for i := 0; i < b.N; i++ {
		buf = AppendFold(buf[:0], "Cùng nhảy cùng hát")
	}
}
