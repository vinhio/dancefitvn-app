// Adapted from jplayer-mobile/cmd/brandcheck/main_test.go at 49de3c2.

package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writePNG writes a w x h picture of fill, with set called on it first, as
// an RGBA (alpha) or opaque RGB-model PNG.
func writePNG(t *testing.T, path string, w, h int, alpha bool, fill color.NRGBA, set func(*image.NRGBA)) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	if set != nil {
		set(img)
	}
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	var buf bytes.Buffer
	if alpha {
		require.NoError(t, png.Encode(&buf, img))
	} else {
		rgb := image.NewRGBA(img.Bounds())
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := img.NRGBAAt(x, y)
				rgb.Set(x, y, color.RGBA{c.R, c.G, c.B, 255})
			}
		}
		require.NoError(t, png.Encode(&buf, rgb))
	}
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
}

// writeJSON writes v as JSON to path, making its folders.
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o644))
}

// brandAssets writes a sound brand assets folder under root: an App Icon
// stack of two layers (400x240 at 1x and 2x) and a Top Shelf image
// (1920x720 at 1x), and returns the folder.
func brandAssets(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, brandAssetsPath)
	type m = map[string]any
	writeJSON(t, filepath.Join(dir, "Contents.json"), m{"assets": []m{
		{"filename": "App Icon.imagestack", "idiom": "tv", "role": "primary-app-icon", "size": "400x240"},
		{"filename": "Top Shelf Image.imageset", "idiom": "tv", "role": "top-shelf-image", "size": "1920x720"},
	}})
	stack := filepath.Join(dir, "App Icon.imagestack")
	writeJSON(t, filepath.Join(stack, "Contents.json"), m{"layers": []m{{"filename": "Front.imagestacklayer"}, {"filename": "Back.imagestacklayer"}}})
	ground := color.NRGBA{20, 21, 24, 255}
	for _, l := range []string{"Front", "Back"} {
		set := filepath.Join(stack, l+".imagestacklayer", "Content.imageset")
		writeJSON(t, filepath.Join(set, "Contents.json"), m{"images": []m{
			{"filename": l + ".png", "idiom": "tv", "scale": "1x"}, {"filename": l + "@2x.png", "idiom": "tv", "scale": "2x"},
		}})
		for _, s := range []int{1, 2} {
			name := l + ".png"
			if s == 2 {
				name = l + "@2x.png"
			}
			if l == "Back" {
				writePNG(t, filepath.Join(set, name), 400*s, 240*s, false, ground, nil)
				continue
			}
			writePNG(t, filepath.Join(set, name), 400*s, 240*s, true, color.NRGBA{}, func(img *image.NRGBA) {
				img.SetNRGBA(200*s, 120*s, color.NRGBA{255, 50, 50, 255})
			})
		}
	}
	shelf := filepath.Join(dir, "Top Shelf Image.imageset")
	writeJSON(t, filepath.Join(shelf, "Contents.json"), m{"images": []m{
		{"filename": "shelf.png", "idiom": "tv", "scale": "1x"}, {"filename": "shelf@2x.png", "idiom": "tv", "scale": "2x"},
	}})
	writePNG(t, filepath.Join(shelf, "shelf.png"), 1920, 720, false, ground, nil)
	writePNG(t, filepath.Join(shelf, "shelf@2x.png"), 3840, 1440, false, ground, nil)

	return dir
}

func TestChecksOnPictures(t *testing.T) {
	t.Parallel()
	opaque := color.NRGBA{200, 120, 40, 255}
	none := color.NRGBA{}
	dot := func(x, y int) func(*image.NRGBA) {
		return func(m *image.NRGBA) { m.SetNRGBA(x, y, color.NRGBA{255, 255, 255, 255}) }
	}
	tests := []struct {
		name string
		pic  func(t *testing.T, path string)
		fn   func(*pic) []string
		bad  string // substring of the problem, empty for none
	}{
		{
			"opaque ok", func(t *testing.T, p string) { writePNG(t, p, 8, 8, false, opaque, nil) },
			func(p *pic) []string { return join(noAlphaChannel(p), allOpaque(p)) }, "",
		},
		{"alpha channel", func(t *testing.T, p string) {
			writePNG(t, p, 8, 8, true, opaque, func(m *image.NRGBA) { m.SetNRGBA(0, 0, color.NRGBA{1, 2, 3, 254}) })
		}, noAlphaChannel, "alpha channel"},
		{"transparent pixel", func(t *testing.T, p string) {
			writePNG(t, p, 8, 8, true, opaque, func(m *image.NRGBA) { m.SetNRGBA(3, 3, none) })
		}, allOpaque, "alpha 0"},
		{
			"wrong size", func(t *testing.T, p string) { writePNG(t, p, 8, 9, false, opaque, nil) },
			func(p *pic) []string { return size(p, 8, 8) }, "size is 8x9",
		},
		{
			"grey ok", func(t *testing.T, p string) { writePNG(t, p, 4, 4, false, color.NRGBA{9, 9, 9, 255}, nil) },
			greyscale, "",
		},
		{
			"not grey", func(t *testing.T, p string) { writePNG(t, p, 4, 4, false, opaque, nil) },
			greyscale, "want grey",
		},
		{
			"circle ok", func(t *testing.T, p string) { writePNG(t, p, 100, 100, true, none, dot(50, 40)) },
			func(p *pic) []string { return insideCircle(p, 20, visibleAlpha) }, "",
		},
		{
			"circle breached", func(t *testing.T, p string) { writePNG(t, p, 100, 100, true, none, dot(95, 50)) },
			func(p *pic) []string { return insideCircle(p, 20, visibleAlpha) }, "want within 20",
		},
		{
			"box ok", func(t *testing.T, p string) { writePNG(t, p, 100, 100, true, none, dot(30, 30)) },
			func(p *pic) []string { return insideRect(p, 50, 50, 0) }, "",
		},
		{
			"box breached", func(t *testing.T, p string) { writePNG(t, p, 100, 100, true, none, dot(10, 30)) },
			func(p *pic) []string { return insideRect(p, 50, 50, 0) }, "outside the 50x50 px box",
		},
		{
			"rect breached", func(t *testing.T, p string) { writePNG(t, p, 100, 60, true, none, dot(50, 2)) },
			func(p *pic) []string { return insideRect(p, 80, 48, 0) }, "outside the 80x48 px box",
		},
		{
			"white ok", func(t *testing.T, p string) { writePNG(t, p, 10, 10, true, none, dot(5, 5)) },
			whiteOnly, "",
		},
		{"not white", func(t *testing.T, p string) {
			writePNG(t, p, 10, 10, true, none, func(m *image.NRGBA) { m.SetNRGBA(5, 5, color.NRGBA{255, 200, 255, 255}) })
		}, whiteOnly, "want white"},
		{
			"white empty", func(t *testing.T, p string) { writePNG(t, p, 10, 10, true, none, nil) },
			whiteOnly, "is empty",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "p.png")
			tc.pic(t, path)
			p, err := load(path)
			require.NoError(t, err)
			got := tc.fn(p)
			if tc.bad == "" {
				assert.Empty(t, got)
				return
			}
			require.NotEmpty(t, got)
			assert.Contains(t, got[0], tc.bad)
		})
	}
}

func TestCheckBrandAssets(t *testing.T) {
	t.Parallel()
	stack := filepath.Join("App Icon.imagestack", "%s.imagestacklayer", "Content.imageset")
	at := func(layer, file string) string {
		return filepath.Join(strings.Replace(stack, "%s", layer, 1), file)
	}
	tests := []struct {
		name  string
		spoil func(t *testing.T, dir string)
		bad   string // substring of a problem, empty for none
	}{
		{"sound", func(*testing.T, string) {}, ""},
		{"back layer with alpha", func(t *testing.T, dir string) {
			writePNG(t, filepath.Join(dir, at("Back", "Back.png")), 400, 240, true, color.NRGBA{20, 21, 24, 200}, nil)
		}, "alpha channel"},
		{"layer of the wrong size", func(t *testing.T, dir string) {
			writePNG(t, filepath.Join(dir, at("Front", "Front@2x.png")), 400, 240, true, color.NRGBA{}, nil)
		}, "size is 400x240, want 800x480"},
		{"front out of the safe zone", func(t *testing.T, dir string) {
			writePNG(t, filepath.Join(dir, at("Front", "Front.png")), 400, 240, true, color.NRGBA{}, func(m *image.NRGBA) {
				m.SetNRGBA(5, 120, color.NRGBA{255, 50, 50, 255})
			})
		}, "outside the 320x192 px box"},
		{"top shelf with alpha", func(t *testing.T, dir string) {
			writePNG(t, filepath.Join(dir, "Top Shelf Image.imageset", "shelf.png"), 1920, 720, true, color.NRGBA{1, 2, 3, 128}, nil)
		}, "alpha channel"},
		{"stray picture", func(t *testing.T, dir string) {
			writePNG(t, filepath.Join(dir, "Top Shelf Image.imageset", "old.png"), 4, 4, false, color.NRGBA{}, nil)
		}, "old.png: is in no Contents.json"},
		{"size tvOS does not take", func(t *testing.T, dir string) {
			data, err := os.ReadFile(filepath.Join(dir, "Contents.json"))
			require.NoError(t, err)
			data = bytes.Replace(data, []byte(`"1920x720"`), []byte(`"1920x1080"`), 1)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "Contents.json"), data, 0o644))
		}, `"1920x1080" is not one tvOS takes`},
		{"missing scale", func(t *testing.T, dir string) {
			writeJSON(t, filepath.Join(dir, "Top Shelf Image.imageset", "Contents.json"),
				map[string]any{"images": []map[string]any{{"filename": "shelf.png", "idiom": "tv", "scale": "1x"}}})
			require.NoError(t, os.Remove(filepath.Join(dir, "Top Shelf Image.imageset", "shelf@2x.png")))
		}, "scales [1x], want [1x 2x]"},
		{"one layer", func(t *testing.T, dir string) {
			writeJSON(t, filepath.Join(dir, "App Icon.imagestack", "Contents.json"),
				map[string]any{"layers": []map[string]any{{"filename": "Back.imagestacklayer"}}})
		}, "1 layers, want 2 to 5"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			dir := brandAssets(t, root)
			tc.spoil(t, dir)
			got := CheckBrandAssets(root)
			if tc.bad == "" {
				assert.Empty(t, got)
				return
			}
			assert.Contains(t, strings.Join(got, "\n"), tc.bad)
		})
	}
}

func TestCheckConfig(t *testing.T) {
	t.Parallel()
	sound := map[string]any{}
	for k, v := range wiring {
		m := sound
		parts := strings.Split(k, ".")
		for _, p := range parts[:len(parts)-1] {
			if _, ok := m[p]; !ok {
				m[p] = map[string]any{}
			}
			m = m[p].(map[string]any) //nolint:forcetypeassert // built above
		}
		m[parts[len(parts)-1]] = v
	}
	root := t.TempDir()
	writeJSON(t, filepath.Join(root, configPath), sound)
	assert.Empty(t, CheckConfig(root))

	sound["icon"] = "other.png"
	delete(sound["tvos"].(map[string]any), "brandAssets") //nolint:forcetypeassert // built above
	writeJSON(t, filepath.Join(root, configPath), sound)
	got := strings.Join(CheckConfig(root), "\n")
	assert.Contains(t, got, `icon is "other.png", want "icon.png"`)
	assert.Contains(t, got, `tvos.brandAssets is "", want "assets/tvos/DanceFITVN.brandassets"`)
}

func TestCheckSums(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, filepath.Dir(sumsPath))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.png"), []byte("a"), 0o644))
	// sha256("a")
	sum := "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sum+"  a.png\n"), 0o644))
	assert.Empty(t, CheckSums(root))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.png"), []byte("b"), 0o644))
	require.NotEmpty(t, CheckSums(root))
	assert.Contains(t, CheckSums(root)[0], "does not match")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.png"), []byte("a"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra.png"), []byte("x"), 0o644))
	assert.Contains(t, CheckSums(root)[0], "extra.png is not listed")
}

func TestCheckShots(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	opaque := color.NRGBA{10, 20, 30, 255}
	writePNG(t, filepath.Join(dir, "ok.png"), 1080, 1920, false, opaque, nil)
	writePNG(t, filepath.Join(dir, "tv.png"), 1920, 1080, false, opaque, nil)
	assert.Empty(t, CheckShots(dir))

	writePNG(t, filepath.Join(dir, "size.png"), 100, 100, false, opaque, nil)
	writePNG(t, filepath.Join(dir, "alpha.png"), 1080, 1920, true, opaque, func(m *image.NRGBA) { m.SetNRGBA(0, 0, color.NRGBA{1, 2, 3, 254}) })
	got := CheckShots(dir)
	require.Len(t, got, 3)
	all := strings.Join(got, "\n")
	assert.Contains(t, all, "is not a store size")
	assert.Contains(t, all, "alpha channel")

	assert.Contains(t, CheckShots(t.TempDir())[0], "no shots")
}

func TestRunRejectsOtherFolders(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	assert.Equal(t, 2, run([]string{"-root", t.TempDir()}, &out, &errb))
	assert.Contains(t, errb.String(), "gdesk.json")
}

// TestBrandOutputsAreSound checks the committed pictures of the repository
// (what generate.sh made): the gdesk.json image fields' rules hold, the
// tvOS brand assets are sound and gdesk.json names them.
func TestBrandOutputsAreSound(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	var out, errb bytes.Buffer
	code := run([]string{"-root", root}, &out, &errb)
	assert.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "ok")
}
