//go:build kitdemo && !tvos

package app

import (
	"log"
	"os"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/kitdemo"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// kitPageEnv names the environment variable that opens one kit page at launch
// (SIMCTL_CHILD_DFV_KIT=M07 for the simulator); without it the gallery shows
// its list of pages.
const kitPageEnv = "DFV_KIT"

// ====================================================================
// ============================= Creation =============================
// ====================================================================

func init() { demoRoot = newKitDemo }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// newKitDemo returns the kit's gallery over the mock API the app was built
// with (Options.Mock). A mock that cannot be read leaves the gallery with no
// data: the brand alone, and the reason in the log.
func newKitDemo(texts map[string]string) ui.Widget {
	d, err := kitdemo.Load(options.Mock, texts)
	if err != nil {
		log.Printf("dancefitvn: kitdemo: %v", err)

		return ui.Center(&kit.Label{Text: texts["splash.brand"], Role: theme.RoleDisplay, Size: 72, Weight: 800, Upper: true})
	}

	return kitdemo.NewGallery(d, os.Getenv(kitPageEnv))
}
