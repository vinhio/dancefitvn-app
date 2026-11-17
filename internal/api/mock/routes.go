package mock

import (
	"encoding/json/v2"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// routesFile is the name of the route table in the mock folder.
const routesFile = "routes.json"

// Scenario is one answer of a route: a status and the file of its body.
type Scenario struct {
	// Status is the HTTP status.
	Status int `json:"status"`
	// File is the body file below the mock folder; nil means no body (204).
	// It may hold the token {{path.id}}.
	File *string `json:"file"`
}

// Route is one row of routes.json.
type Route struct {
	// ID is the route id ("me.courses").
	ID string `json:"id"`
	// Method is the HTTP method.
	Method string `json:"method"`
	// Path is the path below the base path, "{id}" for the one path value.
	Path string `json:"path"`
	// Auth is true when the route needs a bearer token.
	Auth bool `json:"auth"`
	// Request is the sample request file, or a text for a multipart body;
	// nil for none.
	Request *string `json:"request"`
	// Responses maps a scenario name ("default" ...) to its answer.
	Responses map[string]Scenario `json:"responses"`

	segments []string
}

// table is routes.json.
type table struct {
	Version        string  `json:"version"`
	BasePath       string  `json:"base_path"`
	DefaultDelayMs int     `json:"default_delay_ms"`
	Routes         []Route `json:"routes"`
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// loadTable reads and checks routes.json of fsys.
func loadTable(fsys fs.FS) (*table, error) {
	data, err := fs.ReadFile(fsys, routesFile)
	if err != nil {
		return nil, fmt.Errorf("mock: read %s: %w", routesFile, err)
	}
	var t table
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("mock: parse %s: %w: %w", routesFile, core.ErrInvalid, err)
	}
	if t.BasePath != "" && !strings.HasPrefix(t.BasePath, "/") {
		return nil, fmt.Errorf("mock: base path %q: %w", t.BasePath, core.ErrInvalid)
	}
	t.BasePath = strings.TrimSuffix(t.BasePath, "/")
	seen := map[string]bool{}
	for i := range t.Routes {
		r := &t.Routes[i]
		if r.ID == "" || r.Method == "" || !strings.HasPrefix(r.Path, "/") || seen[r.ID] {
			return nil, fmt.Errorf("mock: route %q: %w", r.ID, core.ErrInvalid)
		}
		if _, ok := r.Responses["default"]; !ok {
			return nil, fmt.Errorf("mock: route %q has no default scenario: %w", r.ID, core.ErrInvalid)
		}
		seen[r.ID] = true
		r.segments = strings.Split(strings.TrimPrefix(r.Path, "/"), "/")
	}

	return &t, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Routes returns the routes of routes.json, in file order.
func (t *Transport) Routes() []Route {
	return slices.Clone(t.table.Routes)
}

// ScenarioNames returns the scenario names of a route, sorted; nil for an
// unknown route.
func (t *Transport) ScenarioNames(routeID string) []string {
	r := t.route(routeID)
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.Responses))
	for n := range r.Responses {
		names = append(names, n)
	}
	slices.Sort(names)

	return names
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// route returns the route with the id, nil when there is none.
func (t *Transport) route(id string) *Route {
	for i := range t.table.Routes {
		if t.table.Routes[i].ID == id {
			return &t.table.Routes[i]
		}
	}

	return nil
}
