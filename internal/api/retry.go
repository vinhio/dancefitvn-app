package api

import (
	"context"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// retryDelays are the waits before the first and the second retry of a GET
// that failed on the network (05 §3).
var retryDelays = []time.Duration{500 * time.Millisecond, 1500 * time.Millisecond}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// waitTimer waits d or until ctx ends, and returns ctx's error then.
func waitTimer(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
