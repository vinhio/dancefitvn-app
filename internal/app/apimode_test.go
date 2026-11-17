package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The sets of DFV_LIVE_ROUTES the tests use.
const (
	// hybridRoutes is the list of local/plans/P8.md P8.1b's hybrid smoke.
	hybridRoutes = "app.config,auth.login,auth.refresh,me.home"
	// badRoutes sends me.home live with the sign-in on the mock.
	badRoutes = "app.config,me.home"
)

// fakeSide is a transport that records the routes it got and answers 200
// with its name.
type fakeSide struct {
	name string

	mu     sync.Mutex
	routes []string
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestLiveBuildTagFollowsTheTags checks that liveBuild is the live tag of
// the build and devParts its rule (adapted from jplayer-mobile/internal/ui/
// views/buildtags_test.go at 49de3c2). Run it with each set: go test
// -tags live, -tags live,dancefitvndev.
func TestLiveBuildTagFollowsTheTags(t *testing.T) {
	t.Parallel()

	bi, ok := debug.ReadBuildInfo()
	require.True(t, ok, "the test binary has its build info")
	var tags []string
	for _, s := range bi.Settings {
		if s.Key == "-tags" {
			tags = strings.Split(s.Value, ",")
		}
	}
	live := slices.Contains(tags, "live")
	dev := slices.Contains(tags, "dancefitvndev")
	assert.Equal(t, live, liveBuild, "liveBuild with tags %v", tags)
	assert.Equal(t, dev, devBuild, "devBuild with tags %v", tags)
	assert.Equal(t, dev || !live, devParts, "devParts with tags %v", tags)
}

// TestPlanAPI is the table of planAPI: the build (developer's or not, with
// the mock or not) and the variables DFV_API and DFV_LIVE_ROUTES give the
// mode, the live side's base and its routes.
func TestPlanAPI(t *testing.T) {
	t.Parallel()

	const server = "http://127.0.0.1:8787/api/v1"
	tests := []struct {
		name    string
		dev     bool
		hasMock bool
		api     string
		routes  string
		mode    apiMode
		base    string // the base given to newLive; "" when none is made
		ids     []string
		tags    bool
		err     error // a note logged; nil when none
	}{
		// The build that ships reads no environment.
		{"shipped mock", false, true, server, hybridRoutes, apiModeMock, "", nil, false, nil},
		{"shipped live", false, false, server, hybridRoutes, apiModeLive, api.DefaultBase, nil, false, nil},
		// A developer's build with the mock.
		{"dev mock", true, true, "", "", apiModeMock, "", nil, false, nil},
		{"dev DFV_API", true, true, server, "", apiModeLive, server, nil, true, nil},
		{"dev hybrid", true, true, server, hybridRoutes, apiModeHybrid, server, strings.Split(hybridRoutes, ","), true, nil},
		{"dev hybrid spaces", true, true, server, " auth.login , auth.refresh,,me.home ", apiModeHybrid, server, []string{"auth.login", "auth.refresh", "me.home"}, true, nil},
		{"dev hybrid public only", true, true, server, "app.config", apiModeHybrid, server, []string{"app.config"}, true, nil},
		{"dev hybrid token on mock", true, true, server, badRoutes, apiModeMock, "", nil, false, core.ErrInvalid},
		{"dev hybrid unknown route", true, true, server, "me.nothing", apiModeMock, "", nil, false, core.ErrInvalid},
		{"dev routes without DFV_API", true, true, "", hybridRoutes, apiModeMock, "", nil, false, core.ErrInvalid},
		{"dev bad DFV_API", true, true, "http://192.168.1.9:8787/api/v1", hybridRoutes, apiModeMock, "", nil, false, core.ErrInvalid},
		// A developer's live build (-tags live,dancefitvndev): no mock.
		{"dev live", true, false, "", "", apiModeLive, api.DefaultBase, nil, false, nil},
		{"dev live DFV_API", true, false, server, "", apiModeLive, server, nil, true, nil},
		{"dev live routes ignored", true, false, server, hybridRoutes, apiModeLive, server, nil, true, core.ErrUnsupported},
		{"dev live bad DFV_API", true, false, "lan:8787", "", apiModeLive, api.DefaultBase, nil, false, core.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			env := map[string]string{apiEnv: tt.api, liveRoutesEnv: tt.routes}
			var bases []string
			in := apiIn{
				dev: tt.dev, hasMock: tt.hasMock,
				getenv: func(k string) string {
					require.True(t, tt.dev, "the build that ships reads no environment")

					return env[k]
				},
				newLive: func(base string) (api.Transport, error) {
					bases = append(bases, base)

					return &fakeSide{name: "live"}, nil
				},
			}
			p, err := planAPI(in)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				assert.NotContains(t, err.Error(), "127.0.0.1", "no URL in the log")
				assert.NotContains(t, err.Error(), "192.168", "no URL in the log")
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.mode, p.mode)
			assert.Equal(t, tt.ids, p.routes)
			assert.Equal(t, tt.tags, p.tags)
			if tt.base == "" {
				assert.Empty(t, bases, "no live transport")
				assert.Nil(t, p.live)
			} else {
				assert.Equal(t, []string{tt.base}, bases)
				assert.NotNil(t, p.live)
			}
		})
	}
}

// TestPlanAPILiveFails: a live transport that cannot be made leaves the
// developer's build on the mock, and the live build without a plan.
func TestPlanAPILiveFails(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	in := apiIn{
		dev: true, hasMock: true,
		getenv: func(k string) string {
			return map[string]string{apiEnv: "https://example.test/api/v1", liveRoutesEnv: hybridRoutes}[k]
		},
		newLive: func(string) (api.Transport, error) { return nil, boom },
	}
	p, err := planAPI(in)
	require.ErrorIs(t, err, boom)
	assert.Equal(t, apiModeMock, p.mode)

	in.dev, in.hasMock = false, false
	p, err = planAPI(in)
	require.ErrorIs(t, err, boom)
	assert.Equal(t, apiModeUnknown, p.mode)
}

// TestPlanString: the launch's log line names the mode and a hybrid's
// routes, never a URL.
func TestPlanString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "mock", apiPlan{mode: apiModeMock}.String())
	assert.Equal(t, "live", apiPlan{mode: apiModeLive, live: &fakeSide{}}.String())
	assert.Equal(t, "unknown", apiPlan{}.String())
	assert.Equal(t, "hybrid(2 routes: auth.login,auth.refresh)",
		apiPlan{mode: apiModeHybrid, routes: []string{"auth.login", "auth.refresh"}}.String())
}

// TestWireAPILiveHasNoOverlay: the live plan makes no mock and no overlay;
// the shipped live build talks to the server bare (no tags), a developer's
// server through overlayTags.
func TestWireAPILiveHasNoOverlay(t *testing.T) {
	t.Parallel()

	for _, tags := range []bool{false, true} {
		live := &fakeSide{name: "live"}
		parts, err := wireAPI(apiWireIn{plan: apiPlan{mode: apiModeLive, live: live, tags: tags}})
		require.NoError(t, err)
		assert.Nil(t, parts.mock)
		assert.Nil(t, parts.overlay)
		if tags && devParts {
			assert.Equal(t, overlayTags{inner: live}, parts.tr.inner)
		} else {
			assert.Same(t, live, parts.tr.inner)
		}
	}
}

// TestWireAPIUnknownMode: no plan, no transport.
func TestWireAPIUnknownMode(t *testing.T) {
	t.Parallel()

	_, err := wireAPI(apiWireIn{})
	require.ErrorIs(t, err, core.ErrInvalid)
}

// TestWireAPIHybridRoutes: the routes of the list reach the live side, the
// others the mock in the process through its overlay, and a scenario of a
// live route is refused.
func TestWireAPIHybridRoutes(t *testing.T) {
	t.Parallel()
	skipLiveBuild(t)

	live := &fakeSide{name: "live"}
	ids := strings.Split(hybridRoutes, ",")
	require.NoError(t, api.CheckLiveRoutes(ids))
	parts, err := wireAPI(apiWireIn{
		plan:   apiPlan{mode: apiModeHybrid, live: live, routes: ids},
		mockFS: os.DirFS(mockDir), wait: noWait, logf: func(string, ...any) {}, overlay: store.NewMemory(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = parts.mock.Close() })
	require.NotNil(t, parts.overlay, "the mock's side keeps its overlay")

	for _, rt := range []struct {
		id, method, path string
		live             bool
	}{
		{"app.config", http.MethodGet, "/app/config", true},
		{"me.home", http.MethodGet, "/me/home", true},
		{"courses.show", http.MethodGet, "/courses/set-35", false},
		{"me.get", http.MethodGet, "/me", false},
	} {
		resp, err := parts.tr.Do(t.Context(), &api.Request{RouteID: rt.id, Method: rt.method, Path: rt.path})
		require.NoError(t, err, rt.id)
		if rt.live {
			assert.Equal(t, "live", string(resp.Body), rt.id)
		} else {
			assert.NotEqual(t, "live", string(resp.Body), rt.id)
		}
	}
	assert.Equal(t, []string{"app.config", "me.home"}, live.got())

	require.ErrorIs(t, setScenario(parts.mock, "me.home", "new_user"), errNoMock, "a live route takes no scenario")
	require.NoError(t, setScenario(parts.mock, "me.get", "token_expired"))
}

// TestWireAPIMockInLiveBuild: a live build has no mock to make.
func TestWireAPIMockInLiveBuild(t *testing.T) {
	t.Parallel()
	if !liveBuild {
		t.Skip("a -tags live build's rule")
	}

	for _, m := range []apiMode{apiModeMock, apiModeHybrid} {
		_, err := wireAPI(apiWireIn{plan: apiPlan{mode: m, live: &fakeSide{}}, mockFS: os.DirFS(mockDir), overlay: store.NewMemory()})
		require.ErrorIs(t, err, errNoMock, m.String())
	}
	_, err := newInProcMock(os.DirFS(mockDir), noWait, nil, nil)
	require.ErrorIs(t, err, errNoMock)
}

// TestSetScenarioWithoutMock: no mock, no scenario.
func TestSetScenarioWithoutMock(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, setScenario(nil, "me.home", "new_user"), errNoMock)
	assert.NotContains(t, errNoMock.Error(), "DFV_API", "the error fits a live build too")
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// Do records the route and answers 200 with the side's name.
func (f *fakeSide) Do(_ context.Context, req *api.Request) (*api.Response, error) {
	f.mu.Lock()
	f.routes = append(f.routes, req.RouteID)
	f.mu.Unlock()

	return &api.Response{Status: http.StatusOK, Header: http.Header{}, Body: []byte(f.name)}, nil
}

// got returns the routes the side got.
func (f *fakeSide) got() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.routes)
}

// liveTo is a plan that sends every request to tr, as a developer's server
// (tagged): the wire then makes no mock.
func liveTo(tr api.Transport) apiPlanFunc {
	return func(bool) (apiPlan, error) { return apiPlan{mode: apiModeLive, live: tr, tags: true}, nil }
}

// mockOnly is the plan of the mock in the process, whatever the
// environment of the test's process.
func mockOnly(bool) (apiPlan, error) { return apiPlan{mode: apiModeMock}, nil }

// skipLiveBuild skips a test that needs the mock in the process in a
// -tags live build, which has none.
func skipLiveBuild(t *testing.T) {
	t.Helper()
	if liveBuild {
		t.Skip("needs the mock in the process: not in a -tags live build")
	}
}

// noWait is the mock's delay, none.
func noWait(ctx context.Context, _ time.Duration) error { return ctx.Err() }

// newTestMock returns the repository's mock, closed with the test.
func newTestMock(t *testing.T) *mock.Transport {
	t.Helper()
	mt, err := mock.New(os.DirFS(mockDir), mock.Options{Wait: noWait})
	require.NoError(t, err)
	t.Cleanup(func() { _ = mt.Close() })

	return mt
}

// liveServer is the repository's mock behind its overlay with the pair
// relay, over real HTTP (as cmd/mockserve serves it): the stand-in of the
// live API.
type liveServer struct {
	url  string
	hits atomic.Int64
	ov   *store.Overlay
}

// newLiveServer starts a liveServer, closed with the test.
func newLiveServer(t *testing.T) *liveServer {
	t.Helper()
	mt := newTestMock(t)
	ov := store.NewOverlay(mt, store.NewMemory())
	confirmed, err := os.ReadFile(filepath.Join(mockDir, "tv", "pair-poll.confirmed.json"))
	require.NoError(t, err)
	require.NoError(t, ov.EnablePairRelay(store.PairRelayOptions{Confirmed: confirmed}))
	s := &liveServer{ov: ov}
	h := mt.HandlerOver(ov)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	s.url = srv.URL + "/api/v1"

	return s
}

// shippedPlan is the plan of the shipped live build (planAPI with no
// developer and no mock), its api.DefaultBase checked and replaced by the
// server's address.
func (s *liveServer) shippedPlan(t *testing.T) apiPlanFunc {
	t.Helper()

	return func(hasMock bool) (apiPlan, error) {
		assert.False(t, hasMock, "the live build has no mock")

		return planAPI(apiIn{hasMock: hasMock, newLive: func(base string) (api.Transport, error) {
			assert.Equal(t, api.DefaultBase, base)

			return api.NewLive(api.LiveOptions{Base: s.url})
		}})
	}
}
