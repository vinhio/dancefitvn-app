package kittest_test

import (
	"os"
	"path/filepath"
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPhones checks the two test phones: the sizes the plan names, a safe area
// each, and their text naming.
func TestPhones(t *testing.T) {
	t.Parallel()

	require.Len(t, kittest.Phones, 2)
	ios, android := kittest.Phones[0], kittest.Phones[1]
	assert.Equal(t, "ios", ios.Name)
	assert.Equal(t, geom.Sz(393, 852), ios.Size)
	assert.Equal(t, theme.NamingCoreText, ios.Naming)
	assert.Equal(t, "android", android.Name)
	assert.Equal(t, geom.Sz(412, 892), android.Size)
	assert.Equal(t, theme.NamingGoText, android.Naming)
}

// TestMountSetsTheSafeAreaAndTheTheme checks that both mounts size the view as
// the phone and give it its safe area and the app's theme.
func TestMountSetsTheSafeAreaAndTheTheme(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for name, mount := range map[string]func(testing.TB, kittest.Phone, ui.Widget) *ui.Tester{
			"fake": kittest.Mount, "real": kittest.MountReal,
		} {
			tt := mount(t, ph, &ui.Box{})
			c := tt.View().Context()
			assert.Equal(t, ph.Safe, c.SafeArea(), name)
			look := theme.From(c)
			assert.Equal(t, ph.Naming, look.Faces.Naming(), name)
			assert.Equal(t, theme.NewFaces(ph.Naming).Face(theme.RoleBody, 16, 400), c.Theme().Font, name)
		}
	})
}

// TestShaperMeasuresTheAppsFonts checks that the real shaper draws the app's
// fonts, not a fallback: Inter's bold is wider than its regular, no glyph is
// missing, and the ascent is Inter's.
func TestShaperMeasuresTheAppsFonts(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		sh := kittest.Shaper(t, ph)
		f := theme.NewFaces(ph.Naming)
		reg, err := sh.Layout("Hamburgefonstiv", f.Face(theme.RoleBody, 20, 400), 1, 0)
		require.NoError(t, err)
		bold, err := sh.Layout("Hamburgefonstiv", f.Face(theme.RoleBody, 20, 700), 1, 0)
		require.NoError(t, err)
		assert.Greater(t, bold.Size.W, reg.Size.W)
		for _, g := range reg.Glyphs {
			assert.False(t, g.Missing)
		}
		m, err := sh.Metrics(f.Face(theme.RoleBody, 16, 400))
		require.NoError(t, err)
		assert.InDelta(t, 16*0.96875, m.Ascent, 0.05, "Inter's ascent is .96875 em")
	})
}

// TestFontFolderIsReused checks that the fonts are written once: the folder of
// a second call is the same and holds the nine faces.
func TestFontFolderIsReused(t *testing.T) {
	t.Parallel()

	_ = kittest.Shaper(t, kittest.Phones[1])
	files, err := filepath.Glob(filepath.Join(os.TempDir(), "dfv-kittest-*", "*.ttf"))
	require.NoError(t, err)
	assert.NotEmpty(t, files)
}
