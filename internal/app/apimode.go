package app

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// liveRoutesEnv names, in a developer's build that also has DFV_API, the
// routes sent to that server while the others stay on the mock in the
// process ("app.config,auth.login,auth.refresh,me.home"): the hybrid of
// docs/plan/05-data.md section 3. A list api.CheckLiveRoutes refuses is
// logged (route ids only) and the mock answers everything.
const liveRoutesEnv = "DFV_LIVE_ROUTES"

// devParts tells whether the build holds the parts of a developer and of
// the mock: the shot states, the tags of overlayTags, the mock-media://
// scheme. Every build has them but the shipped live one (-tags live
// without dancefitvndev), whose program scripts/check-livebuild.sh proves
// free of them.
const devParts = devBuild || !liveBuild

// apiMode is where the app's requests go.
type apiMode int

const (
	// apiModeUnknown is no decision: the services fail.
	apiModeUnknown apiMode = iota
	// apiModeMock is the mock in the process with its overlay (05 section
	// 4): every build that has the mock, unless a developer says otherwise.
	apiModeMock
	// apiModeLive is the live API: api.DefaultBase in the shipped live build
	// (-tags live), DFV_API in a developer's.
	apiModeLive
	// apiModeHybrid sends the routes of DFV_LIVE_ROUTES to DFV_API and the
	// others to the mock in the process: a developer's build only.
	apiModeHybrid
)

// apiPlan is where the app's requests go: the mode, the live side's
// transport (live and hybrid) and the routes sent to it (hybrid).
type apiPlan struct {
	mode apiMode
	live api.Transport
	// routes are the route ids sent live (hybrid).
	routes []string
	// tags makes overlayTags give the live side's answers an ETag: a
	// server of a developer (cmd/mockserve) sends none. The real API does
	// (05 section 1), so the shipped live build talks to it bare.
	tags bool
}

// apiPlanFunc decides where the requests go; hasMock tells whether the
// build has the mock in the process. The error, when the plan is usable,
// is only logged.
type apiPlanFunc func(hasMock bool) (apiPlan, error)

// apiIn is what planAPI decides from.
type apiIn struct {
	// dev is a developer's build (devBuild): only it reads the environment.
	dev bool
	// hasMock tells whether the mock is in the process: false in a -tags
	// live build, or when the root package gave none.
	hasMock bool
	// getenv reads DFV_API and DFV_LIVE_ROUTES (os.Getenv); dev only.
	getenv func(string) string
	// newLive makes the live transport to a base URL (newLiveAt).
	newLive func(base string) (api.Transport, error)
}

// apiParts are the transports wireAPI made of a plan.
type apiParts struct {
	// mock is the mock in the process; nil in live mode.
	mock inProcMock
	// overlay keeps what the user changed of the mock's data; nil in live
	// mode.
	overlay mockOverlay
	// tr is the transport the API client is given.
	tr *devTransport
}

// apiWireIn is what wireAPI makes the transports from.
type apiWireIn struct {
	plan   apiPlan
	mockFS fs.FS
	// wait is the mock's delay (mock.Options.Wait).
	wait func(ctx context.Context, d time.Duration) error
	logf func(format string, args ...any)
	// overlay is where the overlay keeps its edits: <data>, or memory for
	// a shot state.
	overlay store.Backend
}

// mockOverlay is the overlay of the mock's data (store.Overlay), as the
// wire uses it. An interface, so that the shipped live build, which makes
// none, does not link the overlay's code.
type mockOverlay interface {
	// Reset forgets every change.
	Reset() error
}

// inProcMock is the mock API in the process (internal/api/mock.Transport,
// mockside_on.go); a -tags live build has none (mockside_off.go) and does
// not link the mock's package.
type inProcMock interface {
	api.Transport
	// SetScenario names the scenario of a route (a shot state's).
	SetScenario(routeID, name string) error
	// Media is the mock's media server.
	Media() mediaServer
	// Close stops the media server.
	Close() error
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// String is the mode as the launch logs it.
func (m apiMode) String() string {
	switch m {
	case apiModeMock:
		return "mock"
	case apiModeLive:
		return "live"
	case apiModeHybrid:
		return "hybrid"
	default:
		return "unknown"
	}
}

// String is the plan as the launch logs it: the mode, and the routes of a
// hybrid. Never the URL.
func (p apiPlan) String() string {
	if p.mode != apiModeHybrid {
		return p.mode.String()
	}

	return "hybrid(" + strconv.Itoa(len(p.routes)) + " routes: " + strings.Join(p.routes, ",") + ")"
}

// planAPI decides where the requests go:
//
//   - the build that ships: the mock when it has it, else (-tags live) the
//     live API at api.DefaultBase. It reads no environment.
//   - a developer's build: the mock, unless DFV_API names a server (http on
//     the loopback only, https anywhere). With DFV_LIVE_ROUTES too, the
//     hybrid: those routes there, the others on the mock, once
//     api.CheckLiveRoutes accepts the list. Without the mock (-tags
//     live,dancefitvndev), the live API at DFV_API, else at
//     api.DefaultBase.
//
// A setting that cannot be followed comes back as the error, with the plan
// that stands instead (the mock, or the live build's base): the caller logs
// it. The error names route ids, never the URL. The plan is apiModeUnknown
// only when the live transport cannot be made.
func planAPI(in apiIn) (apiPlan, error) {
	mockPlan := apiPlan{mode: apiModeMock}
	if !in.dev {
		if in.hasMock {
			return mockPlan, nil
		}

		return liveAt(in, api.DefaultBase, false)
	}
	base, set, berr := devAPIBase(in.getenv)
	routes := strings.TrimSpace(in.getenv(liveRoutesEnv))
	if !in.hasMock {
		if !set {
			base = api.DefaultBase
		}
		p, err := liveAt(in, base, set)
		if err != nil {
			return p, err
		}
		if berr != nil {
			return p, fmt.Errorf("%w; the live build's base is used", berr)
		}
		if routes != "" {
			return p, fmt.Errorf("app: %s ignored: no mock in a live build: %w", liveRoutesEnv, core.ErrUnsupported)
		}

		return p, nil
	}
	if berr != nil {
		return mockPlan, fmt.Errorf("%w; the mock in the process answers", berr)
	}
	if !set {
		if routes != "" {
			return mockPlan, fmt.Errorf("app: %s needs %s; the mock in the process answers: %w", liveRoutesEnv, apiEnv, core.ErrInvalid)
		}

		return mockPlan, nil
	}
	if routes == "" {
		return liveAt(in, base, true)
	}
	ids := splitRoutes(routes)
	if err := api.CheckLiveRoutes(ids); err != nil {
		return mockPlan, fmt.Errorf("app: %s: %w; the mock in the process answers", liveRoutesEnv, err)
	}
	p, err := liveAt(in, base, true)
	if err != nil {
		return mockPlan, fmt.Errorf("%w; the mock in the process answers", err)
	}
	p.mode, p.routes = apiModeHybrid, ids

	return p, nil
}

// wireAPI makes the transports of a plan: the live API bare (the shipped
// live build) or tagged (a developer's server); the mock with its overlay;
// or the hybrid, whose overlay wraps the mock's side only (05 section 4).
// A live build has no mock to make: a plan that needs one fails with
// errNoMock.
func wireAPI(in apiWireIn) (apiParts, error) {
	switch in.plan.mode {
	case apiModeLive:
		return apiParts{tr: newDevTransport(liveSide(in.plan))}, nil
	case apiModeMock, apiModeHybrid:
	default:
		return apiParts{}, fmt.Errorf("app: services: api mode %s: %w", in.plan.mode, core.ErrInvalid)
	}
	if !devParts {
		return apiParts{}, errNoMock
	}
	m, err := newInProcMock(in.mockFS, in.wait, in.logf, in.plan.routes)
	if err != nil {
		return apiParts{}, fmt.Errorf("app: services: %w", err)
	}
	media := m.Media()
	ov := store.NewOverlay(plainMedia{inner: m, media: media}, in.overlay)
	var tr api.Transport = overlayTags{inner: ov, media: media}
	if in.plan.mode == apiModeHybrid {
		tr = api.NewHybrid(liveSide(in.plan), tr, in.plan.routes)
	}

	return apiParts{mock: m, overlay: ov, tr: newDevTransport(tr)}, nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// liveAt is the live plan to base; tags as apiPlan.tags.
func liveAt(in apiIn, base string, tags bool) (apiPlan, error) {
	newLive := in.newLive
	if newLive == nil {
		newLive = newLiveAt
	}
	live, err := newLive(base)
	if err != nil {
		return apiPlan{}, fmt.Errorf("app: live api: %w", err)
	}

	return apiPlan{mode: apiModeLive, live: live, tags: tags}, nil
}

// newLiveAt is api.NewLive at base.
func newLiveAt(base string) (api.Transport, error) {
	return api.NewLive(api.LiveOptions{Base: base})
}

// liveSide is the live transport of p, tagged when its server sends no
// ETag. The shipped live build never tags (devParts).
func liveSide(p apiPlan) api.Transport {
	if devParts && p.tags {
		return overlayTags{inner: p.live}
	}

	return p.live
}

// splitRoutes splits a comma-separated list of route ids, blanks dropped.
func splitRoutes(s string) []string {
	var ids []string
	for id := range strings.SplitSeq(s, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}

	return ids
}

// buildPlan is the plan of the build: the deps' when given (tests), else
// planAPI over the build's flags and, in a developer's build only, the
// environment.
//
// The build that ships decides here, so that its program holds none of the
// developer's variables: with the mock, the mock; without (-tags live),
// api.DefaultBase.
func buildPlan(fn apiPlanFunc, hasMock bool) (apiPlan, error) {
	if fn != nil {
		return fn(hasMock)
	}
	if !devBuild {
		if hasMock {
			return apiPlan{mode: apiModeMock}, nil
		}

		return liveAt(apiIn{}, api.DefaultBase, false)
	}

	return planAPI(apiIn{dev: true, hasMock: hasMock, getenv: os.Getenv})
}
