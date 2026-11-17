package poster_test

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"dancefitvn-app/internal/tokens"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// stage is a root that puts its children at fixed places on the page
// background, as the prototype's capture pages do.
type stage struct {
	ui.Base
	at map[ui.Widget]geom.Point
	// order is the children in the order they were put.
	order []ui.Widget
	// paints counts the frames drawn.
	paints int
}

// put places w with its top-left at (x, y) of the stage.
func (s *stage) put(w ui.Widget, x, y float32) {
	if s.at == nil {
		s.at = map[ui.Widget]geom.Point{}
	}
	s.at[w] = geom.Pt(x, y)
	s.order = append(s.order, w)
	s.Add(w)
}

// drop takes w out of the stage and of the tree.
func (s *stage) drop(w ui.Widget) {
	delete(s.at, w)
	s.order = slices.DeleteFunc(s.order, func(o ui.Widget) bool { return o == w })
	s.Remove(w)
	s.Relayout()
}

// Layout sizes every child to its natural size and places it.
func (s *stage) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	for _, w := range s.order {
		c.Layout(w, ui.Loose(cs.Max))
		c.Place(w, s.at[w])
	}

	return cs.Max
}

// Paint fills the page with --bg, then draws the children.
func (s *stage) Paint(c *ui.Context, p *ui.Painter) {
	s.paints++
	f := s.Frame()
	p.Fill(geom.XYWH(0, 0, f.W, f.H), solid(tokens.Dark.Bg), 0)
	for _, w := range s.order {
		p.Child(c, w)
	}
}

// crop returns the pixels r of im as a new image, at its own origin.
func crop(im image.Image, x0, y0, x1, y1 int) *image.RGBA {
	b := im.Bounds()
	x0, y0, x1, y1 = max(x0, 0), max(y0, 0), min(x1, b.Dx()), min(y1, b.Dy())
	out := image.NewRGBA(image.Rect(0, 0, max(x1-x0, 0), max(y1-y0, 0)))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			out.Set(x-x0, y-y0, im.At(b.Min.X+x, b.Min.Y+y))
		}
	}

	return out
}

// diff is the comparison of 07 section 3.3: a pixel is bad when a channel
// differs by more than 32/255; masked pixels are not counted.
type diff struct {
	compared, bad int
	pic           *image.RGBA // proto | app | difference, for the file
}

// share is the part of the compared pixels that are bad.
func (d diff) share() float64 {
	if d.compared == 0 {
		return 0
	}

	return float64(d.bad) / float64(d.compared)
}

// compare compares want (the prototype) with got (the app) of one size,
// skipping the pixels masked tells.
func compare(want, got *image.RGBA, masked func(x, y int) bool) diff {
	w, h := want.Bounds().Dx(), want.Bounds().Dy()
	d := diff{pic: image.NewRGBA(image.Rect(0, 0, 3*w+8, h))}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a, b := want.RGBAAt(x, y), got.RGBAAt(x, y)
			d.pic.SetRGBA(x, y, a)
			d.pic.SetRGBA(w+4+x, y, b)
			out := color.RGBA{R: 0, G: 0, B: 0, A: 255}
			if masked != nil && masked(x, y) {
				out = color.RGBA{R: 0, G: 0, B: 160, A: 255}
			} else {
				d.compared++
				if abs8(a.R, b.R) > 32 || abs8(a.G, b.G) > 32 || abs8(a.B, b.B) > 32 {
					d.bad++
					out = color.RGBA{R: 255, G: 255, B: 255, A: 255}
				}
			}
			d.pic.SetRGBA(2*w+8+x, y, out)
		}
	}

	return d
}

// abs8 is |a-b|.
func abs8(a, b uint8) int { return int(math.Abs(float64(a) - float64(b))) }

// writePNG writes im under local/shots/app-kit/posters, from the module's root.
func writePNG(t testing.TB, name string, im image.Image) {
	t.Helper()
	dir := filepath.Join(moduleRoot(t), "local", "shots", "app-kit", "posters")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, im); err != nil {
		t.Fatalf("encode: %v", err)
	}
}

// moduleRoot returns the folder of go.mod.
func moduleRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test")
		}
		dir = parent
	}
}

// readPNG decodes a file of the repository, or skips the test when it is not
// there (local/ is not in the repository).
func readPNG(t testing.TB, rel string) image.Image {
	t.Helper()
	f, err := os.Open(filepath.Join(moduleRoot(t), rel))
	if err != nil {
		t.Skipf("no %s: %v", rel, err)
	}
	defer func() { _ = f.Close() }()
	im, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", rel, err)
	}

	return im
}

// bestShift compares want (the prototype's box) with the app's pixels of the
// same size at (ox+dx, oy+dy) of snap, for the shifts of one pixel either way,
// and returns the unshifted comparison, the shift with the fewest bad pixels
// and its comparison. Chrome snaps the edges of a box to device pixels but not
// what is drawn in it (and the capture's page sits at a fractional offset), so
// the same layout lands up to a pixel apart on the two sides.
func bestShift(snap, want *image.RGBA, ox, oy int, masked func(x, y int) bool) (zero diff, bx, by int, best diff) {
	w, h := want.Bounds().Dx(), want.Bounds().Dy()
	first := true
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			d := compare(want, crop(snap, ox+dx, oy+dy, ox+dx+w, oy+dy+h), masked)
			if dx == 0 && dy == 0 {
				zero = d
			}
			if first || d.bad < best.bad {
				first, best, bx, by = false, d, dx, dy
			}
		}
	}

	return zero, bx, by, best
}
