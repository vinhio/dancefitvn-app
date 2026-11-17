//go:build android && cgo

package sys

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// audioDirSteps checks the Android folder: under no_backup, Auto Backup's
// exclusion, and the folder of before gone once moved.
func audioDirSteps(t *testing.T, legacy, dir string) {
	t.Helper()

	assert.NotEqual(t, legacy, dir)
	assert.True(t, strings.Contains(dir, "/no_backup/"), "%s is in no_backup", dir)
	assert.NoDirExists(t, legacy, "the folder of before is gone")
}
