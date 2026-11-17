package sys

import (
	"runtime"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestProcStartAgo reads the start of a process from /proc texts: field
// 22 after the command, which may hold spaces and parentheses.
func TestProcStartAgo(t *testing.T) {
	t.Parallel()

	// Fields 3 to 21 then the start time (field 22) and two more.
	stat := func(cmd, start string) string {
		return "4242 (" + cmd + ") S 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 " + start + " 99 100\n"
	}
	tests := map[string]struct {
		stat, uptime string
		hz           int
		want         time.Duration
		err          bool
	}{
		"plain":        {stat: stat("com.jivecode.dancefitvn", "100000"), uptime: "1002.50 3000.00\n", hz: 100, want: 2500 * time.Millisecond},
		"spaces":       {stat: stat("a (b) c", "50"), uptime: "1.00 0.00", hz: 100, want: 500 * time.Millisecond},
		"just started": {stat: stat("x", "100"), uptime: "1.00", hz: 100, want: 0},
		"no command":   {stat: "4242 x S 1", uptime: "1.00", hz: 100, err: true},
		"short":        {stat: "4242 (x) S 1 2 3", uptime: "1.00", hz: 100, err: true},
		"bad start":    {stat: stat("x", "abc"), uptime: "1.00", hz: 100, err: true},
		"empty uptime": {stat: stat("x", "1"), uptime: "", hz: 100, err: true},
		"bad uptime":   {stat: stat("x", "1"), uptime: "x", hz: 100, err: true},
		"future":       {stat: stat("x", "500"), uptime: "1.00", hz: 100, err: true},
		"no ticks":     {stat: stat("x", "1"), uptime: "1.00", hz: 0, err: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := procStartAgo(tc.stat, tc.uptime, tc.hz)
			if tc.err {
				require.ErrorIs(t, err, core.ErrInvalid)

				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tc.want.Seconds(), got.Seconds(), 1e-9)
		})
	}
}

// TestProcessStartOffPhone checks the desktop's answer: unsupported.
func TestProcessStartOffPhone(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		t.Skip("a phone reads its process's start")
	}

	_, err := ProcessStart()
	require.ErrorIs(t, err, core.ErrUnsupported)
}

// TestAccessibilityDumpOffPhone checks the desktop's answer: unsupported,
// with no canvas or one.
func TestAccessibilityDumpOffPhone(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "ios" {
		t.Skip("the iPhone and the Apple TV read their elements")
	}

	els, err := AccessibilityDump(nil)
	require.ErrorIs(t, err, core.ErrUnsupported)
	assert.Empty(t, els)
}
