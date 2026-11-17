//go:build tvos

package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/model"
	tvenv "dancefitvn-app/internal/ui/tv/env"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The second account a phone pairs while the first one's logout runs.
const (
	secondEmail  = "second.coach@example.com"
	secondAccess = "at-second-session"
)

// holdTransport holds POST /tv/logout until released or until its context
// ends (as a slow server would), handing every other request to inner.
type holdTransport struct {
	inner   api.Transport
	entered chan struct{} // closed when a tv.logout arrived
	release chan struct{} // closed to let it through
	once    sync.Once
	mu      sync.Mutex
	logouts int
	cut     bool // the held tv.logout ended by its context
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestTVLogoutHeldThenPairedKeepsTheNewSession: TV12 left for TV01 and its
// logout runs on, held at POST /tv/logout; a phone confirms a new pairing
// meanwhile. The adoption cuts the logout short (its server call gives
// up), waits for its local sign-out, then installs the new session, which
// the old logout cannot clear: after the release the new session, its
// vault copy and the device document's email are there.
func TestTVLogoutHeldThenPairedKeepsTheNewSession(t *testing.T) {
	t.Parallel()
	secrets := &memSecrets{vals: map[string][]byte{}}
	r, ht := holdRig(t, secrets)
	r.adoptMock(t.Context())
	require.Equal(t, "thaovy.hlv@gmail.com", r.w.lastEmail())

	logoutDone := make(chan error, 1)
	go func() { logoutDone <- r.w.LogoutTV(context.Background()) }()
	select {
	case <-ht.entered:
	case <-time.After(driveLimit):
		t.Fatal("the logout never reached POST /tv/logout")
	}

	adoptDone := make(chan error, 1)
	go func() { adoptDone <- r.w.adopt(secondSession(t)) }()
	var logoutErr, adoptErr error
	gotLogout, gotAdopt := false, false
	r.drive("the logout cut short and the adoption", func() bool {
		select {
		case logoutErr = <-logoutDone:
			gotLogout = true
		case adoptErr = <-adoptDone:
			gotAdopt = true
		default:
		}

		return gotLogout && gotAdopt
	})
	close(ht.release)
	r.drive("the UI queue", func() bool { return true })

	require.NoError(t, logoutErr)
	require.NoError(t, adoptErr)
	assert.True(t, ht.wasCut(), "the held POST /tv/logout ended by the adoption, not by the release")
	assert.Equal(t, 1, ht.logoutCount())
	s, ok := r.w.mgr.Session()
	require.True(t, ok, "the new session is in place")
	assert.Equal(t, secondAccess, s.AccessToken)
	u, ok := r.w.SessionUser()
	require.True(t, ok)
	assert.Equal(t, secondEmail, u.Email)
	assert.True(t, secrets.holds(secondAccess), "the vault keeps the new session")
	assert.Equal(t, secondEmail, r.w.lastEmail(), "the device document keeps the new email")
}

// TestTVLogoutAskedBeforeAnAdoptionIsSkipped: a logout asked before a
// session was adopted, that only gets to run after, clears nothing and
// tells the server nothing: the user signed in again since.
func TestTVLogoutAskedBeforeAnAdoptionIsSkipped(t *testing.T) {
	t.Parallel()
	secrets := &memSecrets{vals: map[string][]byte{}}
	r, ht := holdRig(t, secrets)
	close(ht.release)
	r.adoptMock(t.Context())
	asked := r.w.adopts.Load()
	require.NoError(t, r.w.adopt(secondSession(t)))

	require.NoError(t, r.w.logoutAsked(context.Background(), asked))
	assert.Zero(t, ht.logoutCount(), "no POST /tv/logout")
	s, ok := r.w.mgr.Session()
	require.True(t, ok)
	assert.Equal(t, secondAccess, s.AccessToken)
	assert.Equal(t, secondEmail, r.w.lastEmail())
	assert.True(t, secrets.holds(secondAccess))
}

// TestTVCloseWaitsForTheLogout: at close a logout held at the server is
// waited for briefly, then cut short; its local sign-out is done when
// close's wait returns, so the next launch is signed out.
func TestTVCloseWaitsForTheLogout(t *testing.T) {
	t.Parallel()
	secrets := &memSecrets{vals: map[string][]byte{}}
	r, ht := holdRig(t, secrets)
	r.adoptMock(t.Context())
	logoutDone := make(chan error, 1)
	go func() { logoutDone <- r.w.LogoutTV(context.Background()) }()
	select {
	case <-ht.entered:
	case <-time.After(driveLimit):
		t.Fatal("the logout never reached POST /tv/logout")
	}

	began := time.Now()
	r.w.waitLogout(20*time.Millisecond, driveLimit)
	assert.Less(t, time.Since(began), driveLimit, "close does not wait out the server")
	select {
	case err := <-logoutDone:
		require.NoError(t, err)
	default:
		t.Fatal("the logout has not returned after close's wait")
	}
	assert.True(t, ht.wasCut())
	_, ok := r.w.mgr.Session()
	assert.False(t, ok, "signed out locally")
	assert.Empty(t, secrets.all(), "the vault is empty")
	assert.Empty(t, r.w.lastEmail())
	close(ht.release)

	// Without a logout under way it returns at once.
	began = time.Now()
	r.w.waitLogout(driveLimit, driveLimit)
	assert.Less(t, time.Since(began), time.Second)
}

// Do holds tv.logout, passes the rest.
func (h *holdTransport) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	if req.RouteID != "tv.logout" {
		return h.inner.Do(ctx, req)
	}
	h.mu.Lock()
	h.logouts++
	h.mu.Unlock()
	h.once.Do(func() { close(h.entered) })
	select {
	case <-h.release:
		return h.inner.Do(ctx, req)
	case <-ctx.Done():
		h.mu.Lock()
		h.cut = true
		h.mu.Unlock()

		return nil, ctx.Err()
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// holdRig is a signed-out TV on TV01 whose API is the mock behind a
// holdTransport.
func holdRig(t *testing.T, secrets *memSecrets) (*tvRig, *holdTransport) {
	t.Helper()
	r := newTVRig(t, secrets, "")
	ht := &holdTransport{entered: make(chan struct{}), release: make(chan struct{})}
	r.deps.API = func(bool) (apiPlan, error) {
		mt, err := mock.New(os.DirFS(mockDir), mock.Options{Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }})
		if err != nil {
			return apiPlan{}, err
		}
		t.Cleanup(func() { _ = mt.Close() })
		ht.inner = mt

		return apiPlan{mode: apiModeLive, live: ht, tags: true}, nil
	}
	r.mount("")
	r.drive("TV01", func() bool { return r.route() == tvenv.RouteWelcome })

	return r, ht
}

// secondSession is the mock's confirmed pairing as another account.
func secondSession(t *testing.T) model.Session {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(mockDir, "tv", "pair-poll.confirmed.json"))
	require.NoError(t, err)
	var p model.PairPoll
	require.NoError(t, jsonUnmarshal(raw, &p))
	s := p.Session()
	s.AccessToken, s.RefreshToken = secondAccess, "rt-second-session"
	s.User.ID, s.User.Email = "u_second", secondEmail

	return s
}

// logoutCount is how many tv.logout arrived.
func (h *holdTransport) logoutCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.logouts
}

// wasCut tells whether a held tv.logout ended by its context.
func (h *holdTransport) wasCut() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.cut
}

// holds tells whether a value kept contains b.
func (s *memSecrets) holds(b string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.vals {
		if bytes.Contains(v, []byte(b)) {
			return true
		}
	}

	return false
}

// all returns a copy of the values kept.
func (s *memSecrets) all() map[string][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string][]byte, len(s.vals))
	for k, v := range s.vals {
		out[k] = v
	}

	return out
}
