package sys

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// ProcessStart returns when the system started the app's process: on
// iOS and tvOS the kernel's start time of the process (sysctl
// kern.proc.pid), on Android the start of the process forked from the
// zygote for the launch (/proc/self/stat against /proc/uptime, to the
// 10 ms of the kernel's clock ticks). It times the launch of a
// developer's build (the mark of the first Home frame, local/plans/P8.md
// P8.4a). Elsewhere it fails with core.ErrUnsupported.
func ProcessStart() (time.Time, error) {
	return processStart()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// procStartAgo returns how long ago the process of stat (the text of
// /proc/<pid>/stat) started, the system having been up for uptime (the
// text of /proc/uptime) with hz clock ticks a second (USER_HZ, 100 on
// Android): the start time is field 22 of stat, in ticks since the boot,
// read after the command's closing parenthesis (a command may hold
// spaces and parentheses).
func procStartAgo(stat, uptime string, hz int) (time.Duration, error) {
	const op = "process start"
	if hz <= 0 {
		return 0, fmt.Errorf("sys: %s: clock ticks %d: %w", op, hz, core.ErrInvalid)
	}
	i := strings.LastIndexByte(stat, ')')
	if i < 0 {
		return 0, fmt.Errorf("sys: %s: no command in stat: %w", op, core.ErrInvalid)
	}
	// Fields 3 onwards follow the command: field 22 is the 20th of them.
	f := strings.Fields(stat[i+1:])
	const startField = 22 - 3
	if len(f) <= startField {
		return 0, fmt.Errorf("sys: %s: %d fields after the command: %w", op, len(f), core.ErrInvalid)
	}
	ticks, err := strconv.ParseUint(f[startField], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("sys: %s: start time %q: %w", op, f[startField], core.ErrInvalid)
	}
	u := strings.Fields(uptime)
	if len(u) == 0 {
		return 0, fmt.Errorf("sys: %s: empty uptime: %w", op, core.ErrInvalid)
	}
	up, err := strconv.ParseFloat(u[0], 64)
	if err != nil || up < 0 {
		return 0, fmt.Errorf("sys: %s: uptime %q: %w", op, u[0], core.ErrInvalid)
	}
	upTicks := up * float64(hz)
	if float64(ticks) > upTicks+1 {
		return 0, fmt.Errorf("sys: %s: started after now (%d > %.0f ticks): %w", op, ticks, upTicks, core.ErrInvalid)
	}
	ago := max(0, upTicks-float64(ticks)) / float64(hz)

	return time.Duration(ago * float64(time.Second)), nil
}
