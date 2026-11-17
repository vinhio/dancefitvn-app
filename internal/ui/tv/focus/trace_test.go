//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/trace_test.go at 781bee5.

package focus_test

import (
	"bytes"
	"testing"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/ui/tv/focus"
)

// TestTraceLogsKeysMovesAndFocus: with the trace on, an arrow press writes the
// key, the focused key, the candidates and the pick, and a focus change writes
// its two keys.
func TestTraceLogsKeysMovesAndFocus(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.focusOn("a1")
	var buf bytes.Buffer
	e.m.SetTrace(&buf)
	require.True(t, e.m.Tracing())

	e.tt.Key(keys.KeyRight, 0)
	e.m.Move(focus.DirRight)
	e.m.Tracef("custom %d", 7)

	out := buf.String()
	assert.Contains(t, out, "key Right down=true repeat=false focus=a1")
	assert.Contains(t, out, "move right from=a1 n=")
	assert.Contains(t, out, "cands: n1@")
	assert.Contains(t, out, "focus a1 -> a2")
	assert.Contains(t, out, "custom 7")
	assert.Contains(t, out, "focustrace ")
}

// TestTraceOffWritesNothing: SetTrace(nil) turns it off again.
func TestTraceOffWritesNothing(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.focusOn("a1")
	var buf bytes.Buffer
	e.m.SetTrace(&buf)
	e.m.SetTrace(nil)

	e.tt.Key(keys.KeyRight, 0)
	e.m.Tracef("custom")

	assert.False(t, e.m.Tracing())
	assert.Empty(t, buf.String())
}
