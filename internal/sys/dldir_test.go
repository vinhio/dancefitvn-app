package sys

import (
	"os"
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

// TestAudioDirOffPhone checks that off a phone the audio folder is
// DataDir()/audio, the folder of before P5.1.
func TestAudioDirOffPhone(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		t.Skip("the phones' folder is checked in e2e/sys")
	}
	cfg, err := os.UserConfigDir()
	require.NoError(t, err)
	got, err := AudioDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cfg, AppID, "audio"), got)
}

// TestMoveAudio checks the one-time move of the folder of before: every
// entry moved and the old folder gone, nothing to do the second time, an
// entry the new folder has left where it was.
func TestMoveAudio(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		old      map[string]string // the old folder's files; nil: no old folder
		dest     map[string]string // the new folder's files before the move
		want     map[string]string // the new folder's files after
		left     []string          // the old folder's files after; nil: removed
		conflict bool
	}{
		{
			name: "no old folder",
			dest: map[string]string{"index.json": "new"},
			want: map[string]string{"index.json": "new"},
		},
		{
			name: "empty old folder",
			old:  map[string]string{},
			want: map[string]string{},
		},
		{
			name: "files and parts",
			old:  map[string]string{"index.json": "old", "l1.mp3": "a", "l2.mp3.part": "b"},
			want: map[string]string{"index.json": "old", "l1.mp3": "a", "l2.mp3.part": "b"},
		},
		{
			name:     "a name the new folder has",
			old:      map[string]string{"index.json": "old", "l1.mp3": "a"},
			dest:     map[string]string{"index.json": "new"},
			want:     map[string]string{"index.json": "new", "l1.mp3": "a"},
			left:     []string{"index.json"},
			conflict: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			from, to := filepath.Join(root, "data", "audio"), filepath.Join(root, "nobackup", "audio")
			require.NoError(t, os.MkdirAll(to, 0o700))
			writeFiles(t, to, tc.dest)
			if tc.old != nil {
				require.NoError(t, os.MkdirAll(from, 0o700))
				writeFiles(t, from, tc.old)
			}

			err := moveAudio(from, to)
			if tc.conflict {
				require.ErrorIs(t, err, core.ErrDuplicate)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.want, readFiles(t, to))
			if tc.left == nil {
				assert.NoDirExists(t, from)
			} else {
				got := readFiles(t, from)
				assert.Len(t, got, len(tc.left))
				for _, name := range tc.left {
					assert.Contains(t, got, name)
				}
			}

			// The second time finds nothing new to move.
			again := moveAudio(from, to)
			if tc.conflict {
				require.ErrorIs(t, again, core.ErrDuplicate)
			} else {
				require.NoError(t, again)
			}
			assert.Equal(t, tc.want, readFiles(t, to))
		})
	}
}

// TestMoveAudioSameFolder checks that a folder that stays where it was
// (iOS, the Mac) is left alone.
func TestMoveAudioSameFolder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"l1.mp3": "a"})
	require.NoError(t, moveAudio(dir, dir+string(filepath.Separator)))
	assert.Equal(t, map[string]string{"l1.mp3": "a"}, readFiles(t, dir))
}

// TestSettleAudioDir checks the move and the flag together off a phone,
// where the system has no flag: no error.
func TestSettleAudioDir(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "ios" {
		t.Skip("the flag is checked in e2e/sys")
	}
	root := t.TempDir()
	from, to := filepath.Join(root, "old"), filepath.Join(root, "new")
	require.NoError(t, os.MkdirAll(from, 0o700))
	require.NoError(t, os.MkdirAll(to, 0o700))
	writeFiles(t, from, map[string]string{"l1.mp3": "a"})
	require.NoError(t, settleAudioDir(from, to))
	assert.Equal(t, map[string]string{"l1.mp3": "a"}, readFiles(t, to))
	assert.NoDirExists(t, from)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// writeFiles writes each name's text into dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, text := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600))
	}
}

// readFiles reads every file of dir by name.
func readFiles(t *testing.T, dir string) map[string]string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	got := make(map[string]string, len(entries))
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // G304: the test's temp folder
		require.NoError(t, err)
		got[e.Name()] = string(b)
	}

	return got
}
