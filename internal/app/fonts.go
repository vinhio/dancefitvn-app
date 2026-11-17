// Adapted from jplayer-mobile/internal/app/fonts.go at 49de3c2.

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"dancefitvn-app/assets"

	"github.com/gdeskdev/desk/bridge/text/auto"
	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// fontCacheDir is the folder, under the user's cache folder, the embedded
// fonts are written to: the system's text engines read a font from a
// file, not from memory.
const fontCacheDir = appID + "/fonts"

// firstFont starts the name of the files registered first: gotext makes
// the first family it is given the one a face naming none draws in, which
// on Android must be Inter, the body font (P0 plan §1.5, D5).
const firstFont = "Inter-"

// sumsFile names the file of the fonts' folder that lists their sha256
// (written by scripts/fonts/subset.sh), so that a launch need not read and
// hash every font to name their cached copies.
const sumsFile = "SHA256SUMS.txt"

// fontInstall is what installFonts did.
type fontInstall struct {
	paths   []string // the font files in the cache, in registration order
	written int      // files written because the cache lacked them
	removed int      // files of other builds removed
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// RegisterFonts writes every *.ttf of fsys's top folder into cacheDir, as
// "<first 8 hex digits of its sha256>-<name>" (the sha256 read from the
// folder's SHA256SUMS.txt, computed for a font it does not list), unless a
// file of that name and size is there already, removes the other .ttf files of cacheDir (the
// fonts of an older build), then registers each file with the text engines
// (bridge/text/auto: Core Text on iOS, gotext on Android), Inter
// first. Call it once, before the first window opens: gotext sees only the
// fonts given before its engine is made. The files must stay while the app
// runs. It logs the time it took. A folder without fonts is
// core.ErrNotFound; a file no engine takes for a font, core.ErrInvalid.
func RegisterFonts(fsys fs.FS, cacheDir string) error {
	start := time.Now()
	in, err := installFonts(fsys, cacheDir)
	if err != nil {
		return err
	}
	for _, p := range in.paths {
		if err := auto.RegisterFontFile(p); err != nil {
			return fmt.Errorf("register fonts: %w", err)
		}
	}
	log.Printf("dancefitvn: fonts: %d registered in %.1f ms (%d written, %d removed) from %s",
		len(in.paths), ms(time.Since(start)), in.written, in.removed, cacheDir)

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// registerAppFonts registers the fonts of assets.Fonts from the user's
// cache folder (the temporary folder when there is none). A failure is
// logged, not returned: the system's fonts draw then.
func registerAppFonts() {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	fsys, err := fs.Sub(assets.Fonts, "fonts")
	if err == nil {
		err = RegisterFonts(fsys, filepath.Join(base, filepath.FromSlash(fontCacheDir)))
	}
	if err != nil {
		log.Printf("dancefitvn: fonts: %v", err)
	}
}

// installFonts writes the fonts of fsys into dir as RegisterFonts tells,
// and returns their paths, Inter first.
func installFonts(fsys fs.FS, dir string) (fontInstall, error) {
	var in fontInstall
	names, err := fs.Glob(fsys, "*.ttf")
	if err != nil {
		return in, fmt.Errorf("register fonts: %w", err)
	}
	if len(names) == 0 {
		return in, fmt.Errorf("register fonts: no .ttf file: %w", core.ErrNotFound)
	}
	// fs.Glob sorts by name; the stable sort only moves the first family
	// ahead.
	slices.SortStableFunc(names, func(a, b string) int { return firstRank(a) - firstRank(b) })
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return in, fmt.Errorf("register fonts: %w", err)
	}
	sums := readSums(fsys)
	keep := make(map[string]bool, len(names))
	for _, name := range names {
		st, err := fs.Stat(fsys, name)
		if err != nil {
			return in, fmt.Errorf("register fonts: %w", err)
		}
		// The data is read only when the sums lack the font or the cache
		// lacks the file.
		var data []byte
		sum, ok := sums[name]
		if !ok {
			if data, err = fs.ReadFile(fsys, name); err != nil {
				return in, fmt.Errorf("register fonts: %w", err)
			}
			whole := sha256.Sum256(data)
			sum = hex.EncodeToString(whole[:])
		}
		file := sum[:8] + "-" + path.Base(name)
		p := filepath.Join(dir, file)
		keep[file] = true
		in.paths = append(in.paths, p)
		if have, err := os.Stat(p); err == nil && have.Mode().IsRegular() && have.Size() == st.Size() {
			continue
		}
		if data == nil {
			if data, err = fs.ReadFile(fsys, name); err != nil {
				return in, fmt.Errorf("register fonts: %w", err)
			}
		}
		if err := writeAtomic(dir, p, data); err != nil {
			return in, fmt.Errorf("register fonts: %w", err)
		}
		in.written++
	}
	in.removed = removeStale(dir, keep)

	return in, nil
}

// readSums returns the sha256 of the fonts of fsys by name, from its
// sumsFile ("<64 hex digits>  <name>" lines, as shasum writes them); an
// empty map when there is none. A malformed line is skipped: its font is
// hashed instead.
func readSums(fsys fs.FS) map[string]string {
	sums := map[string]string{}
	data, err := fs.ReadFile(fsys, sumsFile)
	if err != nil {
		return sums
	}
	for line := range strings.Lines(string(data)) {
		sum, name, ok := strings.Cut(strings.TrimRight(line, "\r\n"), "  ")
		if !ok || len(sum) != sha256.Size*2 || strings.Trim(sum, "0123456789abcdef") != "" {
			continue
		}
		sums[name] = sum
	}

	return sums
}

// firstRank orders the font files: Inter's (0) before the others (1).
func firstRank(name string) int {
	if strings.HasPrefix(path.Base(name), firstFont) {
		return 0
	}

	return 1
}

// writeAtomic writes data to p through a temporary file of dir renamed
// over it, so that a launch cut short leaves no partial font.
func writeAtomic(dir, p string, data []byte) error {
	f, err := os.CreateTemp(dir, ".font-*")
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	cerr := f.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(f.Name())

		return err
	}
	if err := os.Rename(f.Name(), p); err != nil {
		_ = os.Remove(f.Name())

		return err
	}

	return nil
}

// removeStale removes the .ttf files of dir keep does not name, and the
// temporary files a write cut short left, and returns how many it removed.
// A file it cannot remove is left: it costs room, not correctness.
func removeStale(dir string, keep map[string]bool) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		name := e.Name()
		stale := (strings.HasSuffix(name, ".ttf") && !keep[name]) || strings.HasPrefix(name, ".font-")
		if !stale || e.IsDir() {
			continue
		}
		if os.Remove(filepath.Join(dir, name)) == nil {
			n++
		}
	}

	return n
}

// ms returns d in milliseconds, for the logs.
func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
