// Adapted from jplayer-tv/internal/store/mirror.go at 781bee5.

package store

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Slot names one of the two places a Mirror keeps a record.
type Slot int

// The slots.
const (
	SlotUnknown Slot = iota
	SlotPrimary
	SlotCopy
)

// LoadInfo tells how the last Mirror.Load went.
type LoadInfo struct {
	From       Slot   // which copy Load returned; SlotUnknown when none
	Seq        uint64 // the sequence number of the record returned
	PrimaryErr error  // why the primary copy was not used; nil when it was fine
	CopyErr    error  // why the copy was not used; nil when it was fine
}

// MirrorOption configures a Mirror.
type MirrorOption func(*Mirror)

// Mirror keeps every key in two Backends (jPlayer TV DT10): a primary
// (NSUserDefaults on tvOS) and a copy (a Caches file). Each record is an envelope
// {seq, savedAt, data}; the copy with the higher seq wins, and a corrupt or
// missing copy gives way to the other. Mirror is itself a Backend and a
// Syncer.
type Mirror struct {
	primary, copy Backend
	now           func() time.Time
	onSlotError   func(Slot, error)

	mu   sync.Mutex
	seqs map[string]uint64 // highest seq seen or written, by key
	last LoadInfo
}

var (
	_ Backend = (*Mirror)(nil)
	_ Syncer  = (*Mirror)(nil)
)

// envelope is what each slot holds.
type envelope struct {
	Seq     uint64         `json:"seq"`
	SavedAt time.Time      `json:"savedAt"`
	Data    jsontext.Value `json:"data"`
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// WithNow sets the source of the savedAt stamp; the default is time.Now.
func WithNow(fn func() time.Time) MirrorOption {
	return func(m *Mirror) {
		if fn != nil {
			m.now = fn
		}
	}
}

// WithSlotError sets who is told when one slot fails while the other
// succeeds.
func WithSlotError(fn func(Slot, error)) MirrorOption {
	return func(m *Mirror) { m.onSlotError = fn }
}

// NewMirror returns a Mirror over primary and copy.
func NewMirror(primary, copy Backend, opts ...MirrorOption) *Mirror {
	m := &Mirror{primary: primary, copy: copy, now: time.Now, seqs: map[string]uint64{}}
	for _, o := range opts {
		o(m)
	}

	return m
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Load reads both slots and returns the data of the valid one with the higher
// seq (the primary on a tie). With no valid slot it fails wrapping
// core.ErrNotFound when both slots are missing, else core.ErrInvalid.
func (m *Mirror) Load(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, pErr := readSlot(m.primary, key)
	c, cErr := readSlot(m.copy, key)
	info := LoadInfo{PrimaryErr: pErr, CopyErr: cErr}
	for _, e := range []envelope{p, c} {
		m.seqs[key] = max(m.seqs[key], e.Seq)
	}
	var win envelope
	switch {
	case pErr == nil && (cErr != nil || p.Seq >= c.Seq):
		win, info.From = p, SlotPrimary
	case cErr == nil:
		win, info.From = c, SlotCopy
	default:
		m.last = info
		if errors.Is(pErr, core.ErrNotFound) && errors.Is(cErr, core.ErrNotFound) {
			return nil, fmt.Errorf("store: load %s: %w", key, core.ErrNotFound)
		}

		return nil, fmt.Errorf("store: load %s: %w: primary: %v; copy: %v", key, core.ErrInvalid, pErr, cErr)
	}
	info.Seq = win.Seq
	m.last = info

	return slices.Clone(win.Data), nil
}

// Save wraps data (valid JSON) in an envelope with the next seq and writes
// it to the primary, then to the copy. It succeeds when at least one slot
// was written; the other slot's failure goes to WithSlotError.
func (m *Mirror) Save(key string, data []byte) error {
	if !jsontext.Value(data).IsValid() {
		return fmt.Errorf("store: save %s: data is not JSON: %w", key, core.ErrInvalid)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, known := m.seqs[key]; !known {
		for _, b := range []Backend{m.primary, m.copy} {
			if e, err := readSlot(b, key); err == nil {
				m.seqs[key] = max(m.seqs[key], e.Seq)
			}
		}
		m.seqs[key] += 0 // marks the key as scanned
	}
	seq := m.seqs[key] + 1
	m.seqs[key] = seq
	raw, err := json.Marshal(envelope{Seq: seq, SavedAt: m.now().UTC(), Data: data})
	if err != nil {
		return fmt.Errorf("store: save %s: %w: %w", key, core.ErrInvalid, err)
	}
	pErr := m.primary.Save(key, raw)
	cErr := m.copy.Save(key, raw)
	switch {
	case pErr != nil && cErr != nil:
		return fmt.Errorf("store: save %s: %w", key, errors.Join(pErr, cErr))
	case pErr != nil:
		m.tell(SlotPrimary, pErr)
	case cErr != nil:
		m.tell(SlotCopy, cErr)
	default:
	}

	return nil
}

// Sync calls Sync on each slot that is a Syncer, and joins the errors.
func (m *Mirror) Sync() error {
	var errs []error
	for _, b := range []Backend{m.primary, m.copy} {
		if s, ok := b.(Syncer); ok {
			errs = append(errs, s.Sync())
		}
	}

	return errors.Join(errs...)
}

// LastLoad returns how the last Load went.
func (m *Mirror) LastLoad() LoadInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.last
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// tell reports a failed slot while the mutex is held; the handler must not
// call the Mirror.
func (m *Mirror) tell(s Slot, err error) {
	if m.onSlotError != nil {
		m.onSlotError(s, err)
	}
}

// readSlot loads key from b and checks the envelope: seq of at least 1 and
// data that is JSON. A missing key wraps core.ErrNotFound (whatever b
// returned); everything else wraps core.ErrInvalid or b's own error.
func readSlot(b Backend, key string) (envelope, error) {
	raw, err := b.Load(key)
	if err != nil {
		return envelope{}, err
	}
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return envelope{}, fmt.Errorf("store: envelope of %s: %w: %w", key, core.ErrInvalid, err)
	}
	switch {
	case e.Seq < 1:
		return envelope{}, fmt.Errorf("store: envelope of %s: seq %d: %w", key, e.Seq, core.ErrInvalid)
	case !e.Data.IsValid():
		return envelope{}, fmt.Errorf("store: envelope of %s: data is not JSON: %w", key, core.ErrInvalid)
	default:
		return e, nil
	}
}
