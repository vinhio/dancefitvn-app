//go:build !dancefitvndev

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestPerfStatesAbsentWithoutDevTag checks the performance states are not
// in the build that ships (nor in the tests' shot states), on the phone
// and on the TV.
func TestPerfStatesAbsentWithoutDevTag(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"PERF-SCROLL", "PERF-SCROLL-M05", "PERF-VIDEO10"} {
		assert.False(t, perfStateKnown(id), id)
	}
}
