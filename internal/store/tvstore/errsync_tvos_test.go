//go:build ios && tvos

package tvstore

import (
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestErrSyncWrapsUnsupported checks that a synchronize answering NO is
// reported with a sentinel of core, which callers test with errors.Is.
func TestErrSyncWrapsUnsupported(t *testing.T) {
	t.Parallel()

	assert.ErrorIs(t, errSync, core.ErrUnsupported)
	assert.Equal(t, "tvstore: synchronize returned false: gdesk: unsupported operation", errSync.Error())
}
