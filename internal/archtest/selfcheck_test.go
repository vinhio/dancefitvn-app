// Adapted from jplayer-mobile/internal/archtest/selfcheck_test.go at 49de3c2:
// every rule of archtest_test.go is fed synthetic fstest.MapFS trees and must
// report exactly what the case lists, so a rule that stops checking fails
// here rather than passing silently on the real repository.

package archtest

import (
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// fakeModule is the module path of the fake repositories.
const fakeModule = "example.test/app"

// fakeCase is one self-check: a fake tree and the violations the rule must
// report for it, each as a substring of one report.
type fakeCase struct {
	name  string
	files map[string]string
	want  []string
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestLayerRule checks layerRule on fake trees: PLAN §4.2 and Q-P1-12.
func TestLayerRule(t *testing.T) {
	t.Parallel()

	m := fakeModule
	ui := m + "/internal/ui/theme"
	fw := gdeskPath + "/framework/ui"
	objc := gdeskPath + "/foundation/ffi/objc"
	tests := []fakeCase{
		// core
		{"core ok", map[string]string{"internal/store/a.go": src("", "fmt", m+"/internal/model", gdeskPath+"/foundation/core")}, nil},
		{"core: subpackage of a core dir", map[string]string{"internal/api/mock/a.go": src("", m+"/internal/api", m+"/internal/model")}, nil},
		{"core: clock is core", map[string]string{"internal/progress/a.go": src("", m+"/internal/clock")}, nil},
		{
			"core imports ui",
			map[string]string{"internal/auth/a.go": src("", ui)},
			[]string{"internal/auth/a.go imports " + ui + " (core layer)"},
		},
		{
			"core: api/mock is scanned",
			map[string]string{"internal/api/mock/a.go": src("", ui)},
			[]string{"internal/api/mock/a.go imports " + ui},
		},
		{
			"core imports framework/ui",
			map[string]string{"internal/model/a.go": src("", fw)},
			[]string{"imports " + fw},
		},
		{
			"core imports a gdesk backend",
			map[string]string{"internal/qr/a.go": src("", gdeskPath+"/bridge/backend/uikit")},
			[]string{"bridge/backend/uikit"},
		},
		{"core imports ffi", map[string]string{"internal/format/a.go": src("", objc)}, []string{"foundation/ffi/objc"}},
		{"core imports jni", map[string]string{"internal/store/a.go": src("", jniPath)}, []string{jniPath}},
		{"core imports sys", map[string]string{"internal/download/a.go": src("", m+"/internal/sys")}, []string{"internal/sys"}},
		{"core imports media", map[string]string{"internal/pairing/a.go": src("", m+"/internal/media")}, []string{"internal/media"}},
		{"core imports app", map[string]string{"internal/api/a.go": src("", m+"/internal/app")}, []string{"internal/app"}},
		{"core imports the module root", map[string]string{"internal/api/a.go": src("", m)}, []string{"imports " + m + " ("}},
		{"core imports a third party", map[string]string{"internal/api/a.go": src("", "golang.org/x/text/unicode/norm")}, []string{"golang.org/x/text"}},
		{"core test imports testify", map[string]string{"internal/api/a_test.go": src("", "github.com/stretchr/testify/require")}, nil},
		{"core non-test imports testify", map[string]string{"internal/api/a.go": src("", "github.com/stretchr/testify/require")}, []string{"testify"}},
		{"core test imports ui", map[string]string{"internal/api/a_test.go": src("", ui)}, []string{"internal/ui/theme"}},
		{"core lookalike dir is not core", map[string]string{"internal/apix/a.go": src("", ui)}, nil},
		// store tvos: a leaf of its own under internal/store
		{"tvstore imports objc", map[string]string{"internal/store/tvstore/a.go": src("ios && tvos", objc, gdeskPath+"/foundation/core")}, nil},
		{
			"tvstore imports the store",
			map[string]string{"internal/store/tvstore/a.go": src("", m+"/internal/store")},
			[]string{"internal/store/tvstore/a.go imports " + m + "/internal/store (store tvos layer)"},
		},
		{"tvstore imports dl", map[string]string{"internal/store/tvstore/a.go": src("", gdeskPath+"/foundation/ffi/dl")}, []string{"foundation/ffi/dl (store tvos layer)"}},
		{"tvstore imports framework/ui", map[string]string{"internal/store/tvstore/a.go": src("", fw)}, []string{fw + " (store tvos layer)"}},
		// engine
		{"engine ok", map[string]string{
			"internal/media/avf/a.go":  src("ios", objc, gdeskPath+"/foundation/ffi/dl", m+"/internal/media", m+"/internal/clock"),
			"internal/media/amp/a.go":  src("android", jniPath, gdeskPath+"/foundation/platform", gdeskPath+"/foundation/core/event"),
			"internal/media/a.go":      src("", gdeskPath+"/foundation/scene"),
			"internal/media/fake/a.go": src("", m+"/internal/media/enginekit", m+"/internal/model"),
		}, nil},
		{"engine imports framework", map[string]string{"internal/media/a.go": src("", fw)}, []string{"imports " + fw + " (engine layer)"}},
		{"engine imports the android driver", map[string]string{"internal/media/amp/a.go": src("", gdeskPath+"/bridge/backend/android")}, []string{"bridge/backend/android"}},
		{"jni subpackages follow jni", map[string]string{"internal/media/amp/a.go": src("", jniPath+"/x")}, nil},
		{"engine imports uikit", map[string]string{"internal/media/avf/a.go": src("", gdeskPath+"/bridge/backend/uikit")}, []string{"uikit"}},
		{"engine imports sys", map[string]string{"internal/media/a.go": src("", m+"/internal/sys")}, []string{"internal/sys"}},
		{"engine imports ui", map[string]string{"internal/media/avftv/a.go": src("", ui)}, []string{"internal/ui/theme"}},
		{"engine imports the system bridge", map[string]string{"internal/media/a.go": src("", gdeskPath+"/bridge/system/secret")}, []string{"bridge/system"}},
		// system
		{"system ok", map[string]string{
			"internal/sys/a.go":      src("", gdeskPath+"/bridge/system/secret", gdeskPath+"/foundation/platform", objc, gdeskPath+"/foundation/ffi/dl", jniPath, m+"/internal/clock"),
			"internal/sys/a_test.go": src("", gdeskPath+"/foundation/platform/fake", "github.com/stretchr/testify/assert"),
		}, nil},
		{"system imports framework", map[string]string{"internal/sys/a.go": src("", fw)}, []string{"imports " + fw + " (system layer)"}},
		{"system imports host", map[string]string{"internal/sys/a.go": src("", hostPath)}, []string{"framework/host"}},
		{"system imports media", map[string]string{"internal/sys/a.go": src("", m+"/internal/media")}, []string{"internal/media"}},
		{"system imports ui", map[string]string{"internal/sys/a.go": src("", ui)}, []string{"internal/ui/theme"}},
		{"system imports the android driver", map[string]string{"internal/sys/a.go": src("", gdeskPath+"/bridge/backend/android")}, []string{"bridge/backend/android"}},
		// player
		{"player ok", map[string]string{
			"internal/player/a.go":      src("", "fmt", gdeskPath+"/foundation/core", gdeskPath+"/foundation/core/event", m+"/internal/media", m+"/internal/progress", m+"/internal/clock", m+"/internal/api", m+"/internal/model"),
			"internal/player/a_test.go": src("", m+"/internal/media/fake", "github.com/stretchr/testify/assert"),
		}, nil},
		{"player imports ui", map[string]string{"internal/player/a.go": src("", ui)}, []string{"imports " + ui + " (player layer)"}},
		{"player imports sys", map[string]string{"internal/player/a.go": src("", m+"/internal/sys")}, []string{"internal/sys"}},
		{"player imports app", map[string]string{"internal/player/a.go": src("", m+"/internal/app")}, []string{"internal/app"}},
		{"player imports framework", map[string]string{"internal/player/a.go": src("", fw)}, []string{"imports " + fw + " (player layer)"}},
		{"player imports scene", map[string]string{"internal/player/a.go": src("", gdeskPath+"/foundation/scene")}, []string{"foundation/scene"}},
		{"player imports a third party", map[string]string{"internal/player/a.go": src("", "golang.org/x/text/unicode/norm")}, []string{"golang.org/x/text"}},
		{"player non-test imports testify", map[string]string{"internal/player/a.go": src("", "github.com/stretchr/testify/require")}, []string{"testify"}},
		{"core imports player", map[string]string{"internal/progress/a.go": src("", m+"/internal/player")}, []string{"internal/player"}},
		{"engine imports player", map[string]string{"internal/media/a.go": src("", m+"/internal/player")}, []string{"internal/player"}},
		{"mobile and tv may import player", map[string]string{
			"internal/ui/mobile/a.go": src("!tvos", m+"/internal/player"),
			"internal/ui/tv/a.go":     src("tvos", m+"/internal/player"),
		}, nil},
		// ui common
		{"ui common ok", map[string]string{
			"internal/ui/theme/a.go": src("", fw, gdeskPath+"/framework/components/headless", gdeskPath+"/bridge/text", gdeskPath+"/foundation/core/color", m+"/assets", m+"/internal/model"),
			"internal/tokens/a.go":   src("", gdeskPath+"/foundation/core/color"),
		}, nil},
		{"ui common imports sys", map[string]string{"internal/ui/kit/a.go": src("", m+"/internal/sys")}, []string{"internal/sys"}},
		{"ui common imports mobile", map[string]string{"internal/ui/poster/a.go": src("", m+"/internal/ui/mobile")}, []string{"internal/ui/mobile"}},
		{"ui common imports host", map[string]string{"internal/ui/icon/a.go": src("", hostPath)}, []string{"framework/host"}},
		{"ui common imports full components", map[string]string{"internal/ui/kit/a.go": src("", gdeskPath+"/framework/components/button")}, []string{"framework/components/button"}},
		// ui mobile and tv
		{"mobile ok", map[string]string{
			"internal/ui/mobile/home/a.go": src("!tvos", fw, gdeskPath+"/framework/components/list", m+"/internal/ui/kit", m+"/internal/media", m+"/internal/sys", m+"/internal/model", m+"/internal/ui/mobile/kitx", m+"/locales"),
		}, nil},
		{"tv ok", map[string]string{"internal/ui/tv/grid/a.go": src("tvos", fw, m+"/internal/ui/theme", m+"/internal/media", m+"/internal/ui/tv/focus")}, nil},
		{"mobile imports tv", map[string]string{"internal/ui/mobile/a.go": src("", m+"/internal/ui/tv")}, []string{"internal/ui/tv"}},
		{"tv imports mobile", map[string]string{"internal/ui/tv/a.go": src("tvos", m+"/internal/ui/mobile/home")}, []string{"internal/ui/mobile/home"}},
		{"mobile imports app", map[string]string{"internal/ui/mobile/a.go": src("", m+"/internal/app")}, []string{"internal/app"}},
		{"tv imports host", map[string]string{"internal/ui/tv/a.go": src("tvos", hostPath)}, []string{"framework/host"}},
		{"mobile imports kits", map[string]string{"internal/ui/mobile/a.go": src("", kitsPath+"/mobile")}, []string{"kits/mobile"}},
		{"mobile imports a backend", map[string]string{"internal/ui/mobile/a.go": src("", gdeskPath+"/bridge/backend/uikit")}, []string{"uikit"}},
		// leaves
		{"leaf ok", map[string]string{"locales/a.go": src("", "embed", gdeskPath+"/foundation/core"), "assets/a.go": src("", "embed")}, nil},
		{"leaf imports the module", map[string]string{"locales/a.go": src("", m+"/internal/model")}, []string{"locales/a.go"}},
		// outside every layer
		{"app and programs are free", map[string]string{
			"internal/app/a.go": src("", fw, hostPath, m+"/internal/ui/theme", objc, jniPath),
			"main.go":           src("", kitsPath+"/mobile", m+"/internal/app"),
			"cmd/x/a.go":        src("", m+"/internal/sys", m+"/internal/media/avf"),
		}, nil},
	}
	runCases(t, tests, layerRule)
}

// TestMobileTVRule checks mobileTVRule on fake trees.
func TestMobileTVRule(t *testing.T) {
	t.Parallel()

	tests := []fakeCase{
		{"apart", map[string]string{
			"internal/ui/mobile/a.go": src("", fakeModule+"/internal/ui/kit"),
			"internal/ui/tv/a.go":     src("tvos", fakeModule+"/internal/ui/kit"),
		}, nil},
		{"mobile imports tv", map[string]string{"internal/ui/mobile/a.go": src("", fakeModule+"/internal/ui/tv")}, []string{"internal/ui/mobile/a.go imports " + fakeModule + "/internal/ui/tv"}},
		{"mobile imports a tv subpackage", map[string]string{"internal/ui/mobile/x/a.go": src("", fakeModule+"/internal/ui/tv/grid")}, []string{"internal/ui/mobile/x/a.go"}},
		{"tv imports mobile", map[string]string{"internal/ui/tv/a.go": src("tvos", fakeModule+"/internal/ui/mobile")}, []string{"internal/ui/tv/a.go"}},
		{"test files count", map[string]string{"internal/ui/tv/a_test.go": src("tvos", fakeModule+"/internal/ui/mobile")}, []string{"a_test.go"}},
		{"tvx lookalike", map[string]string{"internal/ui/mobile/a.go": src("", fakeModule+"/internal/ui/tvx")}, nil},
	}
	runCases(t, tests, mobileTVRule)
}

// TestFFIRule checks ffiRule on fake trees: foundation/ffi and the JNI binding
// are for the engines, the system layer and the app.
func TestFFIRule(t *testing.T) {
	t.Parallel()

	objc := gdeskPath + "/foundation/ffi/objc"
	tests := []fakeCase{
		{"users", map[string]string{
			"internal/media/avf/a.go":      src("ios", objc, gdeskPath+"/foundation/ffi/cf"),
			"internal/media/avftv/a.go":    src("ios", objc),
			"internal/media/amp/a.go":      src("android", gdeskPath+"/foundation/ffi/dl", jniPath),
			"internal/media/amp/s/a.go":    src("android", jniPath),
			"internal/sys/a.go":            src("", objc, jniPath),
			"internal/app/a.go":            src("", objc, jniPath),
			"cmd/x/a.go":                   src("", objc),
			"e2e/a.go":                     src("", objc),
			"main.go":                      src("", objc),
			"internal/media/avf/a_test.go": src("ios", objc),
		}, nil},
		{"store imports objc", map[string]string{"internal/store/a.go": src("", objc)}, []string{"internal/store/a.go imports " + objc}},
		{"tvstore imports objc", map[string]string{"internal/store/tvstore/a.go": src("ios && tvos", objc)}, nil},
		{"enginekit imports dl", map[string]string{"internal/media/enginekit/a.go": src("", gdeskPath+"/foundation/ffi/dl")}, []string{"foundation/ffi/dl"}},
		{"media imports objc", map[string]string{"internal/media/a.go": src("", objc)}, []string{"internal/media/a.go"}},
		{"avf lookalike", map[string]string{"internal/media/avfx/a.go": src("", objc)}, []string{"internal/media/avfx"}},
		{"avf imports jni", map[string]string{"internal/media/avf/a.go": src("ios", jniPath)}, []string{"imports " + jniPath}},
		{"ui imports ffi", map[string]string{"internal/ui/kit/a.go": src("", gdeskPath+"/foundation/ffi")}, []string{"foundation/ffi"}},
		{"cmd subdirectory is not a program", map[string]string{"cmd/x/y/a.go": src("", objc)}, []string{"cmd/x/y/a.go"}},
	}
	runCases(t, tests, ffiRule)
}

// TestHostRule checks hostRule on fake trees.
func TestHostRule(t *testing.T) {
	t.Parallel()

	tests := []fakeCase{
		{"allowed", map[string]string{
			"main.go": src("", hostPath), "internal/app/a.go": src("", hostPath), "internal/app/x/a.go": src("", hostPath+"/x"),
			"cmd/spike/a.go": src("", hostPath), "e2e/a.go": src("", hostPath),
		}, nil},
		{"ui", map[string]string{"internal/ui/mobile/a.go": src("", hostPath)}, []string{"internal/ui/mobile/a.go imports " + hostPath}},
		{"core", map[string]string{"internal/api/a.go": src("", hostPath)}, []string{"internal/api/a.go"}},
		{"cmd subdirectory", map[string]string{"cmd/spike/inner/a.go": src("", hostPath)}, []string{"cmd/spike/inner/a.go"}},
		{"appx lookalike", map[string]string{"internal/appx/a.go": src("", hostPath)}, []string{"internal/appx/a.go"}},
	}
	runCases(t, tests, hostRule)
}

// TestKitsRule checks kitsRule on fake trees.
func TestKitsRule(t *testing.T) {
	t.Parallel()

	tests := []fakeCase{
		{"allowed", map[string]string{"main.go": src("", kitsPath+"/mobile"), "cmd/spike/a.go": src("", kitsPath+"/mobile"), "e2e/a.go": src("", kitsPath+"/mobile")}, nil},
		{"app", map[string]string{"internal/app/a.go": src("", kitsPath+"/mobile")}, []string{"internal/app/a.go imports " + kitsPath + "/mobile"}},
		{"ui", map[string]string{"internal/ui/kit/a.go": src("", kitsPath+"/mobile")}, []string{"internal/ui/kit/a.go"}},
		{"desktop kit anywhere", map[string]string{"main.go": src("", kitsPath+"/desktop")}, []string{"kits/desktop"}},
		{"web kit anywhere", map[string]string{"cmd/x/a.go": src("", kitsPath+"/web/x")}, []string{"kits/web/x"}},
	}
	runCases(t, tests, kitsRule)
}

// TestTVStoreRule checks tvstoreRule on fake trees: only the app, programs
// and e2e import internal/store/tvstore.
func TestTVStoreRule(t *testing.T) {
	t.Parallel()

	tv := fakeModule + "/internal/store/tvstore"
	tests := []fakeCase{
		{"allowed", map[string]string{
			"main.go": src("", tv), "cmd/x/a.go": src("", tv), "e2e/tvstore/a.go": src("", tv),
			"internal/app/a.go": src("ios && tvos", tv), "internal/store/tvstore/a_test.go": src("", tv),
		}, nil},
		{"store", map[string]string{"internal/store/a.go": src("", tv)}, []string{"internal/store/a.go imports " + tv}},
		{"progress", map[string]string{"internal/progress/a.go": src("", tv)}, []string{"internal/progress/a.go"}},
		{"ui tv", map[string]string{"internal/ui/tv/a.go": src("tvos", tv)}, []string{"internal/ui/tv/a.go"}},
		{"sys", map[string]string{"internal/sys/a.go": src("", tv)}, []string{"internal/sys/a.go"}},
	}
	runCases(t, tests, tvstoreRule)
}

// TestAppRule checks appRule on fake trees.
func TestAppRule(t *testing.T) {
	t.Parallel()

	app := fakeModule + "/internal/app"
	tests := []fakeCase{
		{"allowed", map[string]string{"main.go": src("", app), "cmd/x/a.go": src("", app), "e2e/a.go": src("", app), "internal/app/b/a.go": src("", app)}, nil},
		{"ui", map[string]string{"internal/ui/tv/a.go": src("tvos", app)}, []string{"internal/ui/tv/a.go imports " + app}},
		{"sys", map[string]string{"internal/sys/a.go": src("", app+"/x")}, []string{"internal/sys/a.go"}},
	}
	runCases(t, tests, appRule)
}

// TestCgoRule checks cgoRule on fake trees.
func TestCgoRule(t *testing.T) {
	t.Parallel()

	tests := []fakeCase{
		{"engines", map[string]string{"internal/media/avf/a.go": src("ios", "C"), "internal/media/amp/x/a.go": src("android", "C")}, nil},
		{"sys", map[string]string{"internal/sys/a.go": src("ios", "C")}, []string{"internal/sys/a.go uses cgo"}},
		{"avftv has no cgo", map[string]string{"internal/media/avftv/a.go": src("ios", "C")}, []string{"internal/media/avftv/a.go"}},
		{"app", map[string]string{"internal/app/a.go": src("", "C")}, []string{"internal/app/a.go"}},
		{"cmd", map[string]string{"cmd/x/a.go": src("", "C")}, []string{"cmd/x/a.go"}},
	}
	runCases(t, tests, cgoRule)
}

// TestBuildTagsRule checks buildTagsRule on fake trees: the _other.go rule,
// ErrUnsupported, tvos, the engines and the interface shells.
func TestBuildTagsRule(t *testing.T) {
	t.Parallel()

	core := gdeskPath + "/foundation/core"
	unsup := "package p\n\nimport \"" + core + "\"\n\nvar _ = core.ErrUnsupported\n"
	stub := "package sys\n\nimport (\n\t\"fmt\"\n\n\t\"" + core + "\"\n)\n\nfunc F() error { return fmt.Errorf(\"f: %w\", core.ErrUnsupported) }\n"
	tests := []fakeCase{
		{"android && cgo with its !(android && cgo) _other.go", map[string]string{
			"internal/app/env_android.go": "//go:build android && cgo\n\npackage app\n",
			"internal/app/env_other.go":   "//go:build !(android && cgo)\n\npackage app\n",
		}, nil},
		{"the ill-paired _other.go leaves android without cgo bare", map[string]string{
			"internal/app/env_android.go": "//go:build android && cgo\n\npackage app\n",
			"internal/app/env_other.go":   "//go:build !android\n\npackage app\n",
			"internal/app/doc.go":         "package app\n",
		}, []string{"builds no file (besides an untagged doc.go) on android"}},
		{"file-name tags count", map[string]string{
			"internal/app/rot_ios.go":   "package app\n",
			"internal/app/rot_other.go": "//go:build !ios\n\npackage app\n",
		}, nil},
		{"any package with ios files needs _other.go", map[string]string{
			"internal/sys/x_ios.go": "//go:build ios\n\npackage sys\n",
			"internal/sys/doc.go":   "package sys\n",
		}, []string{"internal/sys has files for ios, tvos or android but no _other.go", "on android, android+cgo, darwin"}},
		{"any package with tvos files needs _other.go", map[string]string{
			"internal/sys/x_tvos.go": "//go:build ios && tvos\n\npackage sys\n",
		}, []string{"internal/sys has files for ios, tvos or android but no _other.go", "builds no file"}},
		{"_other.go without a constraint builds on ios too", map[string]string{
			"internal/sys/x_ios.go":   "//go:build ios\n\npackage sys\n",
			"internal/sys/x_other.go": "package sys\n",
		}, []string{"x_other.go must exclude exactly"}},
		{"_other.go that skips the Mac", map[string]string{
			"internal/sys/x_ios.go":   "//go:build ios\n\npackage sys\n",
			"internal/sys/x_other.go": "//go:build android\n\npackage sys\n",
		}, []string{"internal/sys builds no file", "x_other.go must build on darwin"}},
		{"an _other.go that makes errors must refer to ErrUnsupported", map[string]string{
			"internal/sys/x_ios.go":   "//go:build ios\n\npackage sys\n",
			"internal/sys/x_other.go": "//go:build !ios\n\npackage sys\n\nimport \"errors\"\n\nfunc F() error { return errors.New(\"no\") }\n",
		}, []string{"does not refer to core.ErrUnsupported"}},
		{"an _other.go that makes errors via fmt.Errorf", map[string]string{
			"internal/sys/x_ios.go":   "//go:build ios\n\npackage sys\n",
			"internal/sys/x_other.go": "//go:build !ios\n\npackage sys\n\nimport \"fmt\"\n\nfunc F() error { return fmt.Errorf(\"no\") }\n",
		}, []string{"does not refer to core.ErrUnsupported"}},
		{"an _other.go that makes a package error", map[string]string{
			"internal/sys/x_ios.go":   "//go:build ios\n\npackage sys\n",
			"internal/sys/x_other.go": "//go:build !ios\n\npackage sys\n\nimport \"io/fs\"\n\nfunc F() error { return fs.ErrNotExist }\n",
		}, []string{"does not refer to core.ErrUnsupported"}},
		{"an _other.go returning errors via core.ErrUnsupported", map[string]string{
			"internal/sys/x_ios.go":   "//go:build ios\n\npackage sys\n",
			"internal/sys/x_other.go": "//go:build !ios\n\n" + stub,
		}, nil},
		{"an _other.go that only passes on a system error is a fallback", map[string]string{
			"internal/sys/x_ios.go":   "//go:build ios\n\npackage sys\n",
			"internal/sys/x_other.go": "//go:build !ios\n\npackage sys\n\nimport \"os\"\n\nfunc F() (string, error) { return os.UserConfigDir() }\n",
		}, nil},
		{"test-only package needs no _other.go (e2e/engine)", map[string]string{
			"e2e/engine/main_ios_test.go":     "//go:build ios\n\npackage engine\n",
			"e2e/engine/main_android_test.go": "//go:build android && cgo\n\npackage engine\n",
			"e2e/engine/other_test.go":        "//go:build !ios && !(android && cgo)\n\npackage engine\n",
		}, nil},
		{"doc.go with a tag", map[string]string{"internal/sys/doc.go": "//go:build ios\n\npackage sys\n"}, []string{"internal/sys/doc.go must have no build tag"}},
		{"shells: mobile is !tvos with no _other.go", map[string]string{
			"internal/ui/mobile/doc.go": "//go:build !tvos\n\npackage mobile\n",
			"internal/ui/mobile/a.go":   "//go:build !tvos\n\npackage mobile\n",
		}, nil},
		{"shells: tv has a tagged doc.go and an _other.go", map[string]string{
			"internal/ui/tv/doc.go":       "//go:build tvos\n\npackage tv\n",
			"internal/ui/tv/doc_other.go": "//go:build !tvos\n\npackage tv\n",
			"internal/ui/tv/a.go":         "//go:build tvos\n\npackage tv\n",
		}, nil},
		{"shells: tv without _other.go", map[string]string{
			"internal/ui/tv/doc.go": "//go:build tvos\n\npackage tv\n",
		}, []string{"internal/ui/tv has files for ios, tvos or android but no _other.go", "builds no file"}},
		{"shells: tv _other.go that builds on tvos too", map[string]string{
			"internal/ui/tv/doc.go":       "//go:build tvos\n\npackage tv\n",
			"internal/ui/tv/doc_other.go": "package tv\n",
		}, []string{"doc_other.go must exclude exactly"}},
		{"a file named _tvos needs the tag", map[string]string{
			"internal/sys/p_tvos.go":  "package sys\n",
			"internal/sys/p_other.go": "//go:build !tvos\n\npackage sys\n",
		}, []string{"p_tvos.go is named _tvos", "p_other.go must exclude"}},
		{"avf ok", map[string]string{
			"internal/media/avf/doc.go":          "package avf\n",
			"internal/media/avf/player.go":       "//go:build ios && !tvos\n\npackage avf\n",
			"internal/media/avf/player_other.go": "//go:build !ios || tvos\n\n" + unsup,
		}, nil},
		{"avf file built on tvos", map[string]string{
			"internal/media/avf/player.go":       "//go:build ios\n\npackage avf\n\nimport \"C\"\n",
			"internal/media/avf/player_other.go": "//go:build !ios\n\n" + unsup,
		}, []string{"player.go must build only on ios without tvos, but builds on tvos", "player_other.go must build on every platform but ios without tvos, not on tvos"}},
		{"avf file built on the Mac", map[string]string{
			"internal/media/avf/player.go":       "package avf\n\nimport \"C\"\n",
			"internal/media/avf/player_other.go": "//go:build !ios\n\n" + unsup,
		}, []string{"player_other.go must exclude", "player.go must build only on ios without tvos", "must build on every platform but ios without tvos"}},
		{"avf without _other.go", map[string]string{
			"internal/media/avf/player.go": "//go:build ios && !tvos\n\npackage avf\n",
		}, []string{"no _other.go", "builds no file"}},
		{"avf _other.go lacks ErrUnsupported", map[string]string{
			"internal/media/avf/player.go":       "//go:build ios && !tvos\n\npackage avf\n",
			"internal/media/avf/player_other.go": "//go:build !ios || tvos\n\npackage avf\n",
		}, []string{"must refer to core.ErrUnsupported"}},
		{"avf file that never builds", map[string]string{
			"internal/media/avf/player.go":       "//go:build android\n\npackage avf\n",
			"internal/media/avf/player_other.go": "//go:build !ios || tvos\n\n" + unsup,
		}, []string{"player.go never builds on ios without tvos", "builds no file (besides an untagged doc.go) on ios", "player_other.go must exclude"}},
		{"avf cgo file with a cgo-less _other.go may build on ios without cgo", map[string]string{
			"internal/media/avf/player.go":       "//go:build ios && !tvos && cgo\n\npackage avf\n",
			"internal/media/avf/player_other.go": "//go:build !ios || tvos || !cgo\n\n" + unsup,
		}, nil},
		{"avftv ok", map[string]string{
			"internal/media/avftv/player.go":       "//go:build ios && tvos\n\npackage avftv\n",
			"internal/media/avftv/player_other.go": "//go:build !(ios && tvos)\n\n" + unsup,
		}, nil},
		{"avftv file built on ios", map[string]string{
			"internal/media/avftv/player.go":       "//go:build ios\n\npackage avftv\n\nimport _ \"" + ffiPath + "/objc\"\n",
			"internal/media/avftv/player_other.go": "//go:build !(ios && tvos)\n\n" + unsup,
		}, []string{"player.go must build only on ios with tvos, but builds on ios, ios+cgo", "player_other.go must exclude"}},
		{"avftv pure Go file may build everywhere", map[string]string{
			"internal/media/avftv/state.go":        "package avftv\n",
			"internal/media/avftv/player.go":       "//go:build ios && tvos\n\npackage avftv\n",
			"internal/media/avftv/player_other.go": "//go:build !(ios && tvos)\n\n" + unsup,
		}, nil},
		{"engine file that binds ffi may not build everywhere", map[string]string{
			"internal/media/avftv/state.go":        "package avftv\n\nimport _ \"" + ffiPath + "/objc\"\n",
			"internal/media/avftv/player_other.go": "//go:build !(ios && tvos)\n\n" + unsup,
		}, []string{"state.go must build only on ios with tvos", "player_other.go must exclude"}},
		{"engine file that binds jni may not build everywhere", map[string]string{
			"internal/media/amp/p.go":       "package amp\n\nimport _ \"" + jniPath + "\"\n",
			"internal/media/amp/p_other.go": "//go:build !android\n\n" + unsup,
		}, []string{"p.go must build only on android", "p_other.go must exclude"}},
		{"amp subpackage is an engine too", map[string]string{
			"internal/media/amp/jni/j.go":       "//go:build android\n\npackage jni\n",
			"internal/media/amp/jni/j_other.go": "//go:build !android\n\n" + unsup,
		}, nil},
		{"amp with cgo: _other.go must cover android without cgo", map[string]string{
			"internal/media/amp/p.go":       "//go:build android && cgo\n\npackage amp\n",
			"internal/media/amp/p_other.go": "//go:build !android\n\n" + unsup,
		}, []string{"builds no file (besides an untagged doc.go) on android"}},
		{"cmd and e2e are checked too", map[string]string{
			"cmd/spike/x_android.go": "//go:build android\n\npackage main\n",
			"e2e/x_ios.go":           "//go:build ios\n\npackage e2e\n",
		}, []string{"cmd/spike has files", "cmd/spike builds no file", "e2e has files", "e2e builds no file"}},
	}
	runCases(t, tests, buildTagsRule)
}

// TestBannersRule checks bannersRule on fake trees, including e2e and cmd.
func TestBannersRule(t *testing.T) {
	t.Parallel()

	good := "package p\n\n" + bannerBlock("Contracts") + "\n\nvar a int\n"
	tests := []fakeCase{
		{"ok", map[string]string{"internal/store/a.go": good}, nil},
		{"all four in order", map[string]string{"internal/store/a.go": "package p\n\n" + strings.Join([]string{
			bannerBlock("Contracts"), bannerBlock("Creation"), bannerBlock("Main functions"), bannerBlock("Helper Functions"),
		}, "\n\n") + "\n"}, nil},
		{"cmd", map[string]string{"cmd/x/a.go": "package p\n\n// ===\nvar a int\n"}, []string{"cmd/x/a.go:3: malformed banner"}},
		{"e2e", map[string]string{"e2e/a.go": "package p\n\n// ===\nvar a int\n"}, []string{"e2e/a.go:3: malformed banner"}},
		{"main", map[string]string{"main.go": "package p\n\n// ===\nvar a int\n"}, []string{"main.go:3: malformed banner"}},
		{"too short", map[string]string{"internal/store/a.go": strings.Replace(good, strings.Repeat("=", 68), strings.Repeat("=", 67), 1)}, []string{"malformed banner"}},
		{"unknown title", map[string]string{"internal/store/a.go": strings.ReplaceAll(good, "Contracts", "Helpers")}, []string{"malformed banner"}},
		{
			"order",
			map[string]string{"internal/store/a.go": "package p\n\n" + bannerBlock("Creation") + "\n\n" + bannerBlock("Contracts") + "\n"},
			[]string{"banner \"Contracts\" out of order"},
		},
		{
			"repeated",
			map[string]string{"internal/store/a.go": "package p\n\n" + bannerBlock("Creation") + "\n\n" + bannerBlock("Creation") + "\n"},
			[]string{"banner \"Creation\" out of order or repeated"},
		},
		{"blank line before", map[string]string{"internal/store/a.go": "package p\n" + bannerBlock("Contracts") + "\n"}, []string{"needs a blank line before"}},
		{"blank line after", map[string]string{"internal/store/a.go": "package p\n\n" + bannerBlock("Contracts") + "\nvar a int\n"}, []string{"needs a blank line after"}},
	}
	runCases(t, tests, bannersRule)
}

// TestViRule checks viRule on fake trees: precomposed letters, the accent
// list, d with stroke, and the combining marks of NFD text.
func TestViRule(t *testing.T) {
	t.Parallel()

	tests := []fakeCase{
		{"literal in non-test code", map[string]string{"internal/ui/mobile/a.go": "package v\n\nvar s = \"Thư viện\"\n"}, []string{"internal/ui/mobile/a.go has a Vietnamese string literal"}},
		{"raw string", map[string]string{"internal/store/a.go": "package v\n\nvar s = `Đang phát`\n"}, []string{"internal/store/a.go"}},
		{"one rune is enough", map[string]string{"internal/app/a.go": "package v\n\nvar s = \"ok ế\"\n"}, []string{"internal/app/a.go"}},
		{"d with stroke", map[string]string{"internal/app/a.go": "package v\n\nvar s = \"đ\"\n"}, []string{"internal/app/a.go"}},
		{"accent list: a grave", map[string]string{"internal/app/a.go": "package v\n\nvar s = \"à\"\n"}, []string{"internal/app/a.go"}},
		{"accent list: i tilde", map[string]string{"internal/app/a.go": "package v\n\nvar s = \"Ĩ\"\n"}, []string{"internal/app/a.go"}},
		{"escaped precomposed", map[string]string{"internal/app/a.go": "package v\n\nvar s = \"\\u1ea1\"\n"}, []string{"internal/app/a.go"}},
		{"NFD combining acute", map[string]string{"internal/app/a.go": "package v\n\nvar s = \"a\\u0301\"\n"}, []string{"internal/app/a.go"}},
		{"NFD combining dot below", map[string]string{"internal/app/a.go": "package v\n\nvar s = \"a\\u0323\"\n"}, []string{"internal/app/a.go"}},
		{"NFD combining grave typed", map[string]string{"internal/app/a.go": "package v\n\nvar s = \"e\u0300\"\n"}, []string{"internal/app/a.go"}},
		{"comments are free", map[string]string{"internal/app/a.go": "package v\n\n// Thư viện\nvar s = \"library\" // Đang phát\n"}, nil},
		{"test files are free", map[string]string{"internal/app/a_test.go": "package v\n\nvar s = \"Thư viện\"\n"}, nil},
		{"locales is exempt", map[string]string{"locales/locales.go": "package locales\n\nvar s = \"Thư viện\"\n"}, nil},
		{"cmd is scanned", map[string]string{"cmd/spike/a.go": "package v\n\nvar s = \"Thư viện\"\n"}, []string{"cmd/spike/a.go"}},
		{"root file is scanned", map[string]string{"mockfs.go": "package v\n\nvar s = \"Thư viện\"\n"}, []string{"mockfs.go"}},
		{"plain ASCII and other scripts", map[string]string{"internal/app/a.go": "package v\n\nvar s = \"Library \\u00fcber \\u00b7 \\u0394 日本\"\n"}, nil},
		{"combining marks above U+0323 are other scripts' business", map[string]string{"internal/app/a.go": "package v\n\nvar s = \"a\\u0330\"\n"}, nil},
	}
	runCases(t, tests, viRule)
}

// TestIsVietnamese checks the rune classes of isVietnamese at their edges.
func TestIsVietnamese(t *testing.T) {
	t.Parallel()

	tests := []struct {
		r    rune
		want bool
	}{
		{'a', false}, {'Z', false}, {'ü', false}, {'é', true}, {'ý', true}, {'ă', true}, {'ĩ', true}, {'ũ', true}, {'ơ', true}, {'ư', true},
		{'Đ', true}, {'đ', true}, {'Ơ', true}, {'Ư', true}, {'â', true}, {'ê', true}, {'ô', true}, {'ã', true}, {'õ', true},
		{0x1E9F, false}, {0x1EA0, true}, {0x1EF9, true}, {0x1EFA, false},
		{0x02FF, false}, {0x0300, true}, {0x0323, true}, {0x0324, false}, {0x036F, false},
		{'·', false}, {'日', false},
	}
	for _, tc := range tests {
		t.Run(string(tc.r), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, isVietnamese(tc.r), "U+%04X", tc.r)
		})
	}
	for _, r := range "àáãèéìíòóõùúýâêôăĩũ" {
		assert.True(t, isVietnamese(r), "%c", r)
		assert.True(t, utf8.ValidRune(r))
	}
}

// TestLinkedViolations checks the transitive check's matcher on a synthetic
// go list -deps output.
func TestLinkedViolations(t *testing.T) {
	t.Parallel()

	core := transitiveGroups[0]
	system := transitiveGroups[2]
	player := transitiveGroups[3]
	m := fakeModule
	tests := []struct {
		name  string
		group transitiveGroup
		deps  []string
		want  []string
	}{
		{"clean core", core, []string{"fmt", gdeskPath + "/foundation/core", m + "/internal/model", "golang.org/x/text/unicode/norm"}, nil},
		{"core links framework", core, []string{m + "/internal/api", gdeskPath + "/framework/ui"}, []string{gdeskPath + "/framework/ui is linked"}},
		{"core links a backend", core, []string{gdeskPath + "/bridge/backend/uikit"}, []string{"bridge/backend/uikit"}},
		{"core links kits", core, []string{kitsPath + "/mobile"}, []string{"kits/mobile"}},
		{"core links ui", core, []string{m + "/internal/ui/theme"}, []string{m + "/internal/ui/theme is linked"}},
		{"core links sys", core, []string{m + "/internal/sys"}, []string{"internal/sys"}},
		{"core links media", core, []string{m + "/internal/media/avf"}, []string{"internal/media/avf"}},
		{"core links tokens", core, []string{m + "/internal/tokens"}, []string{"internal/tokens"}},
		{"core links jni", core, []string{jniPath}, []string{"bridge/backend/android/jni is linked"}},
		{"system may link jni and its base", system, []string{jniPath, jvmPath, gdeskPath + "/bridge/system/secret"}, nil},
		{"system links the android driver", system, []string{gdeskPath + "/bridge/backend/android"}, []string{"bridge/backend/android is linked"}},
		{"system links media", system, []string{m + "/internal/media"}, []string{"internal/media"}},
		{"clean player", player, []string{"fmt", m + "/internal/media", m + "/internal/api", gdeskPath + "/foundation/scene"}, nil},
		{"player links an engine", player, []string{m + "/internal/media/avf"}, []string{"internal/media/avf"}},
		{"player links ui", player, []string{m + "/internal/ui/kit"}, []string{"internal/ui"}},
		{"player links the framework", player, []string{gdeskPath + "/framework/ui"}, []string{"framework/ui is linked"}},
		{"lookalike module path", core, []string{m + "/internal/uix"}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := linkedViolations(m, tc.deps, tc.group)
			require.Len(t, got, len(tc.want), "%q", got)
			for i, w := range tc.want {
				assert.Contains(t, got[i], w)
			}
		})
	}
}

// TestIsProgram checks the program-directory rule: the root and cmd/<name>.
func TestIsProgram(t *testing.T) {
	t.Parallel()

	cases := []struct {
		dir  string
		want bool
	}{
		{".", true},
		{"cmd/contractcheck", true},
		{"cmd", false},
		{"cmd/contractcheck/internal", false},
		{"internal/app", false},
		{"internal/cmd/x", false},
		{"cmdx/y", false},
	}
	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, isProgram(tc.dir))
		})
	}
}

// TestLoadFS checks that loadFS reads the whole tree, skips dist, testdata,
// vendor and the root-level local, reference, mock and dot directories, and
// keeps internal/api/mock.
func TestLoadFS(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"main.go":                       {Data: []byte(src(""))},
		"mockfs.go":                     {Data: []byte(src(""))},
		"internal/a/a.go":               {Data: []byte(src(""))},
		"internal/api/mock/m.go":        {Data: []byte(src(""))},
		"cmd/x/a.go":                    {Data: []byte(src(""))},
		"cmd/x/dist/gen.go":             {Data: []byte("not go")},
		"internal/a/testdata/t.go":      {Data: []byte("not go")},
		"e2e/a.go":                      {Data: []byte(src(""))},
		"locales/l.go":                  {Data: []byte(src(""))},
		"reference/prototype/x.go":      {Data: []byte("not go")},
		"local/plans/x.go":              {Data: []byte("not go")},
		"mock/dancefitvn-mock-api/x.go": {Data: []byte("not go")},
		".claude/x.go":                  {Data: []byte("not go")},
		"README.md":                     {Data: []byte("# x")},
	}
	r, err := loadFS(fsys, fakeModule)
	require.NoError(t, err)

	var paths []string
	for _, f := range r.files {
		paths = append(paths, f.Path)
	}
	assert.ElementsMatch(t, []string{
		"main.go", "mockfs.go", "internal/a/a.go", "internal/api/mock/m.go", "cmd/x/a.go", "e2e/a.go", "locales/l.go",
	}, paths)
}

// TestModulePath checks the go.mod reader.
func TestModulePath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "dancefitvn-app", modulePath("module dancefitvn-app\n\ngo 1.27.0\n"))
	assert.Empty(t, modulePath("go 1.27.0\n"))
}

// TestNameGOOS checks the file-name constraint reader, tvos included.
func TestNameGOOS(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"a.go": "", "a_ios.go": "ios", "a_android_arm64.go": "android", "a_ios_test.go": "ios",
		"a_tvos.go": "", "a_other.go": "", "ios.go": "", "a_arm64.go": "",
	}
	for name, want := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, want, nameGOOS(name))
		})
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// src returns a Go file with an optional //go:build line and the imports.
func src(build string, imports ...string) string {
	var b strings.Builder
	if build != "" {
		b.WriteString("//go:build " + build + "\n\n")
	}
	b.WriteString("package p\n")
	for _, imp := range imports {
		b.WriteString("\nimport _ \"" + imp + "\"\n")
	}

	return b.String()
}

// bannerBlock returns the three-line banner of title.
func bannerBlock(title string) string {
	ruler := "// " + strings.Repeat("=", 68)
	n := 66 - len(title)
	left := n / 2

	return ruler + "\n// " + strings.Repeat("=", left) + " " + title + " " + strings.Repeat("=", n-left) + "\n" + ruler
}

// runCases runs rule on the fake tree of each case and matches its reports
// one to one against the wanted substrings.
func runCases(t *testing.T, tests []fakeCase, rule func(repo) []string) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fsys := fstest.MapFS{}
			for p, data := range tc.files {
				fsys[p] = &fstest.MapFile{Data: []byte(data)}
			}
			r, err := loadFS(fsys, fakeModule)
			require.NoError(t, err)

			got := rule(r)
			require.Len(t, got, len(tc.want), "reports: %q", got)
			rest := append([]string(nil), got...)
			for _, w := range tc.want {
				found := -1
				for i, g := range rest {
					if strings.Contains(g, w) {
						found = i

						break
					}
				}
				require.GreaterOrEqual(t, found, 0, "no report contains %q in %q", w, got)
				rest = append(rest[:found], rest[found+1:]...)
			}
		})
	}
}
