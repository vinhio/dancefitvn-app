// Adapted from jplayer-mobile/internal/sys/keepawake_test.go at 49de3c2.

package sys

import (
	"runtime"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	platfake "github.com/gdeskdev/desk/foundation/platform/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKeepAwake checks the errors where tests run and that the value is
// remembered for ReapplyKeepAwake: no driver is invalid and remembers
// nothing; off a phone the screen cannot be kept on, but the value is
// kept. The cases run in order: they share the remembered value.
func TestKeepAwake(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		t.Skip("the flags are read back on the simulator and the emulator")
	}
	d := platfake.New()
	tests := []struct {
		name   string
		on     bool
		noDrv  bool
		want   error
		after  bool
		reWant error
	}{
		{"on", true, false, core.ErrUnsupported, true, core.ErrUnsupported},
		{"no driver keeps the last value", false, true, core.ErrInvalid, true, core.ErrInvalid},
		{"off", false, false, core.ErrUnsupported, false, core.ErrUnsupported},
		{"on again", true, false, core.ErrUnsupported, true, core.ErrUnsupported},
		{"off again", false, false, core.ErrUnsupported, false, core.ErrUnsupported},
	}
	for _, tc := range tests {
		var err, rerr error
		if tc.noDrv {
			err, rerr = KeepAwake(nil, tc.on), ReapplyKeepAwake(nil)
		} else {
			err, rerr = KeepAwake(d, tc.on), ReapplyKeepAwake(d)
		}
		require.ErrorIs(t, err, tc.want, tc.name)
		assert.Equal(t, tc.after, KeepingAwake(), tc.name)
		require.ErrorIs(t, rerr, tc.reWant, tc.name)
		assert.Equal(t, tc.after, KeepingAwake(), tc.name+": reapplied")
	}
}
