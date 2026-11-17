package auth_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/store"
)

const mockDir = "../../mock/dancefitvn-mock-api"

// TestMain fails the run when a test leaves a goroutine of auth behind.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && leaked() {
		os.Stderr.WriteString("goroutine leak in package auth\n")
		code = 1
	}
	os.Exit(code)
}

func leaked() bool {
	for range 500 {
		var buf bytes.Buffer
		_ = pprof.Lookup("goroutine").WriteTo(&buf, 2)
		if !strings.Contains(buf.String(), "dancefitvn-app/internal/auth.") {
			return false
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}

	return true
}

// netErr is a failure of the network.
var netErr = &net.OpError{Op: "dial", Err: assertErr("connection refused")}

type assertErr string

func (e assertErr) Error() string { return string(e) }

// spy wraps a transport: it counts the calls by route id, and a hook can
// answer instead of the inner one.
type spy struct {
	inner api.Transport
	hook  func(req *api.Request) (*api.Response, error, bool)

	mu    sync.Mutex
	calls map[string]int
}

func (s *spy) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	s.mu.Lock()
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	s.calls[req.RouteID]++
	s.mu.Unlock()
	if s.hook != nil {
		if resp, err, ok := s.hook(req); ok {
			return resp, err
		}
	}

	return s.inner.Do(ctx, req)
}

func (s *spy) count(route string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.calls[route]
}

// rig is a Manager, its client, and what happened.
type rig struct {
	mock  *mock.Transport
	spy   *spy
	vault *auth.MemoryVault
	mgr   *auth.Manager
	api   *api.Client
	cache *store.Cache

	mu          sync.Mutex
	cacheClears int
	signedOut   []error
	vaultErrs   []string
}

func (r *rig) clears() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.cacheClears
}

func (r *rig) outs() []error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]error(nil), r.signedOut...)
}

func (r *rig) vaultOps() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.vaultErrs...)
}

// newRig builds a Manager and an api.Client over the real mock folder.
func newRig(t *testing.T, tv bool) *rig {
	t.Helper()
	m, err := mock.New(os.DirFS(mockDir), mock.Options{
		Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close() })

	cache := store.OpenCache(filepath.Join(t.TempDir(), "api"), 0)
	t.Cleanup(cache.Close)
	r := &rig{mock: m, spy: &spy{inner: m}, vault: auth.NewMemoryVault(), cache: cache}
	r.mgr, err = auth.New(auth.Options{
		Vault: r.vault,
		TV:    tv,
		ClearCache: func() {
			cache.Clear()
			r.mu.Lock()
			r.cacheClears++
			r.mu.Unlock()
		},
		OnSignedOut: func(reason error) {
			r.mu.Lock()
			r.signedOut = append(r.signedOut, reason)
			r.mu.Unlock()
		},
		OnVaultError: func(op string, _ error) {
			r.mu.Lock()
			r.vaultErrs = append(r.vaultErrs, op)
			r.mu.Unlock()
		},
	})
	require.NoError(t, err)
	r.api, err = api.New(api.Options{
		Transport: r.spy, Version: "1.0.0", Platform: "ios", InstallID: "install-1",
		Tokens: r.mgr, Cache: cache,
	})
	require.NoError(t, err)
	r.mgr.Bind(r.api)

	return r
}

// jsonResp is a JSON answer with a status.
func jsonResp(status int, body string) *api.Response {
	return &api.Response{Status: status, Header: http.Header{}, Body: []byte(body)}
}

// session is a signed-in session for the vault.
func session(access, refresh, user string) model.Session {
	return model.Session{
		Tokens: model.Tokens{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: 900},
		User:   model.UserRef{ID: user, Email: "someone@example.com"},
	}
}
