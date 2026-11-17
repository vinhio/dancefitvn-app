package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Hybrid sends the routes of a list to the live API and the others to the
// mock (05 §3 live_routes). It is safe for concurrent use.
type Hybrid struct {
	live, mock Transport
	liveRoutes map[string]struct{}
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewHybrid returns a Hybrid. liveRoutes are route ids ("me.courses").
func NewHybrid(live, mock Transport, liveRoutes []string) *Hybrid {
	set := make(map[string]struct{}, len(liveRoutes))
	for _, id := range liveRoutes {
		set[id] = struct{}{}
	}

	return &Hybrid{live: live, mock: mock, liveRoutes: set}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Do implements Transport.
func (h *Hybrid) Do(ctx context.Context, req *Request) (*Response, error) {
	t := h.mock
	if _, ok := h.liveRoutes[req.RouteID]; ok {
		t = h.live
	}
	if t == nil {
		return nil, fmt.Errorf("api: no transport for route %q: %w", req.RouteID, core.ErrInvalid)
	}

	return t.Do(ctx, req)
}

// CheckLiveRoutes checks a list of route ids to send to the live API while
// the others stay on the mock. It returns core.ErrInvalid for an unknown id,
// and for a list that sends a route that needs the access token live while
// auth.login or auth.refresh stays on the mock: the live server would get
// tokens the mock made. An empty list is fine.
func CheckLiveRoutes(ids []string) error {
	byID := make(map[string]Route, len(routes))
	for _, r := range routes {
		byID[r.ID] = r
	}
	live := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := byID[id]; !ok {
			return fmt.Errorf("api: live route %q is unknown: %w", id, core.ErrInvalid)
		}
		live[id] = struct{}{}
	}

	var authLive []string
	for _, r := range routes { // table order keeps the message stable
		if _, ok := live[r.ID]; ok && r.Auth {
			authLive = append(authLive, r.ID)
		}
	}
	if len(authLive) == 0 {
		return nil
	}
	var mocked []string
	for _, id := range []string{"auth.login", "auth.refresh"} {
		if _, ok := live[id]; !ok {
			mocked = append(mocked, id)
		}
	}
	if len(mocked) == 0 {
		return nil
	}

	return fmt.Errorf("api: live routes %s need a token but %s stay on the mock: %w",
		strings.Join(authLive, ","), strings.Join(mocked, ","), core.ErrInvalid)
}
