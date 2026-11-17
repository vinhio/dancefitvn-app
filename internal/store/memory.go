// Adapted from jplayer-tv/internal/store/memory.go at 781bee5.

package store

import (
	"fmt"
	"slices"
	"sync"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Memory is a Backend in memory, for tests; its exported fields inject
// failures. Set the exported fields before use, or from the goroutine that
// calls the methods.
type Memory struct {
	LoadErr error // returned by Load when set
	SaveErr error // returned by Save when set
	Saves   int   // successful saves

	mu   sync.Mutex
	data map[string][]byte
}

var _ Backend = (*Memory)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewMemory returns an empty Memory.
func NewMemory() *Memory { return &Memory{data: map[string][]byte{}} }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Load returns a copy of the bytes under key, or LoadErr, or an error
// wrapping core.ErrNotFound.
func (m *Memory) Load(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.LoadErr != nil {
		return nil, m.LoadErr
	}
	b, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("store: memory load %s: %w", key, core.ErrNotFound)
	}

	return slices.Clone(b), nil
}

// Save keeps a copy of data under key, or returns SaveErr.
func (m *Memory) Save(key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SaveErr != nil {
		return m.SaveErr
	}
	m.put(key, data)
	m.Saves++

	return nil
}

// Put writes raw bytes for tests; it ignores SaveErr and does not count.
func (m *Memory) Put(key string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.put(key, data)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

func (m *Memory) put(key string, data []byte) {
	if m.data == nil {
		m.data = map[string][]byte{}
	}
	m.data[key] = slices.Clone(data)
}
