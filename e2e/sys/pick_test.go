// Adapted from jplayer-mobile/e2e/sys/pick_test.go at 49de3c2
// (TestPickImageCancel; the file pickers' tests left out). iPhone and
// Android: tvOS has no photo picker (pick_tvos_test.go).

//go:build (ios && !tvos) || (android && cgo)

package sys

import (
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/sys"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestPickImageCancel opens the photo picker, checks that a second one
// waits for it (core.ErrDuplicate), dismisses it as the user's Cancel
// does, and checks that done gets ErrCanceled (context.Canceled) and
// nothing.
func TestPickImageCancel(t *testing.T) { //nolint:paralleltest // one app a process
	ch := make(chan picture, 1)
	var first, second error
	onUI(t, func() {
		first = sys.PickImage(canvas(), dispatch, func(b []byte, mime string, err error) { ch <- picture{b, mime, err} })
		if first == nil {
			second = sys.PickImage(canvas(), dispatch, func([]byte, string, error) { t.Error("second picker answered") })
		}
	})
	require.NoError(t, first)
	require.ErrorIs(t, second, core.ErrDuplicate)
	expectShown(t, shownImage)
	cancelShown(t, shownImage)
	got := await(t, ch, within, "the photo picker's answer")
	require.ErrorIs(t, got.err, sys.ErrCanceled)
	assert.Empty(t, got.data)
	assert.Empty(t, got.mime)
	expectGone(t)
}
