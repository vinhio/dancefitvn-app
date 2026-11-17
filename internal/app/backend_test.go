// Adapted from jplayer-tv/internal/app/backend_other_test.go at 781bee5.

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/progress"
	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestTVBackendOffTVOSKeepsTheFiles: off tvOS (the Mac, -tags tvos or not)
// there is no NSUserDefaults; the Mirror keeps the files alone, in the
// same envelopes as on the device, and says so once.
func TestTVBackendOffTVOSKeepsTheFiles(t *testing.T) {
	t.Parallel()

	if _, err := tvDefaults(); err == nil {
		t.Skip("NSUserDefaults is there on tvOS")
	}
	dir := filepath.Join(t.TempDir(), tvStateFolder)
	var log logLines
	b := tvBackend(dir, log.logf)
	require.Len(t, log.get(), 1)
	assert.Contains(t, log.get()[0], "no NSUserDefaults")
	assert.Contains(t, log.get()[0], core.ErrUnsupported.Error())

	require.NoError(t, b.Save(store.PrefsKey, []byte(`{"auto_next":false}`)))
	raw, err := os.ReadFile(filepath.Join(dir, store.PrefsKey+".json"))
	require.NoError(t, err, "the record is <state>/prefs.json")
	assert.Equal(t, uint64(1), seqIn(t, raw))
	got, err := tvBackend(dir, log.logf).Load(store.PrefsKey)
	require.NoError(t, err)
	assert.JSONEq(t, `{"auto_next":false}`, string(got))
}

// TestTVDefaultsOffTVOSIsUnsupported checks backend_other.go.
func TestTVDefaultsOffTVOSIsUnsupported(t *testing.T) {
	t.Parallel()

	if _, err := tvDefaults(); err == nil {
		t.Skip("NSUserDefaults is there on tvOS")
	}
	b, err := tvDefaults()
	assert.Nil(t, b)
	assert.ErrorIs(t, err, core.ErrUnsupported)
}

// TestTVBackendRestoresAfterCachesPurge plays P7.5d over a Memory primary
// (NSUserDefaults) and a state folder: the preferences, the progress
// queue and a device document are written, the Caches emptied, and the
// next launch reads them all back; its first save writes the files again.
func TestTVBackendRestoresAfterCachesPurge(t *testing.T) {
	t.Parallel()

	caches := t.TempDir()
	dir := filepath.Join(caches, "com.jivecode.dancefitvn", tvStateFolder)
	primary := store.NewMemory()
	var log logLines
	b := newTVBackend(primary, nil, dir, log.logf)
	ps := store.OpenPrefs(b, true)
	require.NoError(t, ps.Update(func(p *store.Prefs) { p.AutoNext, p.Quality = false, store.QualityP720 }))
	q, err := progress.NewQueue(b, nopSender{})
	require.NoError(t, err)
	require.NoError(t, q.Add(progress.Item{LessonID: "l_1", PositionSec: 72, DurationSec: 600}))
	require.NoError(t, b.Save("device", []byte(`{"last_email":"a@example.invalid"}`)))

	entries, err := os.ReadDir(caches)
	require.NoError(t, err)
	for _, e := range entries {
		require.NoError(t, os.RemoveAll(filepath.Join(caches, e.Name())))
	}

	b = newTVBackend(primary, nil, dir, log.logf) // the next launch
	ps = store.OpenPrefs(b, true)
	require.NoError(t, ps.LoadErr())
	assert.False(t, ps.Get().AutoNext)
	assert.Equal(t, store.QualityP720, ps.Get().Quality)
	assert.Equal(t, store.SlotPrimary, b.LastLoad().From)
	require.ErrorIs(t, b.LastLoad().CopyErr, core.ErrNotFound)
	q, err = progress.NewQueue(b, nopSender{})
	require.NoError(t, err)
	require.Len(t, q.Items(), 1)
	assert.Equal(t, 72, q.Items()[0].PositionSec)
	dev, err := b.Load("device")
	require.NoError(t, err)
	assert.JSONEq(t, `{"last_email":"a@example.invalid"}`, string(dev))

	require.NoError(t, ps.Update(func(p *store.Prefs) { p.AutoNext = true }))
	raw, err := os.ReadFile(filepath.Join(dir, store.PrefsKey+".json"))
	require.NoError(t, err, "the save writes the purged file again")
	assert.Equal(t, uint64(2), seqIn(t, raw))
	assert.Empty(t, log.get())
}

// TestTVBackendLogsAFailingSlot: a slot that fails while the other works
// is logged, by its name, and the save succeeds.
func TestTVBackendLogsAFailingSlot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{"primary", "NSUserDefaults slot failed: locked"},
		{"copy", "file slot failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			primary := store.NewMemory()
			dir := filepath.Join(t.TempDir(), tvStateFolder)
			if tc.name == "primary" {
				primary.SaveErr = errors.New("locked")
			} else {
				require.NoError(t, os.WriteFile(dir, nil, 0o600)) // a file where the folder goes
			}
			var log logLines
			require.NoError(t, newTVBackend(primary, nil, dir, log.logf).Save(store.PrefsKey, []byte(`{}`)))
			require.Len(t, log.get(), 1)
			assert.Contains(t, log.get()[0], tc.want)
		})
	}
}

func TestSlotName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "NSUserDefaults", slotName(store.SlotPrimary))
	assert.Equal(t, "file", slotName(store.SlotCopy))
	assert.Equal(t, "unknown", slotName(store.SlotUnknown))
	assert.Equal(t, "unknown", slotName(store.Slot(9)))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// logLines collects what a logf prints.
type logLines struct {
	mu    sync.Mutex
	lines []string
}

func (l *logLines) logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *logLines) get() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.lines...)
}

// nopSender is a progress.BatchSender never called: nothing flushes.
type nopSender struct{}

func (nopSender) SendProgressBatch(context.Context, model.BatchRequest) (model.BatchResult, error) {
	return model.BatchResult{}, nil
}

// seqIn reads the seq of a Mirror's envelope.
func seqIn(t *testing.T, raw []byte) uint64 {
	t.Helper()

	var e struct {
		Seq uint64 `json:"seq"`
	}
	require.NoError(t, json.Unmarshal(raw, &e))

	return e.Seq
}
