package e2emedia

import (
	"dancefitvn-app/internal/api/mock"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The gate's paths: every link of the gate starts with gatePrefix; a long
// playlist is gatePrefix + "long/<lesson>/index.m3u8".
const (
	gatePrefix = "/g/"
	longPart   = "long/"
	longFile   = "index.m3u8"
	// segSeconds is the length of a segment of the mock's HLS (480p_00N.ts).
	segSeconds = 4
	// segPerLesson is how many segments a lesson of the mock has.
	segPerLesson = 3
)

// The query keys of a signed link, as the API's (05 §1: hls_url with exp
// and sig): exp is a Unix time in seconds.
const (
	keyExp = "exp"
	keySig = "sig"
)

// errClosed is what a closed server answers.
var errClosed = fmt.Errorf("e2emedia: server closed: %w", core.ErrClosed)

// Server is the media of the device tests: the mock's Server (URL) and the
// gate in front of it (Signed, Long, Revoke). Safe for concurrent use.
type Server struct {
	mock *mock.Server
	gate *http.Server
	ln   net.Listener
	done chan struct{}
	base string

	// now is the gate's clock for exp.
	now func() time.Time
	// revoked makes the gate refuse every link, as a CDN does once the
	// signed links expired while the media played.
	revoked atomic.Bool
	// served and refused count the gate's requests answered with a file
	// (or a playlist) and with 403.
	served, refused atomic.Int64
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Start serves dir, the media folder (hls/<lesson>/..., audio/...): the
// mock's Server listening on 127.0.0.1 (Ensure), and the gate on another
// loopback port. logf receives the servers' diagnostics; nil drops them.
func Start(dir string, logf func(format string, args ...any)) (*Server, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("e2emedia: media folder: %w", err)
	}
	s := &Server{mock: mock.NewServer(os.DirFS(dir), logf), now: time.Now}
	if err := s.mock.Ensure(); err != nil {
		return nil, fmt.Errorf("e2emedia: %w", err)
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		_ = s.mock.Close()

		return nil, fmt.Errorf("e2emedia: gate: %w", err)
	}
	s.ln = ln
	s.base = "http://" + ln.Addr().String()
	s.gate = &http.Server{Handler: s, ReadHeaderTimeout: 5 * time.Second}
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		if err := s.gate.Serve(ln); !errors.Is(err, http.ErrServerClosed) && logf != nil {
			logf("e2emedia: gate stopped: %v", err)
		}
	}()

	return s, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// URL returns the link of name, a slash path of the media folder
// ("hls/l_3503/master.m3u8"), on the mock's Server. It makes sure the
// server listens (iOS closes its socket in the background).
func (s *Server) URL(name string) string {
	_ = s.mock.Ensure()

	return s.mock.BaseURL() + "/" + strings.TrimPrefix(name, "/")
}

// Signed returns a signed link of name on the gate, valid until exp: the
// gate answers 403 from exp on, as the CDN does for the API's links.
func (s *Server) Signed(name string, exp time.Time) string {
	return s.base + gatePrefix + strings.TrimPrefix(name, "/") + "?" + query(exp)
}

// Long returns a signed link, valid until exp, of an HLS media playlist of
// the lesson's three segments played repeats times over (a discontinuity
// between rounds): 12·repeats seconds of media, so that the player has
// segments left to fetch when the links are revoked.
func (s *Server) Long(lesson string, repeats int, exp time.Time) string {
	return fmt.Sprintf("%s%s%s%s/%s?%s&n=%d", s.base, gatePrefix, longPart, lesson, longFile, query(exp), max(repeats, 1))
}

// Revoke makes the gate refuse every request (403), or serve again.
func (s *Server) Revoke(on bool) { s.revoked.Store(on) }

// Counts returns how many requests the gate served and refused.
func (s *Server) Counts() (served, refused int64) { return s.served.Load(), s.refused.Load() }

// Requests returns how many requests the mock's Server began.
func (s *Server) Requests() int64 { return s.mock.Requests() }

// Close stops both listeners. Safe to call twice.
func (s *Server) Close() error {
	if s.gate == nil {
		return errClosed
	}
	err := s.gate.Close()
	<-s.done
	s.gate = nil

	return errors.Join(err, s.mock.Close())
}

// ServeHTTP is the gate: 403 for a link revoked or past its exp, a long
// playlist made on the fly, else the file from the mock's Server.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, gatePrefix) {
		http.NotFound(w, r)

		return
	}
	if s.revoked.Load() || s.expired(r) {
		s.refused.Add(1)
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}
	rest := strings.TrimPrefix(r.URL.Path, gatePrefix)
	if strings.HasPrefix(rest, longPart) {
		s.serveLong(w, r, strings.TrimPrefix(rest, longPart))

		return
	}
	s.served.Add(1)
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/" + rest
	r2.URL.RawQuery = ""
	s.mock.ServeHTTP(w, r2)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// expired reports whether the request's exp is past; a link without exp
// never expires.
func (s *Server) expired(r *http.Request) bool {
	v := r.URL.Query().Get(keyExp)
	if v == "" {
		return false
	}
	exp, err := strconv.ParseInt(v, 10, 64)

	return err != nil || !s.now().Before(time.Unix(exp, 0))
}

// serveLong answers rest, "<lesson>/index.m3u8", with the long playlist:
// its segments are links of the gate with the request's exp.
func (s *Server) serveLong(w http.ResponseWriter, r *http.Request, rest string) {
	lesson, file := path.Split(rest)
	lesson = strings.TrimSuffix(lesson, "/")
	n, err := strconv.Atoi(r.URL.Query().Get("n"))
	if file != longFile || lesson == "" || strings.Contains(lesson, "/") || err != nil || n < 1 {
		http.NotFound(w, r)

		return
	}
	s.served.Add(1)
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	_, _ = w.Write([]byte(longPlaylist(lesson, n, r.URL.Query().Get(keyExp))))
}

// longPlaylist is the VOD media playlist of the lesson's segments played
// rounds times, each segment a distinct link (n) so that none is taken
// from a cache.
func longPlaylist(lesson string, rounds int, exp string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n", segSeconds)
	for i := range rounds {
		if i > 0 {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		for k := range segPerLesson {
			fmt.Fprintf(&b, "#EXTINF:%d.000000,\n%shls/%s/480p_%03d.ts?%s=%s&n=%d\n", segSeconds, gatePrefix, lesson, k, keyExp, exp, i*segPerLesson+k)
		}
	}
	b.WriteString("#EXT-X-ENDLIST\n")

	return b.String()
}

// query is the signed query of a link valid until exp.
func query(exp time.Time) string {
	return keyExp + "=" + strconv.FormatInt(exp.Unix(), 10) + "&" + keySig + "=e2e"
}
