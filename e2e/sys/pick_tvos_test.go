//go:build ios && tvos

package sys

import (
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/sys"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestPickImageNoneOnTV checks that the Apple TV has no photo picker:
// core.ErrUnsupported at once, done never called.
func TestPickImageNoneOnTV(t *testing.T) { //nolint:paralleltest // one app a process
	var err error
	onUI(t, func() {
		err = sys.PickImage(canvas(), dispatch, func([]byte, string, error) { t.Error("done ran") })
	})
	require.ErrorIs(t, err, core.ErrUnsupported)
}
