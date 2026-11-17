//go:build tvos

package screens_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/pairing"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/screens"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mockDir is the mock API's folder, from the package directory.
const mockDir = "../../../../mock/dancefitvn-mock-api"

// t0 is the fake clock's start.
var t0 = time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)

// counter is a transport that counts the calls by route id.
type counter struct {
	inner api.Transport

	mu    sync.Mutex
	calls map[string]int
}

// Do counts the call and passes it on.
func (c *counter) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	c.mu.Lock()
	if c.calls == nil {
		c.calls = map[string]int{}
	}
	c.calls[req.RouteID]++
	c.mu.Unlock()

	return c.inner.Do(ctx, req)
}

// count returns how many times route was called.
func (c *counter) count(route string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.calls[route]
}

// staticToken is a token source that never changes.
type staticToken struct{}

func (staticToken) Token() string                                   { return "test-token" }
func (staticToken) Refresh(context.Context, string) (string, error) { return "test-token", nil }

// liveServices are fake services whose Pair is the real pairing.TV over the mock
// API and a fake clock, the way the app's wire makes it: the screen under test is
// the real one, and so is the cadence of the polls.
type liveServices struct {
	*tvtest.Services

	mock  *mock.Transport
	spy   *counter
	fake  *clock.Fake
	queue chan func()
	tv    *pairing.TV
}

// newLive returns the services over the mock folder.
func newLive(t *testing.T) *liveServices {
	t.Helper()
	m, err := mock.New(os.DirFS(mockDir), mock.Options{
		Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	})
	if err != nil {
		t.Skipf("no mock API: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	l := &liveServices{Services: tvtest.NewServices(), mock: m, spy: &counter{inner: m}, fake: clock.NewFake(t0), queue: make(chan func(), 64)}
	l.Services.OnlineNow = true

	return l
}

// Pair starts the real pairing; its answers come back through pump.
func (l *liveServices) Pair(h env.PairHooks) env.Pairing {
	cl, err := api.New(api.Options{Transport: l.spy, Version: "1.0.0", Platform: "tvos", InstallID: "i-1", Tokens: staticToken{}})
	if err != nil {
		panic(err)
	}
	l.tv = pairing.NewTV(pairing.TVOptions{
		Client: cl, Clock: l.fake, Dispatch: func(fn func()) { l.queue <- fn },
		Device:      model.DeviceInfo{Platform: "tvos", Name: "Apple TV", AppVersion: "1.0.0", InstallID: "i-1"},
		OnState:     h.OnState,
		OnConfirmed: func(s model.Session) { h.OnSignedIn(s.User.Email) },
		OnError:     h.OnError,
	})
	l.tv.Start(context.Background())

	return liveTV{l.tv}
}

// pump runs the next function dispatched to the UI thread, waiting for it.
func (l *liveServices) pump(t *testing.T) {
	t.Helper()
	select {
	case fn := <-l.queue:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("the pairing dispatched nothing")
	}
}

// liveTV is *pairing.TV as an env.Pairing.
type liveTV struct{ tv *pairing.TV }

func (p liveTV) Stop()                          { p.tv.Stop() }
func (p liveTV) Remaining() time.Duration       { return p.tv.Remaining() }
func (p liveTV) Current() (pairing.State, bool) { return p.tv.Current() }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// openLive opens TV01 over the real pairing, then TV02 with Select on `w1`; the
// start's answer is run, so a code is on screen.
func openLive(t *testing.T) (*rig, *liveServices, *screens.Code) {
	t.Helper()
	l := newLive(t)
	r := open(t, l, env.RouteWelcome, false).armed()
	require.True(t, r.tt.Key(keys.KeyEnter, 0))
	r.tt.Elapse(tvtest.Frame)
	s, ok := r.sh.Screen().(*screens.Code)
	require.True(t, ok)
	t.Cleanup(func() {
		l.tv.Stop()
		l.tv.Wait()
	})
	l.pump(t) // POST /tv/pair/start
	r.tt.Elapse(tvtest.Frame)

	return r, l, s
}

// step lets d pass on both clocks, the pairing's and the view's: the fake
// clock fires the timer of the next poll, the tester fires the countdown's.
func (l *liveServices) step(r *rig, d time.Duration) {
	l.fake.Advance(d)
	r.tt.Elapse(d)
}

// TestCodeLiveShowsTheMockCode: TV02 over pairing.TV and the mock: the code of
// tv/pair-start.json, its QR address and 05:00.
func TestCodeLiveShowsTheMockCode(t *testing.T) {
	t.Parallel()
	_, l, s := openLive(t)
	assert.Equal(t, mockDisplay, s.CodeText())
	assert.Equal(t, "Mã hết hạn sau 05:00 · mã mới sẽ tự tạo", s.ExpiryText())
	assert.Equal(t, 1, l.spy.count("tv.pair.start"))
	require.NotNil(t, s.QR().Code())
	assert.Equal(t, 29, s.QR().Code().Size(), "version 3")
}

// TestCodePollsEveryThreeSeconds: no poll before 3 s, then one at 3, 6, 9 and 12
// s on the pairing's clock; the countdown goes with it; leaving ends the polls.
func TestCodePollsEveryThreeSeconds(t *testing.T) {
	t.Parallel()
	r, l, s := openLive(t)
	l.step(r, 2999*time.Millisecond)
	assert.Zero(t, l.spy.count("tv.pair.poll"))
	l.step(r, time.Millisecond)
	for i := 1; i <= 4; i++ {
		l.pump(t)
		assert.Equal(t, i, l.spy.count("tv.pair.poll"), "poll %d at %d s", i, 3*i)
		l.step(r, 3*time.Second)
	}
	l.pump(t) // the poll of 15 s
	assert.Equal(t, 5, l.spy.count("tv.pair.poll"))
	assert.Equal(t, "Mã hết hạn sau 04:45 · mã mới sẽ tự tạo", s.ExpiryText())
	assert.Equal(t, 1, l.spy.count("tv.pair.start"), "pending keeps the code")

	require.True(t, r.press(keys.KeyEscape)) // leave TV02
	l.step(r, 30*time.Second)
	assert.Equal(t, 5, l.spy.count("tv.pair.poll"), "no poll after leaving")
	assert.Empty(t, r.tt.Leaks())
}

// TestCodeLiveConfirmed: the mock answering `confirmed` is TV04 with the toast
// of the account's email.
func TestCodeLiveConfirmed(t *testing.T) {
	t.Parallel()
	r, l, _ := openLive(t)
	require.NoError(t, l.mock.SetScenario("tv.pair.poll", "confirmed"))
	l.step(r, 3*time.Second)
	l.pump(t)
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
	assert.Zero(t, r.sh.Stack().Depth())
	assert.Equal(t, "Đã đăng nhập bằng thaovy.hlv@gmail.com", r.sh.ToastHost().Message())
	l.step(r, time.Minute)
	assert.Equal(t, 1, l.spy.count("tv.pair.poll"), "confirmed ends the polls")
}

// TestCodeLiveExpires: the code running out (the 410 of the mock's `expired`
// scenario, or the clock reaching it) starts the pairing again and the screen
// shows the new code with a full countdown.
func TestCodeLiveExpires(t *testing.T) {
	t.Parallel()
	t.Run("410", func(t *testing.T) {
		t.Parallel()
		r, l, s := openLive(t)
		require.NoError(t, l.mock.SetScenario("tv.pair.poll", "expired"))
		l.step(r, 30*time.Second) // the poll of 3 s answers 410
		l.pump(t)                 // the 410: a new start is sent
		l.pump(t)                 // its answer: the new code
		assert.Equal(t, 1, l.spy.count("tv.pair.poll"))
		assert.Equal(t, 2, l.spy.count("tv.pair.start"), "a new code at once")
		r.tt.Elapse(tvtest.Frame)
		assert.Equal(t, mockDisplay, s.CodeText())
		assert.Equal(t, env.RouteCode, r.sh.Stack().Screen())
	})
	t.Run("clock", func(t *testing.T) {
		t.Parallel()
		r, l, s := openLive(t)
		l.step(r, 299*time.Second+500*time.Millisecond)
		l.pump(t) // the poll of 3 s: pending, the next is armed at the expiry
		assert.Equal(t, "Mã hết hạn sau 00:01 · mã mới sẽ tự tạo", s.ExpiryText())
		assert.Equal(t, 1, l.spy.count("tv.pair.start"))
		l.step(r, 500*time.Millisecond) // 300 s: the code has run out
		l.pump(t)                       // the new start's answer
		assert.Equal(t, 2, l.spy.count("tv.pair.start"), "at 300 s: start again")
		r.tt.Elapse(time.Second)
		assert.Equal(t, mockDisplay, s.CodeText())
		assert.Equal(t, "Mã hết hạn sau 05:00 · mã mới sẽ tự tạo", s.ExpiryText(), "the countdown starts over")
	})
}
