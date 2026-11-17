//go:build !tvos

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpgradeScreen checks the screen of a 426 (03 section 9): the title, the
// sentence and the button, no tab bar, the button opening the store page of the
// 426's body, and no way past it: Back is not taken.
func TestUpgradeScreen(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph, false, shellOptions(), func(t *testing.T, f *shelltest.Fake) {
			require.NoError(t, f.SetScenario("app.config", "upgrade_required"))
		})
		r.Boot()
		require.Equal(t, "upgrade", r.Route())
		up, ok := r.Shell.Nav().Screen().(*screens.Upgrade)
		require.True(t, ok)
		assert.True(t, r.Tester.Shows("Cần cập nhật ứng dụng"))
		assert.True(t, r.Tester.Shows("Phiên bản này không còn được hỗ trợ. Cập nhật để tiếp tục dùng DanceFITVN."))
		assert.True(t, r.Tester.Shows("Cập nhật"))
		assert.True(t, r.Shell.Nav().TabBar().Hidden, "no tab bar")

		tap(r, up.Button())
		assert.Equal(t, []string{"https://apps.apple.com/"}, r.Fake.OpenedURLs())
		assert.False(t, r.Tester.Key(keys.KeyBack, 0), "Back leaves the app: there is nothing under the screen")
	})
}
