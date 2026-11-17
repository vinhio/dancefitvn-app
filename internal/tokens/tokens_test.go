package tokens

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/tokens/gen"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestColorsMatchPrototypeCSS checks that every colour of the dark scheme is
// the value of the matching CSS variable of the prototype (D6): the 14 :root
// variables and the scrim of .dim-bg.
func TestColorsMatchPrototypeCSS(t *testing.T) {
	t.Parallel()
	css := prototypeVars(t)
	tests := []struct {
		css string
		got color.Color
	}{
		{"bg", Dark.Bg},
		{"sf", Dark.Surface},
		{"sf2", Dark.Surface2},
		{"bd", Dark.Border},
		{"tx", Dark.Text},
		{"tx2", Dark.Text2},
		{"tx3", Dark.Text3},
		{"pr", Dark.Primary},
		{"on", Dark.OnPrimary},
		{"red", Dark.Red},
		{"ok", Dark.Success},
		{"warn", Dark.Warning},
		{"dg", Dark.Danger},
		{"info", Dark.Info},
		{"scrim", Dark.Scrim},
	}
	for _, tc := range tests {
		t.Run(tc.css, func(t *testing.T) {
			t.Parallel()
			raw, ok := css[tc.css]
			require.True(t, ok, "prototype has no %s", tc.css)
			assert.Equal(t, parseCSSColor(t, raw), tc.got)
		})
	}
}

// TestGeneratedIsCurrent checks that tokens_gen.go is what the generator
// produces from the current tokens file.
func TestGeneratedIsCurrent(t *testing.T) {
	t.Parallel()
	want, err := gen.Generate([]byte(readFile(t, tokensPath)))
	require.NoError(t, err)
	assert.Equal(t, string(want), readFile(t, "tokens_gen.go"), "run go generate ./internal/tokens")
}

// TestTokensJSONDarkIsCSS checks that the dark values of tokens.json are the
// values of the CSS variables of reference/design/tokens.css, so that a drift
// between the two generated outputs shows here.
func TestTokensJSONDarkIsCSS(t *testing.T) {
	t.Parallel()
	var f struct {
		Color map[string][]string `json:"color"`
	}
	require.NoError(t, json.Unmarshal([]byte(readFile(t, tokensPath)), &f))
	css := cssRootVars(t, readFile(t, "../../reference/design/tokens.css"), "color")
	require.Len(t, f.Color, len(css))
	for name, values := range f.Color {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.NotEmpty(t, values)
			assert.Equal(t, css[name], values[0])
		})
	}
}

// TestGenerateRejectsInvalid checks that the generator fails with ErrInvalid
// on a tokens file that drifted from its schema.
func TestGenerateRejectsInvalid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(m map[string]any)
	}{
		{"missing colour", func(m map[string]any) { delete(section(m, "color"), "bg") }},
		{"unknown colour", func(m map[string]any) { section(m, "color")["extra"] = []any{"#000000"} }},
		{"bad colour", func(m map[string]any) { section(m, "color")["bg"] = []any{"#GGGGGG"} }},
		{"bad shadow", func(m map[string]any) { section(m, "shadow")["card"] = "1 12 28 rgba(0,0,0,.28)" }},
		{"missing easing", func(m map[string]any) { delete(section(m, "motion"), "easing") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var m map[string]any
			require.NoError(t, json.Unmarshal([]byte(readFile(t, tokensPath)), &m))
			tc.mutate(m)
			in, err := json.Marshal(m)
			require.NoError(t, err)
			_, err = gen.Generate(in)
			assert.True(t, errors.Is(err, gen.ErrInvalid), "got %v", err)
		})
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// tokensPath is the design tokens file, relative to this package.
const tokensPath = "../../reference/design/tokens.json"

// section returns the object under key of the decoded tokens file.
func section(m map[string]any, key string) map[string]any {
	return m[key].(map[string]any)
}

// readFile returns the content of path, relative to this package.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

// prototypeVars returns the values of the :root variables of the prototype,
// keyed by name without the dashes, and the scrim of .dim-bg under "scrim".
func prototypeVars(t *testing.T) map[string]string {
	t.Helper()
	src := readFile(t, "../../reference/prototype/index.html")
	root := regexp.MustCompile(`:root\{([^}]*)\}`).FindStringSubmatch(src)
	require.NotNil(t, root, "no :root block in the prototype")
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`--([a-z0-9]+):([^;}]+)`).FindAllStringSubmatch(root[1], -1) {
		out[m[1]] = strings.TrimSpace(m[2])
	}
	dim := regexp.MustCompile(`\.dim-bg\{[^}]*background:([^;}]+)`).FindStringSubmatch(src)
	require.NotNil(t, dim, "no .dim-bg rule in the prototype")
	out["scrim"] = strings.TrimSpace(dim[1])
	return out
}

// cssRootVars returns the --<prefix>-* variables of the first :root block of
// src, keyed by the name after the prefix.
func cssRootVars(t *testing.T, src, prefix string) map[string]string {
	t.Helper()
	root := src[:strings.Index(src, "[data-theme")]
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`--`+prefix+`-([A-Za-z0-9]+):([^;}]+)`).FindAllStringSubmatch(root, -1) {
		out[m[1]] = strings.TrimSpace(m[2])
	}
	require.NotEmpty(t, out)
	return out
}

// parseCSSColor reads a CSS colour of the prototype: #RRGGBB or rgba(r,g,b,a).
func parseCSSColor(t *testing.T, s string) color.Color {
	t.Helper()
	if m := regexp.MustCompile(`^rgba\((\d+),\s*(\d+),\s*(\d+),\s*([\d.]+)\)$`).FindStringSubmatch(s); m != nil {
		var c [3]uint8
		for i := range c {
			v, err := strconv.ParseUint(m[i+1], 10, 8)
			require.NoError(t, err)
			c[i] = uint8(v)
		}
		a, err := strconv.ParseFloat(m[4], 32)
		require.NoError(t, err)
		return color.RGBA(c[0], c[1], c[2], float32(a))
	}
	c, err := color.Hex(s)
	require.NoError(t, err, "colour %q", s)
	return c
}
