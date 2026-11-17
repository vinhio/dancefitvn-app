// Adapted from jplayer-mobile/e2e/engine/harness_test.go at 49de3c2 (the harness), cmd/avspike/clock.go (viewClock) and internal/media/avf/classes_ios.go + stats_ios.go (the footprint through task_info).

//go:build ios && tvos

package tvengine

import (
	"context"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avftv"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/gdeskdev/desk/foundation/ffi/dl"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The mock's media the tests play (e2e/prepare-media.sh): lesson l_3503's
// HLS (480×854, 24 fps, 3 × 4 s) and the sample MP3.
const (
	hlsMaster = "hls/l_3503/master.m3u8"
	mp3Sample = "audio/_sample.mp3"
	hlsLength = 12.0
	mp3Length = 30.0
)

// The tests' timing and margins.
const (
	loadWithin    = 10 * time.Second
	playingWithin = 3 * time.Second
	rateSlack     = 0.03
	seekSlack     = 0.05
	eventBuffer   = 8192
)

// clockSlots is how many timers a viewClock tells apart for Stop: an
// engine keeps one or two at once.
const clockSlots = 64

// harness is an engine under test, on the app's UI thread, its plane
// placed by the window's pane.
type harness struct {
	eng    *avftv.Engine
	events chan media.Event
}

// viewClock is the engine's clock.Clock on the view's timers, so that its
// tick fires on the UI thread between frames, allocation-free (the app's
// uiClock, smaller). UI thread only.
type viewClock struct {
	view  *ui.View
	next  clock.Timer
	slots [clockSlots]clockSlot
}

// clockSlot ties a clock.Timer to the view's timer for it.
type clockSlot struct {
	id clock.Timer
	t  ui.Timer
}

// task is what the footprint needs of libSystem: task_info and the task's
// port.
var task struct {
	once  sync.Once
	err   error
	info  dl.Fn
	self  uint32
	mu    sync.Mutex
	words [taskVMInfoWords]uint64
	count uint32
}

// The task_vm_info numbers (mach/task_info.h).
const (
	taskVMInfo      = 22  // TASK_VM_INFO, mach/task_info.h:333
	physFootprintAt = 18  // phys_footprint, the 19th 64-bit word of task_vm_info (task_info.h:358)
	taskVMInfoWords = 128 // room for every revision of task_vm_info
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newHarness makes an engine on the view's clock, its plane under the
// window's canvas and placed by the pane, recording its events; it is
// closed when the test ends.
func newHarness(t *testing.T) *harness {
	t.Helper()

	h := &harness{events: make(chan media.Event, eventBuffer)}
	var err error
	onUI(t, func() {
		h.eng, err = avftv.New(avftv.Config{
			Clock:    &viewClock{view: tv.view},
			Post:     tv.app.Enqueue,
			Underlay: underlay,
			Log:      slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn})),
			OnInterruption: func(began, shouldResume bool) {
				fmt.Printf("tvengine: audio session interruption began=%v should_resume=%v\n", began, shouldResume)
			},
		})
		if err != nil {
			return
		}
		unsub := h.eng.Events().Subscribe(func(ev media.Event) {
			select {
			case h.events <- ev:
			default:
				fmt.Println("tvengine: event dropped")
			}
		})
		tv.root.pane.show(h.eng)
		t.Cleanup(func() {
			onUI(t, func() {
				unsub()
				tv.root.pane.show(nil)
				assert.NoError(t, h.eng.Close())
			})
		})
	})
	require.NoError(t, err)

	return h
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Now returns the wall time.
func (k *viewClock) Now() time.Time { return time.Now() }

// After calls fn on the UI thread once d has passed. It allocates nothing
// when fn is made once.
func (k *viewClock) After(d time.Duration, fn func()) clock.Timer {
	if fn == nil || k.view == nil {
		return 0
	}
	k.next++
	id := k.next
	k.slots[id%clockSlots] = clockSlot{id: id, t: k.view.Context().AfterFunc(d, fn)}

	return id
}

// Stop cancels the call of t and reports whether it was still pending.
func (k *viewClock) Stop(t clock.Timer) bool {
	if t == 0 {
		return false
	}
	s := &k.slots[t%clockSlots]
	if s.id != t {
		return false
	}
	ok := s.t.Stop()
	*s = clockSlot{}

	return ok
}

// load loads src on the UI thread and returns the token.
func (h *harness) load(t *testing.T, src media.Source, opts media.LoadOptions) uint64 {
	t.Helper()

	var (
		token uint64
		err   error
	)
	onUI(t, func() { token, err = h.eng.Load(context.Background(), src, opts) })
	require.NoError(t, err)

	return token
}

// await returns the first event pred accepts, skipping the others; it fails
// the test after within.
func (h *harness) await(t *testing.T, pred func(media.Event) bool, within time.Duration) media.Event {
	t.Helper()

	ev, ok := h.awaitOr(pred, within)
	require.True(t, ok, "no such event within %v", within)

	return ev
}

// awaitOr returns the first event pred accepts and true; false after
// within.
func (h *harness) awaitOr(pred func(media.Event) bool, within time.Duration) (media.Event, bool) {
	timer := time.NewTimer(within)
	defer timer.Stop()
	for {
		select {
		case ev := <-h.events:
			if pred(ev) {
				return ev, true
			}
		case <-timer.C:
			return media.Event{}, false
		}
	}
}

// expectNone fails the test when an event pred accepts comes within d.
func (h *harness) expectNone(t *testing.T, pred func(media.Event) bool, d time.Duration) {
	t.Helper()

	if ev, ok := h.awaitOr(pred, d); ok {
		assert.Fail(t, "unexpected event", "%+v", ev)
	}
}

// drain drops the events queued so far.
func (h *harness) drain() {
	for {
		select {
		case <-h.events:
		default:
			return
		}
	}
}

// readTime reads the position of the load token now (media.TimeReader).
func (h *harness) readTime(t *testing.T, token uint64) float64 {
	t.Helper()

	var (
		v  float64
		ok bool
	)
	onUI(t, func() { v, ok = h.eng.ReadTime(token) })
	require.True(t, ok, "the time of load %d", token)

	return v
}

// tickTime runs a tick now and returns the time it tells.
func (h *harness) tickTime(t *testing.T) float64 {
	t.Helper()

	h.drain()
	onUI(t, func() { avftv.Tick(h.eng) })

	return h.await(t, kind(media.EventTime), time.Second).Time
}

// state is a predicate for await: a state event of s.
func state(s media.PlayState) func(media.Event) bool {
	return func(ev media.Event) bool { return ev.Kind == media.EventState && ev.State == s }
}

// kind is a predicate for await: an event of k.
func kind(k media.EventKind) func(media.Event) bool {
	return func(ev media.Event) bool { return ev.Kind == k }
}

// mediaURL returns the link of a file of the media folder on the mock's
// server.
func mediaURL(name string) string { return tv.media.URL(name) }

// goHeap returns the Go heap in use after a collection, in bytes.
func goHeap() int64 {
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	return int64(ms.HeapAlloc) //nolint:gosec // G115: a heap size far below 2^63
}

// footprint returns the process's physical footprint in bytes, the figure
// Xcode's memory gauge shows (task_vm_info.phys_footprint), 0 when the
// kernel does not tell it.
func footprint() int64 {
	task.once.Do(func() {
		sys, err := dl.Open("/usr/lib/libSystem.B.dylib")
		if err == nil {
			err = sys.Bind(dl.Sym{Fn: &task.info, Name: "task_info"})
		}
		var self dl.Fn
		if err == nil {
			err = sys.Bind(dl.Sym{Fn: &self, Name: "mach_task_self_"})
		}
		if err == nil {
			// mach_task_self() is a macro reading the global
			// mach_task_self_ (mach/mach_init.h): dlsym gives its address.
			task.self = *(*uint32)(unsafe.Pointer(dl.Bytes(uintptr(self)))) //nolint:gosec // G103: a global of libSystem
		}
		task.err = err
	})
	if task.err != nil {
		return 0
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	task.count = taskVMInfoWords * 2
	if task.info.Call(uintptr(task.self), taskVMInfo, uintptr(unsafe.Pointer(&task.words[0])), uintptr(unsafe.Pointer(&task.count))) != 0 { //nolint:gosec // G103: package variables, written during the call
		return 0
	}
	if task.count < (physFootprintAt+1)*2 {
		return 0
	}

	return int64(task.words[physFootprintAt]) //nolint:gosec // G115: a byte count far below 2^63
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// mib is n bytes in MiB.
func mib(n int64) float64 { return float64(n) / (1 << 20) }
