//go:build android

package sys

import (
	"fmt"
	"os"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// androidHZ is the kernel's USER_HZ on Android: the clock ticks of
// /proc/<pid>/stat.
const androidHZ = 100

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// processStart reads the start of the process from /proc (procStartAgo).
func processStart() (time.Time, error) {
	stat, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return time.Time{}, fmt.Errorf("sys: process start: %w", err)
	}
	uptime, err := os.ReadFile("/proc/uptime")
	now := time.Now()
	if err != nil {
		return time.Time{}, fmt.Errorf("sys: process start: %w", err)
	}
	ago, err := procStartAgo(string(stat), string(uptime), androidHZ)
	if err != nil {
		return time.Time{}, err
	}

	return now.Add(-ago), nil
}
