package app

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core"
)

// The helpers of the tests of both shells (wire_helpers_test.go,
// wire_tv_test.go): the UI thread of a test, the vault in memory, the log.

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mockDir is the mock API of the repository, from this package's folder.
const mockDir = "../../mock/dancefitvn-mock-api"

// driveLimit is how long, in real time, a test waits for the app to reach a
// state: the work off the UI thread (the vault, the mock) runs on its own
// goroutines, which the loop lets run between frames. It is a ceiling, not
// a wait: a test that passes never uses it. Under the race detector a frame
// of the real shell costs a hundred times more and the suite's tests share
// the CPUs, so the same states need minutes of the wall clock, not seconds.
var driveLimit = func() time.Duration {
	if kittest.Race {
		return 5 * time.Minute
	}

	return 30 * time.Second
}()

// uiQueue is the UI thread of a test: what the services enqueue waits
// until the test's goroutine, which drives the Tester, runs it.
type uiQueue struct {
	mu  sync.Mutex
	fns []func()
}

// memSecrets is an in-memory auth.SecretStore.
type memSecrets struct {
	mu   sync.Mutex
	vals map[string][]byte
}

// testLog collects the log lines of the services; it may be written after
// the test ended (a goroutine finishing late), unlike t.Logf.
type testLog struct {
	mu    sync.Mutex
	lines []string
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// enqueue keeps fn for drain.
func (q *uiQueue) enqueue(fn func()) bool {
	q.mu.Lock()
	q.fns = append(q.fns, fn)
	q.mu.Unlock()

	return true
}

// drain runs what was enqueued, in order, on the caller's goroutine.
func (q *uiQueue) drain() int {
	n := 0
	for {
		q.mu.Lock()
		fns := q.fns
		q.fns = nil
		q.mu.Unlock()
		if len(fns) == 0 {
			return n
		}
		for _, fn := range fns {
			fn()
			n++
		}
	}
}

// Get returns the value of account, core.ErrNotFound when none.
func (s *memSecrets) Get(account string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.vals[account]
	if !ok {
		return nil, fmt.Errorf("mem secrets: %s: %w", account, core.ErrNotFound)
	}

	return append([]byte(nil), v...), nil
}

// Set keeps value for account.
func (s *memSecrets) Set(account string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vals[account] = append([]byte(nil), value...)

	return nil
}

// Delete forgets account.
func (s *memSecrets) Delete(account string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.vals, account)

	return nil
}

// has tells whether account holds a value.
func (s *memSecrets) has(account string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.vals[account]

	return ok
}

// add is a Logf.
func (l *testLog) add(format string, args ...any) {
	l.mu.Lock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
	l.mu.Unlock()
}

// all returns the lines.
func (l *testLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.lines...)
}

// jsonUnmarshal is json.Unmarshal.
func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// must returns v, panicking on err (test helpers only).
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}
