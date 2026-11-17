// Adapted from jplayer-mobile/internal/media/amp/preflight_android.go at 49de3c2.

//go:build android && cgo

package amp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// preflightTimeout bounds the preflight's request: past it, MediaPlayer's
// own answer stands.
const preflightTimeout = 10 * time.Second

// preflightClient asks for the first bytes of links; made once.
var preflightClient = &http.Client{Timeout: preflightTimeout}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// preflight asks the server of a link for its first two bytes, alongside
// MediaPlayer: a name that does not resolve, a connection refused or an
// HTTP status of 400 and over is a link that cannot be opened, which a
// browser tells at once, while MediaPlayer retries for some 30 s
// (NuCachedSource2) before onError. Anything else (an answer, a timeout,
// a TLS failure the system's own stack may not have, ctx ended) is no
// verdict: nil, and MediaPlayer decides.
func preflight(ctx context.Context, rawURL string) error {
	if !isHTTP(rawURL) {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil //nolint:nilerr // MediaPlayer gets the link as it is, and decides
	}
	req.Header.Set("Range", "bytes=0-1")
	resp, err := preflightClient.Do(req)
	if err != nil {
		var dns *net.DNSError
		switch {
		case ctx.Err() != nil:
			return nil //nolint:nilerr // a cancelled preflight is not a failure: the caller gave up
		case errors.As(err, &dns) && dns.IsNotFound:
			return fmt.Errorf("amp: %s does not resolve: %w", dns.Name, err)
		case errors.Is(err, syscall.ECONNREFUSED):
			return fmt.Errorf("amp: connection refused: %w", err)
		default:
			return nil
		}
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("amp: http status %d", resp.StatusCode)
	}

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// isHTTP tells a link of http or https.
func isHTTP(rawURL string) bool {
	l := strings.ToLower(rawURL)

	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://")
}
