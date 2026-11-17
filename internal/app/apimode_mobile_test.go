//go:build !tvos

package app

import (
	"slices"
	"strings"
	"testing"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLiveBuildStarts: the phone's services of the shipped live build
// (no mock: Mock nil, api.DefaultBase) make no mock and no overlay, talk
// to the server bare and carry a sign-in to Home over HTTP. It runs in
// every build, -tags live included.
func TestLiveBuildStarts(t *testing.T) {
	t.Parallel()

	srv := newLiveServer(t)
	r := newRigOf(t, nil, "")
	r.deps.Mock = nil
	r.deps.API = srv.shippedPlan(t)
	w := r.wire(wireKnobs{})
	_, err := w.Restore(t.Context())
	require.NoError(t, err)
	require.NoError(t, w.Login(t.Context(), "coach@example.invalid", "secret1"))
	home, err := w.Home(t.Context())
	require.NoError(t, err)
	assert.NotEmpty(t, home.MyCourses)

	assert.Nil(t, w.mock, "no mock in the process")
	assert.Nil(t, w.overlay, "no overlay")
	_, bare := w.tr.inner.(*api.Live)
	assert.True(t, bare, "the server is asked bare, not %T", w.tr.inner)
	assert.False(t, w.readBack, "the real API answers a write with what it saved")
	assert.Positive(t, srv.hits.Load())
	assert.Contains(t, r.log.all(), "dancefitvn: API: live")
	for _, l := range r.log.all() {
		assert.False(t, strings.Contains(l, srv.url) || strings.Contains(l, "127.0.0.1"), "no URL logged: %q", l)
	}
	require.ErrorIs(t, setScenario(w.mock, "me.home", "new_user"), errNoMock)
	require.ErrorIs(t, w.ResetOverlay(), core.ErrUnsupported)
}

// TestLiveStateNeedsMock: a developer's state that sets a scenario fails
// with errNoMock over a live API, and so does the overlay's reset.
func TestLiveStateNeedsMock(t *testing.T) {
	t.Parallel()

	srv := newLiveServer(t)
	r := newRigOf(t, nil, "")
	r.deps.Mock = nil
	r.deps.API = srv.shippedPlan(t)
	w := r.wire(wireKnobs{})
	require.NoError(t, w.setup(t.Context()))
	require.ErrorIs(t, scenario("auth.login", "invalid_credentials")(t.Context(), w), errNoMock)
	require.ErrorIs(t, resetOverlay(t.Context(), w), errNoMock)
}

// TestLiveStateStops: a developer's state that needs a scenario of the
// mock (UPGRADE) over a live API logs why and runs no step.
func TestLiveStateStops(t *testing.T) {
	t.Parallel()
	if !devParts {
		t.Skip("the shipped live build has no states")
	}

	srv := newLiveServer(t)
	r := newRigOf(t, nil, "")
	r.deps.Mock = nil
	r.deps.API = srv.shippedPlan(t)
	r.mount(kittest.Phones[0], "UPGRADE", false)
	stopped := "dfv: state UPGRADE stopped: it needs the mock in the process, not a live API"
	r.drive("the state stops", func() bool { return slices.Contains(r.log.all(), stopped) })
	assert.True(t, r.w.noMock.Load())
	assert.NotEqual(t, env.RouteUpgrade, r.sh.Nav().Current().Screen)
}

// TestPhoneHybridWire: the hybrid plan through the phone's services: the
// sign-in and Home go to the server, the course to the mock in the
// process.
func TestPhoneHybridWire(t *testing.T) {
	t.Parallel()
	skipLiveBuild(t)

	srv := newLiveServer(t)
	live, err := api.NewLive(api.LiveOptions{Base: srv.url})
	require.NoError(t, err)
	ids := strings.Split(hybridRoutes, ",")
	r := newRig(t, nil, "")
	r.deps.API = func(hasMock bool) (apiPlan, error) {
		assert.True(t, hasMock)

		return apiPlan{mode: apiModeHybrid, live: live, routes: ids, tags: true}, nil
	}
	w := r.wire(wireKnobs{})
	_, err = w.Restore(t.Context())
	require.NoError(t, err)
	require.NoError(t, w.Login(t.Context(), "coach@example.invalid", "secret1"))
	_, err = w.Home(t.Context())
	require.NoError(t, err)
	n := srv.hits.Load()
	assert.Positive(t, n)
	_, err = w.Course(t.Context(), demoCourse)
	require.NoError(t, err)
	assert.Equal(t, n, srv.hits.Load(), "courses.show stays on the mock")
	require.NotNil(t, w.mock)
	require.NotNil(t, w.overlay)
	assert.Contains(t, r.log.all(), "dancefitvn: API: hybrid(4 routes: "+hybridRoutes+")")
}

// TestShippedLiveBuildHasNoStates: the shipped live build fills no table of
// states (devParts); every other build has them.
func TestShippedLiveBuildHasNoStates(t *testing.T) {
	t.Parallel()

	assert.Equal(t, devParts, len(shotStates) > 0)
	_, err := lookupState("M03")
	assert.Equal(t, devParts, err == nil)
}
