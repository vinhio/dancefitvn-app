package pairing_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/pairing"
)

const mockDir = "../../mock/dancefitvn-mock-api"

var t0 = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)

// TestMain fails the run when a test leaves a goroutine of pairing behind.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && leaked() {
		os.Stderr.WriteString("goroutine leak in package pairing\n")
		code = 1
	}
	os.Exit(code)
}

func leaked() bool {
	for range 500 {
		var buf bytes.Buffer
		_ = pprof.Lookup("goroutine").WriteTo(&buf, 2)
		if !strings.Contains(buf.String(), "internal/pairing.") {
			return false
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}

	return true
}

// offlineErr is a failure of the network.
var offlineErr = &net.OpError{Op: "dial", Err: errString("refused")}

type errString string

func (e errString) Error() string { return string(e) }

// spy wraps a transport: it counts calls by route id, and a hook can answer
// instead of the inner one.
type spy struct {
	inner api.Transport
	hook  func(ctx context.Context, n int, req *api.Request) (*api.Response, error, bool)

	mu    sync.Mutex
	calls map[string]int
}

func (s *spy) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	s.mu.Lock()
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	s.calls[req.RouteID]++
	n := s.calls[req.RouteID]
	hook := s.hook
	s.mu.Unlock()
	if hook != nil {
		if resp, err, ok := hook(ctx, n, req); ok {
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

type staticToken struct{}

func (staticToken) Token() string                                   { return "test-token" }
func (staticToken) Refresh(context.Context, string) (string, error) { return "test-token", nil }

// rig is a TV pairing over the real mock folder and a fake clock.
type rig struct {
	mock *mock.Transport
	spy  *spy
	api  *api.Client
	fake *clock.Fake
	ui   chan func()
	tv   *pairing.TV

	states    []pairing.State
	confirmed []model.Session
	errs      []error
}

// newRig builds the client and the TV; scenarios are "route=name".
func newRig(t *testing.T, scenarios ...[2]string) *rig {
	t.Helper()
	m, err := mock.New(os.DirFS(mockDir), mock.Options{
		Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = m.Close() })
	for _, s := range scenarios {
		require.NoError(t, m.SetScenario(s[0], s[1]))
	}
	r := &rig{mock: m, spy: &spy{inner: m}, fake: clock.NewFake(t0), ui: make(chan func(), 64)}
	r.api, err = api.New(api.Options{Transport: r.spy, Version: "1.0.0", Platform: "tvos", InstallID: "i-1", Tokens: staticToken{}})
	require.NoError(t, err)
	r.tv = pairing.NewTV(pairing.TVOptions{
		Client: r.api, Clock: r.fake, Dispatch: func(fn func()) { r.ui <- fn },
		Device:      model.DeviceInfo{Platform: "tvos", Name: "Apple TV", AppVersion: "1.0.0", InstallID: "i-1"},
		OnState:     func(s pairing.State) { r.states = append(r.states, s) },
		OnConfirmed: func(s model.Session) { r.confirmed = append(r.confirmed, s) },
		OnError:     func(err error) { r.errs = append(r.errs, err) },
	})
	t.Cleanup(func() { r.tv.Stop(); r.tv.Wait() })

	return r
}

// pump runs the next function dispatched to the UI thread.
func (r *rig) pump() { (<-r.ui)() }
