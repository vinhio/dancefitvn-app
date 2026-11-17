// Adapted from jplayer-tv/internal/app/framelog_test.go at 781bee5: the
// lines, the ring, close and the allocations, on a view of its own.

package app

import (
	"bytes"
	"io"
	"runtime/metrics"
	"strconv"
	"strings"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestFrameLogLines: the header, a line per frame the view drew and per
// press, and a mem line, once a second by the view's clock.
func TestFrameLogLines(t *testing.T) {
	t.Parallel()

	t.Run("frames and keys", func(t *testing.T) {
		t.Parallel()

		e := newFrameLogRig(t, 0)
		e.tt.Key(keys.KeyRight, 0)
		e.tt.Elapse(300 * time.Millisecond)
		e.tt.KeyRepeat(keys.KeyRight, 0)
		e.tt.Repaint()
		e.tt.Elapse(time.Second - 300*time.Millisecond) // the first flush
		lines := e.lines()
		require.NotEmpty(t, lines)
		assert.Regexp(t, `^# dfv-framelog scale=1 canvas=1920x1080 start=\S+$`, lines[0])

		var frames, presses, repeats, mems int
		prevAt, prevN := int64(0), 0
		for _, line := range lines[1:] {
			f := strings.Fields(line)
			switch f[0] {
			case "fs":
				require.Len(t, f, 10, line)
				at, n := atoi64(t, f[1]), int(atoi64(t, f[2]))
				assert.GreaterOrEqual(t, at, prevAt, "in time order")
				assert.Greater(t, n, prevN, "each frame once")
				prevAt, prevN = at, n
				for _, us := range f[3:7] {
					assert.GreaterOrEqual(t, atoi64(t, us), int64(0))
				}
				assert.Contains(t, []string{"true", "false"}, f[7])
				assert.Contains(t, []string{"true", "false"}, f[8])
				atoi64(t, f[9])
				frames++
			case "key":
				require.Len(t, f, 4, line)
				assert.Equal(t, "Right", f[1])
				at := atoi64(t, strings.TrimPrefix(f[3], "t="))
				switch f[2] {
				case "down":
					presses++
					assert.Equal(t, frameLogBase.UnixMilli(), at, "the press at the start, by the fake clock")
				case "repeat":
					repeats++
					assert.Equal(t, frameLogBase.Add(300*time.Millisecond).UnixMilli(), at)
				default:
					t.Fatalf("bad key line %q", line)
				}
			case "mem":
				require.Len(t, f, 6, line)
				for _, kv := range f[1:] {
					_, v, ok := strings.Cut(kv, "=")
					require.True(t, ok, line)
					atoi64(t, v)
				}
				assert.NotEqual(t, "heap_live=0", f[3], "the live heap is known")
				mems++
			default:
				t.Fatalf("bad line %q", line)
			}
		}
		assert.Equal(t, 1, presses)
		assert.Equal(t, 1, repeats)
		assert.Equal(t, 1, mems, "a mem line a flush")
		assert.GreaterOrEqual(t, frames, 1, "the repaint")

		// Nothing new: the next second writes the mem line alone; a repaint, one more.
		n := len(lines)
		e.tt.Elapse(time.Second)
		require.Len(t, e.lines(), n+1)
		assert.True(t, strings.HasPrefix(e.lines()[n], "mem "))
		e.tt.Repaint()
		e.tt.Elapse(time.Second)
		require.Len(t, e.lines(), n+3)
		assert.True(t, strings.HasPrefix(e.lines()[n+1], "fs "))
	})

	t.Run("ring wrap", func(t *testing.T) {
		t.Parallel()

		e := newFrameLogRig(t, 4)
		e.fl.flush() // the header, and the frames before
		before := len(e.lines())
		for i := range 6 {
			e.fl.onKey(&ui.KeyEvent{Key: keys.Key(int(keys.KeyA) + i), Down: true})
		}
		e.fl.onKey(&ui.KeyEvent{Key: keys.KeyA}) // a release: not recorded
		e.fl.flush()
		got := e.lines()[before:]
		at := strconv.FormatInt(frameLogBase.UnixMilli(), 10)
		require.Len(t, got, 6)
		assert.Equal(t, []string{
			"# dropped 2 entries",
			"key C down t=" + at,
			"key D down t=" + at,
			"key E down t=" + at,
			"key F down t=" + at,
		}, got[:5])
		assert.True(t, strings.HasPrefix(got[5], "mem t="+at+" "))
	})

	t.Run("close", func(t *testing.T) {
		t.Parallel()

		e := newFrameLogRig(t, 0)
		e.tt.Repaint()
		e.fl.close()
		lines := e.lines()
		require.Len(t, lines, 3, "the header, the repaint and the mem line, written on close")
		e.tt.Key(keys.KeyRight, 0) // the hook is off
		e.tt.Repaint()
		e.tt.Elapse(2 * time.Second)
		assert.Len(t, e.lines(), 3, "nothing after close")
		e.fl.close()
	})

	t.Run("nil", func(t *testing.T) {
		t.Parallel()

		var l *frameLog
		l.attach(nil)
		l.flush()
		l.close()
	})
}

// TestMemMetricNamesExist: the runtime knows every metric of the mem line.
func TestMemMetricNamesExist(t *testing.T) {
	t.Parallel()
	known := map[string]bool{}
	for _, d := range metrics.All() {
		known[d.Name] = true
	}
	for _, name := range memMetrics {
		assert.True(t, known[name], name)
	}
}

// TestFrameLogDoesNotAllocate: a second of frames, a press and the flush
// allocate nothing.
func TestFrameLogDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun counts the whole process's allocations
	e := newFrameLogRig(t, 0)
	e.fl.out = io.Discard
	e.fl.flush()
	ev := &ui.KeyEvent{Key: keys.KeyRight, Down: true}
	allocs := testing.AllocsPerRun(200, func() {
		for range 60 {
			e.fl.last = -1 // a new frame for onFrame
			e.fl.onFrame()
		}
		e.fl.onKey(ev)
		e.fl.flush()
	})
	if kittest.Race {
		t.Logf("allocs per second of frames under -race (not asserted): %v", allocs)

		return
	}
	assert.Zero(t, allocs, "frames, a press and the second's flush")
}

func BenchmarkFrameLogFrame(b *testing.B) {
	e := newFrameLogRig(b, 0)
	e.fl.out = io.Discard
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		e.fl.last = -1
		e.fl.onFrame()
		if i++; i%60 == 0 {
			e.fl.flush()
		}
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// frameLogBase is the fake clock's time at the Tester's start.
var frameLogBase = time.Date(2026, 11, 14, 9, 41, 0, 0, time.UTC)

// frameLogRig is a 1920×1080 view in a Tester with a frame log of its own,
// on the Tester's clock.
type frameLogRig struct {
	tt  *ui.Tester
	fl  *frameLog
	out *bytes.Buffer
}

// newFrameLogRig builds the view and its frame log, ring size (0: the
// default), writing to a buffer.
func newFrameLogRig(tb testing.TB, size int) *frameLogRig {
	tb.Helper()
	e := &frameLogRig{out: &bytes.Buffer{}}
	e.fl = newFrameLog(e.out, func() time.Time { return frameLogBase.Add(e.tt.Now()) }, size)
	tt, err := ui.NewTester(&ui.Text{Value: "frames"}, 1920, 1080, ui.WithFrameStats(), ui.AfterFrame(e.fl.frameFn))
	require.NoError(tb, err)
	e.tt = tt
	e.fl.attach(tt.View())
	tb.Cleanup(func() {
		e.fl.close()
		tt.Close()
	})

	return e
}

// lines returns what out received, a line each.
func (e *frameLogRig) lines() []string {
	s := strings.TrimSuffix(e.out.String(), "\n")
	if s == "" {
		return nil
	}

	return strings.Split(s, "\n")
}

// atoi64 reads a decimal number or fails.
func atoi64(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	require.NoError(t, err, s)

	return n
}
