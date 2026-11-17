package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProbedAwakeLogsTheSystemValue: each call reaches the sink, then the
// system's value is logged; a failure of the sink or of the read is told.
func TestProbedAwakeLogsTheSystemValue(t *testing.T) {
	t.Parallel()
	errSink := errors.New("no window")
	tests := []struct {
		name    string
		fail    error
		read    func() (bool, error)
		call    func(p probedAwake) error
		want    string
		wantErr error
	}{
		{name: "set on held", read: func() (bool, error) { return true, nil },
			call: func(p probedAwake) error { return p.Set(true) }, want: "dfv: keep-awake set asked=true idleTimerDisabled=true"},
		{name: "set off free", read: func() (bool, error) { return false, nil },
			call: func(p probedAwake) error { return p.Set(false) }, want: "dfv: keep-awake set asked=false idleTimerDisabled=false"},
		{name: "read unsupported", read: func() (bool, error) { return false, core.ErrUnsupported },
			call: func(p probedAwake) error { return p.Set(true) }, want: "dfv: keep-awake set asked=true idleTimerDisabled=? (" + core.ErrUnsupported.Error() + ")"},
		{name: "sink fails", fail: errSink, read: func() (bool, error) { return false, nil },
			call: func(p probedAwake) error { return p.Set(true) }, want: "dfv: keep-awake set asked=true failed: no window", wantErr: errSink},
		{name: "reapply", read: func() (bool, error) { return false, nil },
			call: func(p probedAwake) error { return p.Reapply() }, want: "dfv: keep-awake reapply asked=false idleTimerDisabled=false"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sink := &probeSink{fail: tc.fail}
			var lines []string
			p := probedAwake{inner: sink, read: tc.read, logf: func(f string, a ...any) { lines = append(lines, fmt.Sprintf(f, a...)) }}
			err := tc.call(p)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, []string{tc.want}, lines)
		})
	}
}

// TestDevAwakeShipsTheSinkAsIs: the build that ships has no probe.
func TestDevAwakeShipsTheSinkAsIs(t *testing.T) {
	t.Parallel()
	sink := &probeSink{}
	got := devAwake(sink, func(string, ...any) {})
	if devBuild {
		assert.IsType(t, probedAwake{}, got)
	} else {
		assert.Same(t, sink, got)
	}
}

// probeSink is an awakeSink that answers fail.
type probeSink struct{ fail error }

func (s *probeSink) Set(bool) error { return s.fail }
func (s *probeSink) Reapply() error { return s.fail }
