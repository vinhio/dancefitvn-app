package layoutdump_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tiny returns a column of two eyebrows (one name twice), a hidden label and an
// unnamed one.
func tiny() (root ui.Widget, hidden *kit.Label) {
	a, b := kit.Eyebrow("Một"), kit.Eyebrow("Hai")
	hidden = kit.Muted("ẩn", 12)
	hidden.Name, hidden.Hidden = "hidden", true
	plain := &kit.Label{Text: "unnamed"}
	h1 := kit.H1("Tiêu đề")

	return ui.Column(4, a, b, hidden, plain, h1), hidden
}

// TestDumpNumbersNamesAndSkipsHidden checks the keys, the order, the boxes and
// the style a Label gives, on both phones.
func TestDumpNumbersNamesAndSkipsHidden(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		root, _ := tiny()
		tt := kittest.Mount(t, ph, root)
		lay := layoutdump.Dump(tt.View().Context(), tt.View().Root(), "T", "mobile", ph.Size, nil)

		require.Len(t, lay.Elements, 3, "the column is unnamed, the hidden and unnamed labels are left out")
		keys := []string{lay.Elements[0].Key, lay.Elements[1].Key, lay.Elements[2].Key}
		assert.Equal(t, []string{"div.eb", "div.eb#2", "div.h1"}, keys)
		assert.Equal(t, 2, lay.Elements[1].Index)
		assert.Equal(t, 1, lay.Elements[2].Index)
		assert.Equal(t, float64(ph.Size.W), lay.Screen.W)
		assert.Less(t, lay.Elements[0].Y, lay.Elements[1].Y)

		eb := lay.Elements[0]
		assert.Equal(t, "MỘT", eb.Text, "innerText applies text-transform")
		assert.Equal(t, "Inter", eb.Style.FontFamily)
		assert.Equal(t, "11px", eb.Style.FontSize)
		assert.Equal(t, "700", eb.Style.FontWeight)
		assert.Equal(t, "1.4px", eb.Style.LetterSpacing)
		assert.Equal(t, "normal", eb.Style.LineHeight)
		assert.Equal(t, "rgb(142, 145, 150)", eb.Style.Color)

		h1 := lay.Elements[2]
		assert.Equal(t, "Barlow Condensed", h1.Style.FontFamily)
		assert.Equal(t, "34px", h1.Style.LineHeight)
		assert.Equal(t, "1px", h1.Style.LetterSpacing)
		assert.Equal(t, "rgb(244, 244, 242)", h1.Style.Color)
		found, ok := lay.Find("div.eb#2")
		assert.True(t, ok)
		assert.Equal(t, lay.Elements[1], found)
	})
}

// TestCSSColor checks the opaque and translucent writing.
func TestCSSColor(t *testing.T) {
	t.Parallel()

	look := theme.From(kittestContext(t))
	assert.Equal(t, "rgb(255, 208, 40)", layoutdump.CSSColor(look.T.Primary))
	assert.Equal(t, "rgba(0, 0, 0, 0.55)", layoutdump.CSSColor(look.T.Scrim))
}

func kittestContext(t *testing.T) *ui.Context {
	t.Helper()
	tt := kittest.Mount(t, kittest.Phones[0], &kit.Label{Text: "x"})

	return tt.View().Context()
}

// TestShotWritesPictureAndLayout checks the two files, the picture at the
// tester's scale and the layout in DIPs.
func TestShotWritesPictureAndLayout(t *testing.T) {
	t.Parallel()

	ph := kittest.Phones[0]
	root, _ := tiny()
	tt := kittest.Mount(t, ph, root)
	tt.SetScale(2)
	tt.RunUntilIdle()
	dir := t.TempDir()
	require.NoError(t, layoutdump.Shot(tt, "T1", "mobile", ph.Size, dir, nil))

	f, err := os.Open(filepath.Join(dir, "T1.png"))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	cfg, err := png.DecodeConfig(f)
	require.NoError(t, err)
	assert.Equal(t, int(ph.Size.W)*2, cfg.Width)

	data, err := os.ReadFile(filepath.Join(dir, "T1.layout.json"))
	require.NoError(t, err)
	var back layoutdump.Layout
	require.NoError(t, json.Unmarshal(data, &back))
	assert.Equal(t, "T1", back.ID)
	assert.Equal(t, "mobile", back.Platform)
	assert.Equal(t, geom.Sz(393, 852), geom.Sz(float32(back.Screen.W), float32(back.Screen.H)))
	assert.Len(t, back.Elements, 3)
}

type styled struct{ ui.Base }

func (s *styled) LayoutStyle(*ui.Context) (layoutdump.Style, string) {
	return layoutdump.Style{Radius: "14px", Opacity: "0.45"}, "own"
}

// TestStylerWins checks that a widget's own style is taken.
func TestStylerWins(t *testing.T) {
	t.Parallel()

	w := &styled{}
	w.Name = "s"
	tt := kittest.Mount(t, kittest.Phones[0], ui.Column(0, w))
	lay := layoutdump.Dump(tt.View().Context(), tt.View().Root(), "S", "mobile", kittest.Phones[0].Size, nil)
	require.Len(t, lay.Elements, 1)
	assert.Equal(t, "14px", lay.Elements[0].Style.Radius)
	assert.Equal(t, "own", lay.Elements[0].Text)
}

// TestSkipAndPinnedNamesLineUpTheNumbers checks the two ways to count the
// elements of the prototype that a page does not draw.
func TestSkipAndPinnedNamesLineUpTheNumbers(t *testing.T) {
	t.Parallel()

	a, b, c := kit.Eyebrow("a"), kit.Eyebrow("b"), kit.Eyebrow("c")
	c.Name = "div.eb#5"
	tt := kittest.Mount(t, kittest.Phones[0], ui.Column(0, a, b, c))
	lay := layoutdump.Dump(tt.View().Context(), tt.View().Root(), "N", "mobile", kittest.Phones[0].Size, map[string]int{"div.eb": 2})
	require.Len(t, lay.Elements, 3)
	assert.Equal(t, []string{"div.eb#3", "div.eb#4", "div.eb#5"}, []string{lay.Elements[0].Key, lay.Elements[1].Key, lay.Elements[2].Key})
	assert.Equal(t, "div.eb", lay.Elements[2].Name)
	assert.Equal(t, 5, lay.Elements[2].Index)
}

// placer puts its child at (300, 200).
type placer struct{ ui.Base }

// Layout places the child at (300, 200).
func (p *placer) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	for ch := range p.Children() {
		c.Layout(ch, ui.Loose(cs.Max))
		c.Place(ch, geom.Pt(300, 200))
	}

	return cs.Constrain(cs.Max)
}

// scaled is a widget painted under a scale about its centre; the child is
// named and sits inside it.
type scaled struct {
	ui.Base
	s float32
}

// Layout is a 200 by 100 box at the child's top left.
func (w *scaled) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	for ch := range w.Children() {
		c.Layout(ch, ui.Tight(geom.Sz(40, 20)))
		c.Place(ch, geom.Pt(20, 10))
	}

	return cs.Constrain(geom.Sz(200, 100))
}

// PaintTransform scales about the middle of the box.
func (w *scaled) PaintTransform() (scene.Affine, bool) {
	if w.s == 1 {
		return scene.Identity, false
	}

	return scene.Affine{XX: w.s, YY: w.s, X0: 100 * (1 - w.s), Y0: 50 * (1 - w.s)}, true
}

// TestDumpAppliesPaintTransforms: a Transformer's own box and the boxes inside
// it are the ones painted, as the prototype's getBoundingClientRect reports a
// focused element scaled to 1.06 (F-6 of local/plans/P7.md).
func TestDumpAppliesPaintTransforms(t *testing.T) {
	t.Parallel()

	outer := &scaled{s: 1.5}
	outer.Name = "outer"
	inner := &scaled{s: 1}
	inner.Name = "inner"
	leaf := &kit.Label{Text: "x"}
	leaf.Name = "leaf"
	inner.Add(leaf)
	outer.Add(inner)
	host := &placer{}
	host.Add(outer)
	tt := kittest.Mount(t, kittest.Phones[0], host)
	lay := layoutdump.Dump(tt.View().Context(), tt.View().Root(), "T", "tv", geom.Sz(393, 852), nil)

	box := func(key string) geom.Rect {
		e, ok := lay.Find(key)
		require.True(t, ok, key)

		return geom.XYWH(float32(e.X), float32(e.Y), float32(e.W), float32(e.H))
	}
	// outer: 200x100 at (300,200), scaled 1.5 about (400,250): 300x150 at (250,175)
	assert.Equal(t, geom.XYWH(250, 175, 300, 150), box("outer"))
	// inner has no transform of its own: its frame is (320,210) 40x20, which
	// outer's map moves and grows: 1.5 about (400,250)
	assert.Equal(t, geom.XYWH(280, 190, 60, 30), box("inner"))
	// leaf is at (20,10) in inner: (340,220) 40x20 -> (310,205) 60x30
	assert.Equal(t, geom.XYWH(310, 205, 60, 30), box("leaf"))

	// at scale 1 nothing changes
	outer.s = 1
	tt.Elapse(kittest.Frame)
	lay = layoutdump.Dump(tt.View().Context(), tt.View().Root(), "T", "tv", geom.Sz(393, 852), nil)
	assert.Equal(t, geom.XYWH(300, 200, 200, 100), box("outer"))
	assert.Equal(t, geom.XYWH(320, 210, 40, 20), box("inner"))
}
