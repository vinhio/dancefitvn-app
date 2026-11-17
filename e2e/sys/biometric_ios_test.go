// New for P8 (first real iPhone): what Biometry says on a phone, with and
// without the Info.plist's NSFaceIDUsageDescription. No sheet is shown and
// no enrolment is touched: only LAContext canEvaluatePolicy:.

//go:build ios && !tvos

package sys

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/sys"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestBiometryOnDevice checks 03 §3 K13 / F6: without the usage sentence a
// Face ID phone answers BiometryNone (the button hides); with it the answer
// is Face ID, or none when nothing is enrolled. Never a crash.
func TestBiometryOnDevice(t *testing.T) {
	plist, err := os.ReadFile(filepath.Join(filepath.Dir(os.Args[0]), "Info.plist"))
	require.NoError(t, err)
	hasKey := strings.Contains(string(plist), "NSFaceIDUsageDescription")

	kind, err := sys.Biometry()
	t.Logf("biometry: usage key %v, kind %s, err %v", hasKey, kind, err)
	if !hasKey {
		assert.Equal(t, sys.BiometryNone, kind, "no usage sentence: the button must hide")

		return
	}
	assert.Contains(t, []sys.BiometryKind{sys.BiometryFace, sys.BiometryNone}, kind)
}
