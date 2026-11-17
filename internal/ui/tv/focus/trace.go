//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/trace.go at 781bee5.

package focus

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// TraceEnv is the environment variable that turns the focus trace on in a
// Manager made by NewManager : any value but "", "0" and "off" writes
// the trace to stderr. On the tvOS Simulator it is passed to the app with
// SIMCTL_CHILD_DFV_FOCUSTRACE=1 xcrun simctl launch. It is a debugging aid
// and costs a nil check per key and per focus change while off.
const TraceEnv = "DFV_FOCUSTRACE"

// traceMaxCands is how many candidates a move's line lists: the nav bar and a
// full grid of the Courses screen have about twenty.
const traceMaxCands = 64

// tracer is a Manager's trace: where the lines go and the scratch the
// candidate list of a move uses.
type tracer struct {
	w     io.Writer
	t0    time.Time
	cands []ui.Widget
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetTrace sends the trace of the Manager to w, one line per key, per move's
// candidates and pick, per change of focus and per call of Tracef, each with
// the time in milliseconds since the trace began. nil turns it off.
func (m *Manager) SetTrace(w io.Writer) {
	if w == nil {
		m.trace = nil

		return
	}
	m.trace = &tracer{w: w, t0: time.Now()}
}

// Tracing tells whether the trace is on: callers check it before building
// the arguments of Tracef, so that an off trace costs nothing.
func (m *Manager) Tracing() bool { return m.trace != nil }

// Tracef writes a line of the trace, if it is on.
func (m *Manager) Tracef(format string, args ...any) {
	if m.trace == nil {
		return
	}
	m.trace.line(fmt.Sprintf(format, args...))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// envTrace is the trace TraceEnv asks for, nil when it is not set.
func envTrace() io.Writer {
	switch v := os.Getenv(TraceEnv); v {
	case "", "0", "off":
		return nil
	default:
		return os.Stderr
	}
}

// line writes one line with the time since the trace began.
func (t *tracer) line(s string) {
	ms := float64(time.Since(t.t0).Microseconds()) / 1000
	_, _ = io.WriteString(t.w, "focustrace "+strconv.FormatFloat(ms, 'f', 1, 64)+" "+s+"\n")
}

// traceKey logs a key event that reaches the Root.
func (m *Manager) traceKey(e *ui.KeyEvent) {
	m.trace.line(fmt.Sprintf("key %v down=%t repeat=%t focus=%s", e.Key, e.Down, e.Repeat, m.traceFocus()))
}

// traceMove logs what the engine will pick for the arrow b: the focused key,
// the candidates in tree order with their rectangles in the view, and the
// candidate the geometry picks. The nav item's OnMove or any handler of the
// focused widget may still take the press before the engine's own move; the
// "focus" line that follows says where the focus went.
func (m *Manager) traceMove(b Button) {
	t := m.trace
	t.cands = m.e.Candidates(t.cands[:0], nil)
	rects := make([]geom.Rect, len(t.cands))
	from := -1
	cur := m.e.Focused()
	var sb strings.Builder
	for i, w := range t.cands {
		rects[i] = w.Node().WindowFrame()
		if w == cur {
			from = i
		}
		if i < traceMaxCands {
			r := rects[i]
			fmt.Fprintf(&sb, " %s@%g,%g,%gx%g", widgetKey(w), r.X, r.Y, r.W, r.H)
		}
	}
	pick := "-"
	if i := ui.NearestInDirection(rects, from, focusDirection(b.Dir())); i >= 0 {
		pick = widgetKey(t.cands[i])
	}
	t.line(fmt.Sprintf("move %v from=%s n=%d pick=%s cands:%s", b.Dir(), widgetKey(cur), len(t.cands), pick, sb.String()))
	clear(t.cands)
	t.cands = t.cands[:0]
}

// traceFocus is the key of the focused widget, "-" when none.
func (m *Manager) traceFocus() string { return widgetKey(m.e.Focused()) }

// widgetKey names w for the trace: its focus key, else its name, else "?".
func widgetKey(w ui.Widget) string {
	switch x := w.(type) {
	case nil:
		return "-"
	case Item:
		if k := x.FocusKey(); k != "" {
			return k
		}
	}
	if n := w.Node().Name; n != "" {
		return n
	}

	return "?"
}
