// Adapted from jplayer-tv/internal/ui/focus/pick_test.go at 781bee5.

package focus_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// focusVector is one entry of testdata/focus.json (local/vectors): rectangles
// in document order, the start id, a direction and the id the prototype's
// moveFocus reached (the start id when it stays).
type focusVector struct {
	Rects []struct {
		ID string  `json:"id"`
		X  float32 `json:"x"`
		Y  float32 `json:"y"`
		W  float32 `json:"w"`
		H  float32 `json:"h"`
	} `json:"rects"`
	Start string `json:"start"`
	Dir   string `json:"dir"`
	Want  string `json:"want"`
}

// screenVectors is testdata/screen_vectors.json, written by
// testdata/screen_vectors.cjs from the prototype's moveFocus run over the
// rectangles of local/shots/prototype-kit/tv/TV*.layout.json.
type screenVectors struct {
	Screens []struct {
		Screen string `json:"screen"`
		Rects  []struct {
			FID string  `json:"fid"`
			X   float32 `json:"x"`
			Y   float32 `json:"y"`
			W   float32 `json:"w"`
			H   float32 `json:"h"`
		} `json:"rects"`
		Cases []struct {
			From string `json:"from"`
			Dir  string `json:"dir"`
			To   string `json:"to"`
		} `json:"cases"`
	} `json:"screens"`
}

// diffLog collects the disagreements of one test, for DFV_FOCUS_DIFF.
type diffLog struct {
	mu    sync.Mutex
	lines []string
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

var focusDirs = map[string]ui.FocusDirection{
	"ArrowLeft": ui.FocusLeft, "ArrowRight": ui.FocusRight, "ArrowUp": ui.FocusUp, "ArrowDown": ui.FocusDown,
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestFocusVectors runs the 400 synthetic vectors (local/vectors/focus.json)
// through gDesk's ui.NearestInDirection, the engine under View.MoveFocus
// (01 K9, 03 sec 7). A disagreement with the prototype is listed, not a
// failure; an index out of range, or a lost focus, fails. DFV_FOCUS_DIFF=<path>
// writes the list.
func TestFocusVectors(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("testdata/focus.json")
	require.NoError(t, err)
	var vs []focusVector
	require.NoError(t, json.Unmarshal(raw, &vs))
	require.Len(t, vs, 400)

	var log diffLog
	for i, v := range vs {
		rects := make([]geom.Rect, len(v.Rects))
		index := make(map[string]int, len(v.Rects))
		for j, r := range v.Rects {
			rects[j] = geom.XYWH(r.X, r.Y, r.W, r.H)
			index[r.ID] = j
		}
		from, ok := index[v.Start]
		require.True(t, ok, "vector %d: start %s", i, v.Start)
		dir, ok := focusDirs[v.Dir]
		require.True(t, ok, "vector %d: dir %s", i, v.Dir)
		if _, ok := index[v.Want]; !ok {
			require.Failf(t, "want out of range", "vector %d: want %s", i, v.Want)
		}

		got := ui.NearestInDirection(rects, from, dir)
		require.Less(t, got, len(rects), "vector %d", i)
		require.GreaterOrEqual(t, got, -1, "vector %d", i)
		gotID := v.Start // -1: the focus stays
		if got >= 0 {
			gotID = v.Rects[got].ID
		}
		if gotID != v.Want {
			log.add(fmt.Sprintf("synthetic\t%d\t%s\t%s\tprototype=%s\tgdesk=%s", i, v.Start, v.Dir, v.Want, gotID))
		}
	}
	t.Logf("synthetic: %d/%d vectors differ from the prototype", len(log.lines), len(vs))
	log.write(t, fmt.Sprintf("# synthetic %d/%d", len(log.lines), len(vs)))
}

// TestScreenVectors feeds the prototype's TV screens (unscaled rectangles in
// document order) to ui.NearestInDirection and compares each move with the one
// moveFocus reached, per screen (01 K9). Differences are reported, not failed.
func TestScreenVectors(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("testdata/screen_vectors.json")
	require.NoError(t, err)
	var v screenVectors
	require.NoError(t, json.Unmarshal(raw, &v))
	require.Len(t, v.Screens, 11)

	var log diffLog
	total, differ := 0, 0
	for _, st := range v.Screens {
		rects := make([]geom.Rect, len(st.Rects))
		index := make(map[string]int, len(st.Rects))
		for i, r := range st.Rects {
			rects[i] = geom.XYWH(r.X, r.Y, r.W, r.H)
			index[r.FID] = i
		}
		require.Len(t, st.Cases, 4*len(st.Rects), st.Screen)
		n := 0
		for _, c := range st.Cases {
			from, ok := index[c.From]
			require.True(t, ok, "%s: %s", st.Screen, c.From)
			dir, ok := focusDirs[c.Dir]
			require.True(t, ok, "%s: %s", st.Screen, c.Dir)
			got := ui.NearestInDirection(rects, from, dir)
			require.Less(t, got, len(rects), "%s %s %s", st.Screen, c.From, c.Dir)
			gotKey := c.From
			if got >= 0 {
				gotKey = st.Rects[got].FID
			}
			if gotKey != c.To {
				n++
				log.add(fmt.Sprintf("%s\t%s\t%s\tprototype=%s\tgdesk=%s", st.Screen, c.From, c.Dir, c.To, gotKey))
			}
		}
		t.Logf("%s: %d/%d moves differ", st.Screen, n, len(st.Cases))
		total += len(st.Cases)
		differ += n
	}
	assert.Equal(t, 4*(2+1+10+23+12+6+8+7+2+2+10), total, "the vectors were regenerated with another number of cases")
	log.write(t, fmt.Sprintf("# screens %d/%d", differ, total))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// add records one disagreement line.
func (l *diffLog) add(s string) {
	l.mu.Lock()
	l.lines = append(l.lines, s)
	l.mu.Unlock()
}

// write appends the log to the file named by DFV_FOCUS_DIFF, if any, under
// the header line.
func (l *diffLog) write(t *testing.T, header string) {
	t.Helper()
	path := os.Getenv("DFV_FOCUS_DIFF")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()
	_, err = f.WriteString(header + "\n" + strings.Join(l.lines, "\n") + "\n")
	require.NoError(t, err)
}
