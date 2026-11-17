// Adapted from jplayer-mobile/internal/ui/views/viewstest/rig.go at 49de3c2.

package kittest

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/assets"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/bridge/text/coretext"
	"github.com/gdeskdev/desk/bridge/text/gotext"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Phone is one of the two test phones: a screen size, a safe area and the way
// its text engine is asked for faces. Both draw the same layout (PLAN D2); they
// differ in size, insets and text naming only.
type Phone struct {
	// Name is "ios" or "android": for subtest names and shot folders.
	Name string
	// Size is the screen in DIPs.
	Size geom.Size
	// Safe is the safe area the system reports.
	Safe geom.Insets
	// Naming is how the phone's engine is asked for a face: Core Text on iOS,
	// gotext on Android.
	Naming theme.Naming
}

// Phones are both test phones, iOS first (CLAUDE.md section 5): iPhone 16
// 393x852, whose simulator reports a 59 DIP status bar and a 34 DIP home
// indicator; and the 412x892 emulator, whose insets are the ones P2.6 measured
// (local/p2/safearea.md: a 45.333 status bar, a 24 gesture bar).
var Phones = [...]Phone{
	{Name: "ios", Size: geom.Sz(393, 852), Safe: geom.Insets{Top: 59, Bottom: 34}, Naming: theme.NamingCoreText},
	{Name: "android", Size: geom.Sz(412, 892), Safe: geom.Insets{Top: 45.333, Bottom: 24}, Naming: theme.NamingGoText},
}

// Frame is one 60 Hz refresh.
const Frame = time.Second / 60

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// fonts is the folder the app's fonts are written to for Core Text, which
// reads a font from a file; it is made once per process.
var fonts struct {
	once sync.Once
	dir  string
	err  error
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// ForEachPhone runs fn as a parallel subtest for each phone.
func ForEachPhone(t *testing.T, fn func(t *testing.T, ph Phone)) {
	t.Helper()
	for _, ph := range Phones {
		t.Run(ph.Name, func(t *testing.T) {
			t.Parallel()
			fn(t, ph)
		})
	}
}

// Mount mounts root on the phone with the app's theme, at scale 1, the phone's
// safe area set, and runs a frame. Text is shaped by gDesk's fake engine:
// widths and heights are not the fonts', so use it for behavior; MountReal for
// sizes. Under the race detector motion is reduced (Race). The tester closes
// with the test.
func Mount(t testing.TB, ph Phone, root ui.Widget) *ui.Tester {
	t.Helper()

	return mount(t, ph, root, nil)
}

// MountReal is Mount with the phone's real text engine and the app's fonts:
// Core Text on the iOS phone (it skips the test where there is none, off
// Apple systems), gotext on the Android phone. Boxes measured with it are the
// ones a device draws.
func MountReal(t testing.TB, ph Phone, root ui.Widget) *ui.Tester {
	t.Helper()
	sh := Shaper(t, ph)

	return mount(t, ph, root, sh)
}

// Shaper returns a shaper over the phone's real text engine with the app's
// fonts registered; it closes with the test. On the iOS phone it skips the
// test where Core Text is not.
func Shaper(t testing.TB, ph Phone) *text.Shaper {
	t.Helper()
	dir := fontDir(t)
	var eng text.Engine
	var err error
	switch ph.Naming {
	case theme.NamingCoreText:
		if runtime.GOOS != "darwin" && runtime.GOOS != "ios" {
			t.Skip("Core Text runs on Apple systems only")
		}
		for _, p := range fontFiles(t, dir) {
			if err := coretext.RegisterFontFile(p); err != nil {
				t.Fatalf("register %s: %v", p, err)
			}
		}
		eng, err = coretext.New()
	case theme.NamingGoText, theme.NamingUnknown:
		eng, err = gotext.New(gotext.WithDirs(dir))
	default:
		t.Fatalf("unknown naming %d", ph.Naming)
	}
	if err != nil {
		t.Fatalf("text engine: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })

	return text.NewShaper(eng)
}

// PinMotion puts the motion back that Mount reduces under the race detector:
// for a test that pins a curve, a duration or an animation that runs.
func PinMotion(tt *ui.Tester) { tt.View().Context().Theme().ReduceMotion = false }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// mount builds the tester; sh is the shaper, nil for the fake one.
func mount(t testing.TB, ph Phone, root ui.Widget, sh *text.Shaper) *ui.Tester {
	t.Helper()
	th := theme.New(ph.Naming)
	th.ReduceMotion = Race // the frames of every animation cost minutes under the race detector
	opts := []ui.ViewOption{ui.WithTheme(th)}
	if sh != nil {
		opts = append(opts, ui.WithShaper(sh))
	}
	tt, err := ui.NewTester(root, ph.Size.W, ph.Size.H, opts...)
	if err != nil {
		t.Fatalf("tester: %v", err)
	}
	t.Cleanup(tt.Close)
	tt.SetSafeArea(ph.Safe)
	tt.Elapse(Frame)

	return tt
}

// fontDir writes the app's fonts to a folder under the system's temporary one,
// named by the hash of the fonts' list so that a run finds them again, and
// returns it.
func fontDir(t testing.TB) string {
	t.Helper()
	fonts.once.Do(func() { fonts.dir, fonts.err = writeFonts() })
	if fonts.err != nil {
		t.Fatalf("write fonts: %v", fonts.err)
	}

	return fonts.dir
}

// writeFonts copies assets/fonts/*.ttf into <tmp>/dfv-kittest-<hash>/, files
// already there of the right size left alone, each written to a temporary name
// and renamed so that a test process running beside this one never reads half
// a font.
func writeFonts() (string, error) {
	sub, err := fs.Sub(assets.Fonts, "fonts")
	if err != nil {
		return "", err
	}
	names, err := fs.Glob(sub, "*.ttf")
	if err != nil {
		return "", err
	}
	sum, err := fs.ReadFile(sub, "SHA256SUMS.txt")
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(sum)
	dir := filepath.Join(os.TempDir(), "dfv-kittest-"+hex.EncodeToString(h[:4]))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	for _, n := range names {
		data, err := fs.ReadFile(sub, n)
		if err != nil {
			return "", err
		}
		path := filepath.Join(dir, n)
		if st, err := os.Stat(path); err == nil && st.Size() == int64(len(data)) {
			continue
		}
		tmp, err := os.CreateTemp(dir, ".part-*")
		if err != nil {
			return "", err
		}
		_, werr := tmp.Write(data)
		if cerr := tmp.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			_ = os.Remove(tmp.Name())

			return "", werr
		}
		if err := os.Rename(tmp.Name(), path); err != nil {
			return "", err
		}
	}

	return dir, nil
}

// fontFiles returns the paths of the .ttf files of dir, Inter-Regular first.
func fontFiles(t testing.TB, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.ttf"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fonts in %s: %v", dir, err)
	}

	return paths
}
