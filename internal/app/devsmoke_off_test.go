//go:build !dancefitvndev && !tvos

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestProofStatesAbsentWithoutDevTag checks the device-proof states are not
// in the build that ships (nor in the tests' shot states).
func TestProofStatesAbsentWithoutDevTag(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"SMOKE-SAVE", "SMOKE-WATCHED", "SMOKE-AB", "SMOKE-PLAY", "SMOKE-RESUME"} {
		_, ok := shotStates[id]
		assert.False(t, ok, id)
	}
}
