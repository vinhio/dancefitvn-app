package api

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// scriptTransport is a Transport driven by a function; it records the
// requests it saw.
type scriptTransport struct {
	mu    sync.Mutex
	calls []*Request
	fn    func(n int, r *Request) (*Response, error)
}

func (s *scriptTransport) Do(ctx context.Context, req *Request) (*Response, error) {
	s.mu.Lock()
	cp := *req
	cp.Header = req.Header.Clone()
	s.calls = append(s.calls, &cp)
	n := len(s.calls)
	s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return s.fn(n, req)
}

func (s *scriptTransport) seen() []*Request {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]*Request(nil), s.calls...)
}

// always answers every request with resp.
func always(status int, body string, hdr ...string) *scriptTransport {
	return &scriptTransport{fn: func(int, *Request) (*Response, error) {
		h := http.Header{}
		for i := 0; i+1 < len(hdr); i += 2 {
			h.Set(hdr[i], hdr[i+1])
		}
		var b []byte
		if body != "" {
			b = []byte(body)
		}

		return &Response{Status: status, Header: h, Body: b}, nil
	}}
}

// tokenSource is a TokenSource for tests.
type tokenSource struct {
	mu        sync.Mutex
	token     string
	next      string
	err       error
	refreshes int
	gate      chan struct{} // when set, Refresh waits for it to close
}

func (t *tokenSource) Token() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.token
}

func (t *tokenSource) Refresh(ctx context.Context, failed string) (string, error) {
	t.mu.Lock()
	t.refreshes++
	gate := t.gate
	t.mu.Unlock()

	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err != nil {
		return "", t.err
	}
	t.token = t.next

	return t.token, nil
}

func (t *tokenSource) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.refreshes
}

// newTestClient returns a client over tr.
func newTestClient(t *testing.T, tr Transport, mod ...func(*Options)) *Client {
	t.Helper()

	o := Options{Transport: tr, Version: "1.2.3", Platform: "ios", InstallID: "11111111-2222-4333-8444-555555555555"}
	for _, m := range mod {
		m(&o)
	}
	c, err := New(o)
	if err != nil {
		t.Fatal(err)
	}

	return c
}

// mockFile reads a file of the mock API.
func mockFile(t testing.TB, name string) []byte {
	t.Helper()

	b, err := os.ReadFile("../../mock/dancefitvn-mock-api/" + name)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// keyPaths lists the key paths of a JSON document ("a.b", "a[].c").
func keyPaths(t testing.TB, doc []byte) map[string]bool {
	t.Helper()

	var v any
	if err := json.Unmarshal(doc, &v); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	walkKeys(v, "", out)

	return out
}

func walkKeys(v any, prefix string, out map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			p := strings.TrimPrefix(prefix+"."+k, ".")
			out[p] = true
			walkKeys(e, p, out)
		}
	case []any:
		for _, e := range x {
			walkKeys(e, prefix+"[]", out)
		}
	}
}

// TestMain fails the run when a test leaves a goroutine of this package
// behind.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 {
		deadline := time.Now().Add(2 * time.Second)
		for {
			http.DefaultTransport.(*http.Transport).CloseIdleConnections()
			if !leaked() {
				break
			}
			if time.Now().After(deadline) {
				buf := make([]byte, 1<<16)
				os.Stderr.Write(buf[:runtime.Stack(buf, true)])
				code = 1

				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	os.Exit(code)
}

func leaked() bool {
	buf := make([]byte, 1<<20)
	stacks := string(buf[:runtime.Stack(buf, true)])
	for _, g := range strings.Split(stacks, "\n\n") {
		if strings.Contains(g, "dancefitvn-app/internal/api.") && !strings.Contains(g, "testing.(*M)") &&
			!strings.Contains(g, "leaked(") {
			return true
		}
	}

	return false
}
