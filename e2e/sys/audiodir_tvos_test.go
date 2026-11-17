//go:build ios && tvos

package sys

import (
	"os"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/require"

	jsys "dancefitvn-app/internal/sys"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestAudioDirUnsupported checks that the Apple TV, which downloads
// nothing, has no audio folder.
func TestAudioDirUnsupported(t *testing.T) { //nolint:paralleltest // one app a process
	_, err := jsys.AudioDir()
	require.ErrorIs(t, err, core.ErrUnsupported)
	_, err = jsys.PrepareAudioDir()
	require.ErrorIs(t, err, core.ErrUnsupported)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// spaceDir is the folder TestFreeSpace reads on tvOS: the data folder
// (Library/Caches), made.
func spaceDir(t *testing.T) string {
	t.Helper()

	dir, err := jsys.DataDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o700))

	return dir
}
