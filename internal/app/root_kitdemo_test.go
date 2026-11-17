//go:build kitdemo && !tvos

package app

import (
	"os"
	"testing"

	"dancefitvn-app/internal/ui/mobile/kitdemo"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKitDemoRootIsTheGallery checks that the kitdemo build's root is the
// gallery over the mock the app was started with, on both phones of the plan,
// and that DFV_KIT opens one page.
func TestKitDemoRootIsTheGallery(t *testing.T) { //nolint:paralleltest // package-level options, environment
	options = Options{Mock: os.DirFS("../../mock/dancefitvn-mock-api")}
	t.Cleanup(func() { options = Options{} })
	texts := locales.MustTexts()
	for _, size := range [][2]float32{{393, 852}, {412, 892}} {
		g, ok := demoRoot(texts).(*kitdemo.Gallery)
		require.True(t, ok, "the root is the gallery")
		tt, err := ui.NewTester(g, size[0], size[1], viewOptions()...)
		require.NoError(t, err)
		_, _, found := tt.Find("M07")
		assert.True(t, found, "the list names the pages")
		tt.Close()
	}

	t.Setenv(kitPageEnv, "M05")
	g, ok := demoRoot(texts).(*kitdemo.Gallery)
	require.True(t, ok)
	tt, err := ui.NewTester(g, 393, 852, viewOptions()...)
	require.NoError(t, err)
	defer tt.Close()
	_, _, listed := tt.Find("M07")
	assert.False(t, listed, "the page is open, not the list")
}

// TestKitDemoWithoutMockShowsTheSplash checks the fallback of a build that
// has no mock folder.
func TestKitDemoWithoutMockShowsTheSplash(t *testing.T) { //nolint:paralleltest // package-level options
	options = Options{}
	texts := locales.MustTexts()
	tt, err := ui.NewTester(demoRoot(texts), 393, 852, viewOptions()...)
	require.NoError(t, err)
	defer tt.Close()
	assert.True(t, tt.Shows(texts["splash.brand"]))
}
