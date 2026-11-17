package auth

import (
	"encoding/json/v2"
	"fmt"
	"sync"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/model"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Vault keeps the session between launches (05 §6): in production the app's
// secrets (SecretVault over sys.Secrets). Load fails wrapping
// core.ErrNotFound when nothing is kept. Its calls may block (the first
// write of a process on Android takes a second or two), so the Manager calls
// them off the UI thread's locks and never from Token.
type Vault interface {
	Load() (model.Session, error)
	Save(model.Session) error
	Clear() error
}

// SecretStore is the part of sys.Secrets the SecretVault needs. Get fails
// wrapping core.ErrNotFound when nothing is kept for the account; Delete of
// nothing is no error.
type SecretStore interface {
	Get(account string) ([]byte, error)
	Set(account string, value []byte) error
	Delete(account string) error
}

// SecretVault is a Vault on a SecretStore: one value, under one account
// (the install id, 05 §6), holding the session as JSON. It is safe for
// concurrent use as far as the store is.
type SecretVault struct {
	store   SecretStore
	account string
}

// MemoryVault is a Vault in memory, for tests; its exported fields inject
// failures. Set them before use.
type MemoryVault struct {
	LoadErr  error // returned by Load when set
	SaveErr  error // returned by Save when set
	ClearErr error // returned by Clear when set

	mu     sync.Mutex
	sess   model.Session
	has    bool
	saves  int
	clears int
}

var (
	_ Vault = (*SecretVault)(nil)
	_ Vault = (*MemoryVault)(nil)
)

// record is what the SecretVault writes.
type record struct {
	Version int           `json:"version"`
	Session model.Session `json:"session"`
}

const recordVersion = 1

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewSecretVault returns a Vault that keeps the session in store under
// account.
func NewSecretVault(store SecretStore, account string) *SecretVault {
	return &SecretVault{store: store, account: account}
}

// NewMemoryVault returns an empty MemoryVault.
func NewMemoryVault() *MemoryVault { return &MemoryVault{} }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Load reads the session. A value that does not decode is core.ErrInvalid.
func (v *SecretVault) Load() (model.Session, error) {
	raw, err := v.store.Get(v.account)
	if err != nil {
		return model.Session{}, fmt.Errorf("auth: vault load: %w", err)
	}
	var r record
	if err := json.Unmarshal(raw, &r); err != nil || r.Version != recordVersion {
		return model.Session{}, fmt.Errorf("auth: vault load: %w", core.ErrInvalid)
	}

	return r.Session, nil
}

// Save writes the session.
func (v *SecretVault) Save(s model.Session) error {
	raw, err := json.Marshal(record{Version: recordVersion, Session: s})
	if err != nil {
		return fmt.Errorf("auth: vault save: %w: %w", core.ErrInvalid, err)
	}
	if err := v.store.Set(v.account, raw); err != nil {
		return fmt.Errorf("auth: vault save: %w", err)
	}

	return nil
}

// Clear forgets the session.
func (v *SecretVault) Clear() error {
	if err := v.store.Delete(v.account); err != nil {
		return fmt.Errorf("auth: vault clear: %w", err)
	}

	return nil
}

// Load returns the session kept, LoadErr, or an error wrapping
// core.ErrNotFound.
func (v *MemoryVault) Load() (model.Session, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.LoadErr != nil {
		return model.Session{}, v.LoadErr
	}
	if !v.has {
		return model.Session{}, fmt.Errorf("auth: memory vault: %w", core.ErrNotFound)
	}

	return v.sess, nil
}

// Save keeps s, or returns SaveErr.
func (v *MemoryVault) Save(s model.Session) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.SaveErr != nil {
		return v.SaveErr
	}
	v.sess, v.has = s, true
	v.saves++

	return nil
}

// Clear forgets the session, or returns ClearErr (the session stays).
func (v *MemoryVault) Clear() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.ClearErr != nil {
		return v.ClearErr
	}
	v.sess, v.has = model.Session{}, false
	v.clears++

	return nil
}

// Put keeps s without counting and ignoring SaveErr (a test's setup).
func (v *MemoryVault) Put(s model.Session) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.sess, v.has = s, true
}

// Has reports whether a session is kept.
func (v *MemoryVault) Has() bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.has
}

// Saves is the number of successful saves.
func (v *MemoryVault) Saves() int {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.saves
}

// Clears is the number of successful clears.
func (v *MemoryVault) Clears() int {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.clears
}
