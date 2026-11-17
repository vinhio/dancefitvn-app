package sys

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestFreeSpaceOffPhone checks that off a phone the space is unknown
// (Q-P5-9), for an existing folder as for a missing one.
func TestFreeSpaceOffPhone(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		t.Skip("the phones' answer is checked in e2e/sys")
	}
	for name, path := range map[string]string{
		"folder":  t.TempDir(),
		"missing": filepath.Join(t.TempDir(), "missing"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			free, total, err := FreeSpace(path)
			require.ErrorIs(t, err, core.ErrUnsupported)
			assert.Zero(t, free)
			assert.Zero(t, total)
		})
	}
}

// TestStatfsSpace checks the statfs the phones fall back on (iOS) or use
// (Android) on the machine's own disk, and its error for a missing path.
func TestStatfsSpace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		ok   bool
	}{
		{name: "temp folder", path: t.TempDir(), ok: true},
		{name: "missing", path: filepath.Join(t.TempDir(), "missing", "audio"), ok: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			free, total, err := statfsSpace(tc.path)
			if !tc.ok {
				require.Error(t, err)
				assert.Zero(t, free)
				assert.Zero(t, total)

				return
			}
			require.NoError(t, err)
			assert.Positive(t, total)
			assert.GreaterOrEqual(t, free, int64(0))
			assert.LessOrEqual(t, free, total)
		})
	}
}
