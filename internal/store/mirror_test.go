// Adapted from jplayer-tv/internal/store/mirror_test.go at 781bee5
// (t0 and syncMem from its store_test.go; the round trip goes through
// PrefsStore, this app's user of a Backend, instead of jPlayer's Store).

package store_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/store"
)

const k = "settings"

func env(seq int, data string) []byte {
	b, _ := json.Marshal(map[string]any{"seq": seq, "savedAt": t0, "data": jsontext.Value(data)})

	return b
}

func slots(p, c []byte) (*store.Memory, *store.Memory) {
	pm, cm := store.NewMemory(), store.NewMemory()
	if p != nil {
		pm.Put(k, p)
	}
	if c != nil {
		cm.Put(k, c)
	}

	return pm, cm
}

func TestMirrorPicksHigherSeq(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		p, c     []byte
		want     string
		wantFrom store.Slot
		wantSeq  uint64
	}{
		{"primary newer", env(5, `{"a":1}`), env(4, `{"a":2}`), `{"a":1}`, store.SlotPrimary, 5},
		{"copy newer", env(4, `{"a":1}`), env(5, `{"a":2}`), `{"a":2}`, store.SlotCopy, 5},
		{"equal goes to primary", env(3, `{"a":1}`), env(3, `{"a":2}`), `{"a":1}`, store.SlotPrimary, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := store.NewMirror(slots(tc.p, tc.c))
			got, err := m.Load(k)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
			info := m.LastLoad()
			assert.Equal(t, tc.wantFrom, info.From)
			assert.Equal(t, tc.wantSeq, info.Seq)
			assert.NoError(t, info.PrimaryErr)
			assert.NoError(t, info.CopyErr)
		})
	}
}

func TestMirrorFallsBackWhenOneCopyIsBad(t *testing.T) {
	t.Parallel()

	good := env(2, `{"ok":true}`)
	tests := []struct {
		name string
		bad  []byte // nil: slot missing
	}{
		{"garbage bytes", []byte("not json")},
		{"envelope without data", []byte(`{"seq":9,"savedAt":"2026-01-01T00:00:00Z"}`)},
		{"data not json", []byte(`{"seq":9,"data":nope}`)},
		{"seq zero", env(0, `{"bad":true}`)},
		{"seq missing", []byte(`{"data":{"bad":true}}`)},
		{"empty bytes", []byte{}},
		{"slot missing", nil},
	}
	for _, tc := range tests {
		for _, badIsPrimary := range []bool{true, false} {
			name := tc.name + " in copy"
			if badIsPrimary {
				name = tc.name + " in primary"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				p, c := slots(tc.bad, good)
				want := store.SlotCopy
				if !badIsPrimary {
					p, c = slots(good, tc.bad)
					want = store.SlotPrimary
				}
				m := store.NewMirror(p, c)
				got, err := m.Load(k)
				require.NoError(t, err)
				assert.JSONEq(t, `{"ok":true}`, string(got))
				assert.Equal(t, want, m.LastLoad().From)
			})
		}
	}

	t.Run("slot LoadErr", func(t *testing.T) {
		t.Parallel()

		p, c := slots(env(9, `{"x":1}`), good)
		p.LoadErr = errors.New("locked")
		m := store.NewMirror(p, c)
		got, err := m.Load(k)
		require.NoError(t, err)
		assert.JSONEq(t, `{"ok":true}`, string(got))
		assert.ErrorContains(t, m.LastLoad().PrimaryErr, "locked")
	})
}

func TestMirrorBothMissingIsNotFound(t *testing.T) {
	t.Parallel()

	m := store.NewMirror(store.NewMemory(), store.NewMemory())
	_, err := m.Load(k)
	assert.ErrorIs(t, err, core.ErrNotFound)
	assert.Equal(t, store.SlotUnknown, m.LastLoad().From)
}

func TestMirrorBothBadIsInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		p, c []byte
	}{
		{"both garbage", []byte("x"), []byte("y")},
		{"garbage and missing", []byte("x"), nil},
		{"missing and garbage", nil, []byte("y")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := store.NewMirror(slots(tc.p, tc.c)).Load(k)
			require.Error(t, err)
			assert.ErrorIs(t, err, core.ErrInvalid)
			assert.NotErrorIs(t, err, core.ErrNotFound)
		})
	}
}

func TestMirrorSeqContinuesFromHighestSeen(t *testing.T) {
	t.Parallel()

	t.Run("after Load", func(t *testing.T) {
		t.Parallel()

		p, c := slots(env(7, `{}`), env(4, `{}`))
		m := store.NewMirror(p, c)
		_, err := m.Load(k)
		require.NoError(t, err)
		require.NoError(t, m.Save(k, []byte(`{"n":1}`)))
		assert.Equal(t, 8, seqOf(t, p))
		assert.Equal(t, 8, seqOf(t, c))
		require.NoError(t, m.Save(k, []byte(`{"n":2}`)))
		assert.Equal(t, 9, seqOf(t, p))
	})
	t.Run("Save first scans both slots", func(t *testing.T) {
		t.Parallel()

		p, c := slots(env(7, `{}`), env(11, `{}`))
		m := store.NewMirror(p, c)
		require.NoError(t, m.Save(k, []byte(`{"n":1}`)))
		assert.Equal(t, 12, seqOf(t, p))
	})
	t.Run("a corrupt slot counts for nothing", func(t *testing.T) {
		t.Parallel()

		p, c := slots([]byte("junk"), env(3, `{}`))
		require.NoError(t, store.NewMirror(p, c).Save(k, []byte(`{}`)))
		assert.Equal(t, 4, seqOf(t, p))
	})
}

func seqOf(t *testing.T, m *store.Memory) int {
	t.Helper()
	raw, err := m.Load(k)
	require.NoError(t, err)
	var e struct {
		Seq int `json:"seq"`
	}
	require.NoError(t, json.Unmarshal(raw, &e))

	return e.Seq
}

func TestMirrorSaveSucceedsWhenOneSlotFails(t *testing.T) {
	t.Parallel()

	for _, failPrimary := range []bool{true, false} {
		p, c := store.NewMemory(), store.NewMemory()
		var gotSlot store.Slot
		var gotErr error
		m := store.NewMirror(p, c, store.WithSlotError(func(s store.Slot, err error) { gotSlot, gotErr = s, err }))
		wantSlot, failing := store.SlotCopy, c
		if failPrimary {
			wantSlot, failing = store.SlotPrimary, p
		}
		failing.SaveErr = errors.New("nope")
		require.NoError(t, m.Save(k, []byte(`{"a":1}`)))
		assert.Equal(t, wantSlot, gotSlot)
		assert.ErrorContains(t, gotErr, "nope")

		// seq advanced although one slot failed
		failing.SaveErr = nil
		require.NoError(t, m.Save(k, []byte(`{"a":2}`)))
		assert.Equal(t, 2, seqOf(t, p))
		assert.Equal(t, 2, seqOf(t, c))
	}
}

func TestMirrorSaveFailsWhenBothFail(t *testing.T) {
	t.Parallel()

	p, c := store.NewMemory(), store.NewMemory()
	p.SaveErr, c.SaveErr = errors.New("one"), errors.New("two")
	err := store.NewMirror(p, c).Save(k, []byte(`{}`))
	require.Error(t, err)
	assert.ErrorContains(t, err, "store: save settings")
	assert.ErrorContains(t, err, "one")
	assert.ErrorContains(t, err, "two")
}

func TestMirrorRejectsNonJSON(t *testing.T) {
	t.Parallel()

	p, c := store.NewMemory(), store.NewMemory()
	m := store.NewMirror(p, c)
	for _, in := range []string{"", "nope", `{"a":`} {
		assert.ErrorIs(t, m.Save(k, []byte(in)), core.ErrInvalid, in)
	}
	assert.Zero(t, p.Saves+c.Saves)
}

func TestMirrorSyncCallsSyncers(t *testing.T) {
	t.Parallel()

	p := &syncMem{Memory: store.NewMemory(), syncErr: errors.New("p-sync")}
	c := store.NewMemory() // not a Syncer
	m := store.NewMirror(p, c)
	assert.ErrorContains(t, m.Sync(), "p-sync")
	assert.Equal(t, 1, p.syncs)

	c2 := &syncMem{Memory: store.NewMemory()}
	p.syncErr = nil
	m = store.NewMirror(p, c2)
	assert.NoError(t, m.Sync())
	assert.Equal(t, 1, c2.syncs)
	assert.Equal(t, 2, p.syncs)
}

func TestMirrorEnvelopeShape(t *testing.T) {
	t.Parallel()

	p, c := store.NewMemory(), store.NewMemory()
	at := time.Date(2026, 5, 6, 7, 8, 9, 123, time.FixedZone("x", 7*3600))
	m := store.NewMirror(p, c, store.WithNow(func() time.Time { return at }))
	require.NoError(t, m.Save(k, []byte(`{"a": 1}`)))
	rp, _ := p.Load(k)
	rc, _ := c.Load(k)
	assert.Equal(t, rp, rc, "both slots hold the same bytes")

	var raw map[string]jsonRaw
	require.NoError(t, json.Unmarshal(rp, &raw))
	assert.Len(t, raw, 3)
	assert.Equal(t, "1", string(raw["seq"]))
	assert.Equal(t, `"2026-05-06T00:08:09.000000123Z"`, string(raw["savedAt"]))
	assert.JSONEq(t, `{"a":1}`, string(raw["data"]))
}

type jsonRaw = jsontext.Value

func TestMirrorLastLoad(t *testing.T) {
	t.Parallel()

	p, c := slots([]byte("bad"), env(6, `{}`))
	m := store.NewMirror(p, c)
	assert.Equal(t, store.LoadInfo{}, m.LastLoad())
	_, err := m.Load(k)
	require.NoError(t, err)
	info := m.LastLoad()
	assert.Equal(t, store.SlotCopy, info.From)
	assert.Equal(t, uint64(6), info.Seq)
	assert.ErrorIs(t, info.PrimaryErr, core.ErrInvalid)
	assert.NoError(t, info.CopyErr)
}

func TestMirrorRoundTripThroughPrefs(t *testing.T) {
	t.Parallel()

	p, c := store.NewMemory(), store.NewMemory()
	ps := store.OpenPrefs(store.NewMirror(p, c), true)
	require.NoError(t, ps.Update(func(pr *store.Prefs) { pr.Quality = store.QualityP720 }))
	p.Put(store.PrefsKey, []byte("garbage")) // primary corrupted: the copy still serves
	got := store.OpenPrefs(store.NewMirror(p, c), true)
	require.NoError(t, got.LoadErr())
	assert.Equal(t, store.QualityP720, got.Get().Quality)
}

// TestMirrorRestoresAfterCachesPurge is the tvOS case of P7.5d: the copy
// is a Dir in Library/Caches, which the system empties; the primary
// (NSUserDefaults) still serves, and the next save writes the file again.
func TestMirrorRestoresAfterCachesPurge(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "Caches", "state")
	p := store.NewMemory()
	m := store.NewMirror(p, store.NewDir(dir))
	require.NoError(t, m.Save(store.PrefsKey, []byte(`{"auto_next":false}`)))
	require.NoError(t, os.RemoveAll(filepath.Dir(dir)))

	m = store.NewMirror(p, store.NewDir(dir)) // the next launch
	got, err := m.Load(store.PrefsKey)
	require.NoError(t, err)
	assert.JSONEq(t, `{"auto_next":false}`, string(got))
	info := m.LastLoad()
	assert.Equal(t, store.SlotPrimary, info.From)
	require.ErrorIs(t, info.CopyErr, core.ErrNotFound)

	require.NoError(t, m.Save(store.PrefsKey, []byte(`{"auto_next":true}`)))
	raw, err := os.ReadFile(filepath.Join(dir, store.PrefsKey+".json"))
	require.NoError(t, err, "the save writes the purged copy again")
	var e struct {
		Seq uint64 `json:"seq"`
	}
	require.NoError(t, json.Unmarshal(raw, &e))
	assert.Equal(t, uint64(2), e.Seq, "the seq goes on from the primary's")
}

var t0 = time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

type syncMem struct {
	*store.Memory
	syncs   int
	syncErr error
}

func (m *syncMem) Sync() error { m.syncs++; return m.syncErr }
