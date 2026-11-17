//go:build tvos

package tvkit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
)

// skipUnderRace skips an allocation test in a -race build: the detector
// instruments every access (07 section 2.7); the plain build asserts the zero.
func skipUnderRace(t *testing.T) {
	t.Helper()
	if kittest.Race {
		t.Skip("allocation counts are asserted in the build without -race")
	}
}
