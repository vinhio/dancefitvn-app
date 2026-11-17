package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/api"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// apiEnv names the base URL of a mock served by cmd/mockserve
// ("http://127.0.0.1:8787/api/v1"). A developer's build (devBuild) that finds
// it talks to that server instead of the mock in its own process, which is
// how an iPhone and an Apple TV simulator share one mock (the TV's pairing
// code reaches the phone); with DFV_LIVE_ROUTES, only those routes
// (apimode.go). In a developer's live build (-tags live,dancefitvndev) it
// replaces api.DefaultBase. The shipped build never reads it.
const apiEnv = "DFV_API"

// errNoNetwork is what the dead network of devTransport answers, as the
// system reports one (api.ErrOffline sees it as a network failure).
var errNoNetwork = errors.New("network is unreachable")

// devTransport is the transport between the API client and the mock: it
// counts the requests under way, for the shot that waits for the last one
// (devshot_mobile.go); a shot state may make it hold the requests of a
// route (M03B's loading Home) or fail them all as a dead network does
// (DFV_OFFLINE). While the API is the mock, which no airplane mode cuts
// (in the process), the device's network having none fails them too
// (setNetDown, P5 F-5). As the app ships, it otherwise only counts. It is
// safe for concurrent use.
type devTransport struct {
	inner api.Transport

	mu      sync.Mutex
	offline bool
	netDown bool
	gates   map[string]chan struct{}
	busy    int
	served  int
	// byRoute counts the answers by route id (answered).
	byRoute map[string]int
}

var _ api.Transport = (*devTransport)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newDevTransport returns a devTransport over inner.
func newDevTransport(inner api.Transport) *devTransport {
	return &devTransport{inner: inner, gates: map[string]chan struct{}{}, byRoute: map[string]int{}}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Do holds the request while its route is gated (outside the count: a
// held request is no work under way), fails it while offline, else hands it
// to the inner transport.
func (t *devTransport) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	t.mu.Lock()
	gate := t.gates[req.RouteID]
	t.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, fmt.Errorf("app: %s held: %w", req.RouteID, ctx.Err())
		}
	}
	t.mu.Lock()
	if t.offline || t.netDown {
		t.mu.Unlock()

		return nil, &url.Error{Op: req.Method, URL: req.Path, Err: &net.OpError{Op: "dial", Net: "tcp", Err: errNoNetwork}}
	}
	t.busy++
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		t.busy--
		t.served++
		t.byRoute[req.RouteID]++
		t.mu.Unlock()
	}()

	return t.inner.Do(ctx, req)
}

// devAPIBase returns the base URL DFV_API names; set is false when it is
// unset. An http URL must be on this machine (cmd/mockserve); https may be
// anywhere. The value is not a secret but is not logged either. Only a
// developer's build reads it (planAPI).
func devAPIBase(getenv func(string) string) (base string, set bool, err error) {
	base = strings.TrimSpace(getenv(apiEnv))
	if base == "" {
		return "", false, nil
	}
	u, perr := url.Parse(base)
	if perr != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false, fmt.Errorf("app: %s must be an http(s) URL: %w", apiEnv, core.ErrInvalid)
	}
	if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
		return "", false, fmt.Errorf("app: %s over http must be a loopback host: %w", apiEnv, core.ErrInvalid)
	}

	return base, true, nil
}

// isLoopbackHost reports whether host is localhost or a loopback address.
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}

// setOffline turns the dead network on or off.
func (t *devTransport) setOffline(on bool) {
	t.mu.Lock()
	t.offline = on
	t.mu.Unlock()
}

// setNetDown says whether the device has no network.
func (t *devTransport) setNetDown(on bool) {
	t.mu.Lock()
	t.netDown = on
	t.mu.Unlock()
}

// down tells whether the requests fail as a dead network's do: the state's
// knob or the device's network.
func (t *devTransport) down() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.offline || t.netDown
}

// hold holds every request of the route until release.
func (t *devTransport) hold(routeID string) (release func()) {
	ch := make(chan struct{})
	t.mu.Lock()
	t.gates[routeID] = ch
	t.mu.Unlock()
	var once sync.Once

	return func() {
		once.Do(func() {
			t.mu.Lock()
			if t.gates[routeID] == ch {
				delete(t.gates, routeID)
			}
			t.mu.Unlock()
			close(ch)
		})
	}
}

// answered tells how many requests of the route were answered so far.
func (t *devTransport) answered(routeID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.byRoute[routeID]
}

// idle tells whether no request is under way (held ones aside), and how many
// were answered so far.
func (t *devTransport) idle() (idle bool, served int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.busy == 0, t.served
}
