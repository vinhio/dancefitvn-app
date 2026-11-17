//go:build !tvos && !kitdemo

package app

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestMobileRootIsTheShell checks the phone build's root on both phones of
// the plan: the prototype's phone asked for as the window, the shell on the
// splash at first, no directional focus, and a device with no session
// brought to the login by the boot.
func TestMobileRootIsTheShell(t *testing.T) {
	t.Parallel()

	assert.InDelta(t, 393, windowW, 0)
	assert.InDelta(t, 852, windowH, 0)
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, nil, "")
		r.mount(ph, "", false)
		assert.False(t, r.tt.View().DirectionalFocus())
		assert.Equal(t, env.RouteSplash, r.sh.Nav().Current().Screen)
		_, ok := r.sh.Nav().Screen().(*screens.Splash)
		assert.True(t, ok)
		r.drive("the login", func() bool { return r.sh.Nav().Current().Screen == env.RouteLogin })
	})
}

// TestMobileRootUnknownStateStartsAsShipped checks that a state the app does
// not know is logged and the app starts as it ships.
func TestMobileRootUnknownStateStartsAsShipped(t *testing.T) {
	t.Parallel()

	r := newRig(t, nil, "")
	root := newMobileRoot(r.deps, nil, "M99")
	require.NotNil(t, root.widget)
	require.NotNil(t, root.mounted)
	require.NotNil(t, root.close)
	root.close()
}

// TestPhonePlatform checks the X-Platform of each system.
func TestPhonePlatform(t *testing.T) {
	t.Parallel()

	for goos, want := range map[string]string{"android": "android", "ios": "ios", "darwin": "ios"} {
		assert.Equal(t, want, phonePlatform(goos), goos)
	}
}
