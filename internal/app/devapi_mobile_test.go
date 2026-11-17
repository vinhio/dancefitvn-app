//go:build !tvos

package app

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// routeCounter counts the requests on the way to inner.
type routeCounter struct {
	inner api.Transport
	n     atomic.Int64
}

func (c *routeCounter) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	c.n.Add(1)

	return c.inner.Do(ctx, req)
}

// TestPhoneWireTalksToMockserve: with a server of DFV_API (deps.API) the
// phone's services make no mock of their own and send every request there;
// a state's scenario then fails with errNoMock.
func TestPhoneWireTalksToMockserve(t *testing.T) {
	t.Parallel()
	mt, err := mock.New(os.DirFS(mockDir), mock.Options{Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }})
	require.NoError(t, err)
	t.Cleanup(func() { _ = mt.Close() })
	server := &routeCounter{inner: mt}
	r := newRigOf(t, nil, "")
	r.deps.API = liveTo(server)
	r.deps.Mock = nil
	w := r.wire(wireKnobs{})
	_, err = w.Restore(t.Context())
	require.NoError(t, err)
	_, err = w.Config(t.Context())
	require.NoError(t, err)
	assert.Nil(t, w.mock)
	assert.Nil(t, w.overlay)
	assert.True(t, w.readBack, "cmd/mockserve answers a write with the mock's files")
	assert.Positive(t, server.n.Load())
	require.ErrorIs(t, setScenario(w.mock, "me.home", "new_user"), errNoMock)
}

// TestBuildPlanReadsNoEnvInTests: the build the tests run is no developer's
// (unless -tags dancefitvndev): DFV_API is never read, and the plan of a
// build with the mock is the mock.
func TestBuildPlanReadsNoEnvInTests(t *testing.T) {
	t.Parallel()
	if devBuild || liveBuild {
		t.Skip("the plain build's rule")
	}
	p, err := buildPlan(nil, true)
	require.NoError(t, err)
	assert.Equal(t, apiModeMock, p.mode)
}
