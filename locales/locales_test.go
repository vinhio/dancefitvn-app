package locales

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextsHoldsTheP0Keys(t *testing.T) {
	t.Parallel()

	texts, err := Texts()
	require.NoError(t, err)
	cases := map[string]string{
		"splash.brand":     "DANCEFIT",
		"splash.country":   "VIỆT NAM",
		"splash.motto":     "Dễ · Vui · Khoẻ",
		"tv.welcome.login": "Đăng nhập",
		"tv.welcome.more":  "Tìm hiểu thêm",
	}
	for key, want := range cases {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, want, texts[key])
		})
	}
}

func TestMustTextsEqualsTexts(t *testing.T) {
	t.Parallel()

	texts, err := Texts()
	require.NoError(t, err)
	assert.Equal(t, texts, MustTexts())
}

func TestTextsDecodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		data string
		want map[string]string
		ok   bool
	}{
		{name: "flat object", data: `{"a":"x","b":"y"}`, want: map[string]string{"a": "x", "b": "y"}, ok: true},
		{name: "empty object", data: `{}`, want: map[string]string{}, ok: true},
		{name: "nested value", data: `{"a":{"b":"c"}}`},
		{name: "number value", data: `{"a":1}`},
		{name: "array", data: `["a"]`},
		{name: "null", data: `null`},
		{name: "broken", data: `{"a":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := decode([]byte(tc.data))
			if !tc.ok {
				require.ErrorIs(t, err, core.ErrInvalid)
				assert.Nil(t, got)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestMustTextsPanicsOnBadJSON(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() { must(decode([]byte(`{"a":`))) })
	assert.NotPanics(t, func() { must(decode([]byte(`{"a":"b"}`))) })
}

// isDecomposed tells whether s holds a combining mark (U+0300 to U+036F,
// which includes the dot below U+0323 of Vietnamese): text in NFD. gotext draws
// a mark that follows its letter as a separate glyph where it should sit on it
// (GM24 of jPlayer Mobile), so every shown text is precomposed (NFC); the
// stdlib has no normalizer and golang.org/x/text is barred from the ui layers
// (Q-P2-2), so the test looks for the marks.
func isDecomposed(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return r >= 0x300 && r <= 0x36f }) >= 0
}

func TestTextsAreNFC(t *testing.T) {
	t.Parallel()

	texts, err := Texts()
	require.NoError(t, err)
	for key, s := range texts {
		assert.False(t, isDecomposed(s), "%s: %q is not precomposed", key, s)
	}
}

// TestMockTextsAreNFC checks every string of the mock API's JSON files.
func TestMockTextsAreNFC(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "mock", "dancefitvn-mock-api")
	if _, err := os.Stat(root); err != nil {
		t.Skip("no mock API next to the module")
	}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case string:
			assert.False(t, isDecomposed(x), "%s: %q is not precomposed", path, x)
		case []any:
			for _, e := range x {
				walk(path, e)
			}
		case map[string]any:
			for k, e := range x {
				walk(path+"."+k, e)
			}
		default:
		}
	}
	files := 0
	require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			return nil // not every .json under the mock is data
		}
		walk(path, v)
		files++

		return nil
	}))
	assert.Positive(t, files)
}

// kindsOfNewKeys are the marks a line of new-keys.txt may carry.
var kindsOfNewKeys = map[string]bool{"new": true, "03s8": true, "proto": true}

// readNewKeys reads new-keys.txt: key to kind.
func readNewKeys(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile("new-keys.txt")
	require.NoError(t, err)
	out := map[string]string{}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		require.Len(t, f, 2, "new-keys.txt: %q is not '<key> <kind>'", line)
		require.True(t, kindsOfNewKeys[f[1]], "new-keys.txt: %q: unknown kind", line)
		_, dup := out[f[0]]
		require.False(t, dup, "new-keys.txt: %s twice", f[0])
		out[f[0]] = f[1]
	}

	return out
}

// pieces splits a text at its {placeholders} and its line breaks into the
// fragments the prototype holds literally.
func pieces(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		rest := line
		for {
			i := strings.IndexByte(rest, '{')
			j := strings.IndexByte(rest, '}')
			if i < 0 || j < i {
				break
			}
			if f := strings.TrimSpace(rest[:i]); f != "" {
				out = append(out, f)
			}
			rest = rest[j+1:]
		}
		if f := strings.TrimSpace(rest); f != "" {
			out = append(out, f)
		}
	}

	return out
}

// progressNewKeys returns the keys of PROGRESS.md section 7 (the table of
// strings awaiting review, Q13), "home.weekday.0–5" spelled out; nil
// when there is no such file.
func progressNewKeys(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "docs", "plan", "PROGRESS.md"))
	if err != nil {
		return nil
	}
	_, after, ok := strings.Cut(string(data), "## 7. ")
	if !ok {
		return nil
	}
	section, _, _ := strings.Cut(after, "\n## ")
	keys := map[string]bool{}
	for line := range strings.SplitSeq(section, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 3 || !strings.HasPrefix(strings.TrimSpace(cells[1]), "`") {
			continue
		}
		k := strings.Trim(strings.TrimSpace(cells[1]), "`")
		if i := strings.Index(k, "–"); i > 0 && i+len("–") < len(k) {
			prefix := k[:i-1]
			from, to := k[i-1], k[i+len("–")]
			for c := from; c <= to; c++ {
				keys[prefix+string(c)] = true
			}

			continue
		}
		keys[k] = true
	}

	return keys
}

// TestEveryTextIsPrototypeOrNew is the locale test of 07 section 2: every
// value of vi.json is a string of the prototype (local/strings.json, what
// extract_strings.py finds), or its key is in new-keys.txt as a new string
// (Q13, PROGRESS section 7), a template of 03 section 8, or a text the
// prototype builds at run time, whose fragments are then found in the
// prototype's source. The list holds nothing stale: each of its keys is in
// vi.json and its value is not a string extract_strings.py finds.
func TestEveryTextIsPrototypeOrNew(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "local", "strings.json"))
	if err != nil {
		t.Skip("no local/strings.json: run reference/tools/extract_strings.py")
	}
	var list []string
	require.NoError(t, json.Unmarshal(raw, &list))
	found := make(map[string]bool, len(list))
	for _, s := range list {
		found[s] = true
	}
	texts, err := Texts()
	require.NoError(t, err)
	listed := readNewKeys(t)
	src, srcErr := os.ReadFile(filepath.Join("..", "reference", "prototype", "index.html"))

	for key, value := range texts {
		kind, isListed := listed[key]
		switch {
		case found[value] && isListed:
			t.Errorf("%s: %q is a prototype string, drop it from new-keys.txt", key, value)
		case found[value]:
		case !isListed:
			t.Errorf("%s: %q is not in extract_strings output: add it to new-keys.txt with its kind (new, 03s8, proto)", key, value)
		case kind == "proto" && srcErr == nil:
			for _, f := range pieces(value) {
				assert.Contains(t, string(src), f, "%s: %q is not in the prototype", key, f)
			}
		default:
		}
	}
	for key := range listed {
		if _, ok := texts[key]; !ok {
			t.Errorf("new-keys.txt: %s is not in vi.json", key)
		}
	}
}

// TestNewKeysAreInProgress checks that each string new-keys.txt calls new is
// in the table of PROGRESS.md section 7, where the owner reviews it; a new
// string missing there is logged, not failed (the table is a document).
func TestNewKeysAreInProgress(t *testing.T) {
	t.Parallel()

	progress := progressNewKeys(t)
	if progress == nil {
		t.Skip("no docs/plan/PROGRESS.md section 7")
	}
	texts, err := Texts()
	require.NoError(t, err)
	for key, kind := range readNewKeys(t) {
		if kind == "new" && !progress[key] {
			t.Logf("PROGRESS section 7 does not list the new string %s (%q)", key, texts[key])
		}
	}
	for key := range progress {
		if _, ok := texts[key]; !ok {
			t.Logf("PROGRESS section 7 lists %s, which vi.json does not have", key)
		}
	}
}

// TestPiecesSplitsAtPlaceholders checks the helper of the locale test.
func TestPiecesSplitsAtPlaceholders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want []string
	}{
		{"Tất cả", []string{"Tất cả"}},
		{"Tất cả {n}", []string{"Tất cả"}},
		{"{a} · Bài {no} · {tag}", []string{"· Bài", "·"}},
		{"a\nb {x}", []string{"a", "b"}},
		{"{n}", nil},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, pieces(tc.in))
		})
	}
}
