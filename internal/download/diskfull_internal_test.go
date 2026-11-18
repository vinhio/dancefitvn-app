package download

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDiskFullIsNoSpace checks that a write that finds the disk full or over its
// quota is ErrNoSpace (the toast "Không đủ bộ nhớ", Q13), and any other write
// failure is not.
func TestDiskFullIsNoSpace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		full bool
	}{
		{"no space left", &fs.PathError{Op: "write", Path: "x.part", Err: syscall.ENOSPC}, true},
		{"quota exceeded", &os.PathError{Op: "write", Path: "x.part", Err: syscall.EDQUOT}, true},
		{"wrapped", fmt.Errorf("copy: %w", syscall.ENOSPC), true},
		{"read-only volume", &fs.PathError{Op: "write", Path: "x.part", Err: syscall.EROFS}, false},
		{"i/o error", &fs.PathError{Op: "write", Path: "x.part", Err: syscall.EIO}, false},
		{"plain", errors.New("boom"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := diskFull(tc.err)
			if !tc.full {
				assert.NoError(t, got)

				return
			}
			assert.ErrorIs(t, got, ErrNoSpace)
			assert.ErrorIs(t, got, tc.err)
		})
	}
}
