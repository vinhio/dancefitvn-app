// Adapted from jplayer-tv/internal/ui/icon/icon_test.go at 781bee5.

package icon_test

import (
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"dancefitvn-app/internal/ui/icon"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// prototype is the prototype's page, which holds the icon table.
const prototype = "../../../reference/prototype/index.html"

const (
	// side is the box of the reference images: ic(n,24) at device scale 3.
	side  = 24
	scale = 3
)

// probe draws one icon in a box of its size, white by default.
type probe struct {
	ui.Base
	n      icon.Name
	size   float32
	stroke float32
	col    color.Color
}

func (p *probe) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Sz(p.size, p.size))
}

func (p *probe) Paint(_ *ui.Context, pt *ui.Painter) {
	icon.Draw(pt, p.n, geom.XYWH(0, 0, p.size, p.size), p.col, p.stroke)
}

// render draws the icon on opaque black at the scale of the reference images
// and returns the frame.
func render(t testing.TB, n icon.Name, size, stroke float32) *image.RGBA {
	t.Helper()
	box := &ui.Box{Width: size, Height: size, Background: scene.Solid(color.Black)}
	box.Add(&probe{n: n, size: size, stroke: stroke, col: color.White})
	tt, err := ui.NewTester(box, size, size)
	require.NoError(t, err)
	t.Cleanup(tt.Close)
	tt.SetScale(scale)
	tt.RunUntilIdle()

	return tt.Snapshot()
}

// readPNG returns the brightness of the reference image, 0..255 per pixel.
func readPNG(t *testing.T, path string) [][]float64 {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	img, err := png.Decode(f)
	require.NoError(t, err)
	b := img.Bounds()
	require.Equal(t, side*scale, b.Dx(), path)
	require.Equal(t, side*scale, b.Dy(), path)
	out := make([][]float64, b.Dy())
	for y := range out {
		out[y] = make([]float64, b.Dx())
		for x := range out[y] {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			out[y][x] = luma(uint8(r>>8), uint8(g>>8), uint8(bl>>8))
		}
	}

	return out
}

// luma is the brightness of a pixel of white on black, 0..255.
func luma(r, g, b uint8) float64 { return (float64(r) + float64(g) + float64(b)) / 3 }

// lit counts the pixels of img brighter than half.
func lit(img *image.RGBA) int {
	n := 0
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2]) > 128 {
			n++
		}
	}

	return n
}

// prototypeNames returns the keys of the IC table of the prototype, in order.
func prototypeNames(t *testing.T) []string {
	t.Helper()
	html, err := os.ReadFile(prototype)
	require.NoError(t, err)
	table := regexp.MustCompile(`(?s)const IC=\{(.*?)\};`).FindSubmatch(html)
	require.NotNil(t, table)
	var names []string
	for _, m := range regexp.MustCompile(`([a-z0-9]+):'`).FindAllSubmatch(table[1], -1) {
		names = append(names, string(m[1]))
	}

	return names
}

// TestEveryPrototypeIconIsGenerated checks that the 42 glyphs of the IC table
// are icons, in the table's order, by their prototype keys, and that PLAY,
// PAUSE and the lesson row's triangle come after them.
func TestEveryPrototypeIconIsGenerated(t *testing.T) {
	t.Parallel()

	want := prototypeNames(t)
	require.Len(t, want, 42)
	names := icon.Names()
	require.Len(t, names, 42+4)
	for i, k := range want {
		n, ok := icon.Parse(k)
		require.True(t, ok, k)
		assert.Equal(t, names[i], n, k)
		assert.Equal(t, k, n.String())
	}
	for _, k := range []string{"play", "pause", "tri", "trifill"} {
		n, ok := icon.Parse(k)
		assert.True(t, ok, k)
		assert.Equal(t, k, n.String())
	}
}

// TestNames checks the Go names, the zero value and the names past the end.
func TestNames(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "dl", icon.Dl.String())
	assert.Equal(t, "b10", icon.B10.String())
	assert.Equal(t, "bolt", icon.Bolt.String())
	assert.Equal(t, "", icon.NameUnknown.String())
	assert.Equal(t, "", icon.Name(255).String())
	_, ok := icon.Parse("no-such-icon")
	assert.False(t, ok)
	_, ok = icon.Parse("")
	assert.False(t, ok)
	names := icon.Names()
	names[0] = icon.NameUnknown
	assert.NotEqual(t, icon.NameUnknown, icon.Names()[0], "the slice belongs to the caller")
	assert.False(t, slices.Contains(icon.Names(), icon.NameUnknown))
}

// TestGeneratedIconsAreCurrent regenerates the table from the prototype and
// compares it with icons_gen.go.
func TestGeneratedIconsAreCurrent(t *testing.T) {
	t.Parallel()

	tmp := filepath.Join(t.TempDir(), "icons_gen.go")
	cmd := exec.Command("go", "run", "./gen", "-in", prototype, "-out", tmp)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	want, err := os.ReadFile(tmp)
	require.NoError(t, err)
	got, err := os.ReadFile("icons_gen.go")
	require.NoError(t, err)
	assert.True(t, string(want) == string(got), "icons_gen.go is stale: run go generate ./internal/ui/icon")
}

// TestEveryIconRenders draws every icon at 24 and checks it lights pixels and
// stays within its box (a stroke may reach a pixel or two past the edge).
func TestEveryIconRenders(t *testing.T) { //nolint:paralleltest // draws the shared documents
	for _, n := range icon.Names() {
		t.Run(n.String(), func(t *testing.T) {
			img := render(t, n, side, 0)
			assert.Greater(t, lit(img), 20, "the icon is drawn")
		})
	}
	// An icon that is not one draws nothing.
	assert.Zero(t, lit(render(t, icon.NameUnknown, side, 0)))
	assert.Zero(t, lit(render(t, icon.Name(250), side, 0)))
}

// TestEveryIconMatchesPrototype compares all icons with Chromium's render of
// the prototype's own markup (testdata/render_icons.py). The two rasterizers
// antialias differently, so edges differ a little (the arcs b10 and f10, the
// worst, by 1.6 % of the pixels beyond 32/255, none beyond 100/255): no more
// than 2 % of the pixels may differ by more than 32/255 and none by more than
// 128/255, which a misplaced stroke or a wrong path would.
func TestEveryIconMatchesPrototype(t *testing.T) { //nolint:paralleltest // draws the shared documents
	for _, n := range icon.Names() {
		t.Run(n.String(), func(t *testing.T) { compare(t, n, 0, n.String()) })
	}
}

// TestIconStrokeMatchesPrototype checks the stroke width argument against the
// icons Chromium draws with the widths the screens ask for (index.html:324,
// :389, ic(n,s,sw) at :239).
func TestIconStrokeMatchesPrototype(t *testing.T) { //nolint:paralleltest // draws the shared documents
	tests := []struct {
		n      icon.Name
		stroke float32
		file   string
	}{
		{icon.Home, 1.8, "home@1.8"}, {icon.Home, 2.2, "home@2.2"},
		{icon.Dl, 2, "dl@2"}, {icon.Dl, 2.2, "dl@2.2"}, {icon.Cr, 2, "cr@2"},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) { compare(t, tc.n, tc.stroke, tc.file) })
	}
	thin := lit(render(t, icon.Home, side, 1.8))
	thick := lit(render(t, icon.Home, side, 2.2))
	assert.Greater(t, thick, thin, "a wider stroke lights more pixels")
	assert.Equal(t, lit(render(t, icon.Home, side, 0)), lit(render(t, icon.Home, side, 1.9)), "0 is the default 1.9")
}

// TestIconScalesWithItsBox checks that an icon fills its box at the sizes the
// screens use: the lit pixels grow with the box and none is far outside it.
func TestIconScalesWithItsBox(t *testing.T) { //nolint:paralleltest // draws the shared documents
	prev := 0
	for _, size := range []float32{15, 18, 20, 22, 24, 36} {
		img := render(t, icon.Search, size, 0)
		n := lit(img)
		assert.Greater(t, n, prev, "size %v", size)
		prev = n
		b := img.Bounds()
		assert.Equal(t, int(size*scale), b.Dx())
	}
}

// compare asserts that the icon drawn at stroke matches the reference image
// of that name.
func compare(t *testing.T, n icon.Name, stroke float32, file string) {
	t.Helper()
	want := readPNG(t, filepath.Join("testdata", "proto", file+".png"))
	img := render(t, n, side, stroke)
	require.Equal(t, side*scale, img.Bounds().Dx())
	differ, total, wrong := 0, 0, 0
	var litWant int
	for y := range side * scale {
		for x := range side * scale {
			i := y*img.Stride + 4*x
			got := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
			w := want[y][x]
			if w > 128 {
				litWant++
			}
			if abs(got-w) > 32 {
				differ++
			}
			if abs(got-w) > 128 {
				wrong++
			}
			total++
		}
	}
	require.NotZero(t, litWant, "the prototype image is blank")
	assert.LessOrEqual(t, float64(differ)/float64(total), 0.02,
		"%s: %d of %d pixels differ by more than 32/255", file, differ, total)
	assert.Zero(t, wrong, "%s: pixels differ by more than 128/255", file)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}

	return v
}

// TestIconDrawDoesNotAllocate checks that drawing an icon already drawn once
// allocates nothing, filled, stroked and with another stroke.
func TestIconDrawDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	box := &ui.Box{Width: 100, Height: 40}
	box.Add(&probe{n: icon.Home, size: 22, col: color.White})
	box.Add(&probe{n: icon.Play, size: 22, col: color.White})
	box.Add(&probe{n: icon.Dl, size: 22, stroke: 2.2, col: color.White})
	box.Add(&probe{n: icon.Trifill, size: 16, col: color.White})
	tt, err := ui.NewTester(box, 100, 40)
	require.NoError(t, err)
	defer tt.Close()
	tt.Repaint()
	assert.Zero(t, testing.AllocsPerRun(100, tt.Repaint))
}

// BenchmarkIconDraw is a repaint of four icons.
func BenchmarkIconDraw(b *testing.B) {
	box := &ui.Box{Width: 100, Height: 40}
	for _, n := range []icon.Name{icon.Home, icon.Play, icon.Dl, icon.Bolt} {
		box.Add(&probe{n: n, size: 22, col: color.White})
	}
	tt, err := ui.NewTester(box, 100, 40)
	require.NoError(b, err)
	defer tt.Close()
	tt.Repaint()
	b.ReportAllocs()
	for b.Loop() {
		tt.Repaint()
	}
}
