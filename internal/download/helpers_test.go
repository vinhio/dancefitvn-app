package download_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/model"
)

var t0 = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)

// TestMain fails the run when a test leaves a goroutine of download behind.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && leaked() {
		os.Stderr.WriteString("goroutine leak in package download\n")
		code = 1
	}
	os.Exit(code)
}

func leaked() bool {
	for range 500 {
		var buf bytes.Buffer
		_ = pprof.Lookup("goroutine").WriteTo(&buf, 2)
		if !strings.Contains(buf.String(), "internal/download.") {
			return false
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}

	return true
}

// content makes n deterministic bytes and returns them with their sha256.
func content(n int) ([]byte, string) {
	b := make([]byte, n)
	x := uint32(2463534242)
	for i := range b {
		x ^= x << 13
		x ^= x >> 17
		x ^= x << 5
		b[i] = byte(x)
	}
	sum := sha256.Sum256(b)

	return b, hex.EncodeToString(sum[:])
}

// srv is a media server with hooks. n counts requests from 1.
type srv struct {
	*httptest.Server
	body []byte

	mu     sync.Mutex
	ranges []string // Range header of each request ("" none)
	ifRng  []string

	cutAt       func(n int) int // bytes of body after which the connection drops; 0 none
	ignoreRange func(n int) bool
	status      func(n int, r *http.Request) int // non-zero: answer with it and no body
	gate        func(n int, r *http.Request)     // runs first
	wrongStart  func(n int) bool                 // 206 with a Content-Range that does not match

	active, maxActive atomic.Int32
}

func newSrv(t *testing.T, body []byte) *srv {
	t.Helper()
	s := &srv{body: body}
	s.Server = httptest.NewServer(s)
	t.Cleanup(s.Close)

	return s
}

func (s *srv) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.ranges...)
}

func (s *srv) ifRanges() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.ifRng...)
}

func (s *srv) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.ranges = append(s.ranges, r.Header.Get("Range"))
	s.ifRng = append(s.ifRng, r.Header.Get("If-Range"))
	n := len(s.ranges)
	s.mu.Unlock()
	cur := s.active.Add(1)
	defer s.active.Add(-1)
	for {
		old := s.maxActive.Load()
		if cur <= old || s.maxActive.CompareAndSwap(old, cur) {
			break
		}
	}
	if s.gate != nil {
		s.gate(n, r)
	}
	if s.status != nil {
		if st := s.status(n, r); st != 0 {
			w.WriteHeader(st)

			return
		}
	}
	if s.ignoreRange != nil && s.ignoreRange(n) {
		r.Header.Del("Range")
	}
	w.Header().Set("ETag", `"v1"`)
	if s.wrongStart != nil && s.wrongStart(n) && r.Header.Get("Range") != "" {
		w.Header().Set("Content-Range", "bytes 0-9/10")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(s.body[:10])

		return
	}
	var out http.ResponseWriter = w
	if s.cutAt != nil {
		if c := s.cutAt(n); c > 0 {
			out = &cutter{ResponseWriter: w, left: c}
		}
	}
	http.ServeContent(out, r, "x.mp3", t0, bytes.NewReader(s.body))
}

// cutter drops the connection after left bytes of body.
type cutter struct {
	http.ResponseWriter
	left int
}

func (c *cutter) Write(p []byte) (int, error) {
	if len(p) >= c.left {
		_, _ = c.ResponseWriter.Write(p[:c.left])
		c.ResponseWriter.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	}
	c.left -= len(p)

	return c.ResponseWriter.Write(p)
}

// source is a download.Source that answers from a function.
type source struct {
	mu    sync.Mutex
	calls int
	fn    func(n int, id string) (model.Audio, error)
}

func (s *source) Audio(_ context.Context, id string) (model.Audio, error) {
	s.mu.Lock()
	s.calls++
	n := s.calls
	s.mu.Unlock()

	return s.fn(n, id)
}

func (s *source) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.calls
}

// events collects the events of a Manager.
type events struct {
	ch  chan download.Event
	mu  sync.Mutex
	all []download.Event
}

func newEvents() *events { return &events{ch: make(chan download.Event, 1024)} }

func (e *events) on(ev download.Event) {
	e.mu.Lock()
	e.all = append(e.all, ev)
	e.mu.Unlock()
	e.ch <- ev
}

// wait returns the next event of kind (and lesson, when not ""), skipping others.
func (e *events) wait(t *testing.T, kind download.EventKind, lesson string) download.Event {
	t.Helper()
	for ev := range e.ch {
		if ev.Kind == kind && (lesson == "" || ev.LessonID == lesson) {
			return ev
		}
	}
	t.Fatal("events closed")

	return download.Event{}
}

func (e *events) snapshot() []download.Event {
	e.mu.Lock()
	defer e.mu.Unlock()

	return append([]download.Event(nil), e.all...)
}

// rig is a Manager over a media server and a fake clock.
type rig struct {
	mgr   *download.Manager
	srv   *srv
	src   *source
	ev    *events
	fake  *clock.Fake
	dir   string
	body  []byte
	sum   string
	audio func(n int, id string) model.Audio
}

type rigOpt func(*download.Options)

func newRig(t *testing.T, size int, mod ...rigOpt) *rig {
	t.Helper()
	body, sum := content(size)
	r := &rig{srv: newSrv(t, body), ev: newEvents(), fake: clock.NewFake(t0), dir: t.TempDir(), body: body, sum: sum}
	r.audio = func(_ int, id string) model.Audio {
		return model.Audio{LessonID: id, URL: r.srv.URL + "/" + id + ".mp3", SizeBytes: int64(len(body)), SHA256: sum, ExpiresAt: t0.Add(time.Hour)}
	}
	r.src = &source{fn: func(n int, id string) (model.Audio, error) { return r.audio(n, id), nil }}
	r.mgr = r.open(t, r.dir, "u1", mod...)

	return r
}

// open makes a Manager on dir for user and closes it with the test.
func (r *rig) open(t *testing.T, dir, user string, mod ...rigOpt) *download.Manager {
	t.Helper()
	opts := download.Options{
		Client: r.src, Dir: dir, Clock: r.fake, UserID: user, OnEvent: r.ev.on,
		HTTP: &http.Client{Transport: &http.Transport{DisableKeepAlives: true}},
	}
	for _, f := range mod {
		f(&opts)
	}
	m, err := download.NewManager(opts)
	require.NoError(t, err)
	t.Cleanup(m.Close)

	return m
}

func lesson(id string) model.Lesson { return model.Lesson{ID: id, HasAudio: true} }

func readAll(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	b, err := io.ReadAll(f)
	require.NoError(t, err)

	return b
}

// atomicBool is a bool for tests that flip it from the test goroutine.
type atomicBool struct{ v atomic.Bool }

func (a *atomicBool) set(b bool) { a.v.Store(b) }
func (a *atomicBool) get() bool  { return a.v.Load() }

// newBlockingServer sends first bytes of body, tells started, and then
// holds the response until hold closes or the client goes away.
func newBlockingServer(t *testing.T, body []byte, first int, hold <-chan struct{}, started chan<- struct{}) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body[:first])
		w.(http.Flusher).Flush()
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-hold:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(s.Close)

	return s
}

// slowTransport serves a body of 100 000 bytes in reads of 10 000, each
// read waiting for the test: it tells want, then waits for grant.
type slowTransport struct{ want, grant chan struct{} }

func (s *slowTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	body, _ := content(100_000)

	return &http.Response{
		StatusCode: http.StatusOK, Status: "200 OK", Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		Header: http.Header{}, ContentLength: int64(len(body)), Request: r,
		Body: &slowBody{r: bytes.NewReader(body), s: s},
	}, nil
}

type slowBody struct {
	r *bytes.Reader
	s *slowTransport
}

func (b *slowBody) Read(p []byte) (int, error) {
	b.s.want <- struct{}{}
	<-b.s.grant

	return b.r.Read(p[:min(len(p), 10_000)])
}

func (b *slowBody) Close() error { return nil }

func course(id string) download.CourseInfo { return download.CourseInfo{ID: id} }

var sprintf = fmt.Sprintf
