//go:build darwin || linux

package sys

import (
	"fmt"
	"syscall"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// statfsSpace returns the bytes free to an unprivileged process
// (f_bavail blocks) and the size (f_blocks) of the file system holding
// path, as statfs(2) tells them. darwin covers iOS (GOOS=ios) and linux
// Android: the Mac's tests read it too.
func statfsSpace(path string) (free, total int64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, fmt.Errorf("statfs: %w", err)
	}
	bsize := int64(st.Bsize)         //nolint:gosec,unconvert // G115: a block size; uint32 on darwin, int64 on linux
	free = int64(st.Bavail) * bsize  //nolint:gosec // G115: the block counts of a real volume fit
	total = int64(st.Blocks) * bsize //nolint:gosec // G115: as free
	if bsize <= 0 || total <= 0 {
		return 0, 0, fmt.Errorf("statfs: block size %d, %d blocks: %w", bsize, st.Blocks, core.ErrInvalid)
	}

	return free, total, nil
}
