// Adapted from jplayer-tv/internal/app/framelog.go at 781bee5 (and the
// memSampler of its framesum.go): trimmed to lines on the console, no
// file, no summary mode, no media commands; on the developer's build with
// DFV_FRAMELOG=1 (local/plans/P7.md P7.5c: the frames of focus moves, the
// memory of the R1 gate).

package app

import (
	"io"
	"runtime/metrics"
	"strconv"
	"time"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// frameLogSize is how many entries, frames and key presses, the frame log
// keeps between two flushes: 17 s at 60 frames a second, for a flush every
// second. The oldest unflushed entries give way when it is full, and the
// next flush says how many.
const frameLogSize = 1024

// frameLogEvery is how often, by the view's clock, the frame log writes the
// entries it keeps, and a mem line.
const frameLogEvery = time.Second

// frameLogBufSize is the initial capacity of the flush buffer: a frame line
// is about 60 bytes.
const frameLogBufSize = 64 << 10

// frameEntryKind tells a frame log entry's kind.
type frameEntryKind uint8

// Kinds of frame log entries.
const (
	frameEntryUnknown frameEntryKind = iota
	frameEntryFrame
	frameEntryKey
)

// frameEntry is one entry of the frame log: a frame's timing or a key
// press, at the clock's time.
type frameEntry struct {
	kind   frameEntryKind
	at     int64 // Unix milliseconds
	n      int   // the view's frame count (FrameStats.Frames)
	t      ui.FrameTiming
	key    keys.Key
	repeat bool
}

// memMetrics are the runtime/metrics a mem line reads, in its order:
// goroutines, live heap, memory mapped by the Go runtime, GC cycles.
var memMetrics = [...]string{
	"/sched/goroutines:goroutines",
	"/gc/heap/live:bytes",
	"/memory/classes/total:bytes",
	"/gc/cycles/total:gc-cycles",
}

// memSampler reads memMetrics into samples made once.
type memSampler struct {
	samples []metrics.Sample
}

// frameLog records the view's frames and key presses into a fixed ring,
// allocating nothing per frame or press, and writes them once a second by
// the view's clock, a line per entry and a mem line:
//
//	fs <unix ms> <frame n> <layout µs> <paint µs> <present µs> <total µs> <gpu> <unchanged> <pixels>
//	key <name> down|repeat t=<unix ms>
//	mem t=<unix ms> goroutines=<n> heap_live=<bytes> mapped=<bytes> gc=<n>
//
// after a header line starting with "# dfv-framelog". The view's frames are
// timed only with GDESK_FRAMESTATS=1 (ui.FrameStatsEnv): without it there
// are no fs lines. UI thread only.
type frameLog struct {
	now  func() time.Time
	out  io.Writer
	view *ui.View

	ring    []frameEntry
	written uint64 // entries recorded since the start
	flushed uint64 // entries written out
	dropped uint64 // entries overwritten before a flush, not yet told
	last    int    // the frame count of the last frame recorded

	buf    []byte
	header bool // the header line went out
	closed bool
	mem    memSampler

	frameFn func() // onFrame, made once
	tickFn  func() // tick, made once
	unhook  func() // the view's key hook
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newFrameLog returns a frame log writing to out on the clock now (time.Now
// when nil), with a ring of size entries (0: frameLogSize).
func newFrameLog(out io.Writer, now func() time.Time, size int) *frameLog {
	if size <= 0 {
		size = frameLogSize
	}
	if now == nil {
		now = time.Now
	}
	if out == nil {
		out = io.Discard
	}
	l := &frameLog{now: now, out: out, ring: make([]frameEntry, size), buf: make([]byte, 0, frameLogBufSize)}
	l.frameFn = l.onFrame
	l.tickFn = l.tick
	l.mem.samples = make([]metrics.Sample, len(memMetrics))
	for i, name := range memMetrics {
		l.mem.samples[i].Name = name
	}

	return l
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// attach starts recording v, whose AfterFrame is onFrame: its key presses
// through a hook that takes none, and a flush every second.
func (l *frameLog) attach(v *ui.View) {
	if l == nil || l.closed || l.view != nil || v == nil {
		return
	}
	l.view = v
	l.unhook = v.OnKey(l.onKey)
	v.Context().AfterFunc(frameLogEvery, l.tickFn)
}

// flush writes the entries recorded since the last flush, the header line
// first the first time, then a mem line. It allocates nothing once the
// buffer has grown to a second's lines.
func (l *frameLog) flush() {
	if l == nil || l.closed {
		return
	}
	b := l.buf[:0]
	if !l.header {
		l.header = true
		b = l.appendHeader(b)
	}
	if l.dropped > 0 {
		b = append(b, "# dropped "...)
		b = strconv.AppendUint(b, l.dropped, 10)
		b = append(b, " entries\n"...)
		l.dropped = 0
	}
	for ; l.flushed < l.written; l.flushed++ {
		b = appendFrameEntry(b, &l.ring[l.flushed%uint64(len(l.ring))])
	}
	b = l.appendMem(b, l.now().UnixMilli())
	l.buf = b
	_, _ = l.out.Write(b)
}

// close writes what is left and takes the key hook off. A second close
// does nothing.
func (l *frameLog) close() {
	if l == nil || l.closed {
		return
	}
	l.flush()
	l.closed = true
	if l.unhook != nil {
		l.unhook()
		l.unhook = nil
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onFrame is the view's AfterFrame: it records the frame just shown, once
// per timed frame.
func (l *frameLog) onFrame() {
	if l.view == nil || l.closed {
		return // the view's first frame, before attach
	}
	st := l.view.FrameStats()
	if st.Frames == l.last {
		return
	}
	l.last = st.Frames
	e := l.next()
	*e = frameEntry{kind: frameEntryFrame, at: l.now().UnixMilli(), n: st.Frames, t: st.Last}
}

// onKey is the view's key hook: it records the press and takes none.
func (l *frameLog) onKey(ev *ui.KeyEvent) bool {
	if l.closed || !ev.Down {
		return false
	}
	e := l.next()
	*e = frameEntry{kind: frameEntryKey, at: l.now().UnixMilli(), key: ev.Key, repeat: ev.Repeat}

	return false
}

// next returns the ring's next entry, giving the oldest unflushed one up
// when the ring is full.
func (l *frameLog) next() *frameEntry {
	size := uint64(len(l.ring))
	if l.written-l.flushed == size {
		l.flushed++
		l.dropped++
	}
	e := &l.ring[l.written%size]
	l.written++

	return e
}

// tick flushes and arranges the next one.
func (l *frameLog) tick() {
	if l.closed || l.view == nil {
		return
	}
	l.flush()
	l.view.Context().AfterFunc(frameLogEvery, l.tickFn)
}

// appendHeader appends the header line: the scale, the canvas, the start.
func (l *frameLog) appendHeader(b []byte) []byte {
	b = append(b, "# dfv-framelog scale="...)
	canvas := "?"
	if l.view != nil {
		b = strconv.AppendFloat(b, float64(l.view.Context().Scale()), 'g', -1, 32)
		if c := l.view.Canvas(); c != nil {
			s := c.Size()
			canvas = strconv.FormatFloat(float64(s.W), 'g', -1, 32) + "x" + strconv.FormatFloat(float64(s.H), 'g', -1, 32)
		}
	} else {
		b = append(b, '?')
	}
	b = append(b, " canvas="...)
	b = append(b, canvas...)
	b = append(b, " start="...)
	b = l.now().UTC().AppendFormat(b, time.RFC3339Nano)

	return append(b, '\n')
}

// appendMem samples the process and appends the mem line at now (Unix ms).
func (l *frameLog) appendMem(b []byte, now int64) []byte {
	metrics.Read(l.mem.samples)
	b = append(b, "mem t="...)
	b = strconv.AppendInt(b, now, 10)
	for i, name := range [...]string{" goroutines=", " heap_live=", " mapped=", " gc="} {
		b = append(b, name...)
		b = strconv.AppendUint(b, l.mem.value(i), 10)
	}

	return append(b, '\n')
}

// value returns the i-th metric of the last read, 0 when the runtime does
// not know it.
func (m *memSampler) value(i int) uint64 {
	if m.samples[i].Value.Kind() != metrics.KindUint64 {
		return 0
	}

	return m.samples[i].Value.Uint64()
}

// appendFrameEntry appends e's line.
func appendFrameEntry(b []byte, e *frameEntry) []byte {
	switch e.kind {
	case frameEntryFrame:
		b = append(b, "fs "...)
		b = strconv.AppendInt(b, e.at, 10)
		b = append(b, ' ')
		b = strconv.AppendInt(b, int64(e.n), 10)
		for _, d := range [...]time.Duration{e.t.Layout, e.t.Paint, e.t.Present, e.t.Total} {
			b = append(b, ' ')
			b = strconv.AppendInt(b, d.Microseconds(), 10)
		}
		b = append(b, ' ')
		b = strconv.AppendBool(b, e.t.GPU)
		b = append(b, ' ')
		b = strconv.AppendBool(b, e.t.Unchanged)
		b = append(b, ' ')
		b = strconv.AppendInt(b, int64(e.t.Pixels), 10)
	case frameEntryKey:
		b = append(b, "key "...)
		b = append(b, e.key.String()...)
		if e.repeat {
			b = append(b, " repeat t="...)
		} else {
			b = append(b, " down t="...)
		}
		b = strconv.AppendInt(b, e.at, 10)
	case frameEntryUnknown:
		return b
	default:
		return b
	}

	return append(b, '\n')
}
