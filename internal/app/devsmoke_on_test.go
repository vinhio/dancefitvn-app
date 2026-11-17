//go:build dancefitvndev && !tvos

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestProofStatesRegisteredUnderDevTag checks the P4.9 D3 device-proof states
// exist in a developer's build (the tag-less twin checks they do not).
func TestProofStatesRegisteredUnderDevTag(t *testing.T) {
	t.Parallel()
	for _, id := range []string{smokeSaveState, smokeWatchState, smokeABState} {
		st, ok := shotStates[id]
		assert.True(t, ok, id)
		assert.NotEmpty(t, st.steps, id)
		assert.NotNil(t, st.prepare, id)
	}
}
