//go:build tvos

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestTVPerfStateUnderDevTag checks PERF-VIDEO10 is a TV state exactly in
// a developer's build.
func TestTVPerfStateUnderDevTag(t *testing.T) {
	t.Parallel()
	assert.Equal(t, devBuild, perfStateKnown("PERF-VIDEO10"))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// perfStateKnown tells whether the TV's shot states hold id.
func perfStateKnown(id string) bool {
	_, ok := tvShotStates[id]

	return ok
}
