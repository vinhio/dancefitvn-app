//go:build !tvos

// Adapted from jplayer-mobile/internal/app/shots_test.go at 49de3c2
// (TestShots: one picture and one layout per state and per system, 3
// pixels to the DIP like the prototype's shots, Core Text for iOS and
// gotext for Android), through internal/ui/kit/layoutdump and the states
// of states_mobile.go, which the developer's build on a device draws too.

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"

	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// shotScale is the pixels to the DIP of the shots (3, as the prototype's).
const shotScale = 3

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestShots draws the states of DFV_SHOT_STATES (default M01,M02) on both
// phones with their real text engines into $DFV_SHOTS/<os>/<ID>.{png,
// layout.json}, for cmd/shotcmp (scripts/shot.sh <IDs> tester). Each state
// goes through the real flow over the mock, as on a device. It skips when
// DFV_SHOTS is not set, and under the race detector (a 3x raster costs
// minutes there).
func TestShots(t *testing.T) { //nolint:paralleltest // writes the shared shot folders
	out := os.Getenv("DFV_SHOTS")
	if out == "" {
		t.Skip("DFV_SHOTS names the folder of the shots")
	}
	if kittest.Race {
		t.Skip("the raster of 3x pictures is too slow under the race detector")
	}
	states := strings.Split(os.Getenv("DFV_SHOT_STATES"), ",")
	if states[0] == "" {
		states = []string{"M01", "M02"}
	}
	for _, ph := range kittest.Phones {
		t.Run(ph.Name, func(t *testing.T) {
			for _, id := range states {
				t.Run(id, func(t *testing.T) {
					_, err := lookupState(id)
					require.NoError(t, err)
					r := newRig(t, nil, "")
					r.mount(ph, id, true)
					file := filepath.Join(must(r.deps.CacheDir()), shotFolder, id+".layout.json")
					r.drive("the state "+id, func() bool {
						_, err := os.Stat(file)

						return err == nil
					})
					r.tt.SetScale(shotScale)
					r.tt.Elapse(kittest.Frame)
					require.NoError(t, layoutdump.Shot(r.tt, id, "mobile", ph.Size, filepath.Join(out, ph.Name), nil))
				})
			}
		})
	}
}
