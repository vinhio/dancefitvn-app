//go:build !tvos

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestDumpA11y checks the dump of Home (M03) on both phones: the content's
// tree with the window first and the tabs' labels and actions, no system's
// elements off a phone (and why), then the files of devA11y: the state's
// and, after a key press, a second one, and the end logged.
func TestDumpA11y(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, nil, "")
		r.mount(ph, "M03", false)
		dir := filepath.Join(must(r.deps.CacheDir()), shotFolder)
		r.drive("the layout of M03", func() bool {
			_, err := os.Stat(filepath.Join(dir, "M03.layout.json"))

			return err == nil
		})
		rep := dumpA11y("M03", r.tt.View())
		require.NotEmpty(t, rep.Tree)
		assert.Equal(t, "M03", rep.Name)
		assert.Equal(t, "window", rep.Tree[0].Role)
		assert.Empty(t, rep.System)
		assert.NotEmpty(t, rep.SystemNote)
		i := slices.IndexFunc(rep.Tree, func(n a11yNode) bool {
			return n.Label == "Khoá học" && slices.Contains(n.Actions, "press")
		})
		require.GreaterOrEqual(t, i, 0, "the Khoá học tab")
		assert.Positive(t, rep.Tree[i].Depth)

		d, err := newDevA11y("M03", r.tt.View(), dir, "down", r.log.add)
		require.NoError(t, err)
		d.start()
		r.drive("the dumps", func() bool { return hasLine(r.log.all(), "dfv: a11y done M03") })
		for _, name := range []string{"M03", "M03-m1"} {
			b, err := os.ReadFile(filepath.Join(dir, name+".a11y.json"))
			require.NoError(t, err, name)
			var back a11yReport
			require.NoError(t, json.Unmarshal(b, &back), name)
			assert.Equal(t, name, back.Name)
			assert.NotEmpty(t, back.Tree, name)
			assert.True(t, hasLine(r.log.all(), "dfv: a11y ready "+name+" "), name)
		}
	})
}

// TestA11yKeys checks the names of the moves: the remote's arrows, select
// and menu; anything else is refused.
func TestA11yKeys(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		want keys.Key
		ok   bool
	}{
		"right":  {keys.KeyRight, true},
		"Down":   {keys.KeyDown, true},
		"select": {keys.KeyEnter, true},
		"menu":   {keys.KeyEscape, true},
		"tab":    {keys.KeyUnknown, false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			k, ok := a11yKey(name)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, k)
		})
	}
	_, err := newDevA11y("X", nil, "", "right, x", func(string, ...any) {})
	require.ErrorIs(t, err, core.ErrInvalid)
}
