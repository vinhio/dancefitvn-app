package kit_test

import (
	"encoding/json"
	"image"
	stdcolor "image/color"
	"os"
	"path/filepath"
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stageItem is a widget placed at a point of the stage, width wide when width
// is not zero (the prototype's boxes have a width the layout gives them).
type stageItem struct {
	w     ui.Widget
	at    geom.Point
	width float32
}

// stage is a root that places its items at absolute points, on --bg: the way a
// prototype state is a screen with every component at a known box.
type stage struct {
	ui.Base
	items []stageItem
}

// placeAt returns an item at (x, y) that is width wide (0: as wide as it needs).
func placeAt(w ui.Widget, x, y, width float32) stageItem {
	return stageItem{w: w, at: geom.Pt(x, y), width: width}
}

// newStage returns a stage of items.
func newStage(items ...stageItem) *stage {
	s := &stage{items: items}
	for _, it := range items {
		s.Add(it.w)
	}

	return s
}

// Layout places the items.
func (s *stage) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	for _, it := range s.items {
		cons := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
		if it.width > 0 {
			cons = ui.Constraints{Min: geom.Sz(it.width, 0), Max: geom.Sz(it.width, ui.Unbounded)}
		}
		c.Layout(it.w, cons)
		c.Place(it.w, it.at)
	}

	return cs.Constrain(cs.Max)
}

// Paint fills the screen with --bg and paints the items.
func (s *stage) Paint(c *ui.Context, p *ui.Painter) {
	p.Fill(s.Bounds(), scene.Solid(theme.From(c).T.Bg), 0)
	s.Base.Paint(c, p)
}

// mountStage mounts the items on the phone with the fake text engine.
func mountStage(t testing.TB, ph kittest.Phone, items ...stageItem) *ui.Tester {
	t.Helper()

	return kittest.Mount(t, ph, newStage(items...))
}

// mountStageReal mounts the items with the phone's real text engine.
func mountStageReal(t testing.TB, ph kittest.Phone, items ...stageItem) *ui.Tester {
	t.Helper()

	return kittest.MountReal(t, ph, newStage(items...))
}

// frameOf returns where w is in the window.
func frameOf(w ui.Widget) geom.Rect { return w.Node().WindowFrame() }

// protoElement is one element of a layout.json of the prototype.
type protoElement struct {
	Key  string  `json:"key"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
	Text string  `json:"text"`
}

// protoBox returns the box of the element key of the prototype state (a file of
// local/shots/prototype-kit/mobile); it skips the test when local/ is absent.
func protoBox(t testing.TB, state, key string) geom.Rect {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test")
		}
		dir = parent
	}
	data, err := os.ReadFile(filepath.Join(dir, "local", "shots", "prototype-kit", "mobile", state+".layout.json"))
	if err != nil {
		t.Skipf("prototype capture not available: %v", err)
	}
	var doc struct {
		Elements []protoElement `json:"elements"`
	}
	require.NoError(t, json.Unmarshal(data, &doc))
	for _, e := range doc.Elements {
		if e.Key == key {
			return geom.XYWH(float32(e.X), float32(e.Y), float32(e.W), float32(e.H))
		}
	}
	t.Fatalf("%s has no element %q", state, key)

	return geom.Rect{}
}

// rectNear asserts that got is want within tol DIPs on every side.
func rectNear(t testing.TB, want, got geom.Rect, tol float32, msg string) {
	t.Helper()
	if os.Getenv("KIT_DELTAS") != "" {
		t.Logf("DELTA %-28s dx %+.2f dy %+.2f dw %+.2f dh %+.2f", msg, got.X-want.X, got.Y-want.Y, got.W-want.W, got.H-want.H)
	}
	assert.InDelta(t, want.X, got.X, float64(tol), "%s: x (want %v got %v)", msg, want, got)
	assert.InDelta(t, want.Y, got.Y, float64(tol), "%s: y (want %v got %v)", msg, want, got)
	assert.InDelta(t, want.W, got.W, float64(tol), "%s: w (want %v got %v)", msg, want, got)
	assert.InDelta(t, want.H, got.H, float64(tol), "%s: h (want %v got %v)", msg, want, got)
}

// pixelAt returns the pixel of the snapshot at (x, y).
func pixelAt(img *image.RGBA, x, y int) stdcolor.RGBA { return img.RGBAAt(x, y) }

// pixelNear asserts that the pixel (x, y) is want within tol on each channel.
func pixelNear(t testing.TB, img *image.RGBA, x, y int, want color.Color, tol int, msg string) {
	t.Helper()
	got := pixelAt(img, x, y)
	d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
	if d(got.R, want.R) > tol || d(got.G, want.G) > tol || d(got.B, want.B) > tol {
		t.Errorf("%s: pixel (%d,%d) is %v, want %v +-%d", msg, x, y, got, want, tol)
	}
}

// over returns top (with its alpha) drawn over the opaque bottom.
func over(top, bottom color.Color) color.Color {
	a := float32(top.A) / 255
	mix := func(t, b uint8) uint8 { return uint8(float32(t)*a + float32(b)*(1-a) + .5) }

	return color.Color{R: mix(top.R, bottom.R), G: mix(top.G, bottom.G), B: mix(top.B, bottom.B), A: 255}
}

// palette is the Night Stage colors tests compare pixels with.
func palette() *theme.Look {
	th := theme.New(theme.NamingGoText)
	l, _ := th.Ext[*theme.Look]()

	return l
}

// part returns the first widget named name below root, in tree order.
func part(t testing.TB, root ui.Widget, name string) ui.Widget {
	t.Helper()
	var find func(w ui.Widget) ui.Widget
	find = func(w ui.Widget) ui.Widget {
		for c := range w.Node().Children() {
			if c.Node().Name == name {
				return c
			}
			if f := find(c); f != nil {
				return f
			}
		}

		return nil
	}
	w := find(root)
	if w == nil {
		t.Fatalf("no widget named %q", name)
	}

	return w
}
