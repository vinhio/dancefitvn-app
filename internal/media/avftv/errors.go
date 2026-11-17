// Adapted from jplayer-tv/internal/media/avf/errors.go at 781bee5.

package avftv

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/media"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Error domains of NSError that name a failure to reach the stream.
const (
	domainURL       = "NSURLErrorDomain"
	domainCFNetwork = "kCFErrorDomainCFNetwork"
	domainCoreMedia = "CoreMediaErrorDomain"
)

// coreMediaHTTP404 is CoreMedia's error for an HTTP 404, as measured.
const coreMediaHTTP404 = -12938

// Codes of NSURLErrorDomain, shared by kCFErrorDomainCFNetwork's
// kCFURLError…, that name a failure of the network (isNetwork).
const (
	urlTimedOut            = -1001
	urlCannotFindHost      = -1003
	urlCannotConnectToHost = -1004
	urlConnectionLost      = -1005
	urlDNSLookupFailed     = -1006
	urlNotConnected        = -1009
	urlDataNotAllowed      = -1020
)

// Codes only kCFErrorDomainCFNetwork has that name a failure of the
// network (isNetwork).
const (
	cfHostNotFound       = 1
	cfHostUnknown        = 2
	cfHTTPConnectionLost = 302
)

// maxDomain bounds the length of a domain kept in an error's text.
const maxDomain = 64

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// streamError is the error of a stream that failed, from the NSError's
// domain and code and those of its underlying error (under is "" when it
// has none): "avf: stream failed: <domain> <code>[ (<under> <underCode>)]"
// (D-T4-9). It never holds a URL or a localized description, which can
// echo the URL; the domains are reduced to identifier characters in case
// one ever carries more.
//
// It wraps exactly one class (D-T13-8): media.ErrNetwork when either error
// is a failure of the network (isNetwork), media.ErrSource otherwise. It
// also keeps the sentinel it wrapped before T13: core.ErrNotFound when
// either error is a failure to reach the stream (isUnreachable),
// core.ErrUnsupported otherwise (OQ-T4-5); media.ErrNetwork wraps
// core.ErrNotFound itself.
func streamError(domain string, code int, under string, underCode int) error {
	network := isNetwork(domain, code) || (under != "" && isNetwork(under, underCode))
	sentinel := core.ErrUnsupported
	if isUnreachable(domain, code) || (under != "" && isUnreachable(under, underCode)) {
		sentinel = core.ErrNotFound
	}
	domain = cleanDomain(domain)
	text := fmt.Sprintf("avf: stream failed: %s %d", domain, code)
	if under != "" {
		text = fmt.Sprintf("avf: stream failed: %s %d (%s %d)", domain, code, cleanDomain(under), underCode)
	}
	if network {
		return fmt.Errorf("%s: %w", text, media.ErrNetwork)
	}

	return fmt.Errorf("%s: %w: %w", text, media.ErrSource, sentinel)
}

// isNetwork tells whether an NSError of domain and code is a failure of
// the network rather than of the stream (D-T13-8): of the URL loading
// system or of CFNetwork, a timeout (-1001), an unknown host (-1003, and
// -1006 for a failed DNS lookup), a refused connection (-1004), a lost
// connection (-1005), no connection (-1009) or no data allowed (-1020);
// CFNetwork also names an unknown host 1 or 2 (kCFHostErrorHostNotFound,
// kCFHostErrorUnknown) and a lost HTTP connection 302
// (kCFErrorHTTPConnectionLost). An HTTP error, a body that is not a
// playlist (-1002), a host App Transport Security does not allow (-1022)
// and every CoreMedia or AVFoundation error are of the source.
func isNetwork(domain string, code int) bool {
	switch domain {
	case domainURL:
		return isNetworkURLCode(code)
	case domainCFNetwork:
		return isNetworkURLCode(code) || code == cfHostNotFound || code == cfHostUnknown || code == cfHTTPConnectionLost
	default:
		return false
	}
}

// isNetworkURLCode tells whether code, of NSURLErrorDomain or its
// CFNetwork twin kCFURLError…, is a failure of the network.
func isNetworkURLCode(code int) bool {
	switch code {
	case urlTimedOut, urlCannotFindHost, urlCannotConnectToHost, urlConnectionLost,
		urlDNSLookupFailed, urlNotConnected, urlDataNotAllowed:
		return true
	default:
		return false
	}
}

// isUnreachable tells whether an NSError of domain and code says the
// stream could not be reached or read: any error of the URL loading system
// or of CFNetwork (OQ-T4-5), and CoreMedia's HTTP 404. Measured on the
// tvOS Simulator (TestNotFoundFails and the others, T4.2b): a 404 is
// NSURLErrorDomain -1100 over CoreMediaErrorDomain -12938, a refused
// connection NSURLErrorDomain -1004, plain HTTP to a host App Transport
// Security does not allow NSURLErrorDomain -1022, and a body that is not a
// playlist NSURLErrorDomain -1002 over CoreMediaErrorDomain -1002, which
// therefore counts as unreachable too.
func isUnreachable(domain string, code int) bool {
	switch domain {
	case domainURL, domainCFNetwork:
		return true
	case domainCoreMedia:
		return code == coreMediaHTTP404
	default:
		return false
	}
}

// cleanDomain keeps the letters, digits, dots, dashes and underscores of
// s, at most maxDomain of them; "unknown" when none is left.
func cleanDomain(s string) string {
	out := make([]byte, 0, min(len(s), maxDomain))
	for i := 0; i < len(s) && len(out) < maxDomain; i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-' || c == '_' {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "unknown"
	}

	return string(out)
}
