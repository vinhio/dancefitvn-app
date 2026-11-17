package store_test

import (
	"bytes"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

// TestMain fails the run when a test leaves a goroutine of the store behind.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && leaked() {
		os.Stderr.WriteString("goroutine leak in package store\n")
		code = 1
	}
	os.Exit(code)
}

// leaked reports a goroutine started by the package that does not end.
func leaked() bool {
	for range 500 {
		var buf bytes.Buffer
		_ = pprof.Lookup("goroutine").WriteTo(&buf, 2)
		if !strings.Contains(buf.String(), "dancefitvn-app/internal/store.") {
			return false
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}

	return true
}
