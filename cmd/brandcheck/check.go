// Adapted from jplayer-mobile/cmd/brandcheck/check.go at 49de3c2.

package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // store shots may be JPEG
	_ "image/png"  // the decoder of every brand picture
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The sizes and rules of docs/brand/BRAND.md section 3, which come from
// the gDesk CLI's gdesk.json fields (kits/cli/config.go) and the stores'.
const (
	// iconSize is gdesk.json icon, ios.iconDark, ios.iconTinted.
	iconSize = 1024
	// adaptiveSize is android.icon's three layers: 108 dp at 432 px.
	adaptiveSize = 432
	// adaptivePx is the pixels to the dp of an adaptive layer (432 / 108).
	adaptivePx = 4
	// safeRadius is the 66 dp safe circle of a launcher layer, in px.
	safeRadius = 33 * adaptivePx
	// visibleBox is the 72 dp a launcher shows of a layer, in px.
	visibleBox = 72 * adaptivePx
	// splashSize is android.splash.icon: 288 dp at 1152 px.
	splashSize = 1152
	// splashRadius is the radius of the 192 dp circle of the splash icon,
	// in px.
	splashRadius = 96 * 4
	// notificationSize is android.notificationIcon: 24 dp at 96 px.
	notificationSize = 96
	// notificationLive is the 20 dp live area of a 24 dp small icon, in px.
	notificationLive = 20 * 4
	// playIconSize is Google Play's icon.
	playIconSize = 512
	// visibleAlpha is the alpha above which a pixel counts as content.
	visibleAlpha = 16
	// whiteSlack is how far from 255 a channel of the notification icon may
	// stand where the pixel is partly transparent (rounding of the
	// un-premultiplied edge).
	whiteSlack = 8
	// tvSafe is the share of each side of a tvOS App Icon in whose middle
	// the front layer's content stays, clear of parallax's crop (chosen,
	// as jPlayer TV's, not Apple's figure).
	tvSafe = 0.8
	// minLayers and maxLayers bound a tvOS image stack (kits/cli).
	minLayers, maxLayers = 2, 5
)

// Where the sources, the outputs and the stored hashes live under the root.
const (
	outDir          = "docs/brand/mobile/out"
	iconPath        = "icon.png"
	notifPath       = "assets/brand/notification.png"
	sumsPath        = "docs/brand/source/SHA256SUMS"
	brandAssetsPath = "assets/tvos/DanceFITVN.brandassets"
	configPath      = "gdesk.json"
	// ground is the Night Stage ground (reference/design/tokens.json
	// color.bg), the launch color of every system.
	ground = "#141518"
)

// storeSizes are the pixel sizes a store shot may have: iPhone 6.9",
// iPad 13", Android phone 9:16, Android tablet, Apple TV HD and 4K.
var storeSizes = [][2]int{{1320, 2868}, {2064, 2752}, {1080, 1920}, {2560, 1600}, {1920, 1080}, {3840, 2160}}

// tvRoles are the sizes, in points, each role of a tvOS brand asset may
// declare, with the scales each size needs (kits/cli: Xcode 26.6's actool
// for tvOS 17).
var tvRoles = map[string]map[string][]string{
	"primary-app-icon":     {"400x240": {"1x", "2x"}, "1280x768": {"1x"}},
	"top-shelf-image":      {"1920x720": {"1x", "2x"}},
	"top-shelf-image-wide": {"2320x720": {"1x", "2x"}},
}

// wiring is what gdesk.json's image and color fields must name: the
// outputs of docs/brand/generate.sh and the Night Stage ground.
var wiring = map[string]string{
	"icon":                      iconPath,
	"ios.iconDark":              outDir + "/ios-icon-dark.png",
	"ios.iconTinted":            outDir + "/ios-icon-tinted.png",
	"ios.launchBackground":      ground,
	"android.icon.foreground":   outDir + "/android/ic_foreground.png",
	"android.icon.background":   outDir + "/android/ic_background.png",
	"android.icon.monochrome":   outDir + "/android/ic_monochrome.png",
	"android.splash.icon":       outDir + "/android/splash.png",
	"android.splash.background": ground,
	"android.notificationIcon":  notifPath,
	"tvos.brandAssets":          brandAssetsPath,
	"tvos.launchBackground":     ground,
}

// pic is a decoded picture with what its file said about alpha.
type pic struct {
	img      *image.NRGBA
	hasAlpha bool // the file has an alpha channel (or a palette that may carry one)
}

// contents is an asset catalog's Contents.json, of the members brandcheck
// reads: a brand assets folder's assets, an image stack's layers, an
// image set's images.
type contents struct {
	Assets []struct {
		Filename string `json:"filename"`
		Role     string `json:"role"`
		Size     string `json:"size"`
	} `json:"assets"`
	Layers []struct {
		Filename string `json:"filename"`
	} `json:"layers"`
	Images []struct {
		Filename string `json:"filename"`
		Scale    string `json:"scale"`
	} `json:"images"`
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// load decodes a PNG or JPEG file into non-premultiplied RGBA.
func load(path string) (*pic, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()
	img, _, err := image.Decode(bufio.NewReader(f))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}
	cfg, _, err := image.DecodeConfig(bufio.NewReader(f))
	if err != nil {
		return nil, fmt.Errorf("config %s: %w", filepath.Base(path), err)
	}
	b := img.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Bounds(), img, b.Min, draw.Src)

	return &pic{img: n, hasAlpha: modelHasAlpha(cfg.ColorModel)}, nil
}

// readContents decodes the Contents.json of an asset catalog folder.
func readContents(dir string) (*contents, error) {
	data, err := os.ReadFile(filepath.Join(dir, "Contents.json"))
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	var c contents
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(filepath.Base(dir), "Contents.json"), err)
	}

	return &c, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Check runs every check of the brand's outputs under root and returns the
// problems found, one line each; none means the brand is sound.
func Check(root string) []string {
	var out []string
	add := func(rel string, fn func(*pic) []string) {
		p, err := load(filepath.Join(root, rel))
		if err != nil {
			out = append(out, fmt.Sprintf("%s: %v", rel, err))
			return
		}
		for _, m := range fn(p) {
			out = append(out, rel+": "+m)
		}
	}
	// iOS: the plain icon must have no alpha channel at all (ITMS-90717).
	add(iconPath, func(p *pic) []string {
		return join(size(p, iconSize, iconSize), noAlphaChannel(p), allOpaque(p))
	})
	add(outDir+"/ios-icon-dark.png", func(p *pic) []string { return size(p, iconSize, iconSize) })
	add(outDir+"/ios-icon-tinted.png", func(p *pic) []string {
		return join(size(p, iconSize, iconSize), allOpaque(p), greyscale(p))
	})
	// Android adaptive layers, splash and notification icons.
	add(outDir+"/android/ic_foreground.png", adaptiveLayer)
	add(outDir+"/android/ic_monochrome.png", adaptiveLayer)
	add(outDir+"/android/ic_background.png", func(p *pic) []string {
		return join(size(p, adaptiveSize, adaptiveSize), allOpaque(p))
	})
	add(outDir+"/android/splash.png", func(p *pic) []string {
		return join(size(p, splashSize, splashSize), insideCircle(p, splashRadius, visibleAlpha))
	})
	add(notifPath, func(p *pic) []string {
		return join(size(p, notificationSize, notificationSize), whiteOnly(p),
			insideRect(p, notificationLive, notificationLive, 0))
	})
	// Google Play.
	add(outDir+"/play/icon-512.png", func(p *pic) []string {
		return join(size(p, playIconSize, playIconSize), allOpaque(p))
	})
	add(outDir+"/play/feature-1024x500.png", func(p *pic) []string {
		return join(size(p, 1024, 500), noAlphaChannel(p), allOpaque(p))
	})
	out = append(out, CheckBrandAssets(root)...)
	out = append(out, CheckConfig(root)...)

	return append(out, CheckSums(root)...)
}

// adaptiveLayer checks a foreground or monochrome layer: 432 px, content in
// the 66 dp circle, nothing outside the 72 dp the launcher shows.
func adaptiveLayer(p *pic) []string {
	return join(
		size(p, adaptiveSize, adaptiveSize),
		insideCircle(p, safeRadius, visibleAlpha),
		insideRect(p, visibleBox, visibleBox, 0),
	)
}

// CheckBrandAssets verifies the tvOS brand assets under root: each asset
// of the folder's Contents.json has a role and size tvOS takes and the
// scales that size needs; each picture has its declared size times its
// scale; an image stack has 2 to 5 layers, its back layer (the last)
// opaque without alpha, its front layer's content in the middle tvSafe of
// each side; a Top Shelf image is opaque without alpha; and every PNG of
// the folder is one of those.
func CheckBrandAssets(root string) []string {
	dir := filepath.Join(root, brandAssetsPath)
	top, err := readContents(dir)
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", brandAssetsPath, err)}
	}
	var out []string
	used := map[string]bool{}
	problem := func(rel string, ms ...string) {
		for _, m := range ms {
			out = append(out, filepath.ToSlash(filepath.Join(brandAssetsPath, rel))+": "+m)
		}
	}
	// set checks the image set at rel against the size w x h points.
	set := func(rel string, w, h int, scales []string, fn func(*pic) []string) {
		c, err := readContents(filepath.Join(dir, rel))
		if err != nil {
			problem(rel, err.Error())
			return
		}
		var got []string
		for _, im := range c.Images {
			got = append(got, im.Scale)
			n, err := strconv.Atoi(strings.TrimSuffix(im.Scale, "x"))
			if err != nil || n < 1 {
				problem(rel, fmt.Sprintf("scale %q is not 1x, 2x or 3x", im.Scale))
				continue
			}
			file := filepath.Join(rel, im.Filename)
			used[file] = true
			p, err := load(filepath.Join(dir, file))
			if err != nil {
				problem(file, err.Error())
				continue
			}
			problem(file, join(size(p, w*n, h*n), fn(p))...)
		}
		sort.Strings(got)
		if !slices.Equal(got, scales) {
			problem(rel, fmt.Sprintf("scales %v, want %v", got, scales))
		}
	}
	for _, a := range top.Assets {
		scales, ok := tvRoles[a.Role][a.Size]
		if !ok {
			problem(a.Filename, fmt.Sprintf("role %q of size %q is not one tvOS takes", a.Role, a.Size))
			continue
		}
		w, h, _ := parseSize(a.Size)
		switch filepath.Ext(a.Filename) {
		case ".imagestack":
			stack, err := readContents(filepath.Join(dir, a.Filename))
			if err != nil {
				problem(a.Filename, err.Error())
				continue
			}
			if n := len(stack.Layers); n < minLayers || n > maxLayers {
				problem(a.Filename, fmt.Sprintf("%d layers, want %d to %d", n, minLayers, maxLayers))
				continue
			}
			for i, l := range stack.Layers {
				rule := func(*pic) []string { return nil }
				switch i {
				case len(stack.Layers) - 1:
					rule = func(p *pic) []string { return join(noAlphaChannel(p), allOpaque(p)) }
				case 0:
					rule = func(p *pic) []string {
						b := p.img.Bounds()
						return insideRect(p, int(float64(b.Dx())*tvSafe), int(float64(b.Dy())*tvSafe), visibleAlpha)
					}
				default:
				}
				set(filepath.Join(a.Filename, l.Filename, "Content.imageset"), w, h, scales, rule)
			}
		case ".imageset":
			set(a.Filename, w, h, scales, func(p *pic) []string { return join(noAlphaChannel(p), allOpaque(p)) })
		default:
			problem(a.Filename, "is neither an image stack nor an image set")
		}
	}
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if strings.EqualFold(filepath.Ext(path), ".png") && !used[rel] {
			problem(rel, "is in no Contents.json")
		}
		return nil
	})
	if err != nil {
		problem("", err.Error())
	}

	return out
}

// CheckConfig verifies that gdesk.json's image and color fields name the
// brand's outputs and the Night Stage ground (wiring).
func CheckConfig(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, configPath))
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", configPath, err)}
	}
	var c any
	if err := json.Unmarshal(data, &c); err != nil {
		return []string{fmt.Sprintf("%s: %v", configPath, err)}
	}
	keys := make([]string, 0, len(wiring))
	for k := range wiring {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []string
	for _, k := range keys {
		if got := member(c, k); got != wiring[k] {
			out = append(out, fmt.Sprintf("%s: %s is %q, want %q", configPath, k, got, wiring[k]))
		}
	}

	return out
}

// CheckSums verifies docs/brand/source/SHA256SUMS against the files next
// to it: every listed file matches, and every file of the folder is listed.
func CheckSums(root string) []string {
	f, err := os.Open(filepath.Join(root, sumsPath))
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", sumsPath, err)}
	}
	defer func() { _ = f.Close() }()
	dir := filepath.Dir(filepath.Join(root, sumsPath))
	listed := map[string]bool{}
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		sum, name, ok := strings.Cut(line, "  ")
		if !ok {
			out = append(out, fmt.Sprintf("%s: bad line %q", sumsPath, line))
			continue
		}
		listed[name] = true
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			out = append(out, fmt.Sprintf("%s: %s: %v", sumsPath, name, err))
			continue
		}
		h := sha256.Sum256(data)
		if hex.EncodeToString(h[:]) != sum {
			out = append(out, fmt.Sprintf("%s: %s does not match its hash (source files are never edited)", sumsPath, name))
		}
	}
	if err := sc.Err(); err != nil {
		out = append(out, fmt.Sprintf("%s: %v", sumsPath, err))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return append(out, fmt.Sprintf("%s: %v", filepath.Dir(sumsPath), err))
	}
	for _, e := range entries {
		if e.Name() != "SHA256SUMS" && !e.IsDir() && !listed[e.Name()] {
			out = append(out, fmt.Sprintf("%s: %s is not listed", sumsPath, e.Name()))
		}
	}

	return out
}

// CheckShots verifies the store shots under dir: each PNG or JPEG has one of
// the store sizes and no alpha (App Store Connect refuses it).
func CheckShots(dir string) []string {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".png", ".jpg", ".jpeg":
			files = append(files, path)
		default:
		}
		return nil
	})
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", dir, err)}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return []string{dir + ": no shots"}
	}
	var out []string
	for _, path := range files {
		p, err := load(path)
		if err != nil {
			out = append(out, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		b := p.img.Bounds()
		ok := false
		for _, s := range storeSizes {
			ok = ok || (b.Dx() == s[0] && b.Dy() == s[1])
		}
		if !ok {
			out = append(out, fmt.Sprintf("%s: %dx%d is not a store size %v", path, b.Dx(), b.Dy(), storeSizes))
		}
		for _, m := range join(noAlphaChannel(p), allOpaque(p)) {
			out = append(out, path+": "+m)
		}
	}

	return out
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// join concatenates the problem lists.
func join(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		out = append(out, l...)
	}

	return out
}

// modelHasAlpha reports whether a decoded file's color model carries alpha.
func modelHasAlpha(m color.Model) bool {
	switch m {
	case color.RGBAModel, color.RGBA64Model, color.GrayModel, color.Gray16Model, color.YCbCrModel, color.CMYKModel:
		return false
	default:
		return true
	}
}

// parseSize reads an asset catalog size, "400x240".
func parseSize(s string) (w, h int, ok bool) {
	a, b, found := strings.Cut(s, "x")
	w, errW := strconv.Atoi(a)
	h, errH := strconv.Atoi(b)

	return w, h, found && errW == nil && errH == nil
}

// member returns the string at the dotted path of a decoded JSON value,
// or "" when there is none.
func member(v any, path string) string {
	for k := range strings.SplitSeq(path, ".") {
		m, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		v = m[k]
	}
	s, _ := v.(string)

	return s
}

// size checks the picture's pixel size.
func size(p *pic, w, h int) []string {
	b := p.img.Bounds()
	if b.Dx() != w || b.Dy() != h {
		return []string{fmt.Sprintf("size is %dx%d, want %dx%d", b.Dx(), b.Dy(), w, h)}
	}

	return nil
}

// noAlphaChannel checks that the file has no alpha channel.
func noAlphaChannel(p *pic) []string {
	if p.hasAlpha {
		return []string{"has an alpha channel, want RGB"}
	}

	return nil
}

// allOpaque checks that every pixel has alpha 255.
func allOpaque(p *pic) []string {
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if a := p.img.NRGBAAt(x, y).A; a != 255 {
				return []string{fmt.Sprintf("pixel (%d,%d) has alpha %d, want 255", x, y, a)}
			}
		}
	}

	return nil
}

// greyscale checks that every pixel has equal red, green and blue (the iOS
// tinted icon is tinted by the system).
func greyscale(p *pic) []string {
	b := p.img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if c := p.img.NRGBAAt(x, y); c.R != c.G || c.G != c.B {
				return []string{fmt.Sprintf("pixel (%d,%d) is %d,%d,%d, want grey", x, y, c.R, c.G, c.B)}
			}
		}
	}

	return nil
}

// insideCircle checks that every pixel with alpha above minAlpha has its
// center inside the circle of radius r px around the picture's center.
func insideCircle(p *pic, r float64, minAlpha uint8) []string {
	b := p.img.Bounds()
	cx, cy := float64(b.Dx())/2, float64(b.Dy())/2
	worst, wx, wy := 0.0, 0, 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if p.img.NRGBAAt(x, y).A <= minAlpha {
				continue
			}
			if d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy); d > worst {
				worst, wx, wy = d, x, y
			}
		}
	}
	if worst > r {
		return []string{fmt.Sprintf("content reaches %.1f px from the center at (%d,%d), want within %.0f", worst, wx, wy, r)}
	}

	return nil
}

// insideRect checks that no pixel with alpha above minAlpha lies outside the
// centered rectangle of w x h px.
func insideRect(p *pic, w, h int, minAlpha uint8) []string {
	b := p.img.Bounds()
	x0, y0 := (b.Dx()-w)/2, (b.Dy()-h)/2
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if x >= x0 && x < x0+w && y >= y0 && y < y0+h {
				continue
			}
			if a := p.img.NRGBAAt(x, y).A; a > minAlpha {
				return []string{fmt.Sprintf("pixel (%d,%d) has alpha %d outside the %dx%d px box", x, y, a, w, h)}
			}
		}
	}

	return nil
}

// whiteOnly checks that every pixel with alpha is white: full white where
// opaque, near white where the edge is partly transparent.
func whiteOnly(p *pic) []string {
	b := p.img.Bounds()
	seen := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := p.img.NRGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			seen = true
			slack := uint8(0)
			if c.A < 255 {
				slack = whiteSlack
			}
			if c.R < 255-slack || c.G < 255-slack || c.B < 255-slack {
				return []string{fmt.Sprintf("pixel (%d,%d) is %d,%d,%d (alpha %d), want white", x, y, c.R, c.G, c.B, c.A)}
			}
		}
	}
	if !seen {
		return []string{"is empty"}
	}

	return nil
}

// errNoRoot is returned by checkRoot when the folder is not the
// repository's root.
var errNoRoot = errors.New("not the repository root (no gdesk.json)")

// checkRoot verifies that dir looks like the repository root.
func checkRoot(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, configPath)); err != nil {
		return fmt.Errorf("%s: %w", dir, errNoRoot)
	}

	return nil
}
