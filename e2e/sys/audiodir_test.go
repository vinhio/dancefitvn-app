//go:build (ios && !tvos) || (android && cgo)

package sys

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jsys "dancefitvn-app/internal/sys"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestAudioDir puts a file in the audio folder of before P5.1,
// DataDir()/audio, prepares the audio folder and finds the file in it:
// on Android moved from files/.config/… to the no-backup folder (the old
// folder gone), on iOS left where it was and the folder flagged out of
// backups. A second call finds nothing more to do.
func TestAudioDir(t *testing.T) { //nolint:paralleltest // one app a process
	data, err := jsys.DataDir()
	require.NoError(t, err)
	legacy := filepath.Join(data, "audio")
	require.NoError(t, os.MkdirAll(legacy, 0o700))
	probe := "e2e-probe.mp3"
	require.NoError(t, os.WriteFile(filepath.Join(legacy, probe), []byte("probe"), 0o600))

	dir, err := jsys.PrepareAudioDir()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(filepath.Join(dir, probe)) })
	want, err := jsys.AudioDir()
	require.NoError(t, err)
	assert.Equal(t, want, dir)
	t.Logf("audio folder: %s (before: %s)", dir, legacy)
	assert.FileExists(t, filepath.Join(dir, probe))
	audioDirSteps(t, legacy, dir)

	again, err := jsys.PrepareAudioDir()
	require.NoError(t, err)
	assert.Equal(t, dir, again)
	assert.FileExists(t, filepath.Join(dir, probe))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// spaceDir is the folder TestFreeSpace reads: the audio folder, made.
func spaceDir(t *testing.T) string {
	t.Helper()

	dir, err := jsys.PrepareAudioDir()
	require.NoError(t, err)

	return dir
}
