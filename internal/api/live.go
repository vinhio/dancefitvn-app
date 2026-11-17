package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// DefaultBase is the base URL of the real DanceFITVN API (05 §1). Dev
// builds can replace it; a shipped build uses it as is.
const DefaultBase = "https://dancefitvn.com/mobile/api/v1"

const (
	liveTimeout   = 15 * time.Second
	liveMaxBody   = 8 << 20
	liveMaxRedirs = 5
)

// LiveOptions configures NewLive.
type LiveOptions struct {
	// Base is the base URL, "https://dancefitvn.com/mobile/api/v1"; required, http
	// or https.
	Base string
	// Client is the HTTP client; nil gives one with a 15 s timeout. The
	// Live keeps its own copy and sets its redirect policy.
	Client *http.Client
	// Wait pauses between retries of a GET; nil waits for real. Tests inject
	// an instant one.
	Wait func(ctx context.Context, d time.Duration) error
}

// Live is the Transport that talks HTTP to the real API. A GET that fails on
// the network is retried twice, after 0.5 s and 1.5 s; other methods are
// never retried. It is safe for concurrent use.
type Live struct {
	base   string
	client *http.Client
	wait   func(ctx context.Context, d time.Duration) error
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewLive returns a Live; core.ErrInvalid when Base is not an http(s) URL.
func NewLive(o LiveOptions) (*Live, error) {
	u, err := url.Parse(o.Base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("api: base must be an http or https url: %w", core.ErrInvalid)
	}

	cl := &http.Client{Timeout: liveTimeout}
	if o.Client != nil {
		c := *o.Client
		cl = &c
	}
	cl.CheckRedirect = sameHostRedirect

	l := &Live{base: strings.TrimRight(o.Base, "/"), client: cl, wait: o.Wait}
	if l.wait == nil {
		l.wait = waitTimer
	}

	return l, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Do implements Transport.
func (l *Live) Do(ctx context.Context, req *Request) (*Response, error) {
	raw := l.base + req.Path
	if len(req.Query) > 0 {
		raw += "?" + req.Query.Encode()
	}

	for attempt := 0; ; attempt++ {
		resp, err := l.once(ctx, raw, req)
		if err == nil {
			return resp, nil
		}
		if req.Method != http.MethodGet || attempt >= len(retryDelays) || ctx.Err() != nil || !isNetwork(err) {
			return nil, err
		}
		if werr := l.wait(ctx, retryDelays[attempt]); werr != nil {
			return nil, werr
		}
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// once makes one HTTP exchange.
func (l *Live) once(ctx context.Context, raw string, req *Request) (*Response, error) {
	var body io.Reader
	if req.Body != nil {
		body = bytes.NewReader(req.Body)
	}

	hr, err := http.NewRequestWithContext(ctx, req.Method, raw, body)
	if err != nil {
		return nil, fmt.Errorf("api: build request: %w", core.ErrInvalid)
	}
	for k, vs := range req.Header {
		for _, v := range vs {
			hr.Header.Add(k, v)
		}
	}

	hresp, err := l.client.Do(hr)
	if err != nil {
		return nil, err
	}
	defer hresp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(hresp.Body, liveMaxBody+1))
	if err != nil {
		return nil, err
	}
	if len(data) > liveMaxBody {
		return nil, fmt.Errorf("api: answer larger than %d bytes: %w", liveMaxBody, ErrServer)
	}
	if len(data) == 0 || hresp.StatusCode == http.StatusNoContent || hresp.StatusCode == http.StatusNotModified {
		data = nil
	}

	return &Response{Status: hresp.StatusCode, Header: hresp.Header.Clone(), Body: data}, nil
}

// sameHostRedirect follows redirects to the host of the first request only;
// another host gets the 3xx answer itself, so the token never leaves.
func sameHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= liveMaxRedirs || req.URL.Host != via[0].URL.Host {
		return http.ErrUseLastResponse
	}

	return nil
}
