//go:build live

package app

import (
	"context"
	"io/fs"
	"time"
)

// liveBuild is true in the live build (-tags live): the root package
// embeds no mock (mockfs_live.go), the app does not link the mock's
// package, and it talks to the live API (apimode.go).
const liveBuild = true

// newInProcMock fails: a live build has no mock in the process.
func newInProcMock(fs.FS, func(ctx context.Context, d time.Duration) error,
	func(format string, args ...any), []string,
) (inProcMock, error) {
	return nil, errNoMock
}
