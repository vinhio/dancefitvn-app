// Adapted from jplayer-mobile/internal/archtest/archtest_test.go at 49de3c2
// (pure rule(repo) functions, loadFS, packageBuild, engineBuild, the
// go list -deps transitive check, isVietnamese) and from
// jplayer-tv/internal/archtest/archtest_test.go at 781bee5 (tvos tag);
// layers per docs/plan/PLAN.md §4.2 and local/plans/P1.md Q-P1-12.

package archtest

import (
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// gdeskPath is the root of the gDesk module.
const gdeskPath = "github.com/gdeskdev/desk"

// hostPath is the gDesk package tree that only the app and programs may import.
const hostPath = gdeskPath + "/framework/host"

// kitsPath is the gDesk kits layer; only programs may import it.
const kitsPath = gdeskPath + "/kits"

// jniPath is the Android JNI binding, the one gDesk backend package that the
// Android engine, the system layer and the app may import (PLAN §4.2).
const jniPath = gdeskPath + "/bridge/backend/android/jni"

// jvmPath is the package the JNI binding itself builds on.
const jvmPath = gdeskPath + "/bridge/backend/android/internal/jvm"

// ffiPath is the gDesk foreign-function tree (objc, cf, dl).
const ffiPath = gdeskPath + "/foundation/ffi"

// bannerTitles are the banner titles of gDesk's gFly style, in order.
var bannerTitles = []string{"Contracts", "Creation", "Main functions", "Helper Functions"}

// coreDirs are the core packages (PLAN §4.2) and, through underAny, their
// subpackages: stdlib, foundation/core and each other only.
var coreDirs = []string{
	"internal/model", "internal/api", "internal/api/mock", "internal/auth", "internal/store",
	"internal/progress", "internal/download", "internal/format", "internal/qr", "internal/pairing", "internal/clock",
	"internal/avatar", "internal/safe",
}

// uiCommonDirs are the shared interface packages: tokens, theme, icons,
// posters and the kit widgets.
var uiCommonDirs = []string{
	"internal/tokens", "internal/ui/theme", "internal/ui/icon", "internal/ui/poster", "internal/ui/kit",
}

// tvstoreDir is the NSUserDefaults backend of the tvOS store (P7.5d, Q-P7-16):
// under the core's internal/store but a layer of its own, a leaf over
// foundation/ffi/objc that only the app imports.
const tvstoreDir = "internal/store/tvstore"

// leafDirs are the resource packages that import nothing of the module.
var leafDirs = []string{"locales", "assets"}

// layer is one row of the PLAN §4.2 table: the packages it covers (but the
// subtrees in except, which have a layer of their own), the packages of this
// module they may import (besides themselves), and the gDesk trees (besides
// foundation/core) they may import.
type layer struct {
	name    string
	dirs    []string
	except  []string
	modules []string
	gdesk   []string
}

// layers are the import allow-lists. Standard library imports are always
// allowed; test files also take testify.
var layers = []layer{
	{
		name:    "core",
		dirs:    coreDirs,
		except:  []string{tvstoreDir},
		modules: coreDirs,
		gdesk:   []string{gdeskPath + "/foundation/core"},
	},
	{
		// The NSUserDefaults backend imports nothing of the module: it meets
		// store.Backend by its method set alone.
		name:  "store tvos",
		dirs:  []string{tvstoreDir},
		gdesk: []string{gdeskPath + "/foundation/core", ffiPath + "/objc"},
	},
	{
		// foundation/scene is the video surface type that internal/media
		// exposes; foundation/ffi is narrowed further by ffiRule.
		name:    "engine",
		dirs:    []string{"internal/media"},
		modules: slices.Concat(coreDirs, []string{"internal/media"}),
		gdesk: []string{
			gdeskPath + "/foundation/core", gdeskPath + "/foundation/ffi", gdeskPath + "/foundation/platform",
			gdeskPath + "/foundation/scene", jniPath,
		},
	},
	{
		// The playback core (PLAN §4.2, Q-P4-10): the engine's interface,
		// never an engine package of a platform.
		name:    "player",
		dirs:    []string{"internal/player"},
		modules: slices.Concat(coreDirs, []string{"internal/media", "internal/player"}),
		gdesk:   []string{gdeskPath + "/foundation/core"},
	},
	{
		name:    "system",
		dirs:    []string{"internal/sys"},
		modules: slices.Concat(coreDirs, []string{"internal/sys"}),
		gdesk: []string{
			gdeskPath + "/foundation/core", gdeskPath + "/foundation/platform", ffiPath,
			gdeskPath + "/bridge/system", jniPath,
		},
	},
	{
		name:  "leaf",
		dirs:  leafDirs,
		gdesk: []string{gdeskPath + "/foundation/core"},
	},
	{
		// bridge/text carries the font kit of the theme.
		name:    "ui common",
		dirs:    uiCommonDirs,
		modules: slices.Concat(coreDirs, uiCommonDirs, []string{"assets", "locales"}),
		gdesk: []string{
			gdeskPath + "/foundation", gdeskPath + "/framework/ui", gdeskPath + "/framework/components/headless",
			gdeskPath + "/bridge/text",
		},
	},
	{
		name:    "ui mobile",
		dirs:    []string{"internal/ui/mobile"},
		modules: slices.Concat(coreDirs, uiCommonDirs, []string{"assets", "locales", "internal/media", "internal/player", "internal/sys", "internal/ui/mobile"}),
		gdesk: []string{
			gdeskPath + "/foundation", gdeskPath + "/framework/ui", gdeskPath + "/framework/components",
			gdeskPath + "/bridge/text",
		},
	},
	{
		name:    "ui tv",
		dirs:    []string{"internal/ui/tv"},
		modules: slices.Concat(coreDirs, uiCommonDirs, []string{"assets", "locales", "internal/media", "internal/player", "internal/sys", "internal/ui/tv"}),
		gdesk: []string{
			gdeskPath + "/foundation", gdeskPath + "/framework/ui", gdeskPath + "/framework/components",
			gdeskPath + "/bridge/text",
		},
	},
}

// ffiUsers are the places that may import foundation/ffi: the Apple engines,
// the Android engine (dl), the system layer and the app.
var ffiUsers = []string{
	"internal/media/avf", "internal/media/avftv", "internal/media/amp", "internal/sys", "internal/app", tvstoreDir,
}

// jniUsers are the places that may import the JNI binding.
var jniUsers = []string{"internal/media/amp", "internal/sys", "internal/app"}

// transitiveGroup is a set of packages whose go list -deps output must hold
// none of the packages banned (module-relative trees or gDesk paths), except
// the ones in allow.
type transitiveGroup struct {
	name  string
	dirs  []string
	bans  []string
	allow []string
}

// transitiveGroups are what the linked dependencies of the core, the engines
// and the system layer must not contain, on every platform (the objects of
// the transitive check, as jplayer-mobile TestStoreBuildLinksNoOnlineFeature).
var transitiveGroups = []transitiveGroup{
	{
		name: "core",
		dirs: coreDirs,
		bans: []string{
			gdeskPath + "/framework", gdeskPath + "/bridge/backend", kitsPath,
			"@internal/ui", "@internal/app", "@internal/sys", "@internal/media", "@internal/tokens", "@internal/player",
		},
	},
	{
		name: "engine",
		dirs: []string{"internal/media", "internal/media/avf", "internal/media/avftv", "internal/media/amp", "internal/media/enginekit", "internal/media/fake"},
		bans: []string{
			gdeskPath + "/framework", gdeskPath + "/bridge/backend", kitsPath,
			"@internal/ui", "@internal/app", "@internal/sys",
		},
		allow: []string{jniPath, jvmPath},
	},
	{
		name: "system",
		dirs: []string{"internal/sys"},
		bans: []string{
			gdeskPath + "/framework", gdeskPath + "/bridge/backend", kitsPath,
			"@internal/ui", "@internal/app", "@internal/media",
		},
		allow: []string{jniPath, jvmPath},
	},
	{
		// The player links the media interface and the core, no engine of a
		// platform, no interface, no system layer.
		name: "player",
		dirs: []string{"internal/player"},
		bans: []string{
			gdeskPath + "/framework", gdeskPath + "/bridge/backend", kitsPath,
			"@internal/ui", "@internal/app", "@internal/sys", "@internal/tokens",
			"@internal/media/avf", "@internal/media/avftv", "@internal/media/amp",
		},
	},
}

// platform is one build environment the build rules evaluate files in.
type platform struct {
	name string
	// goos is the GOOS of the platform (tvOS is GOOS=ios with the tvos tag).
	goos string
	// tags are the GOOS-implied tags that hold, besides cgo.
	tags []string
	// cgo is whether cgo is enabled.
	cgo bool
}

// has reports whether tag holds on p.
func (p platform) has(tag string) bool { return slices.Contains(p.tags, tag) }

// platforms are the combinations of {ios, tvos, android, darwin, linux} and
// {no cgo, cgo} that every build constraint is evaluated in.
var platforms = func() []platform {
	base := []platform{
		{name: "ios", goos: "ios", tags: []string{"ios", "darwin", "unix"}},
		{name: "tvos", goos: "ios", tags: []string{"ios", "tvos", "darwin", "unix"}},
		{name: "android", goos: "android", tags: []string{"android", "linux", "unix"}},
		{name: "darwin", goos: "darwin", tags: []string{"darwin", "unix"}},
		{name: "linux", goos: "linux", tags: []string{"linux", "unix"}},
	}
	var out []platform
	for _, p := range base {
		out = append(out, p)
		p.cgo = true
		p.name += "+cgo"
		out = append(out, p)
	}

	return out
}()

// engines maps the engine packages to the platforms they are for.
var engines = map[string]struct {
	label string
	on    func(platform) bool
}{
	"internal/media/avf":   {"ios without tvos", func(p platform) bool { return p.has("ios") && !p.has("tvos") }},
	"internal/media/avftv": {"ios with tvos", func(p platform) bool { return p.has("ios") && p.has("tvos") }},
	"internal/media/amp":   {"android", func(p platform) bool { return p.has("android") }},
}

// docTagDirs are the interface shells whose doc.go carries the shell's build
// tag (mobile is !tvos, tv is tvos).
var docTagDirs = []string{"internal/ui/mobile", "internal/ui/tv"}

// knownOS are the GOOS values that a file name suffix may select.
var knownOS = []string{
	"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "js",
	"linux", "nacl", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows",
}

// knownArch are the GOARCH values that a file name suffix may select.
var knownArch = []string{
	"386", "amd64", "arm", "arm64", "loong64", "mips", "mipsle", "mips64", "mips64le",
	"ppc64", "ppc64le", "riscv64", "s390x", "wasm",
}

// vietnameseLetters are the letters, besides U+1EA0-U+1EF9, that Vietnamese
// writes with a diacritic or its own stroke.
const vietnameseLetters = "àáâãèéêìíòóôõùúýăđĩũơưÀÁÂÃÈÉÊÌÍÒÓÔÕÙÚÝĂĐĨŨƠƯ"

// repo is the Go code of the repository that the rules read.
type repo struct {
	// module is the module path from go.mod.
	module string
	// files are the .go files of the repository.
	files []sourceFile
}

// sourceFile is one Go file, with what the rules read from it.
type sourceFile struct {
	// Path is the file's path from the repository root, slash-separated.
	Path string
	// Dir is the file's directory from the repository root.
	Dir string
	// Base is the file's name.
	Base string
	// Imports are the import paths of the file.
	Imports []string
	// Build is the file's //go:build expression, or nil without one.
	Build constraint.Expr
	// Lines are the file's lines, for the banner rules.
	Lines []string
	// Strings are the values of the string literals of the file.
	Strings []string
	// RefsUnsupported is whether the file mentions core.ErrUnsupported.
	RefsUnsupported bool
	// ReturnsError is whether a function of the file returns an error.
	ReturnsError bool
}

func (f sourceFile) isTest() bool { return strings.HasSuffix(f.Base, "_test.go") }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestLayerImports checks the real repository against layerRule (PLAN §4.2).
func TestLayerImports(t *testing.T) {
	t.Parallel()

	assert.Empty(t, layerRule(loadRepo(t)))
}

// TestCoreDoesNotImportUI checks that the core imports no interface, app,
// system or engine package nor a gDesk drawing layer.
func TestCoreDoesNotImportUI(t *testing.T) {
	t.Parallel()

	r := loadRepo(t)
	assert.Empty(t, layerRule(only(r, "core")))
	assert.NotEmpty(t, coreFiles(r), "no core file found: coreDirs out of date")
}

// TestMobileAndTVApart checks mobileTVRule: neither shell imports the other.
func TestMobileAndTVApart(t *testing.T) {
	t.Parallel()

	assert.Empty(t, mobileTVRule(loadRepo(t)))
}

// TestFFIImports checks the real repository against ffiRule.
func TestFFIImports(t *testing.T) {
	t.Parallel()

	assert.Empty(t, ffiRule(loadRepo(t)))
}

// TestOnlyAppImportsHost checks the real repository against hostRule.
func TestOnlyAppImportsHost(t *testing.T) {
	t.Parallel()

	assert.Empty(t, hostRule(loadRepo(t)))
}

// TestOnlyMainImportsKits checks the real repository against kitsRule.
func TestOnlyMainImportsKits(t *testing.T) {
	t.Parallel()

	assert.Empty(t, kitsRule(loadRepo(t)))
}

// TestOnlyProgramsImportApp checks the real repository against appRule.
func TestOnlyProgramsImportApp(t *testing.T) {
	t.Parallel()

	assert.Empty(t, appRule(loadRepo(t)))
}

// TestOnlyAppImportsTVStore checks the real repository against tvstoreRule.
func TestOnlyAppImportsTVStore(t *testing.T) {
	t.Parallel()

	assert.Empty(t, tvstoreRule(loadRepo(t)))
}

// TestCgoOnlyInEngines checks the real repository against cgoRule.
func TestCgoOnlyInEngines(t *testing.T) {
	t.Parallel()

	assert.Empty(t, cgoRule(loadRepo(t)))
}

// TestBuildTags checks the real repository against buildTagsRule.
func TestBuildTags(t *testing.T) {
	t.Parallel()

	assert.Empty(t, buildTagsRule(loadRepo(t)))
}

// TestBanners checks the real repository against bannersRule.
func TestBanners(t *testing.T) {
	t.Parallel()

	assert.Empty(t, bannersRule(loadRepo(t)))
}

// TestNoVietnameseLiterals checks the real repository against viRule.
func TestNoVietnameseLiterals(t *testing.T) {
	t.Parallel()

	assert.Empty(t, viRule(loadRepo(t)))
}

// TestLinkedDepsStayInTheirLayer checks what the go command links (go list
// -deps) for iOS, tvOS, Android and the Mac: the core, the engines and the
// system layer pull in no package of a layer they must not reach, however
// indirectly.
func TestLinkedDepsStayInTheirLayer(t *testing.T) {
	t.Parallel()

	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go command")
	}
	mod, err := os.ReadFile("../../go.mod")
	require.NoError(t, err)
	module := modulePath(string(mod))
	targets := []struct{ name, goos, tags string }{
		{"ios", "ios", ""},
		{"tvos", "ios", "tvos"},
		{"android", "android", ""},
		{"darwin", "darwin", ""},
	}
	for _, target := range targets {
		for _, g := range transitiveGroups {
			t.Run(target.name+"/"+g.name, func(t *testing.T) {
				t.Parallel()

				args := []string{"list", "-deps"}
				if target.tags != "" {
					args = append(args, "-tags", target.tags)
				}
				for _, d := range g.dirs {
					if _, err := os.Stat("../../" + d); err == nil {
						args = append(args, "./"+d)
					}
				}
				cmd := exec.Command(goTool, args...)
				cmd.Dir = "../.."
				cmd.Env = append(os.Environ(), "GOOS="+target.goos, "GOARCH=arm64", "CGO_ENABLED=1", "GOFLAGS=")
				out, err := cmd.CombinedOutput()
				require.NoError(t, err, "%s", out)
				assert.Empty(t, linkedViolations(module, strings.Fields(string(out)), g))
			})
		}
	}
}

// layerRule: every file of a layer imports only the standard library, the
// trees its layer lists, and testify (tests only). Subpackages follow their
// parent. Files outside every layer (app, programs, tests of other kinds) are
// free here; the narrower rules below cover them.
func layerRule(r repo) []string {
	var out []string
	for _, f := range r.files {
		for _, l := range layers {
			if !l.covers(f.Dir) {
				continue
			}
			for _, imp := range f.Imports {
				if !layerAllows(r.module, l, f, imp) {
					out = append(out, f.Path+" imports "+imp+" ("+l.name+" layer)")
				}
			}
		}
	}

	return out
}

// covers reports whether dir belongs to the layer: under one of its dirs and
// none of its exceptions.
func (l layer) covers(dir string) bool {
	return underAny(dir, l.dirs...) && !underAny(dir, l.except...)
}

// layerAllows reports whether the file of layer l may import imp.
func layerAllows(module string, l layer, f sourceFile, imp string) bool {
	switch {
	case isStdlib(imp):
		return true
	case underAny(imp, module):
		rel := strings.TrimPrefix(imp, module+"/")

		return imp != module && underAny(rel, l.modules...)
	case underAny(imp, gdeskPath):
		return underAny(imp, l.gdesk...)
	case f.isTest():
		return underAny(imp, "github.com/stretchr/testify")
	default:
		return false
	}
}

// mobileTVRule: internal/ui/mobile and internal/ui/tv import each other never.
func mobileTVRule(r repo) []string {
	out := violating(r.files,
		func(f sourceFile) bool { return underAny(f.Dir, "internal/ui/mobile") },
		func(imp string) bool { return underAny(imp, r.module+"/internal/ui/tv") })

	return append(out, violating(r.files,
		func(f sourceFile) bool { return underAny(f.Dir, "internal/ui/tv") },
		func(imp string) bool { return underAny(imp, r.module+"/internal/ui/mobile") })...)
}

// ffiRule: only the Apple engines, the Android engine, internal/sys and the
// app (and programs) import foundation/ffi; only the Android engine, the
// system layer and the app import the JNI binding.
func ffiRule(r repo) []string {
	out := violating(r.files,
		func(f sourceFile) bool {
			return !isProgram(f.Dir) && !underAny(f.Dir, ffiUsers...) && !underAny(f.Dir, "e2e")
		},
		func(imp string) bool { return underAny(imp, ffiPath) })

	return append(out, violating(r.files,
		func(f sourceFile) bool {
			return !isProgram(f.Dir) && !underAny(f.Dir, jniUsers...) && !underAny(f.Dir, "e2e")
		},
		func(imp string) bool { return underAny(imp, jniPath) })...)
}

// hostRule: only the app tree, programs and e2e import gDesk's host tree.
func hostRule(r repo) []string {
	return violating(r.files,
		func(f sourceFile) bool { return !isProgram(f.Dir) && !underAny(f.Dir, "internal/app", "e2e") },
		func(imp string) bool { return underAny(imp, hostPath) })
}

// kitsRule: only programs and e2e import gDesk's kits, and nobody the desktop
// or web kits.
func kitsRule(r repo) []string {
	got := violating(r.files,
		func(f sourceFile) bool { return !isProgram(f.Dir) && !underAny(f.Dir, "e2e") },
		func(imp string) bool { return underAny(imp, kitsPath) })

	return append(got, violating(r.files,
		func(sourceFile) bool { return true },
		func(imp string) bool { return underAny(imp, kitsPath+"/desktop", kitsPath+"/web") })...)
}

// tvstoreRule: only the app, programs and e2e import the NSUserDefaults
// backend (Q-P7-16); the core reaches it through store.Backend.
func tvstoreRule(r repo) []string {
	return violating(r.files,
		func(f sourceFile) bool {
			return !isProgram(f.Dir) && !underAny(f.Dir, "internal/app", "e2e", tvstoreDir)
		},
		func(imp string) bool { return underAny(imp, r.module+"/"+tvstoreDir) })
}

// appRule: only programs and e2e import internal/app.
func appRule(r repo) []string {
	return violating(r.files,
		func(f sourceFile) bool { return !isProgram(f.Dir) && !underAny(f.Dir, "internal/app", "e2e") },
		func(imp string) bool { return underAny(imp, r.module+"/internal/app") })
}

// cgoRule (CLAUDE.md §5): only internal/media/avf and internal/media/amp
// import "C".
func cgoRule(r repo) []string {
	var out []string
	for _, f := range r.files {
		if slices.Contains(f.Imports, "C") && !underAny(f.Dir, "internal/media/avf", "internal/media/amp") {
			out = append(out, f.Path+" uses cgo outside the engines")
		}
	}

	return out
}

// buildTagsRule: doc.go has no constraint (except in the interface shells);
// a file named _tvos names the tvos tag; a package with files for ios,
// tvos or android has an _other.go and builds some file in every platform;
// every _other.go builds on the Mac and is exclusive with a platform file;
// the engine packages build their files only on their platform, and their
// _other.go refer to core.ErrUnsupported (as does any _other.go that returns
// errors). Constraints are evaluated for {ios, tvos, android, darwin, linux}
// x {no cgo, cgo}.
func buildTagsRule(r repo) []string {
	var out []string
	dirs := map[string][]sourceFile{}
	for _, f := range r.files {
		if f.isTest() {
			continue
		}
		dirs[f.Dir] = append(dirs[f.Dir], f)
	}
	for _, dir := range sortedKeys(dirs) {
		files := dirs[dir]
		for _, f := range files {
			if f.Base == "doc.go" && (f.Build != nil || nameGOOS(f.Base) != "") && !underAny(dir, docTagDirs...) {
				out = append(out, f.Path+" must have no build tag")
			}
			if strings.HasSuffix(f.Base, "_tvos.go") && (f.Build == nil || !exprMentions(f.Build, "tvos")) {
				out = append(out, f.Path+" is named _tvos but its //go:build does not name tvos")
			}
		}
		out = append(out, packageBuild(dir, files)...)
		for _, root := range sortedKeys(engines) {
			if underAny(dir, root) {
				out = append(out, engineBuild(root, files)...)
			}
		}
	}

	return out
}

// packageBuild applies the rules of every package with platform files.
func packageBuild(dir string, files []sourceFile) []string {
	var out []string
	darwin := platforms[slices.IndexFunc(platforms, func(p platform) bool { return p.name == "darwin" })]
	var tagged, others []sourceFile
	for _, f := range files {
		switch {
		case f.Base == "doc.go" && !docTagged(dir, f):
		case strings.HasSuffix(f.Base, "_other.go"):
			others = append(others, f)
		case mentionsPlatform(f) && !builds(f, darwin):
			tagged = append(tagged, f)
		}
	}
	if len(tagged) > 0 && len(others) == 0 {
		out = append(out, dir+" has files for ios, tvos or android but no _other.go")
	}
	if len(tagged) > 0 {
		var missing []string
		for _, p := range platforms {
			if !slices.ContainsFunc(files, func(f sourceFile) bool { return (f.Base != "doc.go" || docTagged(dir, f)) && builds(f, p) }) {
				missing = append(missing, p.name)
			}
		}
		if len(missing) > 0 {
			out = append(out, dir+" builds no file (besides an untagged doc.go) on "+strings.Join(missing, ", "))
		}
	}
	for _, o := range others {
		var skipped []string
		for _, p := range platforms {
			if p.goos == "darwin" && !builds(o, p) {
				skipped = append(skipped, p.name)
			}
		}
		if len(skipped) > 0 {
			out = append(out, o.Path+" must build on "+strings.Join(skipped, ", ")+", where go test runs")
		}
		exclusive := slices.ContainsFunc(tagged, func(f sourceFile) bool {
			return !slices.ContainsFunc(platforms, func(p platform) bool { return builds(f, p) && builds(o, p) })
		})
		if !exclusive {
			out = append(out, o.Path+" must exclude exactly what a platform file of its package builds")
		}
		if o.ReturnsError && !o.RefsUnsupported {
			out = append(out, o.Path+" returns errors but does not refer to core.ErrUnsupported")
		}
	}

	return out
}

// docTagged reports whether f is a doc.go that carries the build tag of its
// interface shell.
func docTagged(dir string, f sourceFile) bool {
	return f.Base == "doc.go" && f.Build != nil && underAny(dir, docTagDirs...)
}

// engineBuild applies the rules of the engine package root: its files build
// only on its platform (and cgo where they say so), and its _other.go builds
// everywhere else and refers to core.ErrUnsupported.
func engineBuild(root string, files []sourceFile) []string {
	var out []string
	eng := engines[root]
	for _, f := range files {
		if f.Base == "doc.go" {
			continue
		}
		if strings.HasSuffix(f.Base, "_other.go") {
			var missing, extra []string
			for _, p := range platforms {
				if !eng.on(p) && !builds(f, p) {
					missing = append(missing, p.name)
				}
				if eng.on(p) && p.cgo && builds(f, p) {
					extra = append(extra, p.name)
				}
			}
			if len(missing) > 0 {
				out = append(out, f.Path+" must build on every platform but "+eng.label+", not on "+strings.Join(missing, ", "))
			}
			if len(extra) > 0 {
				out = append(out, f.Path+" must not build on "+strings.Join(extra, ", "))
			}
			if !f.RefsUnsupported {
				out = append(out, f.Path+" must refer to core.ErrUnsupported")
			}

			continue
		}
		if !slices.ContainsFunc(platforms, func(p platform) bool { return eng.on(p) && builds(f, p) }) {
			out = append(out, f.Path+" never builds on "+eng.label)
		}
		if !bindsPlatform(f) {
			continue
		}
		var extra []string
		for _, p := range platforms {
			if !eng.on(p) && builds(f, p) {
				extra = append(extra, p.name)
			}
		}
		if len(extra) > 0 {
			out = append(out, f.Path+" must build only on "+eng.label+", but builds on "+strings.Join(extra, ", "))
		}
	}

	return out
}

// bindsPlatform reports whether f uses cgo, foundation/ffi or the JNI
// binding: such a file of an engine builds only on the engine's platform,
// while a pure Go file (state, config, limits) may build everywhere so that
// go test checks it on the Mac.
func bindsPlatform(f sourceFile) bool {
	return slices.ContainsFunc(f.Imports, func(imp string) bool {
		return imp == "C" || underAny(imp, ffiPath, jniPath)
	})
}

// bannersRule: every banner block is three well-formed lines with a known
// title, in order, set off by blank lines.
func bannersRule(r repo) []string {
	var out []string
	for _, f := range r.files {
		lines := f.Lines
		seen := -1
		for i := 0; i < len(lines); i++ {
			if !strings.HasPrefix(lines[i], "// ===") {
				continue
			}
			title, ok := banner(lines, i)
			if !ok {
				out = append(out, f.Path+":"+strconv.Itoa(i+1)+": malformed banner block")
				i += 2

				continue
			}
			order := slices.Index(bannerTitles, title)
			if order <= seen {
				out = append(out, f.Path+":"+strconv.Itoa(i+2)+": banner "+strconv.Quote(title)+" out of order or repeated")
			}
			seen = order
			if i > 0 && lines[i-1] != "" {
				out = append(out, f.Path+":"+strconv.Itoa(i+1)+": banner needs a blank line before it")
			}
			if i+3 < len(lines) && lines[i+3] != "" {
				out = append(out, f.Path+":"+strconv.Itoa(i+3)+": banner needs a blank line after it")
			}
			i += 2
		}
	}

	return out
}

// viRule: no non-test file outside locales/ has a string literal with
// Vietnamese text (CLAUDE.md §4): letters with diacritics, d with stroke, and
// the combining marks U+0300-U+0323 that an NFD text carries.
func viRule(r repo) []string {
	var out []string
	for _, f := range r.files {
		if f.isTest() || underAny(f.Dir, "locales") {
			continue
		}
		for _, s := range f.Strings {
			if strings.IndexFunc(s, isVietnamese) >= 0 {
				out = append(out, f.Path+" has a Vietnamese string literal "+strconv.Quote(s))
			}
		}
	}

	return out
}

// linkedViolations returns the packages of deps (a go list -deps output) that
// group g bans. A ban is a gDesk path, or "@tree" for a tree of this module;
// g.allow lists gDesk paths exempt from the bans.
func linkedViolations(module string, deps []string, g transitiveGroup) []string {
	var out []string
	for _, pkg := range deps {
		if underAny(pkg, g.allow...) {
			continue
		}
		for _, ban := range g.bans {
			full := ban
			if tree, ok := strings.CutPrefix(ban, "@"); ok {
				full = module + "/" + tree
			}
			if underAny(pkg, full) {
				out = append(out, pkg+" is linked ("+g.name+" bans "+ban+")")
			}
		}
	}

	return out
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// isProgram reports whether dir is a program directory: the root "." or a
// standalone command directly under cmd/ (cmd/<name>, not deeper).
func isProgram(dir string) bool {
	return dir == "." || (strings.HasPrefix(dir, "cmd/") && strings.Count(dir, "/") == 1)
}

// only returns r restricted to the files of the named layer.
func only(r repo, name string) repo {
	var keep []layer
	for _, l := range layers {
		if l.name == name {
			keep = append(keep, l)
		}
	}
	out := repo{module: r.module}
	for _, f := range r.files {
		for _, l := range keep {
			if l.covers(f.Dir) {
				out.files = append(out.files, f)

				break
			}
		}
	}

	return out
}

// coreFiles returns the files of the core layer.
func coreFiles(r repo) []sourceFile { return only(r, "core").files }

// loadRepo reads the real repository, two levels above the package.
func loadRepo(t *testing.T) repo {
	t.Helper()

	root := os.DirFS("../..")
	mod, err := fs.ReadFile(root, "go.mod")
	require.NoError(t, err)
	r, err := loadFS(root, modulePath(string(mod)))
	require.NoError(t, err)
	require.NotEmpty(t, r.files)

	return r
}

// loadFS reads every Go file of fsys except those under the root-level
// local/, reference/, mock/ and dot directories, and those under dist,
// testdata, vendor and node_modules anywhere.
func loadFS(fsys fs.FS, module string) (repo, error) {
	r := repo{module: module}
	var paths []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			skip := slices.Contains([]string{"dist", "testdata", "vendor", "node_modules"}, name)
			if !strings.Contains(p, "/") {
				skip = skip || slices.Contains([]string{"local", "reference", "mock"}, name)
			}
			if p != "." && (skip || strings.HasPrefix(name, ".")) {
				return fs.SkipDir
			}

			return nil
		}
		if strings.HasSuffix(p, ".go") {
			paths = append(paths, p)
		}

		return nil
	})
	if err != nil {
		return repo{}, err
	}
	for _, p := range paths {
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return repo{}, err
		}
		f, err := parseSource(p, data)
		if err != nil {
			return repo{}, err
		}
		r.files = append(r.files, f)
	}

	return r, nil
}

// modulePath returns the path of the module line of a go.mod file.
func modulePath(gomod string) string {
	for _, line := range strings.Split(gomod, "\n") {
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest)
		}
	}

	return ""
}

// parseSource reads the facts the rules need from the Go file at p.
func parseSource(p string, data []byte) (sourceFile, error) {
	file, err := parser.ParseFile(token.NewFileSet(), p, data, 0)
	if err != nil {
		return sourceFile{}, err
	}
	f := sourceFile{
		Path:  p,
		Dir:   path.Dir(p),
		Base:  path.Base(p),
		Lines: strings.Split(string(data), "\n"),
	}
	coreName := ""
	for _, imp := range file.Imports {
		ip, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return sourceFile{}, err
		}
		f.Imports = append(f.Imports, ip)
		if path.Base(ip) == "core" {
			coreName = "core"
			if imp.Name != nil {
				coreName = imp.Name.Name
			}
		}
	}
	for _, line := range f.Lines {
		if strings.HasPrefix(line, "package ") {
			break
		}
		if constraint.IsGoBuild(line) {
			f.Build, err = constraint.Parse(line)
			if err != nil {
				return sourceFile{}, err
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.BasicLit:
			if n.Kind == token.STRING {
				if s, err := strconv.Unquote(n.Value); err == nil {
					f.Strings = append(f.Strings, s)
				}
			}
		case *ast.SelectorExpr:
			if x, ok := n.X.(*ast.Ident); ok && coreName != "" && x.Name == coreName && n.Sel.Name == "ErrUnsupported" {
				f.RefsUnsupported = true
			}
		case *ast.FuncDecl:
			if returnsError(n) && makesError(n) {
				f.ReturnsError = true
			}
		}

		return true
	})

	return f, nil
}

// returnsError reports whether fn has an error among its results.
func returnsError(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, res := range fn.Type.Results.List {
		if id, ok := res.Type.(*ast.Ident); ok && id.Name == "error" {
			return true
		}
	}

	return false
}

// makesError reports whether the body of fn makes an error of its own, as a
// stub of an unsupported platform does: it calls errors.New or fmt.Errorf, or
// names a package error value (pkg.ErrX). A function that only passes on an
// error of the system (os.UserConfigDir) is a working fallback, not a stub.
func makesError(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		x, isIdent := sel.X.(*ast.Ident)
		switch {
		case isIdent && ((x.Name == "errors" && sel.Sel.Name == "New") || (x.Name == "fmt" && sel.Sel.Name == "Errorf")):
			found = true
		case strings.HasPrefix(sel.Sel.Name, "Err"):
			found = true
		}

		return true
	})

	return found
}

// isStdlib reports whether the import path is of the standard library (or
// the pseudo-package "C"): its first element has no dot.
func isStdlib(imp string) bool {
	first, _, _ := strings.Cut(imp, "/")

	return !strings.Contains(first, ".")
}

// violating lists "path imports import" for each import of each file that
// in accepts and bad accepts.
func violating(files []sourceFile, in func(sourceFile) bool, bad func(string) bool) []string {
	var out []string
	for _, f := range files {
		if !in(f) {
			continue
		}
		for _, imp := range f.Imports {
			if bad(imp) {
				out = append(out, f.Path+" imports "+imp)
			}
		}
	}

	return out
}

// underAny reports whether p is one of bases or lies below one of them.
func underAny(p string, bases ...string) bool {
	for _, base := range bases {
		if p == base || strings.HasPrefix(p, base+"/") {
			return true
		}
	}

	return false
}

// sortedKeys returns the keys of m, sorted.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	return keys
}

// nameGOOS returns the GOOS that the file name selects (name_GOOS.go,
// name_GOOS_GOARCH.go, with an optional _test), or "". tvos is not a GOOS: a
// _tvos name selects nothing (buildTagsRule asks for a //go:build line).
func nameGOOS(base string) string {
	parts := strings.Split(strings.TrimSuffix(strings.TrimSuffix(base, ".go"), "_test"), "_")
	if len(parts) < 2 {
		return ""
	}
	if slices.Contains(knownArch, parts[len(parts)-1]) && len(parts) >= 3 {
		parts = parts[:len(parts)-1]
	}
	if last := parts[len(parts)-1]; slices.Contains(knownOS, last) {
		return last
	}

	return ""
}

// builds reports whether the compiler builds f in platform p, from its file
// name and its //go:build line.
func builds(f sourceFile, p platform) bool {
	if goos := nameGOOS(f.Base); goos != "" && !p.has(goos) {
		return false
	}
	if f.Build == nil {
		return true
	}

	return f.Build.Eval(func(tag string) bool {
		return p.has(tag) || (tag == "cgo" && p.cgo)
	})
}

// mentionsPlatform reports whether f's name or build line names ios, tvos
// or android.
func mentionsPlatform(f sourceFile) bool {
	if goos := nameGOOS(f.Base); goos == "ios" || goos == "android" {
		return true
	}

	return f.Build != nil && exprMentions(f.Build, "ios", "android", "tvos")
}

// exprMentions reports whether expr names one of tags.
func exprMentions(expr constraint.Expr, tags ...string) bool {
	switch e := expr.(type) {
	case *constraint.TagExpr:
		return slices.Contains(tags, e.Tag)
	case *constraint.NotExpr:
		return exprMentions(e.X, tags...)
	case *constraint.AndExpr:
		return exprMentions(e.X, tags...) || exprMentions(e.Y, tags...)
	case *constraint.OrExpr:
		return exprMentions(e.X, tags...) || exprMentions(e.Y, tags...)
	default:
		return false
	}
}

// isVietnamese reports whether r belongs to Vietnamese text: the Latin
// Extended Additional block (U+1EA0 to U+1EF9), the accented vowels, the
// breve, circumflex and horn forms, d with stroke, and the combining marks
// U+0300 to U+0323 of a decomposed text.
func isVietnamese(r rune) bool {
	if r >= 0x1EA0 && r <= 0x1EF9 {
		return true
	}
	if r >= 0x0300 && r <= 0x0323 {
		return true
	}

	return unicode.IsLetter(r) && strings.ContainsRune(vietnameseLetters, r)
}

// banner returns the title of the banner block that starts at lines[i], and
// whether the three lines form a well-formed block.
func banner(lines []string, i int) (string, bool) {
	ruler := "// " + strings.Repeat("=", 68)
	if i+2 >= len(lines) || lines[i] != ruler || lines[i+2] != ruler {
		return "", false
	}
	for _, title := range bannerTitles {
		n := 66 - len(title)
		left := n / 2
		want := "// " + strings.Repeat("=", left) + " " + title + " " + strings.Repeat("=", n-left)
		if lines[i+1] == want {
			return title, true
		}
	}

	return "", false
}
