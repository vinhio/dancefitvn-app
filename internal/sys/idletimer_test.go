//go:build !ios

package sys

import (
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIdleTimerDisabledUnsupported: without UIKit there is no idle timer to read.
func TestIdleTimerDisabledUnsupported(t *testing.T) {
	t.Parallel()
	on, err := IdleTimerDisabled()
	require.ErrorIs(t, err, core.ErrUnsupported)
	assert.False(t, on)
}
