package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func el(name string, x, y, w, h float64) Element {
	return Element{Name: name, Index: 1, X: x, Y: y, W: w, H: h}
}

func lay(w float64, es ...Element) Layout {
	return Layout{Screen: Screen{W: w, H: 852}, Elements: es}
}

func opts() Options { return Options{Tol: 1, Margin: 24, Bottoms: []float64{38, 100}, CheckText: true} }

func TestCompareLayoutBoxes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		app  Element
		tol  float64
		want string
	}{
		{"identical", el("a", 20, 100, 353, 40), 1, "ok"},
		{"within 1 pt", el("a", 20.9, 100.5, 353, 40.9), 1, "ok"},
		{"over 1 pt", el("a", 20, 101.5, 353, 40), 1, "FAIL"},
		{"tv within 2 px", el("a", 22, 100, 353, 40), 2, "ok"},
		{"tv over 2 px", el("a", 22.5, 100, 353, 40), 2, "FAIL"},
	} {
		o := opts()
		o.Tol = c.tol
		rows, ok := CompareLayout(lay(393, el("a", 20, 100, 353, 40)), lay(393, c.app), o)
		require.Len(t, rows, 1, c.name)
		assert.Equal(t, c.want, rows[0].Status, c.name)
		assert.Equal(t, c.want == "ok", ok, c.name)
	}
}

func TestCompareLayoutMissingExtra(t *testing.T) {
	t.Parallel()
	proto := lay(393, el("a", 0, 0, 10, 10), el("b", 0, 20, 10, 10))
	app := lay(393, el("a", 0, 0, 10, 10), el("c", 0, 40, 10, 10))
	rows, ok := CompareLayout(proto, app, opts())
	assert.True(t, ok, "missing is only reported by default")
	st := map[string]string{}
	for _, r := range rows {
		st[r.Key] = r.Status
	}
	assert.Equal(t, map[string]string{"a#1": "ok", "b#1": "missing", "c#1": "extra"}, st)

	o := opts()
	o.RequireAll = true
	_, ok = CompareLayout(proto, app, o)
	assert.False(t, ok)

	o = opts()
	o.Exclude = regexp.MustCompile("^b$")
	rows, _ = CompareLayout(proto, app, o)
	for _, r := range rows {
		assert.NotEqual(t, "b#1", r.Key)
	}
}

func TestCompareLayoutMatchesByIndex(t *testing.T) {
	t.Parallel()
	a1, a2 := el("btn", 0, 0, 10, 10), el("btn", 0, 50, 10, 10)
	a2.Index = 2
	b1, b2 := a1, a2
	b2.Y = 53
	rows, ok := CompareLayout(lay(393, a1, a2), lay(393, b1, b2), opts())
	assert.False(t, ok)
	assert.Equal(t, "ok", rows[0].Status)
	assert.Equal(t, "btn#2", rows[1].Key)
	assert.Equal(t, "FAIL", rows[1].Status)
}

func TestCompareLayoutStyle(t *testing.T) {
	t.Parallel()
	base := Style{FontFamily: `"Be Vietnam Pro", sans-serif`, FontSize: "14px", FontWeight: "700", Color: "rgb(255, 208, 40)", Background: "rgba(0, 0, 0, 0)", Radius: "12px", LetterSpacing: "normal", Opacity: "1"}
	for _, c := range []struct {
		name   string
		mut    func(*Element)
		issues int
	}{
		{"same", func(e *Element) {}, 0},
		{"app omits fields", func(e *Element) { e.Style = Style{} }, 0},
		{"transparent channels ignored", func(e *Element) { e.Style.Background = "rgba(9, 9, 9, 0)" }, 0},
		{"family quotes", func(e *Element) { e.Style.FontFamily = "be vietnam pro" }, 0},
		{"bold keyword", func(e *Element) { e.Style.FontWeight = "bold" }, 0},
		{"font size", func(e *Element) { e.Style.FontSize = "15px" }, 1},
		{"weight", func(e *Element) { e.Style.FontWeight = "600" }, 1},
		{"color", func(e *Element) { e.Style.Color = "rgb(255, 208, 60)" }, 1},
		{"radius", func(e *Element) { e.Style.Radius = "16px" }, 1},
		{"background", func(e *Element) { e.Style.Background = "rgb(0, 0, 0)" }, 1},
		{"text case folded", func(e *Element) { e.Text = " hello  WORLD " }, 0},
		{"text differs", func(e *Element) { e.Text = "other" }, 1},
		{"two fields", func(e *Element) { e.Style.FontSize = "13px"; e.Style.Radius = "0px" }, 2},
	} {
		p := el("t", 0, 0, 10, 10)
		p.Text, p.Style = "Hello world", base
		a := p
		c.mut(&a)
		rows, ok := CompareLayout(lay(393, p), lay(393, a), opts())
		assert.Len(t, rows[0].Issues, c.issues, c.name)
		assert.Equal(t, c.issues == 0, ok, c.name)
	}
}

func TestStyleIgnoredWithoutText(t *testing.T) {
	t.Parallel()
	p, a := el("box", 0, 0, 10, 10), el("box", 0, 0, 10, 10)
	p.Style = Style{FontSize: "14px", Color: "rgb(1, 2, 3)"}
	a.Style = Style{FontSize: "20px", Color: "rgb(9, 9, 9)"}
	_, ok := CompareLayout(lay(393, p), lay(393, a), opts())
	assert.True(t, ok, "inherited text style of a box without text is noise")
}

func TestStretchRule(t *testing.T) {
	t.Parallel()
	proto := lay(393,
		el("full", 20, 100, 353, 48),   // both margins 20
		el("right", 330, 10, 44, 44),   // right-anchored
		el("centre", 150, 200, 93, 30), // equal margins
		el("left", 20, 300, 100, 30),   // left only
	)
	good := lay(412,
		el("full", 20, 100, 372, 48),
		el("right", 349, 10, 44, 44),
		el("centre", 159.5, 200, 93, 30),
		el("left", 20, 300, 100, 30),
	)
	rows, ok := CompareLayout(proto, good, opts())
	assert.True(t, ok, "%+v", rows)

	_, ok = CompareLayout(proto, lay(412,
		el("full", 20, 100, 353, 48), // not stretched
		el("right", 349, 10, 44, 44), el("centre", 159.5, 200, 93, 30), el("left", 20, 300, 100, 30)), opts())
	assert.False(t, ok)

	o := opts()
	o.NoStretch = true
	_, ok = CompareLayout(proto, good, o)
	assert.False(t, ok)
}

func TestStretchRuleSpreadRows(t *testing.T) {
	t.Parallel()
	// space-between: 5 items, equal 46.8 gaps, the first at 20 and the last ending at 373
	between := lay(393,
		el("a", 20, 100, 26, 16), el("b", 92.8, 100, 28, 32), el("c", 167.6, 100, 58, 58),
		el("d", 272.4, 100, 28, 32), el("e", 346.8, 100, 26.2, 16))
	// +19 over five items: 4 gaps, 4.75 each
	good := lay(412,
		el("a", 20, 100, 26, 16), el("b", 97.55, 100, 28, 32), el("c", 177.1, 100, 58, 58),
		el("d", 286.65, 100, 28, 32), el("e", 365.8, 100, 26.2, 16))
	rows, ok := CompareLayout(between, good, opts())
	assert.True(t, ok, "%+v", rows)
	// space-around: three items, margins half a gap past the padding 20
	around := lay(393, el("a", 56.8, 50, 40, 40), el("b", 170.9, 50, 61, 40), el("c", 305.6, 50, 30.5, 40))
	// +19 over three items: 19/6, 19/2, 19*5/6
	goodAround := lay(412, el("a", 59.97, 50, 40, 40), el("b", 180.4, 50, 61, 40), el("c", 321.4, 50, 30.5, 40))
	rows, ok = CompareLayout(around, goodAround, opts())
	assert.True(t, ok, "%+v", rows)
	_, ok = CompareLayout(around, lay(412, el("a", 56.8, 50, 40, 40), el("b", 170.9, 50, 61, 40), el("c", 305.6, 50, 30.5, 40)), opts())
	assert.False(t, ok)
}

// lay2 is lay with a height.
func lay2(w, h float64, els ...Element) Layout {
	l := lay(w, els...)
	l.Screen.H = h

	return l
}

func TestStretchRuleVertical(t *testing.T) {
	t.Parallel()
	proto := lay2(393, 852,
		el("top", 20, 54, 353, 52),        // top-anchored only: stays
		el("under", 68, 792, 231, 19),     // scrolls behind the bars (before them): stays
		el("tabs", 0, 768, 393, 84),       // bottom edge: moves by 40
		el("tab", 98, 769, 98, 83),        // inside the tab bar: moves with it
		el("sticky", 20, 760, 353, 54),    // 38 above the bottom: moves by 40
		el("stickyIcon", 98, 775, 20, 20), // inside it, after it: moves with it
		el("sheet", 0, 446, 393, 406),     // a sheet: moves by 40
		el("sheetRow", 20, 479, 353, 32),  // inside the sheet: moves by 40
		el("dim", 0, 0, 393, 852),         // both edges: grows by 40
		el("mid", 100, 411, 193, 30),      // vertically centred: moves by 20
		el("flow", 20, 300, 353, 40),      // content in the flow, outside any anchor: stays
	)
	good := lay2(393, 892,
		el("top", 20, 54, 353, 52),
		el("under", 68, 792, 231, 19),
		el("tabs", 0, 808, 393, 84),
		el("tab", 98, 809, 98, 83),
		el("sticky", 20, 800, 353, 54),
		el("stickyIcon", 98, 815, 20, 20),
		el("sheet", 0, 486, 393, 406),
		el("sheetRow", 20, 519, 353, 32),
		el("dim", 0, 0, 393, 892),
		el("mid", 100, 431, 193, 30),
		el("flow", 20, 300, 353, 40),
	)
	rows, ok := CompareLayout(proto, good, opts())
	assert.True(t, ok, "%+v", rows)

	bad := lay2(393, 892,
		el("top", 20, 54, 353, 52), el("under", 68, 792, 231, 19), el("tabs", 0, 768, 393, 84), el("tab", 98, 809, 98, 83), el("sticky", 20, 800, 353, 54), el("stickyIcon", 98, 815, 20, 20),
		el("sheet", 0, 486, 393, 406), el("sheetRow", 20, 519, 353, 32), el("dim", 0, 0, 393, 892), el("mid", 100, 431, 193, 30), el("flow", 20, 300, 353, 40))
	_, ok = CompareLayout(proto, bad, opts())
	assert.False(t, ok, "a tab bar that does not follow the bottom edge")

	o := opts()
	o.NoStretch = true
	_, ok = CompareLayout(proto, good, o)
	assert.False(t, ok)
}

func TestStretchRuleNested(t *testing.T) {
	t.Parallel()
	// a card with a row in it (padding 14): the row grows, its flexible text
	// grows, the chevron moves; three tabs of one size share; two boxes of a
	// centred pair move by half; a strip that scrolls off the screen stays.
	proto := lay(393,
		el("card", 20, 100, 353, 60),
		el("ico", 34, 110, 36, 36),
		el("text", 82, 112, 245, 19),
		el("chev", 339, 112, 20, 20),
		el("tabs", 0, 700, 393, 84),
		el("t1", 0, 701, 131, 83), el("t2", 131, 701, 131, 83), el("t3", 262, 701, 131, 83),
		el("a", 100, 300, 46, 60), el("b", 247, 300, 46, 60),
		el("c1", 20, 500, 156, 100), el("c2", 188, 500, 156, 100), el("c3", 356, 500, 156, 100),
	)
	app := lay(412,
		el("card", 20, 100, 372, 60),
		el("ico", 34, 110, 36, 36),
		el("text", 82, 112, 264, 19),
		el("chev", 358, 112, 20, 20),
		el("tabs", 0, 700, 412, 84),
		el("t1", 0, 701, 137.33, 83), el("t2", 137.33, 701, 137.33, 83), el("t3", 274.67, 701, 137.33, 83),
		el("a", 109.5, 300, 46, 60), el("b", 256.5, 300, 46, 60),
		el("c1", 20, 500, 156, 100), el("c2", 188, 500, 156, 100), el("c3", 356, 500, 156, 100),
	)
	rows, ok := CompareLayout(proto, app, opts())
	assert.True(t, ok, "%+v", rows)
}

func TestColorEqual(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"rgb(1, 2, 3)", "rgb(1, 2, 3)", true},
		{"rgb(1, 2, 3)", "rgba(1, 2, 4, 1)", true},
		{"rgb(1, 2, 3)", "#010203", true},
		{"rgba(255, 255, 255, 0.5)", "rgba(255, 255, 255, 0.55)", false},
		{"rgba(255, 255, 255, 0.5)", "#ffffff80", true},
		{"rgba(0, 0, 0, 0)", "transparent", false},
		{"rgb(1, 2, 3)", "rgb(1, 2, 9)", false},
	} {
		assert.Equal(t, c.want, colorEqual(c.a, c.b), c.a+" "+c.b)
	}
}

func TestBorderEqual(t *testing.T) {
	t.Parallel()
	assert.True(t, borderEqual("0px rgb(1, 1, 1)", "0px rgb(9, 9, 9)"))
	assert.True(t, borderEqual("1px rgb(1, 1, 1)", "1px rgb(1, 1, 1)"))
	assert.False(t, borderEqual("1px rgb(1, 1, 1)", "2px rgb(1, 1, 1)"))
	assert.False(t, borderEqual("1px rgb(1, 1, 1)", "1px rgb(9, 9, 9)"))
}

func solid(w, h int, c color.RGBA) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return im
}

func fill(im *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			im.Set(x, y, c)
		}
	}
}

func TestComparePixelsThreshold(t *testing.T) {
	t.Parallel()
	grey := color.RGBA{100, 100, 100, 255}
	for _, c := range []struct {
		name   string
		patch  int // side of a patch in a 100x100 crop
		delta  uint8
		mask   []Rect
		status string
	}{
		{"identical", 0, 0, nil, "ok"},
		{"1.21 % off", 11, 100, nil, "ok"},
		{"2.25 % off", 15, 100, nil, "FAIL"},
		{"small channel delta", 50, 30, nil, "ok"},
		{"large channel delta", 50, 40, nil, "FAIL"},
		{"patch masked", 50, 100, []Rect{{0, 0, 50, 50}}, "ok"},
		{"everything masked", 50, 100, []Rect{{0, 0, 100, 100}}, "masked"},
	} {
		pi, ai := solid(100, 100, grey), solid(100, 100, grey)
		fill(ai, image.Rect(0, 0, c.patch, c.patch), color.RGBA{100 + c.delta, 100, 100, 255})
		pp, ap := Picture{pi, 1}, Picture{ai, 1}
		bad := image.NewGray(image.Rect(0, 0, 100, 100))
		r := ComparePixels(pp, ap, Box{0, 0, 100, 100}, Box{0, 0, 100, 100}, c.mask, 0, bad, pixelMax)
		assert.Equal(t, c.status, r.Status, c.name)
	}
}

func TestComparePixelsResamples(t *testing.T) {
	t.Parallel()
	pi := solid(50, 50, color.RGBA{10, 20, 30, 255})
	ai := solid(150, 150, color.RGBA{10, 20, 30, 255}) // 3x app against 1x prototype
	r := ComparePixels(Picture{pi, 1}, Picture{ai, 3}, Box{10, 10, 20, 20}, Box{10, 10, 20, 20}, nil, 0, image.NewGray(image.Rect(0, 0, 50, 50)), pixelMax)
	assert.Equal(t, "ok", r.Status)
	assert.Equal(t, 400, r.Compared)
}

func TestMasks(t *testing.T) {
	t.Parallel()
	proto := Layout{Screen: Screen{393, 852}, Elements: []Element{el("video", 0, 100, 393, 200), el("x", 0, 400, 10, 10)}}
	m := Masks{Rects: []Rect{{1, 2, 3, 4}}, StatusBar: 54, Names: regexp.MustCompile("^video$")}
	rects := m.maskRects(proto)
	assert.Equal(t, []Rect{{1, 2, 3, 4}, {0, 0, 393, 54}, {0, 100, 393, 200}}, rects)
	assert.True(t, maskedAt(10, 10, 393, 852, 1, rects, 0))
	assert.True(t, maskedAt(10, 150, 393, 852, 1, rects, 0))
	assert.False(t, maskedAt(10, 450, 393, 852, 1, rects, 0))
	assert.True(t, maskedAt(0, 851, 393, 852, 1, nil, 49), "frame corner")
	assert.False(t, maskedAt(200, 800, 393, 852, 1, nil, 49))
	id, r, err := parseMask("M02:1,2,3.5,4")
	require.NoError(t, err)
	assert.Equal(t, "M02", id)
	assert.Equal(t, Rect{1, 2, 3.5, 4}, r)
	_, _, err = parseMask("1,2,3")
	assert.Error(t, err)
}

func TestDetectScale(t *testing.T) {
	t.Parallel()
	assert.InDelta(t, 3, detectScale(1179, 393, 0), 1e-9)
	assert.InDelta(t, 1, detectScale(1920, 1920, 0), 1e-9)
	assert.InDelta(t, 2, detectScale(1179, 393, 2), 1e-9)
	assert.InDelta(t, 3, detectScale(1182, 393, 0), 1e-9, "3x with the 3 px frame column")
	assert.InDelta(t, 1, detectScale(394, 393, 0), 1e-9)
	assert.InDelta(t, 1.02, detectScale(400, 393, 0), 1e-9)
}

func writeState(t *testing.T, dir, id string, l Layout, im image.Image) {
	t.Helper()
	b, err := json.Marshal(l)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, id+".layout.json"), b, 0o644))
	f, err := os.Create(filepath.Join(dir, id+".png"))
	require.NoError(t, err)
	require.NoError(t, png.Encode(f, im))
	require.NoError(t, f.Close())
}

func TestRunEndToEnd(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pd, ad := filepath.Join(root, "p"), filepath.Join(root, "a")
	require.NoError(t, os.Mkdir(pd, 0o755))
	require.NoError(t, os.Mkdir(ad, 0o755))
	bg := color.RGBA{20, 20, 20, 255}
	pi := solid(393, 852, bg)
	fill(pi, image.Rect(20, 100, 373, 148), color.RGBA{255, 208, 40, 255})
	pl := Layout{ID: "S1", Platform: "mobile", Screen: Screen{393, 852}, Elements: []Element{el("btn", 20, 100, 353, 48)}}
	writeState(t, pd, "S1", pl, pi)
	writeState(t, ad, "S1", pl, pi) // same
	// S2: the app draws the button in another color.
	ai := solid(393, 852, bg)
	fill(ai, image.Rect(20, 100, 373, 148), color.RGBA{255, 0, 0, 255})
	writeState(t, pd, "S2", pl, pi)
	writeState(t, ad, "S2", pl, ai)
	// S3: Android width, same layout stretched, no pixel compare.
	al := Layout{Screen: Screen{412, 892}, Elements: []Element{el("btn", 20, 100, 372, 48)}}
	writeState(t, pd, "S3", pl, pi)
	writeState(t, ad, "S3", al, solid(412, 892, bg))

	rep, js, diff := filepath.Join(root, "r.md"), filepath.Join(root, "r.json"), filepath.Join(root, "diff")
	common := []string{"-proto", pd, "-app", ad, "-report", rep, "-json", js, "-diff", diff, "-mode", "layout,crop,full"}
	run1 := func(states string, extra ...string) (int, string) {
		var out, errb bytes.Buffer
		code := run(append(append([]string{}, common...), append(extra, "-states", states)...), &out, &errb)
		return code, out.String() + errb.String()
	}
	code, out := run1("S1")
	assert.Equal(t, exitPass, code, out)
	code, out = run1("S2")
	assert.Equal(t, exitFail, code, out)
	assert.Contains(t, out, "pixels")
	assert.FileExists(t, filepath.Join(diff, "S2.png"))
	code, out = run1("S2", "-mask", "S2:20,100,353,48")
	assert.Equal(t, exitPass, code, "masked: "+out)
	code, out = run1("S2", "-mask-name", "^btn$")
	assert.Equal(t, exitPass, code, out)
	code, out = run1("S3")
	assert.Equal(t, exitPass, code, out)
	assert.Contains(t, out, "pixel compare skipped")
	code, _ = run1("S1,S3")
	assert.Equal(t, exitPass, code)
	b, err := os.ReadFile(js)
	require.NoError(t, err)
	var res []StateResult
	require.NoError(t, json.Unmarshal(b, &res))
	assert.Len(t, res, 2)
	assert.FileExists(t, rep)
	code, _ = run1("NOPE")
	assert.Equal(t, exitUsage, code)
	code, _ = run1("S1", "-mode", "bogus")
	assert.Equal(t, exitUsage, code)
	code, _ = run1("S1", "-mask", "bad")
	assert.Equal(t, exitUsage, code)
}

func TestRunProto3xFrameColumn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pd, ad, p3 := filepath.Join(root, "p"), filepath.Join(root, "a"), filepath.Join(root, "p3")
	for _, d := range []string{pd, ad, p3} {
		require.NoError(t, os.Mkdir(d, 0o755))
	}
	bg, ink := color.RGBA{20, 20, 20, 255}, color.RGBA{255, 208, 40, 255}
	l := Layout{ID: "S1", Platform: "mobile", Screen: Screen{393, 852}, Elements: []Element{el("btn", 20, 100, 80, 40)}}
	writeState(t, pd, "S1", l, solid(393, 852, bg))
	// App at 3x: the box at 60..300 x 300..420.
	ai := solid(1179, 2556, bg)
	fill(ai, image.Rect(60, 300, 300, 420), ink)
	writeState(t, ad, "S1", l, ai)
	// Prototype at 3x: 1182 wide, the content starts at column 3.
	pi := solid(1182, 2556, bg)
	fill(pi, image.Rect(63, 300, 303, 420), ink)
	f, err := os.Create(filepath.Join(p3, "S1.png"))
	require.NoError(t, err)
	require.NoError(t, png.Encode(f, pi))
	require.NoError(t, f.Close())
	var out, errb bytes.Buffer
	code := run([]string{"-proto", pd, "-app", ad, "-proto-3x", p3, "-states", "S1", "-mode", "layout,crop",
		"-report", filepath.Join(root, "report.md"), "-json", filepath.Join(root, "report.json"), "-diff", filepath.Join(root, "diff")}, &out, &errb)
	assert.Equal(t, exitPass, code, out.String()+errb.String())
}

func TestHelp(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	assert.Equal(t, exitPass, run([]string{"-h"}, &out, &errb))
	assert.Contains(t, errb.String(), "-mode")
	assert.Contains(t, errb.String(), "-mask")
}

func TestAlignPictureOrigin(t *testing.T) {
	t.Parallel()
	white, red := color.RGBA{255, 255, 255, 255}, color.RGBA{255, 0, 0, 255}
	// A 394 wide prototype shot: column 0 is the frame, content starts at 1.
	framed := solid(394, 852, white)
	fill(framed, image.Rect(0, 0, 1, 852), color.RGBA{0, 0, 0, 255})
	fill(framed, image.Rect(1, 10, 11, 20), red)
	scr := Screen{W: 393, H: 852}
	for _, c := range []struct {
		name      string
		im        image.Image
		scr       Screen
		org       Origin
		wantW     int
		wantScale float64
		redAt     int // x of the first red pixel in row 10, -1: none
	}{
		{"auto 394 wide", framed, scr, Origin{Auto: true}, 393, 1, 0},
		{"explicit 1,0", framed, scr, Origin{X: 1}, 393, 1, 0},
		{"zero origin keeps the frame", framed, scr, Origin{}, 394, 1, 1},
		{"auto exact width", solid(393, 852, white), scr, Origin{Auto: true}, 393, 1, -1},
		{"auto 3x exact", solid(1179, 2556, white), scr, Origin{Auto: true}, 1179, 3, -1},
		{"auto 3x framed", solid(1182, 2556, white), scr, Origin{Auto: true}, 1179, 3, -1},
		{"auto tv", solid(1920, 1080, white), Screen{W: 1920, H: 1080}, Origin{Auto: true}, 1920, 1, -1},
		{"auto far off is a scale, not a frame", solid(400, 852, white), scr, Origin{Auto: true}, 400, 1.02, -1},
	} {
		p := alignPicture(c.im, c.scr, 0, c.org)
		assert.Equal(t, c.wantW, p.Img.Bounds().Dx(), c.name)
		assert.InDelta(t, c.wantScale, p.Scale, 0.001, c.name)
		first := -1
		for x := 0; x < p.Img.Bounds().Dx(); x++ {
			if r, g, _, _ := p.Img.At(p.Img.Bounds().Min.X+x, 10).RGBA(); r > 0 && g == 0 {
				first = x
				break
			}
		}
		assert.Equal(t, c.redAt, first, c.name)
	}
}

func TestOriginMakesCropsAlign(t *testing.T) {
	t.Parallel()
	white, red := color.RGBA{255, 255, 255, 255}, color.RGBA{255, 0, 0, 255}
	proto := solid(394, 852, white)
	fill(proto, image.Rect(21, 100, 101, 140), red) // layout box x 20..100
	app := solid(393, 852, white)
	fill(app, image.Rect(20, 100, 100, 140), red)
	box := Box{20, 100, 80, 40}
	pp := alignPicture(proto, Screen{W: 393, H: 852}, 0, Origin{Auto: true})
	ap := alignPicture(app, Screen{W: 393, H: 852}, 0, Origin{Auto: true})
	assert.Equal(t, 0, ComparePixels(pp, ap, box, box, nil, 0, nil, pixelMax).Bad)
	raw := Picture{Img: proto, Scale: 1}
	assert.Greater(t, ComparePixels(raw, ap, box, box, nil, 0, nil, pixelMax).Bad, 0)
}

func TestParseOrigin(t *testing.T) {
	t.Parallel()
	o, err := parseOrigin("")
	require.NoError(t, err)
	assert.True(t, o.Auto)
	o, err = parseOrigin("1,2")
	require.NoError(t, err)
	assert.Equal(t, Origin{X: 1, Y: 2}, o)
	_, err = parseOrigin("x")
	assert.Error(t, err)
}

func TestSameTextCappedByTheCapture(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("ab ", 40)
	cut := strings.TrimSpace(long)[:captureTextCap]
	assert.True(t, sameText(cut, long), "the capture keeps 80 units")
	assert.True(t, sameText("Khác", "KHÁC"), "case folds")
	assert.False(t, sameText("abc", "abcd"), "a short text is not a cut one")
	assert.False(t, sameText(cut, strings.Repeat("ab ", 20)+strings.Repeat("xy ", 20)), "the first 80 units must agree")
}

// TestTextThresholdAndWaiver: a patch of 5 % of the crop fails an element with
// no text, passes one with text (-text-max 8 %), and a waived failure is reported
// as waived while the state passes; the 3x folder is read at scale 3 from 3,0.
func TestTextThresholdAndWaiver(t *testing.T) {
	t.Parallel()
	grey := color.RGBA{100, 100, 100, 255}
	l := func(text string) Layout {
		e := el("box", 0, 0, 100, 100)
		e.Text = text
		return Layout{ID: "S", Screen: Screen{100, 100}, Elements: []Element{e}}
	}
	run := func(text string, mod func(c *Config)) StateResult {
		pdir, adir := t.TempDir(), t.TempDir()
		pi, ai := solid(100, 100, grey), solid(100, 100, grey)
		fill(ai, image.Rect(0, 0, 22, 22), color.RGBA{255, 100, 100, 255}) // 4.84 %
		writeState(t, pdir, "S", l(text), pi)
		writeState(t, adir, "S", l(text), ai)
		c := Config{ProtoDir: pdir, AppDir: adir, Layout: true, Crop: true, Opts: Options{Tol: 1, Margin: 24}, TextMax: 0.08, MinCrop: 4}
		if mod != nil {
			mod(&c)
		}
		r, err := CompareState(c, "S")
		require.NoError(t, err)
		return r
	}
	assert.False(t, run("", nil).Pass, "no text: 1.5 %")
	assert.True(t, run("Hi", nil).Pass, "text: 8 %")
	r := run("", func(c *Config) { c.Waived = map[string]bool{"S:box#1": true} })
	assert.True(t, r.Pass)
	assert.Equal(t, "waived", r.Rows[0].Status)
}

// TestStretchRuleMovedAndBottomInset covers the two switches of the Android rule
// that the boxes alone cannot tell: -vmove (the splash's centred column and its
// bottom bar) and -bottom-inset (the tab bar grows with the system's inset).
func TestStretchRuleMovedAndBottomInset(t *testing.T) {
	t.Parallel()
	proto := lay2(393, 852,
		el("title", 61.5, 388, 270, 72), // child of a centred column, margins 388 / 392
		el("bar", 136, 727, 120, 5),     // bottom:120, 120 wide
		el("tabs", 0, 768, 393, 84),
		el("ico", 38, 781, 22, 22),
	)
	app := lay2(412, 892,
		el("title", 71, 408, 270, 72),
		el("bar", 146, 767, 120, 5),
		el("tabs", 0, 794, 412, 98), // 3-button navigation: inset 48, 14 above 34
		el("ico", 38, 807, 22, 22),
	)
	o := opts()
	_, ok := CompareLayout(proto, app, o)
	assert.False(t, ok, "without the switches the splash and the tab bar differ")

	o.Moved = map[string]float64{"title#1": 0.5, "bar#1": 1}
	o.BottomExtra = 14
	rows, ok := CompareLayout(proto, app, o)
	assert.True(t, ok, "%+v", rows)
}
