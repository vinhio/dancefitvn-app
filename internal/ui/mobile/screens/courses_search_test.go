//go:build !tvos

// Adapted from jplayer-mobile/internal/l10n/search_spec_test.go at 49de3c2 (the table
// of queries and what each finds, checked against the screen) and from its
// alloc_spec_test.go (steady: warm up, then AllocsPerRun).

package screens_test

import (
	"strconv"
	"testing"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/screens"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The filters by their index, as the buttons are.
const (
	fAll = iota
	fRegular
	fSpecial
	fIntro
)

// searchCase is one row of the table: the filter on, the text typed, and the ids of
// the cards that show with the prototype's search (plain) and with the accents
// folded (fold, K18); a nil fold is the same list.
type searchCase struct {
	name   string
	filter int
	query  string
	plain  []string
	fold   []string
}

// regular is the 12 regular courses of the mock, in order.
var regular = []string{
	"set-35", "set-34", "set-33", "set-32", "set-31", "set-30", "set-29", "set-28", "set-27", "set-26", "set-25", "set-24",
}

// searchCases are the queries of P3.4: the prototype's (`trim().toLowerCase()` and
// `includes` on `name` and `quote`) against the folded search of 03 section 4.
var searchCases = []searchCase{
	{"empty shows all", fAll, "", mockCourses, nil},
	{"a part of the name", fAll, "set 3", []string{"set-35", "set-34", "set-33", "set-32", "set-31", "set-30"}, nil},
	{"upper case", fAll, "SET 35", []string{"set-35"}, nil},
	{"tagline without marks", fAll, "cung nhay", nil, []string{"set-35"}},
	{"tagline with marks", fAll, "cùng nhảy", []string{"set-35"}, nil},
	{"tagline in lower case", fAll, "hooray", []string{"set-33"}, nil},
	{"tagline in upper case", fAll, "DANCING ON", []string{"set-31"}, nil},
	{"trimmed", fAll, "  set 34 ", []string{"set-34"}, nil},
	{"BOM and no-break space are trimmed", fAll, "\ufeffset 35\u00a0", []string{"set-35"}, nil},
	{"NEL is not trimmed", fAll, "\u0085set 35", nil, nil},
	{"name with marks", fAll, "giới thiệu", []string{"intro"}, nil},
	{"name without marks", fAll, "gioi thieu", nil, []string{"intro"}},
	{"decomposed input", fAll, "cu\u0300ng nha\u0309y", nil, []string{"set-35"}},
	{"punctuation and marks", fAll, "kiêu hãnh… lên!", []string{"set-34"}, nil},
	{"punctuation without marks", fAll, "kieu hanh… len!", nil, []string{"set-34"}},
	{"nothing", fAll, "zzz", nil, nil},
	{"the category is not searched", fAll, "đặc", nil, nil},
	{"the brand is in every name", fAll, "dancefit", mockCourses, nil},
	{"regular only", fRegular, "", regular, nil},
	{"regular with a query", fRegular, "set 2", []string{"set-29", "set-28", "set-27", "set-26", "set-25", "set-24"}, nil},
	{"regular, no special", fRegular, "hot", nil, nil},
	{"special", fSpecial, "", []string{"hot-extra", "hot-extra-2"}, nil},
	{"special with a query", fSpecial, "hot", []string{"hot-extra", "hot-extra-2"}, nil},
	{"special, the number", fSpecial, "extra 2", []string{"hot-extra-2"}, nil},
	{"special by the brand", fSpecial, "dancefit", []string{"hot-extra", "hot-extra-2"}, nil},
	{"intro", fIntro, "", []string{"intro"}, nil},
	{"intro, no set", fIntro, "set", nil, nil},
	{"intro by name", fIntro, "dancefit", []string{"intro"}, nil},
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestCoursesSearchTable checks the cards that show, the count under the field and the
// empty text for each filter and query of the table, on both phones: the search of the
// prototype's code, or the folded one if SearchFoldAccents says so. The screen is made
// once per phone and the cases follow one another on it, as a user's keys do.
func TestCoursesSearchTable(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atCourses(t, ph, false)
		c := coursesOf(t, r).Content()
		for _, tc := range searchCases {
			want := tc.plain
			if screens.SearchFoldAccents && tc.fold != nil {
				want = tc.fold
			}
			c.SetFilter(tc.filter)
			c.SetQuery(tc.query)
			r.Tester.RunUntilIdle()

			var got []string
			for i, id := range mockCourses {
				if !c.Row(i).Node().Hidden {
					got = append(got, id)
				}
			}
			assert.Equal(t, want, got, tc.name)
			assert.Equal(t, len(want), c.ShownRows(), tc.name)
			assert.True(t, r.Tester.Shows("Hiển thị "+strconv.Itoa(len(want))+" khoá học"), tc.name)
			assert.Equal(t, len(want) == 0, r.Tester.Shows("Không tìm thấy khoá học “"+tc.query+"”"), "%s: the empty text, with q as typed", tc.name)
			assert.Equal(t, 15, c.Count(fAll), "%s: the buttons count the answer, not the result", tc.name)
			assert.True(t, r.Tester.Shows("Tất cả 15"), tc.name)
		}
	})
}

// TestCoursesSearchBothSettings checks, whatever the switch is, that the table's two
// columns are both what a setting of the switch gives: the folded list holds the plain
// one.
func TestCoursesSearchBothSettings(t *testing.T) {
	t.Parallel()

	for _, tc := range searchCases {
		if tc.fold == nil {
			continue
		}
		assert.NotEmpty(t, tc.fold, tc.name)
		assert.Subset(t, mockCourses, tc.fold, tc.name)
		assert.Subset(t, tc.fold, tc.plain, "%s: folding finds what the plain search finds", tc.name)
	}
}

// TestCoursesKeystrokeDoesNotAllocate checks that the search adds no allocation to a
// key: the text field's own allocations (the tester's events, the field's string) are
// what a key into a bare field costs, and filtering, laying out and drawing the cards
// that come and go cost nothing more, on both phones. The one thing that allocates is
// the text of an empty result (`Không tìm thấy khoá học “q”`), made once per query
// that finds nothing.
func TestCoursesKeystrokeDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atCourses(t, ph, false)
		c := coursesOf(t, r).Content()
		tap(r, c.Input())
		key := func() {
			r.Tester.Type("s")
			r.Tester.Frame()
			r.Tester.Key(keys.KeyBackspace, 0)
			r.Tester.Frame()
		}
		for range 2 {
			key()
		}
		withSearch := testing.AllocsPerRun(5, key)

		in := kit.NewInput(icon.Search, "")
		in.Search = true
		bare := kittest.Mount(t, ph, in)
		kittest.PinMotion(bare)
		bare.ClickAt(in.Node().WindowFrame().Center())
		bareKey := func() {
			bare.Type("s")
			bare.Frame()
			bare.Key(keys.KeyBackspace, 0)
			bare.Frame()
		}
		for range 2 {
			bareKey()
		}
		assert.LessOrEqual(t, withSearch, testing.AllocsPerRun(5, bareKey), ph.Name+": the search adds nothing to a key")

		// the filter, the layout and the frame on their own
		queries := []string{"set 3", "set 35", "set", "hot", "", "extra 2"}
		i := 0
		step := func() {
			c.SetQuery(queries[i%len(queries)])
			i++
			r.Tester.Frame()
		}
		for range 2 * len(queries) {
			step()
		}
		assert.Zero(t, testing.AllocsPerRun(len(queries)*2, step), ph.Name+": a query, the cards that come and go, a frame")

		// a tap on a filter button: the cards that come and go cost nothing, and with the
		// buttons' names joined once (kit.Segmented), nor does the layout and the frame
		c.SetQuery("")
		step2 := func() {
			c.SetFilter(i % 4)
			i++
		}
		for range 8 {
			step2()
			r.Tester.Frame()
		}
		assert.Zero(t, testing.AllocsPerRun(12, step2), ph.Name+": the filter's cards, without the frame")
		withFrame := func() {
			step2()
			r.Tester.Frame()
		}
		assert.Zero(t, testing.AllocsPerRun(8, withFrame), ph.Name+": the filter's cards, the layout and the frame")

		c.SetQuery("zzz")
		r.Tester.Frame()
		assert.Zero(t, testing.AllocsPerRun(20, r.Tester.Repaint), ph.Name+": the empty text standing still")
	}
}

// BenchmarkCoursesKeystroke is a query of the search: filter, layout and a frame.
func BenchmarkCoursesKeystroke(b *testing.B) {
	r := atCourses(b, kittest.Phones[0], false)
	c := coursesOf(b, r).Content()
	queries := []string{"set 3", "set 35", "set", "hot", "", "extra 2"}
	for i := range 12 {
		c.SetQuery(queries[i%len(queries)])
		r.Tester.Frame()
	}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		c.SetQuery(queries[i%len(queries)])
		i++
		r.Tester.Frame()
	}
}
