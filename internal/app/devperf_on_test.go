//go:build dancefitvndev && !tvos

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPerfStatesRegisteredUnderDevTag checks the performance states exist
// in a developer's build (the tag-less twin checks they do not).
func TestPerfStatesRegisteredUnderDevTag(t *testing.T) {
	t.Parallel()
	for _, id := range []string{perfScrollState, perfScrollM05State, perfVideoState} {
		st, ok := shotStates[id]
		assert.True(t, ok, id)
		assert.NotEmpty(t, st.steps, id)
		assert.NotNil(t, st.prepare, id)
	}
}
