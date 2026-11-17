//go:build !live

package app

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"dancefitvn-app/internal/api/mock"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// liveBuild is false in a build that has the mock in the process: the
// root package embeds mock/dancefitvn-mock-api (mockfs.go), and the app
// talks to it unless a developer's DFV_API says otherwise (apimode.go).
const liveBuild = false

// mockSide is the mock in the process, as the wire sees it (inProcMock).
// In a hybrid, the routes sent live take no scenario: the mock never
// answers them.
type mockSide struct {
	*mock.Transport
	live map[string]struct{}
}

var _ inProcMock = (*mockSide)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newInProcMock returns the mock of the folder fsys; liveRoutes are the
// routes a hybrid sends live.
func newInProcMock(fsys fs.FS, wait func(ctx context.Context, d time.Duration) error,
	logf func(format string, args ...any), liveRoutes []string,
) (inProcMock, error) {
	t, err := mock.New(fsys, mock.Options{Wait: wait, Logf: logf})
	if err != nil {
		return nil, err
	}
	live := make(map[string]struct{}, len(liveRoutes))
	for _, id := range liveRoutes {
		live[id] = struct{}{}
	}

	return &mockSide{Transport: t, live: live}, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetScenario names the scenario of a route the mock answers; a route a
// hybrid sends live is refused with errNoMock.
func (m *mockSide) SetScenario(routeID, name string) error {
	if _, ok := m.live[routeID]; ok {
		return fmt.Errorf("app: %s is sent live (%s): %w", routeID, liveRoutesEnv, errNoMock)
	}

	return m.Transport.SetScenario(routeID, name)
}

// Media is the mock's media server.
func (m *mockSide) Media() mediaServer { return m.MediaServer() }
