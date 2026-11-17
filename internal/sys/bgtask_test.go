//go:build !ios || tvos

package sys

import (
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestBeginBackgroundTaskOffPhone checks the errors off the iPhone: a task
// with no name is refused before the system is asked, and a system with no
// background task (the Mac, Android, tvOS) answers core.ErrUnsupported with
// no end to call.
func TestBeginBackgroundTaskOffPhone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		task    string
		expired func()
		want    error
	}{
		{"no name", "", nil, core.ErrInvalid},
		{"no expiry handler", "downloads", nil, core.ErrUnsupported},
		{"with an expiry handler", "downloads", func() {}, core.ErrUnsupported},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			end, err := BeginBackgroundTask(tc.task, tc.expired)
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, end)
		})
	}
}
