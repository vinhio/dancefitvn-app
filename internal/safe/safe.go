package safe

import (
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// ErrPanic is a call that panicked and was stopped by a guard. It is a
// core.ErrInvalid too: what the call was given could not be handled.
var ErrPanic = fmt.Errorf("safe: panic: %w", core.ErrInvalid)

// logf is where a recovered panic is told, with its stack.
var logf atomic.Pointer[func(format string, args ...any)]

// expected holds the guard names whose panics a test provokes on purpose.
var (
	expectedMu sync.Mutex
	expected   = map[string]bool{}
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// SetLog sets where recovered panics are told; nil goes back to the log
// package's.
func SetLog(f func(format string, args ...any)) {
	if f == nil {
		logf.Store(nil)

		return
	}
	logf.Store(&f)
}

// Expect says that a test provokes panics under the guard called name, or
// at the gDesk entry point called name (core.Panic.Where, HandlePanic): in a
// test binary every other guard lets its panic through, so that a
// programming error is not hidden by the guard that exists for broken
// input. It is for tests only.
func Expect(name string) {
	expectedMu.Lock()
	expected[name] = true
	expectedMu.Unlock()
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Call runs fn and returns its error; a panic of fn becomes an error that
// wraps ErrPanic, logged with name and the stack. In a test binary a panic
// of a guard no test Expects goes on.
func Call(name string, fn func() error) (err error) {
	defer func() {
		if v := recover(); v != nil {
			err = recovered(name, v)
		}
	}()

	return fn()
}

// Value is Call for a function that returns a value too: the zero value
// when fn panics.
func Value[T any](name string, fn func() (T, error)) (v T, err error) {
	defer func() {
		if r := recover(); r != nil {
			var zero T
			v, err = zero, recovered(name, r)
		}
	}()

	return fn()
}

// Run is Call for a function that returns nothing, for a goroutine's body:
// a panic is logged and the goroutine ends.
func Run(name string, fn func()) {
	_ = Call(name, func() error {
		fn()

		return nil
	})
}

// HandlePanic handles a panic gDesk recovered where it runs the app's
// functions (queued UI work, ui.Go's work and then, image loads): it is
// logged with p.Where and the stack, as a guard's, and the app goes on. In
// a test binary it panics again with p unless a test Expects p.Where, so
// that a programming error still fails the test.
func HandlePanic(p *core.Panic) {
	if testing.Testing() && !isExpected(p.Where) {
		panic(p)
	}
	tell(fmt.Sprintf("dancefitvn: panic in %s: %v\n%s", p.Where, p.Value, p.Stack))
}

// Install makes HandlePanic gDesk's panic handler (core.SetPanicHandler),
// for the whole process. The app calls it once at startup.
func Install() { core.SetPanicHandler(HandlePanic) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// recovered tells a panic and returns it as an error; in a test binary it
// panics again unless the guard is expected.
func recovered(name string, v any) error {
	if testing.Testing() && !isExpected(name) {
		panic(v)
	}
	tell(fmt.Sprintf("dancefitvn: panic in %s: %v\n%s", name, v, debug.Stack()))

	return fmt.Errorf("%s: %v: %w", name, v, ErrPanic) //nolint:errorlint // the panic value is for the log
}

// isExpected tells whether a test Expects panics under the guard or gDesk
// entry point called name.
func isExpected(name string) bool {
	expectedMu.Lock()
	defer expectedMu.Unlock()

	return expected[name]
}

// tell logs text, a recovered panic with its stack, through SetLog's
// function or the log package.
func tell(text string) {
	if f := logf.Load(); f != nil {
		(*f)("%s", text)
	} else {
		log.Print(text)
	}
}
