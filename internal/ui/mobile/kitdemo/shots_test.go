//go:build !tvos

// The shots test is adapted from jplayer-mobile/internal/app/shots_test.go at
// 49de3c2 (TestShots: one picture and one layout per state and per system,
// 3 pixels to the DIP like the prototype's shots, Core Text for iOS and gotext
// for Android), through internal/ui/kit/layoutdump.

package kitdemo_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/mobile/kitdemo"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/require"
)

// shotScale is the pixels to the DIP of the shots (3, as the prototype's).
const shotScale = 3

// settle is how long a page is let run before it is shot: the sheet's 250 ms
// slide and the dim's 200 ms fade are over, the toast's 2200 ms are not.
const settle = 600 * time.Millisecond

// shotDir returns the folder of the shots: DFV_KIT_SHOTS, "off" to skip, else
// local/shots/app-kit of the repository (skipped when there is no local/).
func shotDir(t *testing.T) string {
	t.Helper()
	switch v := os.Getenv("DFV_KIT_SHOTS"); v {
	case "off":
		t.Skip("DFV_KIT_SHOTS=off")
	case "":
		root := repoRoot(t)
		if _, err := os.Stat(filepath.Join(root, "local")); err != nil {
			t.Skip("no local/ folder for the shots")
		}

		return filepath.Join(root, "local", "shots", "app-kit")
	default:
		return v
	}

	return ""
}

// TestKitShots draws every page on both systems and writes
// <dir>/<os>/<ID>.{png,layout.json}, 3 pixels to the DIP (and <dir>/<os>@1x
// at 1 when DFV_KIT_SHOTS_1X is set), for cmd/shotcmp to compare with
// local/shots/prototype-kit/mobile. The iOS shots use Core Text (Mac only),
// the Android ones gotext with the app's fonts; the TV page is TV05 on Core
// Text in <dir>/tv. It skips under the race detector (the raster at 3x costs
// minutes there), without a local/ folder and with DFV_KIT_SHOTS=off.
func TestKitShots(t *testing.T) { //nolint:paralleltest // writes the shared shot folders
	if kittest.Race {
		t.Skip("the raster of 3x pictures is too slow under the race detector")
	}
	dir := shotDir(t)
	d := loadData(t)
	scales := []float32{shotScale}
	if os.Getenv("DFV_KIT_SHOTS_1X") != "" {
		scales = append(scales, 1)
	}
	for _, ph := range kittest.Phones {
		t.Run(ph.Name, func(t *testing.T) {
			for _, p := range kitdemo.Pages() {
				if p.TV {
					continue
				}
				for _, scale := range scales {
					folder, name := ph.Name, p.ID
					if scale == 1 {
						folder, name = folder+"@1x", name+"@1x"
					}
					t.Run(name, func(t *testing.T) {
						s := p.Build(d)
						tt := kittest.MountReal(t, ph, s.Root)
						tt.SetScale(scale)
						tt.Elapse(settle)
						tt.RunUntilIdle()
						require.NoError(t, layoutdump.Shot(tt, p.ID, "mobile", ph.Size, filepath.Join(dir, folder), s.Skip))
					})
				}
			}
		})
	}
	t.Run("tv", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("Core Text runs on Apple systems only")
		}
		for _, p := range kitdemo.Pages() {
			if !p.TV {
				continue
			}
			s := p.Build(d)
			tt := mountTV(t, s.Root)
			tt.Elapse(settle)
			tt.RunUntilIdle()
			require.NoError(t, layoutdump.Shot(tt, p.ID, "tv", kitdemo.TVSize, filepath.Join(dir, "tv"), s.Skip))
		}
	})
}

// mountTV mounts root on the 1920 x 1080 stage with Core Text and the app's
// theme at scale 1 (the TV prototype's shots are 1x).
func mountTV(t *testing.T, root ui.Widget) *ui.Tester {
	t.Helper()
	ph := kittest.Phones[0] // Core Text, the engine of the Apple TV
	tt, err := ui.NewTester(root, kitdemo.TVSize.W, kitdemo.TVSize.H,
		ui.WithTheme(theme.New(ph.Naming)), ui.WithShaper(kittest.Shaper(t, ph)))
	require.NoError(t, err)
	t.Cleanup(tt.Close)
	tt.Elapse(kittest.Frame)

	return tt
}
