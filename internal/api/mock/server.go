// Adapted from jplayer-mobile/internal/yt/server.go at 49de3c2 (Ensure, listenLocked: tcp4 on 127.0.0.1, same port again after the system closed the socket) and from cmd/avspike/server.go (the file server: Content-Type, Range through http.ServeContent).

package mock

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// loopback is the only address the server listens on.
const loopback = "127.0.0.1"

// probeTimeout bounds the dial with which Ensure asks the socket.
const probeTimeout = 100 * time.Millisecond

// probeAttempts is how many times Ensure dials before it believes the socket
// is dead while its serve goroutine still runs; probeGap separates them.
const (
	probeAttempts = 3
	probeGap      = 10 * time.Millisecond
)

// drainTimeout bounds how long a replaced server waits for its requests.
const drainTimeout = time.Second

// contentTypes are the types the server tells. Go's MIME table on iOS and
// tvOS knows none of the first three, and AVPlayer needs them.
var contentTypes = map[string]string{
	".m3u8": "application/vnd.apple.mpegurl",
	".ts":   "video/mp2t",
	".mp3":  "audio/mpeg",
	".mp4":  "video/mp4",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
}

// Server serves the media folder on 127.0.0.1 with Range, for AVPlayer and
// MediaPlayer, which need an URL (docs/plan/04-media-engine.md §5). A file
// a lesson lacks is answered with the _sample of its kind. Safe for
// concurrent use.
type Server struct {
	files fs.FS
	logf  func(format string, args ...any)

	requests atomic.Int64
	inflight atomic.Int64
	probe    func(port int) bool // test seam; nil dials the port
	epoch    atomic.Uint64

	mu     sync.Mutex
	srv    *http.Server
	ln     net.Listener
	done   chan struct{}
	port   int
	dead   bool
	closed bool
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewServer returns a server of files, the media folder. It listens on the
// first Ensure.
func NewServer(files fs.FS, logf func(format string, args ...any)) *Server {
	if logf == nil {
		logf = func(string, ...any) {}
	}

	return &Server{files: files, logf: logf}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Ensure listens when the server never did, and listens again when the
// socket died (iOS closes it in the background), on the port of before when
// that is free, so that the URLs already given stay right. After Close it
// returns an error wrapping core.ErrClosed.
func (s *Server) Ensure() error {
	s.mu.Lock()
	srv, port, closed := s.srv, s.port, s.closed
	s.mu.Unlock()
	if closed {
		return errClosed
	}
	// The serve goroutine is the source of truth: when it returned, the
	// socket is dead (listenLocked sees dead). Only while it runs is the
	// socket asked, since Serve does not always report a socket the system
	// closed.
	alive := true
	if srv != nil && !s.serveReturned(srv) {
		alive = s.probeAlive(port)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errClosed
	}
	if srv != nil && !alive && s.srv == srv && s.inflight.Load() == 0 {
		// a single failed probe never kills a server that is answering.
		s.dead = true
	}

	return s.listenLocked()
}

// serveReturned reports whether the serve goroutine of srv has returned.
func (s *Server) serveReturned(srv *http.Server) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.srv != srv || s.done == nil {
		return false
	}
	select {
	case <-s.done:
		s.dead = true

		return true
	default:
		return false
	}
}

// probeAlive dials the port, probeAttempts times at most, and reports whether
// one dial got through.
func (s *Server) probeAlive(port int) bool {
	for i := range probeAttempts {
		if i > 0 {
			time.Sleep(probeGap)
		}
		if s.probe != nil {
			if s.probe(port) {
				return true
			}

			continue
		}
		c, err := net.DialTimeout("tcp4", net.JoinHostPort(loopback, strconv.Itoa(port)), probeTimeout)
		if err == nil {
			_ = c.Close()

			return true
		}
	}

	return false
}

// BaseURL returns "http://127.0.0.1:<port>", "" before the first Ensure.
func (s *Server) BaseURL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.port == 0 {
		return ""
	}

	return "http://" + net.JoinHostPort(loopback, strconv.Itoa(s.port))
}

// Port returns the port, 0 before the first Ensure.
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.port
}

// Epoch counts the ports the server has had: an URL made before the last
// change is wrong.
func (s *Server) Epoch() uint64 { return s.epoch.Load() }

// Requests returns how many requests the server began.
func (s *Server) Requests() int64 { return s.requests.Load() }

// Close stops listening and waits for the goroutine of the listener. The
// server does not listen again. Safe to call twice.
func (s *Server) Close() error {
	s.mu.Lock()
	srv, done := s.srv, s.done
	s.srv, s.ln, s.done, s.dead, s.closed = nil, nil, nil, false, true
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	err := srv.Close()
	<-done
	if errors.Is(err, net.ErrClosed) {
		return nil
	}

	return err
}

// Resolve returns the name of the file that answers name, a slash path
// below the media folder: itself, or the _sample of its kind when a lesson
// has no file (hls/<id>/<rest> to hls/_sample/<rest>, audio/<id>.mp3 to
// audio/_sample.mp3). ok is false when there is none.
func (s *Server) Resolve(name string) (string, bool) {
	if s.isFile(name) {
		return name, true
	}
	parts := strings.Split(name, "/")
	alt := ""
	switch {
	case len(parts) >= 3 && parts[0] == "hls":
		alt = "hls/_sample/" + strings.Join(parts[2:], "/")
	case len(parts) == 2 && parts[0] == "audio" && path.Ext(name) == ".mp3":
		alt = "audio/_sample.mp3"
	}
	if alt != "" && s.isFile(alt) {
		return alt, true
	}

	return "", false
}

// ServeHTTP answers GET and HEAD of a file with its type and Range. The
// query (exp, sig of the signed URLs) is ignored.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.requests.Add(1)
	s.inflight.Add(1)
	defer s.inflight.Add(-1)
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	file, ok := s.Resolve(name)
	if !ok {
		http.NotFound(w, r)

		return
	}
	rs, mod, closeFn, err := s.open(file)
	if err != nil {
		http.NotFound(w, r)

		return
	}
	defer closeFn()
	ct, ok := contentTypes[strings.ToLower(path.Ext(file))]
	if !ok {
		ct = mime.TypeByExtension(path.Ext(file))
	}
	if ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	http.ServeContent(w, r, file, mod, rs)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// listenLocked makes sure a live listener serves: it does nothing when there
// is one, otherwise it takes the old port or a new one on 127.0.0.1 alone and
// starts the goroutine that serves. s.mu is held.
func (s *Server) listenLocked() error {
	if s.srv != nil && !s.dead {
		return nil
	}
	if s.srv != nil { // the socket died: finish the old server
		old, done := s.srv, s.done
		s.srv, s.ln, s.done, s.dead = nil, nil, nil, false
		// drain first: a request in flight is answered, not cut.
		ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
		_ = old.Shutdown(ctx)
		cancel()
		_ = old.Close()
		<-done
	}
	ln, err := net.Listen("tcp4", net.JoinHostPort(loopback, strconv.Itoa(s.port)))
	if err != nil && s.port != 0 {
		s.logf("mock: loopback port %d is taken: %v", s.port, err)
		s.port = 0
		ln, err = net.Listen("tcp4", net.JoinHostPort(loopback, "0"))
	}
	if err != nil {
		return fmt.Errorf("mock: listen on %s: %w", loopback, err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if port != s.port {
		s.port = port
		s.epoch.Add(1)
	}
	srv := &http.Server{
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      time.Minute,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    8192,
		ErrorLog:          log.New(logWriter{s.logf}, "", 0),
	}
	done := make(chan struct{})
	s.srv, s.ln, s.done, s.dead = srv, ln, done, false
	go func() {
		defer close(done)
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			s.mu.Lock()
			if s.srv == srv {
				s.dead = true
			}
			s.mu.Unlock()
			s.logf("mock: loopback server stopped: %v", err)
		}
	}()

	return nil
}

// isFile reports whether name is a regular file of the folder.
func (s *Server) isFile(name string) bool {
	if name == "" || !fs.ValidPath(name) {
		return false
	}
	st, err := fs.Stat(s.files, name)

	return err == nil && st.Mode().IsRegular()
}

// open opens name for ServeContent; a file that cannot seek is read whole.
func (s *Server) open(name string) (io.ReadSeeker, time.Time, func(), error) {
	f, err := s.files.Open(name)
	if err != nil {
		return nil, time.Time{}, nil, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()

		return nil, time.Time{}, nil, err
	}
	if rs, ok := f.(io.ReadSeeker); ok {
		return rs, st.ModTime(), func() { _ = f.Close() }, nil
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, time.Time{}, nil, err
	}

	return bytes.NewReader(data), st.ModTime(), func() {}, nil
}

// logWriter writes the lines of the http.Server's error log to a logf.
type logWriter struct{ logf func(format string, a ...any) }

// Write logs p, a line.
func (w logWriter) Write(p []byte) (int, error) {
	w.logf("mock: loopback server: %s", strings.TrimSpace(string(p)))

	return len(p), nil
}
