// Adapted from jplayer-mobile/internal/sys/services_test.go at 49de3c2
// (noDispatch, stubCanvas, TestURLChecks, TestServicesOffPhone) and
// scheme_test.go (stubCanvas).

package sys

import (
	"runtime"
	"strings"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// stubCanvas is a canvas the checks accept; the tests never reach a call
// of it.
type stubCanvas struct{ platform.Canvas }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestOpenURLChecksArguments checks the errors every platform gives before
// the system is asked: no canvas, no dispatch, no URL, no scheme.
func TestOpenURLChecksArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		canvas   platform.Canvas
		dispatch bool
		raw      string
	}{
		{"no canvas", nil, true, "https://example.com"},
		{"no dispatch", stubCanvas{}, false, "https://example.com"},
		{"empty", stubCanvas{}, true, ""},
		{"no scheme", stubCanvas{}, true, "example.com/a"},
		{"path", stubCanvas{}, true, "/a/b"},
		{"bad escape", stubCanvas{}, true, "%zz"},
		{"no scheme name", stubCanvas{}, true, "://x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var d Dispatch
			if tc.dispatch {
				d = noDispatch(t)
			}
			err := OpenURL(tc.canvas, d, tc.raw, nil)
			require.ErrorIs(t, err, core.ErrInvalid)
			assert.True(t, strings.HasPrefix(err.Error(), "sys: open URL:"), err.Error())
		})
	}
}

// TestOpenURLOffPhone checks that a desktop has no browser to hand a URL
// to: core.ErrUnsupported, nothing dispatched.
func TestOpenURLOffPhone(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		t.Skip("opening a URL is checked on the simulator and the emulator")
	}
	err := OpenURL(stubCanvas{}, noDispatch(t), "https://example.com", func(error) { t.Error("done ran") })
	require.ErrorIs(t, err, core.ErrUnsupported)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// noDispatch is a dispatch the tests never expect to be called: the
// service fails before it would.
func noDispatch(t *testing.T) Dispatch {
	t.Helper()

	return func(func()) { t.Error("dispatched") }
}
