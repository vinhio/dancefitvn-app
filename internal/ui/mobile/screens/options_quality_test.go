//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVideoQualityRowFollowsPrefs checks screen 07 asks the preferences
// whether the quality has an effect (Prefs.QualitySupported, K21): the row
// shows where it has, and is left out where it has none (Android).
func TestVideoQualityRowFollowsPrefs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		supported bool
	}{
		{"the engine caps the variant", true},
		{"no variant to pick", false},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f, err := shelltest.New(shelltest.MockFS(t))
				require.NoError(t, err)
				require.NoError(t, f.SeedSession())
				f.FakePrefs().SetQualitySupported(tc.supported)
				r := shelltest.MountRig(t, ph, false, f, shell.Options{})
				r.Boot()
				r.Shell.Detail("set-35")
				r.Tester.Elapse(kittest.Frame)
				r.Settle()
				r.Shell.PlayVideo("set-35", "l_3503", 3)
				r.Settle()
				require.Equal(t, env.RouteVideo, r.Route())
				tap(r, named(r.Shell, optionsOpen, 1))
				r.Tester.Elapse(300 * time.Millisecond)
				_, ok := r.Dump("sheet").Find("quality")
				assert.Equal(t, tc.supported, ok)
			})
		}
	})
}
