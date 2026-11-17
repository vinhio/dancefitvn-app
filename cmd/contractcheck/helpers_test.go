package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api/mock"
)

// mockDir is the real contract, relative to this package.
const mockDir = "../../mock/dancefitvn-mock-api"

// testEmail and testPassword are made-up; no real account is used.
const (
	testEmail    = "tester@example.invalid"
	testPassword = "Unique-Test-Password-7731"
)

// TestMain fails the run when a connection or server goroutine outlives its
// test.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && leaked() {
		os.Stderr.WriteString("goroutine leak: http connection or test server still running\n")
		code = 1
	}
	os.Exit(code)
}

// leaked reports a goroutine of the HTTP stack that does not end, after
// giving the scheduler many chances to finish it.
func leaked() bool {
	for range 2000 {
		var buf bytes.Buffer
		_ = pprof.Lookup("goroutine").WriteTo(&buf, 2)
		s := buf.String()
		if !strings.Contains(s, "http.(*persistConn)") && !strings.Contains(s, "httptest.(*Server)") &&
			!strings.Contains(s, "mock.(*Server).listenLocked") {
			return false
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}

	return true
}

// exec runs the command with args and returns the exit code and the two
// streams.
func exec(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(context.Background(), args, &out, &errOut, func(string) string { return "" })

	return code, out.String(), errOut.String()
}

// serveMock serves the mock folder dir as the "server" and returns its base
// URL; wrap, when not nil, may answer a request itself and returns whether it
// did.
func serveMock(t *testing.T, dir string, wrap func(w http.ResponseWriter, r *http.Request) bool) string {
	t.Helper()
	mt, err := mock.New(os.DirFS(dir), mock.Options{
		Wait:            func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
		KeepAudioDigest: true,
	})
	require.NoError(t, err)
	h := mt.Handler()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wrap != nil && wrap(w, r) {
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(func() {
		srv.Close()
		_ = mt.Close()
	})

	return srv.URL + apiPath
}

// liveArgs are the flags of a run against base with the test account.
func liveArgs(base string, extra ...string) []string {
	return append([]string{"-mock", mockDir, "-base", base, "-email", testEmail, "-password", testPassword}, extra...)
}

// mutatedMock copies the JSON of the mock folder into a temporary folder,
// links its media, and lets edit change decoded files by name.
func mutatedMock(t *testing.T, edit map[string]func(any)) string {
	t.Helper()
	dst := t.TempDir()
	src, err := filepath.Abs(mockDir)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(filepath.Join(src, "media"), filepath.Join(dst, "media")))
	err = fs.WalkDir(os.DirFS(src), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p == "media" {
				return fs.SkipDir
			}

			return os.MkdirAll(filepath.Join(dst, p), 0o755)
		}
		data, err := os.ReadFile(filepath.Join(src, p))
		if err != nil {
			return err
		}
		if fn := edit[p]; fn != nil {
			var v any
			if err := json.Unmarshal(data, &v); err != nil {
				return err
			}
			fn(v)
			if data, err = json.Marshal(v); err != nil {
				return err
			}
		}

		return os.WriteFile(filepath.Join(dst, p), data, 0o644)
	})
	require.NoError(t, err)

	return dst
}

// recorder logs the method and path of every request.
type recorder struct {
	mu   sync.Mutex
	list []string
}

func (r *recorder) add(req *http.Request) {
	r.mu.Lock()
	r.list = append(r.list, req.Method+" "+strings.TrimPrefix(req.URL.Path, apiPath))
	r.mu.Unlock()
}

func (r *recorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.list...)
}
