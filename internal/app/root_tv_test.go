//go:build tvos

package app

import (
	"testing"

	"dancefitvn-app/internal/ui/tv/shell"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTVRootIsTheShell checks the TV build's root (go test -tags tvos): a
// 1920×1080 window under directional focus with a transparent background,
// the TV shell as root, TV01 once the session is read, whose arrows move
// the focus between w1 and w2.
func TestTVRootIsTheShell(t *testing.T) {
	t.Parallel()
	assert.InDelta(t, 1920, windowW, 0)
	assert.InDelta(t, 1080, windowH, 0)

	r := newTVRig(t, nil, "")
	root := newTVRoot(r.deps, locales.MustTexts(), "")
	_, ok := root.widget.(*shell.Shell)
	require.True(t, ok, "the root is %T", root.widget)
	tt, err := ui.NewTester(root.widget, windowW, windowH, viewOptions()...)
	require.NoError(t, err)
	t.Cleanup(tt.Close)
	t.Cleanup(root.close)
	assert.True(t, tt.View().DirectionalFocus())
	root.mounted(tt.View())
	r.tt = tt
	r.sh = root.widget.(*shell.Shell) //nolint:forcetypeassert // checked above
	r.drive("TV01", func() bool { return r.route() == "welcome" && focusKeyOf(r) == "w1" })
	assert.True(t, tt.Key(keys.KeyRight, 0))
	assert.Equal(t, "w2", focusKeyOf(r))
	assert.True(t, tt.Key(keys.KeyLeft, 0))
	assert.Equal(t, "w1", focusKeyOf(r))
}

// TestTVStatesAreKnown: every state of the shot script is looked up, and an
// unknown one names them.
func TestTVStatesAreKnown(t *testing.T) {
	t.Parallel()
	skipLiveBuild(t)
	for _, id := range []string{"TV01", "TV02", "TV04", "TV05", "TV06", "TV07", "TV08", "TV09", "TV10", "TV11", "TV12", "TV-toast", "TV01-confirm", "TV02-expired", "TV04-offline", "TV-PAIR"} {
		_, err := lookupTVState(id)
		assert.NoError(t, err, id)
	}
	_, err := lookupTVState("M02")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TV-toast")
}
