//go:build !tvos

// Adapted from jplayer-mobile/internal/l10n/search_spec_test.go at 49de3c2
// (TestSpecNorm: the vectors of the query's normal form, here for both settings of
// the accent switch, K18) and from its alloc_spec_test.go (the filter allocates
// nothing).

package screens

import (
	"testing"

	"dancefitvn-app/internal/model"

	"github.com/stretchr/testify/assert"
)

// TestSearchNormalForm checks the normal form of a query and of a text searched in:
// JavaScript's trim (white space of both ends, the BOM, not NEL), the lower case, and
// with the switch on the Vietnamese marks and đ gone, composed or not.
func TestSearchNormalForm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, in, plain, fold string
	}{
		{"sample of 07 section 2.5", "Phố lên Đèn", "phố lên đèn", "pho len den"},
		{"already plain", "pho len den", "pho len den", "pho len den"},
		{"empty", "", "", ""},
		{"only spaces", "   ", "", ""},
		{"trimmed", "  Hà Nội \t", "hà nội", "ha noi"},
		{"capital D with stroke", "ĐÀ NẴNG", "đà nẵng", "da nang"},
		{"small d with stroke", "đường", "đường", "duong"},
		{"decomposed input (NFD)", "Pho\u0302\u0301 len", "pho\u0302\u0301 len", "pho len"},
		{"all the tones", "àáảãạ", "àáảãạ", "aaaaa"},
		{"upper case", "SONG 2", "song 2", "song 2"},
		{"digits and punctuation stay", "Song 2 · Phố", "song 2 · phố", "song 2 · pho"},
		{"other scripts stay", "日本語", "日本語", "日本語"},
		{"no-break space is trimmed (String.trim)", " pho ", "pho", "pho"},
		{"BOM is trimmed (String.trim)", "\ufeffpho", "pho", "pho"},
		{"line separators are trimmed", " pho \n", "pho", "pho"},
		{"NEL is not (String.trim)", "\u0085pho", "\u0085pho", "\u0085pho"},
		{"inner spaces stay", "set  35", "set  35", "set  35"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.plain, string(appendQuery(nil, tc.in, false)), "the prototype's")
			assert.Equal(t, tc.fold, string(appendQuery(nil, tc.in, true)), "folded")
			assert.Equal(t, tc.plain, string(appendQuery([]byte("x"), tc.in, false)[1:]), "appends")
		})
	}
}

// TestSearchTextIsNotTrimmed checks that the text searched in is lower-cased and not
// trimmed (`C[id].name.toLowerCase()`), whatever the switch.
func TestSearchTextIsNotTrimmed(t *testing.T) {
	t.Parallel()

	assert.Equal(t, " set 35 ", string(appendHay(nil, " SET 35 ", false)))
	assert.Equal(t, " set 35 ", string(appendHay(nil, " SET 35 ", true)))
	assert.Equal(t, "cùng", string(appendHay(nil, "CÙNG", false)))
	assert.Equal(t, "cung", string(appendHay(nil, "CÙNG", true)))
}

// TestSearchSwitchIsOffForThePrototype checks that the switch is the prototype's
// until the owner decides K18.
func TestSearchSwitchIsOffForThePrototype(t *testing.T) {
	t.Parallel()

	assert.False(t, SearchFoldAccents, "Q-P3-11: the prototype's code wins; K18 is pending")
}

// TestCourseFilterAllocatesNothing checks that the filter, once its buffers have the
// room, allocates nothing for any query, category or result.
func TestCourseFilterAllocatesNothing(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	courses := []model.Course{
		{ID: "set-35", Name: "DanceFIT Set 35", Tagline: "Cùng nhảy cùng hát", Category: model.CategoryRegular},
		{ID: "hot-extra", Name: "DanceFIT Hot Extra", Tagline: "Hot Extra", Category: model.CategorySpecial},
		{ID: "intro", Name: "Giới thiệu DanceFIT", Category: model.CategoryIntro},
	}
	f := newCourseFilter(courses)
	queries := []string{"", "s", "set 3", "  SET 35 ", "cùng", "zzz", "\ufeffđặc biệt ", "giới thiệu dancefit"}
	cats := []model.Category{model.CategoryUnknown, model.CategoryRegular, model.CategorySpecial, model.CategoryIntro}
	for _, q := range queries {
		for _, c := range cats {
			f.apply(c, q)
		}
	}
	assert.Zero(t, testing.AllocsPerRun(100, func() {
		for _, q := range queries {
			for _, c := range cats {
				f.apply(c, q)
			}
		}
	}))
}

// BenchmarkCourseFilter is a key of the search over 15 courses.
func BenchmarkCourseFilter(b *testing.B) {
	var courses []model.Course
	for range 15 {
		courses = append(courses, model.Course{Name: "DanceFIT Set 35", Tagline: "Cùng nhảy cùng hát", Category: model.CategoryRegular})
	}
	f := newCourseFilter(courses)
	b.ReportAllocs()
	for b.Loop() {
		f.apply(model.CategoryUnknown, "cùng nhảy")
	}
}
