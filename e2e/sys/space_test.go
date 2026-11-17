//go:build ios || (android && cgo)

package sys

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jsys "dancefitvn-app/internal/sys"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestFreeSpace reads the free space of the audio folder (the data
// folder on tvOS, which downloads nothing) and checks it against statfs:
// free within the volume, and at least what statfs tells free less 1 %
// (iOS counts the purgeable space too; Android reads statfs itself, the
// disk changing a little in between).
func TestFreeSpace(t *testing.T) { //nolint:paralleltest // one app a process
	dir := spaceDir(t)
	free, total, err := jsys.FreeSpace(dir)
	require.NoError(t, err)
	var st syscall.Statfs_t
	require.NoError(t, syscall.Statfs(dir, &st))
	bsize := int64(st.Bsize)          //nolint:gosec,unconvert // G115: a block size; uint32 on darwin, int64 on linux
	avail := int64(st.Bavail) * bsize //nolint:gosec // G115: a real volume's blocks
	size := int64(st.Blocks) * bsize  //nolint:gosec // G115: as avail
	t.Logf("free %d of %d bytes; statfs %d of %d bytes (free - statfs = %d)",
		free, total, avail, size, free-avail)
	assert.Positive(t, free)
	assert.LessOrEqual(t, free, total)
	assert.GreaterOrEqual(t, free, avail-avail/100, "at least statfs's free space")
	assert.InEpsilon(t, float64(size), float64(total), 0.01, "the volume's size")

	_, _, err = jsys.FreeSpace(filepath.Join(dir, "missing", "folder"))
	t.Logf("a missing folder: %v", err)
}
