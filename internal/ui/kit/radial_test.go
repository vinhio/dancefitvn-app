// Adapted from jplayer-tv/internal/ui/kit/glow_test.go at 781bee5.

package kit_test

import (
	"image/png"
	"math"
	"os"
	"testing"

	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRadialAtCompositesLikeCSS(t *testing.T) {
	t.Parallel()

	bg := color.RGB(10, 20, 30)
	red, blue := color.RGBA(200, 0, 0, .5), color.RGBA(0, 0, 200, .4)
	layers := []kit.Radial{
		{Shape: kit.RadialEllipse, CX: .5, CY: .5, Color: red, End: .5},
		{Shape: kit.RadialEllipse, CX: .5, CY: .5, Color: blue, End: 1},
	}
	const w, h = 400, 200

	// At the centre: blue at its alpha over bg, then red at its alpha over that
	// (the first layer is on top).
	r, g, b := kit.RadialAt(bg, layers, w, h, w/2, h/2)
	br, bgg, bb := 10+(0-10)*.4, 20+(0-20)*.4, 30+(200-30)*.4
	assert.InDelta(t, br+(200-br)*.5, r, 1.5)
	assert.InDelta(t, bgg+(0-bgg)*.5, g, 1.5)
	assert.InDelta(t, bb+(0-bb)*.5, b, 1.5)

	// Far corner: the ellipse of the layer reaches it at 1, past red's 50 %.
	r, g, b = kit.RadialAt(bg, layers[:1], w, h, 0, 0)
	assert.Equal(t, [3]float64{10, 20, 30}, [3]float64{r, g, b})

	// farthest-corner ellipse: radii sqrt(2) times the farthest sides (200 and
	// 100), so the red has faded to half of its alpha at a quarter of the
	// ellipse's radius along the width (a ray of 25 % of End = 50 %).
	rx := 200 * math.Sqrt2
	r, _, _ = kit.RadialAt(bg, layers[:1], w, h, w/2+rx*.25, h/2)
	a := .5 * (1 - .25/.5)
	assert.InDelta(t, 10+(200-10)*a, r, 1.5)
	// And along the height the same share of the height's radius.
	ry := 100 * math.Sqrt2
	r2, _, _ := kit.RadialAt(bg, layers[:1], w, h, w/2, h/2+ry*.25)
	assert.InDelta(t, r, r2, 1.5, "the ellipse has the box's aspect")
}

func TestRadialCircleReachesTheFarthestCorner(t *testing.T) {
	t.Parallel()

	// The login's `circle at 85% 5%, rgba(255,208,40,.3), transparent 45%` on a
	// 393x852 screen: the farthest corner is the bottom-left one.
	const w, h = 393, 852
	l := []kit.Radial{{Shape: kit.RadialCircle, CX: .85, CY: .05, Color: color.RGBA(255, 208, 40, .3), End: .45}}
	bg := tokens.Dark.Bg
	cx, cy := .85*w, .05*h
	ray := math.Hypot(cx, h-cy)

	at := func(dist float64) float64 {
		r, _, _ := kit.RadialAt(bg, l, w, h, cx, cy+dist)
		return r
	}
	alpha := float64(l[0].Color.A) / 255 // .3 as a byte
	centre := float64(bg.R) + (255-float64(bg.R))*alpha
	assert.InDelta(t, centre, at(0), 1e-9)
	assert.InDelta(t, float64(bg.R)+(255-float64(bg.R))*alpha*.5, at(ray*.225), 1e-6, "half way to the end")
	assert.InDelta(t, float64(bg.R), at(ray*.45), 1e-9, "faded out at 45 % of the ray")
	assert.InDelta(t, float64(bg.R), at(ray), 1e-9)
	// A circle: the same distance sideways is the same colour.
	r, _, _ := kit.RadialAt(bg, l, w, h, cx-ray*.1, cy)
	assert.InDelta(t, at(ray*.1), r, 1e-6)
}

func TestRadialOverIsPremultiplied(t *testing.T) {
	t.Parallel()

	l := []kit.Radial{{Shape: kit.RadialEllipse, CX: .5, CY: .35, Color: color.RGBA(255, 208, 40, .32), End: .6}}
	r, g, b, a := kit.RadialOverAt(l, 100, 100, 50, 35)
	alpha := float64(l[0].Color.A) / 255 // .32 as a byte
	assert.InDelta(t, alpha, a, 1e-9)
	assert.InDelta(t, alpha, r, 1e-9)
	assert.InDelta(t, 208.0/255*alpha, g, 1e-9)
	assert.InDelta(t, 40.0/255*alpha, b, 1e-9)
	_, _, _, a = kit.RadialOverAt(l, 100, 100, 0, 100)
	assert.Zero(t, a, "faded out")

	img := kit.RenderRadialsOver(l, 20, 20, nil)
	require.NotNil(t, img)
	assert.False(t, img.Opaque)
	assert.Greater(t, img.Pix[(7*20+10)*4+3], uint8(60), "alpha near the centre")
	assert.Zero(t, img.Pix[19*img.Stride+3], "none in the corner")
	for i := 0; i < len(img.Pix); i += 4 {
		assert.LessOrEqual(t, img.Pix[i], img.Pix[i+3], "premultiplied")
	}
}

func TestRenderRadialsReusesTheImage(t *testing.T) {
	t.Parallel()

	layers := []kit.Radial{{CX: 0, CY: 0, Color: color.RGBA(255, 0, 0, .5), End: .7}}
	img := kit.RenderRadials(color.RGB(0, 0, 0), layers, 16, 9, nil)
	require.NotNil(t, img)
	assert.Equal(t, 16, img.Width)
	assert.Equal(t, 9, img.Height)
	assert.Equal(t, uint64(1), img.Version)
	assert.Equal(t, uint8(255), img.Pix[3], "opaque")
	assert.True(t, img.Opaque)
	first := append([]byte(nil), img.Pix...)

	again := kit.RenderRadials(color.RGB(0, 0, 0), layers, 16, 9, img)
	assert.Same(t, img, again, "the image of that size is drawn into again")
	assert.Equal(t, uint64(2), again.Version, "with a new Version")
	assert.Equal(t, first, again.Pix)

	other := kit.RenderRadials(color.RGB(0, 0, 0), layers, 8, 8, img)
	assert.NotSame(t, img, other, "another size needs another image")
	assert.Nil(t, kit.RenderRadials(color.RGB(0, 0, 0), layers, 0, 8, nil))
}

// TestRadialMatchesPrototypeM01 renders the splash background of the
// prototype, `radial-gradient(ellipse at 50% 45%, rgba(229,56,59,.45),
// transparent 60%), var(--bg)` (index.html:328), at 393x852 and compares it
// with the capture of M01 at points away from the status bar, the dancer, the
// texts, the progress bar and the device frame.
func TestRadialMatchesPrototypeM01(t *testing.T) {
	t.Parallel()

	f, err := os.Open("../../../local/shots/prototype-1x/mobile/M01.png")
	if err != nil {
		t.Skip("no local/shots (it is not in the repository)")
	}
	defer f.Close()
	shot, err := png.Decode(f)
	require.NoError(t, err)
	require.InDelta(t, 393, shot.Bounds().Dx(), 1) // the capture of #m is 394 wide
	require.Equal(t, 852, shot.Bounds().Dy())

	layers := []kit.Radial{{Shape: kit.RadialEllipse, CX: .5, CY: .45, Color: color.RGBA(229, 56, 59, .45), End: .6}}
	img := kit.RenderRadials(tokens.Dark.Bg, layers, 393, 852, nil)
	require.NotNil(t, img)

	var worst int
	var n int
	check := func(x, y int) {
		pr, pg, pb, _ := shot.At(x, y).RGBA()
		o := y*img.Stride + x*4
		for i, want := range []uint8{uint8(pr >> 8), uint8(pg >> 8), uint8(pb >> 8)} {
			d := int(img.Pix[o+i]) - int(want)
			if d < 0 {
				d = -d
			}
			worst = max(worst, d)
		}
		n++
	}
	for _, y := range []int{90, 130, 170, 210, 250, 290} {
		for x := 20; x < 380; x += 30 {
			check(x, y)
		}
	}
	for _, y := range []int{545, 580, 620, 660, 690} {
		for x := 20; x < 380; x += 30 {
			check(x, y)
		}
	}
	// Beside the dancer (x 100 to 293) and the brand (x 60 to 332).
	for _, y := range []int{325, 345, 480, 500, 540} {
		for _, x := range []int{25, 40, 350, 370} {
			check(x, y)
		}
	}
	assert.Greater(t, n, 100)
	assert.LessOrEqual(t, worst, 3, "channel difference at %d sample points", n)
}

func TestRenderRadialsDoesNotAllocateIntoAnImage(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	layers := []kit.Radial{{Shape: kit.RadialCircle, CX: .2, CY: .1, Color: color.RGBA(255, 0, 0, .55), End: .7}}
	img := kit.RenderRadials(color.RGB(0, 0, 0), layers, 48, 27, nil)
	assert.Zero(t, testing.AllocsPerRun(10, func() { kit.RenderRadials(color.RGB(0, 0, 0), layers, 48, 27, img) }))
	over := kit.RenderRadialsOver(layers, 48, 27, nil)
	assert.Zero(t, testing.AllocsPerRun(10, func() { kit.RenderRadialsOver(layers, 48, 27, over) }))
}
