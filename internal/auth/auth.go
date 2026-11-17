package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// ErrSignedOut is the session the server no longer accepts. It is
// api.ErrSignedOut: one sentinel for the client and for auth.
var ErrSignedOut = api.ErrSignedOut

// Remote is the part of *api.Client the Manager calls.
type Remote interface {
	Login(ctx context.Context, email, password string, dev model.DeviceInfo) (model.Session, error)
	LoginReplacing(ctx context.Context, email, password string, dev model.DeviceInfo, replaceID string) (model.Session, error)
	Refresh(ctx context.Context, refreshToken string) (model.Tokens, error)
	Logout(ctx context.Context) error
	LogoutTV(ctx context.Context) error
}

var _ Remote = (*api.Client)(nil)

// Options configures New.
type Options struct {
	// Vault keeps the session between launches; required.
	Vault Vault
	// Remote is the API; it may be set later with Bind, because the api.Client
	// needs the Manager as its TokenSource first.
	Remote Remote
	// Device describes this device in a login; the fields left empty are the
	// client's own.
	Device model.DeviceInfo
	// TV makes Logout call tv.logout instead of auth.logout.
	TV bool
	// ClearCache empties the API cache (store.Cache.Clear). The cache keys do
	// not carry the user, so the Manager calls it on every sign-out and when
	// a different user signs in. Nil for none.
	ClearCache func()
	// OnSignedOut is called when the session ends without the user asking:
	// the refresh token was refused, or the owner called SignOut/Handle. It
	// runs once per session, on the goroutine that found out, with no lock
	// held; hop to the UI thread (app.Enqueue) before touching the UI. The
	// reason wraps ErrSignedOut. Logout does not call it. Nil for none.
	OnSignedOut func(reason error)
	// OnVaultError is told of a Vault failure that did not stop the
	// operation: "load", "save" or "clear" and the error. It never gets the
	// session. Nil for none.
	OnVaultError func(op string, err error)
}

// Manager holds the session: it logs in, restores it at launch, renews the
// access token on 401 token_expired (api.TokenSource) and signs out.
//
// It is safe for concurrent use. Restore, Login, Adopt, Refresh and Logout
// call the network or the Vault, and the Vault's first write can take a
// second or two on Android: call them from a goroutine, not the UI thread.
// Token never blocks and never calls the Remote, as api.Client calls it
// with its own lock held.
type Manager struct {
	opts Options

	// vaultMu serializes the Vault's calls and the state changes that go with
	// them; the I/O of the Vault happens under it, never under mu.
	vaultMu sync.Mutex

	mu       sync.Mutex // guards the fields below; held briefly, never over I/O
	remote   Remote
	sess     model.Session
	has      bool
	gen      uint64 // bumped by every sign-in and sign-out
	inflight *flight
}

var _ api.TokenSource = (*Manager)(nil)

// flight is one refresh in progress.
type flight struct {
	done chan struct{}
	tok  string
	err  error
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// New returns a signed-out Manager; Restore loads the session of the Vault.
// It fails with core.ErrInvalid without a Vault.
func New(o Options) (*Manager, error) {
	if o.Vault == nil {
		return nil, fmt.Errorf("auth: vault required: %w", core.ErrInvalid)
	}

	return &Manager{opts: o, remote: o.Remote}, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Bind sets the Remote (the api.Client built with this Manager as its
// TokenSource). Call it before Restore or Login.
func (m *Manager) Bind(r Remote) {
	m.mu.Lock()
	m.remote = r
	m.mu.Unlock()
}

// Restore loads the session of the Vault at launch. It reports false when
// none is kept; a session that cannot be read is cleared from the Vault and
// reported false too.
func (m *Manager) Restore() (model.Session, bool) {
	m.vaultMu.Lock()
	defer m.vaultMu.Unlock()

	s, err := m.opts.Vault.Load()
	switch {
	case errors.Is(err, core.ErrNotFound):
		return model.Session{}, false
	case err != nil:
		m.vaultErr("load", err)
		if cerr := m.opts.Vault.Clear(); cerr != nil {
			m.vaultErr("clear", cerr)
		}

		return model.Session{}, false
	case s.AccessToken == "" || s.RefreshToken == "":
		if cerr := m.opts.Vault.Clear(); cerr != nil {
			m.vaultErr("clear", cerr)
		}

		return model.Session{}, false
	default:
	}
	m.mu.Lock()
	m.sess, m.has = s, true
	m.gen++
	m.mu.Unlock()

	return s, true
}

// Session returns the current session, false when signed out.
func (m *Manager) Session() (model.Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.sess, m.has
}

// Token implements api.TokenSource: the access token, "" when signed out.
func (m *Manager) Token() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.sess.AccessToken
}

// Login signs in. A failure is the *api.Error of the server (invalid_credentials,
// account_locked, device_limit with Body.Devices ...) or wraps api.ErrOffline;
// errors.As and errors.Is see through the wrapping. On success the session
// is kept in the Vault, and the API cache is cleared when another user was
// signed in before (or one the Manager does not know).
func (m *Manager) Login(ctx context.Context, email, password string) (model.Session, error) {
	return m.LoginReplacing(ctx, email, password, "")
}

// LoginReplacing is Login for an account at its limit of devices: replaceID is
// the id of one of the devices of the device_limit error (Body.Devices), which
// the server revokes after it verified the password, then it completes the
// login. An empty replaceID is Login. A 400 validation_failed with
// Body.Fields["replace_device_id"] means the id does not fit.
func (m *Manager) LoginReplacing(ctx context.Context, email, password, replaceID string) (model.Session, error) {
	m.mu.Lock()
	r := m.remote
	m.mu.Unlock()
	if r == nil {
		return model.Session{}, fmt.Errorf("auth: login: no remote: %w", core.ErrInvalid)
	}
	s, err := r.LoginReplacing(ctx, email, password, m.opts.Device, replaceID)
	if err != nil {
		return model.Session{}, fmt.Errorf("auth: login: %w", err)
	}

	return s, m.install(s, "login")
}

// Adopt makes s the session: a TV pairing was confirmed (api.PairPoll.Session).
func (m *Manager) Adopt(s model.Session) error { return m.install(s, "adopt") }

// Refresh implements api.TokenSource. failed is the access token the server
// refused. When the session already holds another token the new one is
// returned without a call; calls that fail with the same token share one
// request to the server (single-flight; the leader's ctx governs it).
//
// A refresh token the server refuses (401) signs out: the Vault and the
// cache are cleared, OnSignedOut is called, and the error wraps
// ErrSignedOut. A network failure (api.ErrOffline), a 5xx, or a canceled ctx
// keeps the session and returns the error. The new token pair is saved to
// the Vault; both tokens rotate.
func (m *Manager) Refresh(ctx context.Context, failed string) (string, error) {
	m.mu.Lock()
	if !m.has {
		m.mu.Unlock()

		return "", fmt.Errorf("auth: refresh: %w", ErrSignedOut)
	}
	if cur := m.sess.AccessToken; cur != failed {
		m.mu.Unlock()

		return cur, nil
	}
	if f := m.inflight; f != nil {
		m.mu.Unlock()
		select {
		case <-f.done:
			return f.tok, f.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	f := &flight{done: make(chan struct{})}
	m.inflight = f
	gen, rt, r := m.gen, m.sess.RefreshToken, m.remote
	m.mu.Unlock()

	f.tok, f.err = m.renew(ctx, r, gen, rt)

	m.mu.Lock()
	if m.inflight == f {
		m.inflight = nil
	}
	m.mu.Unlock()
	close(f.done)

	return f.tok, f.err
}

// Logout ends the session: it tells the server (auth.logout, or tv.logout
// for a TV) as far as it can, ignoring the failure, then clears the Vault
// and the API cache. Downloaded files stay (05 §6). The error is only the
// Vault's: when it cannot forget the session, the user would be signed in
// again at the next launch. OnSignedOut is not called.
func (m *Manager) Logout(ctx context.Context) error {
	m.mu.Lock()
	has, r := m.has, m.remote
	m.mu.Unlock()
	if has && r != nil {
		if m.opts.TV {
			_ = r.LogoutTV(ctx)
		} else {
			_ = r.Logout(ctx)
		}
	}

	return m.end(nil, 0, false)
}

// SignOut ends the session locally, as the server's refusal does: the Vault
// and the cache are cleared and OnSignedOut(reason) is called when a session
// was there. A nil reason is reported as ErrSignedOut.
func (m *Manager) SignOut(reason error) {
	if reason == nil {
		reason = ErrSignedOut
	}
	_ = m.end(reason, 0, false)
}

// Handle signs out when err is the server refusing the session
// (errors.Is(err, ErrSignedOut)): the second 401 after a refresh. Call it
// on every error of an API call; it reports whether it was one.
func (m *Manager) Handle(err error) bool {
	if !errors.Is(err, ErrSignedOut) {
		return false
	}
	m.SignOut(err)

	return true
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// renew trades the refresh token rt (of generation gen) for a new pair.
func (m *Manager) renew(ctx context.Context, r Remote, gen uint64, rt string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("auth: refresh: no remote: %w", core.ErrInvalid)
	}
	t, err := r.Refresh(ctx, rt)
	if err != nil {
		var ae *api.Error
		if errors.As(err, &ae) && ae.Status == http.StatusUnauthorized {
			reason := fmt.Errorf("auth: refresh: %w: %w", ErrSignedOut, err)
			_ = m.end(reason, gen, true)

			return "", reason
		}

		return "", fmt.Errorf("auth: refresh: %w", err)
	}
	if t.AccessToken == "" {
		return "", fmt.Errorf("auth: refresh: %w: no access token", api.ErrServer)
	}

	m.vaultMu.Lock()
	defer m.vaultMu.Unlock()
	m.mu.Lock()
	if m.gen != gen || !m.has {
		m.mu.Unlock()

		return "", fmt.Errorf("auth: refresh: %w", ErrSignedOut)
	}
	if t.RefreshToken == "" {
		t.RefreshToken = m.sess.RefreshToken
	}
	m.sess.Tokens = t
	s := m.sess
	m.mu.Unlock()
	if err := m.opts.Vault.Save(s); err != nil {
		m.vaultErr("save", err)
	}

	return t.AccessToken, nil
}

// install makes s the session and saves it. When the user differs from the
// one before, the cache is cleared.
func (m *Manager) install(s model.Session, op string) error {
	if s.AccessToken == "" || s.RefreshToken == "" {
		return fmt.Errorf("auth: %s: %w: session without tokens", op, core.ErrInvalid)
	}
	m.vaultMu.Lock()
	defer m.vaultMu.Unlock()

	m.mu.Lock()
	prev, had := m.sess.User.ID, m.has
	m.sess, m.has = s, true
	m.gen++
	m.mu.Unlock()

	if !had || prev != s.User.ID {
		m.clearCache()
	}
	if err := m.opts.Vault.Save(s); err != nil {
		m.vaultErr("save", err)
	}

	return nil
}

// end signs out: state, Vault, cache. With check it does nothing unless the
// session is still generation gen (another sign-in since means the refusal
// was about an old session). A non-nil reason is told to OnSignedOut. The
// error is the Vault's clear failure.
func (m *Manager) end(reason error, gen uint64, check bool) error {
	m.vaultMu.Lock()
	m.mu.Lock()
	if (check && m.gen != gen) || (!m.has && reason != nil) {
		m.mu.Unlock()
		m.vaultMu.Unlock()

		return nil
	}
	was := m.has
	m.sess, m.has = model.Session{}, false
	m.gen++
	m.mu.Unlock()

	err := m.opts.Vault.Clear()
	if err != nil {
		m.vaultErr("clear", err)
		err = fmt.Errorf("auth: sign out: %w", err)
	}
	// A session the server ended holds a refresh token it refuses, and the
	// biometric copy holds the same one (Q-P3.2b-1): it goes too. A Logout
	// the user asked for keeps it (K13).
	if slot, ok := m.opts.Vault.(BioSlot); ok && was && reason != nil {
		if cerr := slot.ClearBio(); cerr != nil {
			m.vaultErr("clear", cerr)
		}
	}
	m.clearCache()
	m.vaultMu.Unlock()

	if was && reason != nil && m.opts.OnSignedOut != nil {
		m.opts.OnSignedOut(reason)
	}

	return err
}

func (m *Manager) clearCache() {
	if m.opts.ClearCache != nil {
		m.opts.ClearCache()
	}
}

func (m *Manager) vaultErr(op string, err error) {
	if m.opts.OnVaultError != nil {
		m.opts.OnVaultError(op, err)
	}
}
