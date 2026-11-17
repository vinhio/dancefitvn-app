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

// BioCheck asks the user to prove themselves with the device's biometry
// (sys.Biometrics.Authenticate with the app's prompt, 03 §3 "Đăng nhập
// DanceFITVN") and blocks until the system's sheet answers: nil when it
// recognized them. Its errors pass through BioLogin unchanged in kind:
// a dismissed sheet wraps context.Canceled.
type BioCheck func(ctx context.Context) error

// BioSlot is a Vault with a second copy of the session for the
// biometric sign-in (K13): BioVault.
type BioSlot interface {
	Vault
	// LoadBio returns the copy; the error wraps core.ErrNotFound when
	// none is kept.
	LoadBio() (model.Session, error)
	// ClearBio forgets the copy.
	ClearBio() error
	// HasBio tells whether a copy is kept, from memory after the first
	// call.
	HasBio() bool
}

// The errors of BioLogin of its own.
var (
	// ErrNoBio is no copy kept for the biometric sign-in: the user has
	// not signed in with the password on this device, or the copy was
	// cleared.
	ErrNoBio = fmt.Errorf("auth: no biometric sign-in kept: %w", core.ErrNotFound)
	// ErrBioExpired is the copy's refresh token refused by the server
	// (refresh_invalid: revoked, expired, logout revoked it, Q-P3-5): the
	// copy is cleared and the login screen says the session expired.
	ErrBioExpired = fmt.Errorf("auth: the biometric sign-in expired: %w", ErrSignedOut)
)

// BioVault is a Vault that keeps, besides the session (main), a copy
// for the biometric sign-in (slot, K13): every Save writes both, so the
// copy follows each login and each rotation of the refresh token; Clear
// (Logout, a refused session) forgets the main one only, so after
// "Đăng xuất" the user signs in again with their face or finger. The
// copy holds the refresh token, the user and the device, never the
// access token. It is safe for concurrent use as far as its Vaults are.
type BioVault struct {
	main, slot Vault
	keep       func() bool

	mu    sync.Mutex
	known bool // has is what the slot holds
	has   bool
}

var _ BioSlot = (*BioVault)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewBioVault returns a BioVault over main (the session) and slot (the
// copy: in production a SecretVault of its own account). keep tells, at
// each Save, whether the device can sign in with its biometry
// (sys.Biometrics.Available); when it cannot, Save clears the copy
// instead of writing it. A nil keep always keeps.
func NewBioVault(main, slot Vault, keep func() bool) *BioVault {
	return &BioVault{main: main, slot: slot, keep: keep}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Load reads the main session.
func (v *BioVault) Load() (model.Session, error) { return v.main.Load() }

// Save writes the main session, then the copy (or clears it, keep
// saying no). A failure of the copy does not undo the main one; both are
// reported.
func (v *BioVault) Save(s model.Session) error {
	if err := v.main.Save(s); err != nil {
		return err
	}
	if v.keep != nil && !v.keep() {
		if err := v.ClearBio(); err != nil {
			return fmt.Errorf("auth: bio vault: %w", err)
		}

		return nil
	}
	err := v.slot.Save(bioCopy(s))
	v.mu.Lock()
	v.known, v.has = err == nil, err == nil
	v.mu.Unlock()
	if err != nil {
		return fmt.Errorf("auth: bio vault save: %w", err)
	}

	return nil
}

// Clear forgets the main session; the copy stays (K13).
func (v *BioVault) Clear() error { return v.main.Clear() }

// LoadBio reads the copy; a copy without refresh token counts as none.
func (v *BioVault) LoadBio() (model.Session, error) {
	s, err := v.slot.Load()
	if err == nil && s.RefreshToken == "" {
		err = fmt.Errorf("auth: bio vault: no refresh token: %w", core.ErrNotFound)
	}
	v.mu.Lock()
	switch {
	case err == nil:
		v.known, v.has = true, true
	case errors.Is(err, core.ErrNotFound):
		v.known, v.has = true, false
	default:
	}
	v.mu.Unlock()
	if err != nil {
		return model.Session{}, fmt.Errorf("auth: bio vault load: %w", err)
	}

	return s, nil
}

// ClearBio forgets the copy.
func (v *BioVault) ClearBio() error {
	err := v.slot.Clear()
	if err == nil {
		v.mu.Lock()
		v.known, v.has = true, false
		v.mu.Unlock()
	}

	return err
}

// HasBio tells whether a copy is kept: read once from the slot, then
// from memory (Save, LoadBio and ClearBio keep it current). Cheap enough
// for the UI thread after the first call, which reads the slot.
func (v *BioVault) HasBio() bool {
	v.mu.Lock()
	known, has := v.known, v.has
	v.mu.Unlock()
	if known {
		return has
	}
	_, err := v.LoadBio()

	return err == nil
}

// HasBio tells whether the Vault keeps a copy for the biometric sign-in
// (BioSlot.HasBio); false when it is no BioSlot. The login screen shows
// "Đăng nhập bằng Face ID / Vân tay" when this and the device's biometry
// say yes (K13).
func (m *Manager) HasBio() bool {
	slot, ok := m.opts.Vault.(BioSlot)

	return ok && slot.HasBio()
}

// BioLogin signs in with the copy of the Vault (a BioSlot) after the
// user proved themselves (03 §3 bioLogin): it reads the copy (ErrNoBio
// when none), runs check (the system's sheet; its error is returned,
// wrapped, so a dismissed sheet wraps context.Canceled), trades the
// copy's refresh token for new tokens (auth.refresh) and makes the
// result the session, saved to both copies. A refresh token the server
// refuses (401: refresh_invalid) clears the copy and returns an error
// wrapping ErrBioExpired (and so ErrSignedOut); a network failure or a
// 5xx keeps the copy. Call it from a goroutine: check blocks on the UI
// thread's sheet.
func (m *Manager) BioLogin(ctx context.Context, check BioCheck) (model.Session, error) {
	slot, ok := m.opts.Vault.(BioSlot)
	if !ok {
		return model.Session{}, fmt.Errorf("auth: bio login: %w", ErrNoBio)
	}
	m.mu.Lock()
	r := m.remote
	m.mu.Unlock()
	if r == nil {
		return model.Session{}, fmt.Errorf("auth: bio login: no remote: %w", core.ErrInvalid)
	}
	if check == nil {
		return model.Session{}, fmt.Errorf("auth: bio login: no check: %w", core.ErrInvalid)
	}
	m.vaultMu.Lock()
	kept, err := slot.LoadBio()
	m.vaultMu.Unlock()
	switch {
	case errors.Is(err, core.ErrNotFound):
		return model.Session{}, fmt.Errorf("auth: bio login: %w", ErrNoBio)
	case err != nil:
		m.vaultErr("load", err)

		return model.Session{}, fmt.Errorf("auth: bio login: %w", err)
	default:
	}
	if err := check(ctx); err != nil {
		return model.Session{}, fmt.Errorf("auth: bio login: %w", err)
	}
	t, err := r.Refresh(ctx, kept.RefreshToken)
	if err != nil {
		var ae *api.Error
		if errors.As(err, &ae) && ae.Status == http.StatusUnauthorized {
			m.vaultMu.Lock()
			if cerr := slot.ClearBio(); cerr != nil {
				m.vaultErr("clear", cerr)
			}
			m.vaultMu.Unlock()

			return model.Session{}, fmt.Errorf("auth: bio login: %w: %w", ErrBioExpired, err)
		}

		return model.Session{}, fmt.Errorf("auth: bio login: %w", err)
	}
	if t.AccessToken == "" {
		return model.Session{}, fmt.Errorf("auth: bio login: %w: no access token", api.ErrServer)
	}
	if t.RefreshToken == "" {
		t.RefreshToken = kept.RefreshToken
	}
	s := model.Session{Tokens: t, User: kept.User, Device: kept.Device}
	if err := m.install(s, "bio login"); err != nil {
		return model.Session{}, err
	}

	return s, nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// bioCopy is what the slot keeps of s: no access token.
func bioCopy(s model.Session) model.Session {
	return model.Session{
		Tokens: model.Tokens{
			TokenType:        s.TokenType,
			RefreshToken:     s.RefreshToken,
			RefreshExpiresIn: s.RefreshExpiresIn,
		},
		User:   s.User,
		Device: s.Device,
	}
}
