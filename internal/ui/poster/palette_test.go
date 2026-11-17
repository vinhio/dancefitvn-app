package poster_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"dancefitvn-app/internal/ui/poster"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// protoID maps a prototype course key to the API id.
func protoID(k string) string {
	switch k {
	case "hotextra":
		return "hot-extra"
	case "hotextra2":
		return "hot-extra-2"
	}

	return "set-" + strings.TrimPrefix(k, "set")
}

// hex returns the color of a '#RRGGBB' literal.
func hex(t *testing.T, s string) color.Color {
	t.Helper()
	c, err := color.Hex(s)
	require.NoError(t, err)

	return c
}

// TestPaletteMatchesPrototype reads PAL and the addC lines of the prototype
// and compares them with the palette, for the rows addC defines.
func TestPaletteMatchesPrototype(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join(moduleRoot(t), "reference", "prototype", "index.html"))
	if err != nil {
		t.Skipf("no prototype: %v", err)
	}
	src := string(data)

	m := regexp.MustCompile(`const PAL=\[(.*?)\];`).FindStringSubmatch(src)
	require.NotNil(t, m)
	var pal [][2]string
	for _, p := range regexp.MustCompile(`\['(#[0-9A-Fa-f]{6})','(#[0-9A-Fa-f]{6})'\]`).FindAllStringSubmatch(m[1], -1) {
		pal = append(pal, [2]string{p[1], p[2]})
	}
	require.Len(t, pal, 12)

	want := map[string]poster.Colors{}
	add := func(id string, g [2]string, ink string, vinyl bool) {
		want[id] = poster.Colors{From: hex(t, g[0]), To: hex(t, g[1]), Ink: hex(t, ink), Vinyl: vinyl}
	}
	lineRe := regexp.MustCompile(`addC\('(set\d+|hotextra2?)',\{[^\n]*?g:(?:PAL\[(\d+)\]|\['(#[0-9A-Fa-f]{6})','(#[0-9A-Fa-f]{6})'\])(?:,ink:'(#[0-9A-Fa-f]{6})',vinyl:true)?`)
	for _, l := range lineRe.FindAllStringSubmatch(src, -1) {
		var g [2]string
		if l[2] != "" {
			i, _ := strconv.Atoi(l[2])
			g = pal[i]
		} else {
			g = [2]string{l[3], l[4]}
		}
		ink, vinyl := "#fff", false
		if l[5] != "" {
			ink, vinyl = l[5], true
		}
		add(protoID(l[1]), g, ink, vinyl)
	}
	// Set 30..24 come from a loop: PAL[(35-n)%12] (index.html:254), and are
	// not matched by the line pattern above.
	for n := 30; n >= 24; n-- {
		add("set-"+strconv.Itoa(n), pal[(35-n)%len(pal)], "#fff", false)
	}
	require.Len(t, want, 14, "Set 35..24 and the two Hot Extras")

	for id, w := range want {
		got, ok := poster.Lookup(id)
		assert.True(t, ok, id)
		assert.Equal(t, w, got, id)
	}
}

// TestPaletteHasTheSixteenCourses checks the table of 02 section 5 and the
// unknown-id rule.
func TestPaletteHasTheSixteenCourses(t *testing.T) {
	t.Parallel()
	ids := poster.IDs()
	assert.Len(t, ids, 16)
	for _, id := range ids {
		_, ok := poster.Lookup(id)
		assert.True(t, ok, id)
	}
	intro, _ := poster.Lookup("intro")
	assert.Equal(t, hex(t, "#2E2E35"), intro.From)
	assert.Equal(t, hex(t, "#E5383B"), intro.To)
	s36, _ := poster.Lookup("set-36")
	assert.Equal(t, hex(t, "#3A3D44"), s36.From)
	assert.Equal(t, hex(t, "#16171A"), s36.To)
	hot, _ := poster.Lookup("hot-extra-2")
	assert.True(t, hot.Vinyl)
	assert.Equal(t, hex(t, "#8A1424"), hot.Ink)

	other, ok := poster.Lookup("not-a-course")
	assert.False(t, ok)
	assert.Equal(t, hex(t, "#2A2D33"), other.From, "--sf2")
	assert.Equal(t, hex(t, "#141518"), other.To, "--bg")
	assert.False(t, other.Vinyl)
}
