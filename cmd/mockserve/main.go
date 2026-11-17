// Command mockserve serves the mock API of mock/dancefitvn-mock-api, with
// the store.Overlay in front of it and its pair relay on, over HTTP on the
// loopback address, so that two dev builds (an iPhone and an Apple TV
// simulator) talk to one mock: the code a TV shows is the one the phone
// confirms, and the TV's next poll answers with the session. It is a
// developer's tool; it binds to this machine only and logs no bodies, tokens
// or addresses of users. See README.md.
//
// Exit codes: 0 stopped by a signal, 2 usage or start failure.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/store"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Exit codes.
const (
	exitOK    = 0
	exitUsage = 2
)

const (
	defaultAddr = "127.0.0.1:8787"
	defaultMock = "mock/dancefitvn-mock-api"
	apiPrefix   = "/api/v1"
	mediaPrefix = "/media/"
	mockScheme  = "mock-media://"
)

// options are the flags.
type options struct {
	Addr    string
	Mock    string
	Overlay string
	Verbose bool
	// wait replaces the mock's delay; tests only.
	wait func(ctx context.Context, d time.Duration) error
}

// server is what main starts: the handler and what it holds.
type server struct {
	mock    *mock.Transport
	overlay *store.Overlay
	public  atomic.Value // string: "http://<addr>", set once the listener is up
	logf    func(format string, args ...any)
}

// fileBackend is a store.Backend that keeps the overlay record in one file.
type fileBackend struct{ path string }

var _ store.Backend = fileBackend{}

// mediaSwap is an api.Transport that turns the media URLs of the answers of
// inner into another prefix.
type mediaSwap struct {
	inner api.Transport
	from  func() string // "" while there is nothing to swap
	to    func() string
}

var _ api.Transport = mediaSwap{}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newServer loads the mock folder, puts the overlay (with the pair relay)
// before it and returns the server.
func newServer(o options, logf func(string, ...any)) (*server, error) {
	mt, err := mock.New(os.DirFS(o.Mock), mock.Options{Logf: logf, Wait: o.wait})
	if err != nil {
		return nil, fmt.Errorf("load mock %s: %w", o.Mock, err)
	}
	s := &server{mock: mt, logf: logf}
	s.public.Store("")
	var b store.Backend = store.NewMemory()
	if o.Overlay != "" {
		b = fileBackend{path: o.Overlay}
	}
	media := mt.MediaServer()
	// mock -> plain mock-media:// (what the overlay records) -> overlay ->
	// this server's /media/.
	plain := mediaSwap{inner: mt,
		from: func() string {
			if u := media.BaseURL(); u != "" {
				return u + "/"
			}

			return ""
		},
		to: func() string { return mockScheme },
	}
	s.overlay = store.NewOverlay(plain, b)
	confirmed, err := os.ReadFile(filepath.Join(o.Mock, "tv", "pair-poll.confirmed.json"))
	if err != nil {
		_ = mt.Close()

		return nil, fmt.Errorf("pair relay: %w", err)
	}
	pr := store.PairRelayOptions{Confirmed: confirmed}
	pr.Expired, _ = os.ReadFile(filepath.Join(o.Mock, "errors", "pair_code_expired.json"))
	pr.Invalid, _ = os.ReadFile(filepath.Join(o.Mock, "errors", "pair_code_invalid.json"))
	if err := s.overlay.EnablePairRelay(pr); err != nil {
		_ = mt.Close()

		return nil, fmt.Errorf("pair relay: %w", err)
	}

	return s, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run parses the flags, serves until ctx ends and returns the exit code.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	o, code := parseFlags(args, stderr)
	if code >= 0 {
		return code
	}
	ln, err := listenLoopback(o.Addr)
	if err != nil {
		fmt.Fprintf(stderr, "mockserve: %v\n", err)

		return exitUsage
	}
	logf := func(string, ...any) {}
	if o.Verbose {
		logf = func(f string, a ...any) { fmt.Fprintf(stderr, "mockserve: "+f+"\n", a...) }
	}
	s, err := newServer(o, logf)
	if err != nil {
		_ = ln.Close()
		fmt.Fprintf(stderr, "mockserve: %v\n", err)

		return exitUsage
	}
	defer func() { _ = s.mock.Close() }()
	s.public.Store("http://" + ln.Addr().String())
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	fmt.Fprintf(stdout, "mockserve: API at http://%s%s (DFV_API), media at /media/, Ctrl-C stops\n", ln.Addr(), apiPrefix)
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()
	select {
	case err := <-done:
		fmt.Fprintf(stderr, "mockserve: %v\n", err)

		return exitUsage
	case <-ctx.Done():
	}
	sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(sctx)
	<-done

	return exitOK
}

// Handler serves /api/v1/... through the overlay and /media/... from the
// mock's media folder.
func (s *server) Handler() http.Handler {
	h := s.mock.HandlerOver(mediaSwap{inner: s.overlay,
		from: func() string { return mockScheme },
		to: func() string {
			if p, _ := s.public.Load().(string); p != "" {
				return p + mediaPrefix
			}

			return ""
		},
	})
	mux := http.NewServeMux()
	mux.Handle(apiPrefix+"/", h)
	mux.Handle(mediaPrefix, http.StripPrefix("/media", s.mock.MediaServer()))
	mux.HandleFunc("/", http.NotFound)

	return mux
}

// Do asks inner and swaps the media prefix in the body of its answer.
func (m mediaSwap) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	resp, err := m.inner.Do(ctx, req)
	if err != nil || resp == nil || len(resp.Body) == 0 {
		return resp, err
	}
	from, to := m.from(), m.to()
	if from == "" || to == "" || from == to || !bytes.Contains(resp.Body, []byte(from)) {
		return resp, nil
	}
	out := *resp

	out.Body = bytes.ReplaceAll(resp.Body, []byte(from), []byte(to))

	return &out, nil
}

// Load implements store.Backend.
func (f fileBackend) Load(key string) ([]byte, error) {
	if key != store.OverlayKey {
		return nil, fmt.Errorf("mockserve: key %q: %w", key, core.ErrNotFound)
	}
	b, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("mockserve: overlay file: %w", core.ErrNotFound)
	}

	return b, err
}

// Save implements store.Backend: a temp file renamed over the target.
func (f fileBackend) Save(key string, data []byte) error {
	if key != store.OverlayKey {
		return nil
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}

	return os.Rename(tmp, f.path)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// parseFlags returns the options, or the exit code (>= 0) when the program
// must stop (-h gives 0).
func parseFlags(args []string, stderr io.Writer) (options, int) {
	var o options
	fs := flag.NewFlagSet("mockserve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.Addr, "addr", defaultAddr, "listen address; loopback only (127.0.0.1, ::1 or localhost)")
	fs.StringVar(&o.Mock, "mock", defaultMock, "mock API folder")
	fs.StringVar(&o.Overlay, "overlay", "", "file that keeps the overlay (edits of the demo user) between runs; empty keeps it in memory")
	fs.BoolVar(&o.Verbose, "v", false, "log the mock's own lines (never bodies or tokens)")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: mockserve [-addr 127.0.0.1:8787] [-mock dir] [-overlay file] [-v]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return o, exitOK
		}

		return o, exitUsage
	}
	if fs.NArg() > 0 {
		fs.Usage()

		return o, exitUsage
	}

	return o, -1
}

// listenLoopback listens on addr after it is shown to be loopback: a host
// that is empty, a name other than localhost or an address that is not
// loopback (0.0.0.0, a LAN address) is refused.
func listenLoopback(addr string) (net.Listener, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("-addr must be host:port: %w", err)
	}
	if strings.EqualFold(host, "localhost") {
		host = "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, errors.New("-addr must be a loopback address (127.0.0.1, ::1 or localhost): the mock has no login of its own")
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	return ln, nil
}
