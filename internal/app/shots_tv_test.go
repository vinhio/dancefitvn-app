//go:build tvos

// The TV's shots in process (scripts/shot.sh <IDs> tvtester): the pattern
// of shots_mobile_test.go, at 1x on the 1920×1080 stage (Q-P7-7) with Core
// Text, through the states of states_tv.go, which the developer's build on
// the Apple TV simulator draws too.

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestTVShots draws the states of DFV_SHOT_STATES (default TV01,TV02) into
// $DFV_SHOTS/tv/<ID>.{png,layout.json}, for cmd/shotcmp. Each state goes
// through the real flow over the mock, as on the simulator. It skips when
// DFV_SHOTS is not set, and under the race detector.
func TestTVShots(t *testing.T) { //nolint:paralleltest // writes the shared shot folder
	out := os.Getenv("DFV_SHOTS")
	if out == "" {
		t.Skip("DFV_SHOTS names the folder of the shots")
	}
	if kittest.Race {
		t.Skip("the raster of the shots is too slow under the race detector")
	}
	states := strings.Split(os.Getenv("DFV_SHOT_STATES"), ",")
	if states[0] == "" {
		states = []string{"TV01", "TV02"}
	}
	for _, id := range states {
		t.Run(id, func(t *testing.T) {
			_, err := lookupTVState(id)
			require.NoError(t, err)
			r := newTVRig(t, nil, "")
			r.deps.ShotWall = func() time.Time { return tvDemoNow.Add(r.tt.View().Context().Now()) }
			r.mountWith(id, true)
			file := filepath.Join(must(r.deps.CacheDir()), shotFolder, id+".layout.json")
			r.drive("the state "+id, func() bool {
				_, err := os.Stat(file)

				return err == nil
			})
			r.tt.Elapse(tvtest.Frame)
			lay := layoutdump.Dump(r.tt.View().Context(), r.tt.View().Root(), id, "tv", geom.Sz(tvtest.Width, tvtest.Height), nil)
			require.NoError(t, lay.Write(filepath.Join(out, "tv"), r.tt.Snapshot()))
			st, _ := lookupTVState(id)
			assertTVShot(t, lay, st, tvFocusKey(r.sh))
		})
	}
}

// TestTVStatesSayWhatTheyShow: every TV state names keys of its content, so
// that neither the runner nor TestTVShots takes a screen still waiting for
// its data (one stand-in widget, d1) for the state.
func TestTVStatesSayWhatTheyShow(t *testing.T) {
	t.Parallel()
	for _, id := range tvStateIDs() {
		st, err := lookupTVState(id)
		require.NoError(t, err)
		assert.NotEmpty(t, st.shows, id)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// assertTVShot fails the shot of st whose layout lacks a key of its content
// or whose focus is not the state's (focused is the focused item's key).
func assertTVShot(t *testing.T, lay layoutdump.Layout, st tvShotState, focused string) {
	t.Helper()
	have := make(map[string]bool, len(lay.Elements))
	for _, e := range lay.Elements {
		have[e.Key] = true
	}
	require.Greater(t, len(lay.Elements), len(st.shows), "the layout has %d elements", len(lay.Elements))
	for _, k := range st.shows {
		assert.True(t, have[k], "the layout has no %q", k)
	}
	if st.focus != "" {
		assert.Equal(t, st.focus, focused, "the focus")
	}
}
