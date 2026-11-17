package progress_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/internal/model"
)

// TestMain fails the run when a test leaves a goroutine of progress behind.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && leaked() {
		os.Stderr.WriteString("goroutine leak in package progress\n")
		code = 1
	}
	os.Exit(code)
}

func leaked() bool {
	for range 500 {
		var buf bytes.Buffer
		_ = pprof.Lookup("goroutine").WriteTo(&buf, 2)
		if !strings.Contains(buf.String(), "internal/progress.") {
			return false
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}

	return true
}

// offlineErr is what api.Client returns when the network is down.
var offlineErr = &net.OpError{Op: "dial", Err: assertErr("refused")}

type assertErr string

func (e assertErr) Error() string { return string(e) }

// fakeAPI is a Sender and BatchSender driven by functions; it records calls.
type fakeAPI struct {
	mu      sync.Mutex
	saves   []saveCall
	batches [][]model.BatchItem
	saveFn  func(n int) (model.ProgressResult, error)
	batchFn func(n int, r model.BatchRequest) (model.BatchResult, error)
	gate    chan struct{} // when set, batch calls wait for it (after recording)
	entered chan struct{} // signalled when a batch call is in
}

type saveCall struct {
	lessonID string
	req      model.ProgressRequest
}

func (f *fakeAPI) SaveProgress(_ context.Context, id string, r model.ProgressRequest) (model.ProgressResult, error) {
	f.mu.Lock()
	f.saves = append(f.saves, saveCall{id, r})
	n := len(f.saves)
	fn := f.saveFn
	f.mu.Unlock()
	if fn != nil {
		return fn(n)
	}

	return model.ProgressResult{LessonID: id, PositionSec: r.PositionSec}, nil
}

func (f *fakeAPI) SendProgressBatch(ctx context.Context, r model.BatchRequest) (model.BatchResult, error) {
	f.mu.Lock()
	f.batches = append(f.batches, append([]model.BatchItem(nil), r.Items...))
	n := len(f.batches)
	fn, gate, entered := f.batchFn, f.gate, f.entered
	f.mu.Unlock()
	if entered != nil {
		entered <- struct{}{}
	}
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return model.BatchResult{}, ctx.Err()
		}
	}
	if fn != nil {
		return fn(n, r)
	}

	return model.BatchResult{Accepted: len(r.Items)}, nil
}

func (f *fakeAPI) saveCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.saves)
}

func (f *fakeAPI) batchSizes() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]int, len(f.batches))
	for i, b := range f.batches {
		out[i] = len(b)
	}

	return out
}

// ui is a Dispatch that queues functions; run executes them on the test
// goroutine, as the UI thread would.
type ui struct{ ch chan func() }

func newUI() *ui { return &ui{ch: make(chan func(), 64)} }

func (u *ui) dispatch(fn func()) { u.ch <- fn }

// drain runs n dispatched functions, waiting for each.
func (u *ui) drain(n int) {
	for range n {
		(<-u.ch)()
	}
}
