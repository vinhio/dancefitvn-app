package app

import (
	"testing"

	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestNewThemeIsNightStage checks the view's theme: the Night Stage look
// with the system's naming of faces.
func TestNewThemeIsNightStage(t *testing.T) {
	t.Parallel()

	got, want := newTheme(), theme.New(theme.DefaultNaming())
	require.NotNil(t, got)
	assert.Equal(t, want.Background, got.Background)
	assert.Equal(t, want.Text, got.Text)
	assert.Equal(t, want.Font, got.Font)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// frameOf returns the window frame of the widget named name; the test
// fails when there is none.
func frameOf(t *testing.T, tt *ui.Tester, name string) geom.Rect {
	t.Helper()
	_, r, ok := tt.Find(name)
	require.True(t, ok, "no widget named %q", name)

	return r
}
