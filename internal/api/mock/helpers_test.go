package mock

import (
	"bytes"
	"context"
	"io/fs"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
)

// mockDir is the real contract, relative to this package.
const mockDir = "../../../mock/dancefitvn-mock-api"

// TestMain fails the run when a media server goroutine outlives its test.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && leaked() {
		os.Stderr.WriteString("goroutine leak: media server still running\n")
		code = 1
	}
	os.Exit(code)
}

// leaked reports a goroutine of the server that does not end, after giving
// the scheduler many chances to finish it.
func leaked() bool {
	for range 2000 {
		var buf bytes.Buffer
		_ = pprof.Lookup("goroutine").WriteTo(&buf, 2)
		if !strings.Contains(buf.String(), "mock.(*Server).listenLocked") {
			return false
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}

	return true
}

// waitRecorder is a Wait that returns at once and records the delays.
type waitRecorder struct {
	mu sync.Mutex
	d  []time.Duration
}

func (w *waitRecorder) wait(ctx context.Context, d time.Duration) error {
	w.mu.Lock()
	w.d = append(w.d, d)
	w.mu.Unlock()

	return ctx.Err()
}

func (w *waitRecorder) delays() []time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()

	return append([]time.Duration(nil), w.d...)
}

// newTransport returns a Transport over the real mock folder with no delay.
func newTransport(t *testing.T, mod ...func(*Options)) *Transport {
	t.Helper()
	opts := Options{Wait: (&waitRecorder{}).wait}
	for _, f := range mod {
		f(&opts)
	}
	tr, err := New(os.DirFS(mockDir), opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tr.Close()) })

	return tr
}

// bearer returns headers with an Authorization token.
func bearer() http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer test-token")

	return h
}

// get is a GET request with a token.
func get(path string) *api.Request {
	return &api.Request{Method: http.MethodGet, Path: path, Header: bearer()}
}

// noRedirectClient is an HTTP client that keeps no connection alive, so
// that no goroutine outlives a test.
func noKeepAliveClient() *http.Client {
	return &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 10 * time.Second}
}

// emptyFS is a folder with nothing in it.
type emptyFS struct{}

func (emptyFS) Open(string) (fs.File, error) { return nil, fs.ErrNotExist }
