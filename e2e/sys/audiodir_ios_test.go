//go:build ios && !tvos

package sys

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jsys "dancefitvn-app/internal/sys"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// audioDirSteps checks the iOS folder: DataDir()/audio, where it was,
// flagged out of iCloud and computer backups.
func audioDirSteps(t *testing.T, legacy, dir string) {
	t.Helper()

	assert.Equal(t, legacy, dir, "the folder stays in Application Support")
	excluded, err := jsys.ExcludedFromBackup(dir)
	require.NoError(t, err)
	assert.True(t, excluded, "NSURLIsExcludedFromBackupKey")
}
