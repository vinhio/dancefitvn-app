//go:build dancefitvndev && !tvos

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDownloadSmokesRegisteredUnderDevTag checks the download smokes of
// P5.1c exist in a developer's build, signed in by their prepare.
func TestDownloadSmokesRegisteredUnderDevTag(t *testing.T) {
	t.Parallel()
	for _, id := range []string{smokeDLState, smokeOfflineState, smokeRunningState} {
		st, ok := shotStates[id]
		assert.True(t, ok, id)
		assert.NotEmpty(t, st.steps, id)
		assert.NotNil(t, st.prepare, id)
	}
	assert.Equal(t, 5, dlLessonNo(), "DFV_DL_LESSON unset")
}
