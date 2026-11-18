//go:build tvos

package tvkit_test

import (
	"math"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/tv/focus"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// boxTol is the tolerance of a box against the prototype's layout.json: 2 px
// (docs/plan/P7 section 5, P7.1 done-when).
const boxTol = 2

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// near asserts that got is within tol of want on every side.
func near(t *testing.T, want, got geom.Rect, tol float32, what string) {
	t.Helper()
	assert.InDelta(t, want.X, got.X, float64(tol), "%s x: want %v got %v", what, want, got)
	assert.InDelta(t, want.Y, got.Y, float64(tol), "%s y: want %v got %v", what, want, got)
	assert.InDelta(t, want.W, got.W, float64(tol), "%s w: want %v got %v", what, want, got)
	assert.InDelta(t, want.H, got.H, float64(tol), "%s h: want %v got %v", what, want, got)
}

// newNav returns the nav bar with the app's strings.
func newNav(on int) *tvkit.Nav {
	tx := texts()

	return tvkit.NewNav(on, tx["splash.brand"], [tvkit.NavItems]string{tx["tabs.home"], tx["tv.nav.courses"], tx["tv.nav.music"], tx["tv.nav.settings"]})
}

// element returns the element key of a layout dump of the tester's tree.
func element(t *testing.T, tt *ui.Tester, key string) geom.Rect {
	t.Helper()
	lay := layoutdump.Dump(tt.View().Context(), tt.View().Root(), "T", "tv", geom.Sz(tvtest.Width, tvtest.Height), nil)
	e, ok := lay.Find(key)
	require.True(t, ok, "no element %q", key)

	return geom.XYWH(float32(e.X), float32(e.Y), float32(e.W), float32(e.H))
}

// TestNavBoxesMatchPrototype compares the bar, its four items and the avatar
// with TV04 (item 0 on) and TV12 (item 3 on, nav0 focused: the scaled box).
func TestNavBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	t.Run("TV04", func(t *testing.T) {
		t.Parallel()
		root := &stage{}
		nav := newNav(0)
		root.putSized(nav, 0, 0, tvtest.Width, tvtest.Height)
		tt := mount(t, root)
		for _, key := range []string{"div.tnav", "nav0", "nav1", "nav2", "nav3", "div.tav"} {
			near(t, tvtest.ProtoBox(t, "TV04", key), element(t, tt, key), boxTol, "TV04 "+key)
		}
	})
	t.Run("TV12", func(t *testing.T) {
		t.Parallel()
		root := &stage{}
		nav := newNav(3)
		root.putSized(nav, 0, 0, tvtest.Width, tvtest.Height)
		tt := mount(t, root)
		tt.View().Focus(nav.Item(0))
		tt.Elapse(tvkit.Transition)
		for _, key := range []string{"div.tnav", "nav0", "nav1", "nav2", "nav3", "div.tav"} {
			near(t, tvtest.ProtoBox(t, "TV12", key), element(t, tt, key), boxTol, "TV12 "+key)
		}
	})
}

// TestNavLook checks the fill, the ink and the weight of the items (TV04, TV12):
// on is --sf2 and --tx 700, the others --tx2 500 with no fill, focused --pr and
// --on at once with the scale 1.04 growing over 150 ms.
func TestNavLook(t *testing.T) {
	t.Parallel()
	root := &stage{}
	nav := newNav(0)
	root.putSized(nav, 0, 0, tvtest.Width, tvtest.Height)
	tt := mount(t, root)
	ctx := tt.View().Context()
	style := func(i int) (string, string, string, string) {
		s, txt := nav.Item(i).LayoutStyle(ctx)

		return s.Background, s.Color, s.FontWeight, txt
	}
	bg, col, w, txt := style(0)
	assert.Equal(t, "rgb(42, 45, 51)", bg)
	assert.Equal(t, "rgb(244, 244, 242)", col)
	assert.Equal(t, "700", w)
	assert.Equal(t, "Trang chủ", txt)
	bg, col, w, _ = style(1)
	assert.Equal(t, "rgba(0, 0, 0, 0)", bg)
	assert.Equal(t, "rgb(244, 244, 242)", col, "--tx, not the prototype's --tx2 (K42)")
	assert.Equal(t, "500", w)

	tt.View().Focus(nav.Item(1))
	tt.Elapse(tvtest.Frame)
	bg, col, w, _ = style(1)
	assert.Equal(t, "rgb(255, 208, 40)", bg, "the fill is --pr at once")
	assert.Equal(t, "rgb(20, 21, 24)", col, "the ink is --on at once")
	assert.Equal(t, "500", w)
	assert.Greater(t, nav.Item(1).Scale(), float32(1))
	assert.Less(t, nav.Item(1).Scale(), tvkit.NavFocusScale, "still growing")
	tt.Elapse(tvkit.Transition)
	assert.InDelta(t, tvkit.NavFocusScale, nav.Item(1).Scale(), .0001)
	assert.Equal(t, float32(1.04), tvkit.NavFocusScale)

	// the item that is on and focused: --pr, bold
	tt.View().Focus(nav.Item(0))
	tt.Elapse(tvkit.Transition)
	bg, col, w, _ = style(0)
	assert.Equal(t, "rgb(255, 208, 40)", bg)
	assert.Equal(t, "rgb(20, 21, 24)", col)
	assert.Equal(t, "700", w)
	assert.InDelta(t, 1, nav.Item(1).Scale(), .0001, "the item left is back at 1")
}

// TestNavIsChromeAndSelects: the items are `nav0`..`nav3`, in a focus.Chrome,
// and Select on one tells the nav.
func TestNavIsChromeAndSelects(t *testing.T) {
	t.Parallel()
	root := &stage{}
	nav := newNav(0)
	root.putSized(nav, 0, 0, tvtest.Width, tvtest.Height)
	var got []int
	nav.OnSelect = func(i int) { got = append(got, i) }
	tt := mount(t, root)

	for i := range tvkit.NavItems {
		it := nav.Item(i)
		assert.Equal(t, tvkit.NavKey(i), it.FocusKey())
		assert.Equal(t, tvkit.NavKey(i), it.Name)
		assert.True(t, it.Activate(false))
	}
	assert.Equal(t, []int{0, 1, 2, 3}, got)
	assert.Nil(t, nav.Item(4))
	assert.Nil(t, nav.Item(-1))
	_ = tt
	var _ focus.Item = nav.Item(0)
}

// TestNavWalk walks the bar by key on the real layout: Right to the last
// item and back, edges stay.
func TestNavWalk(t *testing.T) {
	t.Parallel()
	root := &stage{}
	nav := newNav(0)
	root.putSized(nav, 0, 0, tvtest.Width, tvtest.Height)
	tt := mount(t, root)
	tt.View().Focus(nav.Item(0))
	for i := 1; i < tvkit.NavItems; i++ {
		require.True(t, tt.Key(keys.KeyRight, 0))
		assert.Same(t, nav.Item(i), tt.View().Focused())
	}
	assert.False(t, tt.Key(keys.KeyRight, 0))
	assert.Same(t, nav.Item(3), tt.View().Focused())
	for i := 2; i >= 0; i-- {
		require.True(t, tt.Key(keys.KeyLeft, 0))
		assert.Same(t, nav.Item(i), tt.View().Focused())
	}
	assert.False(t, tt.Key(keys.KeyLeft, 0))
	assert.False(t, tt.Key(keys.KeyUp, 0))
	assert.False(t, tt.Key(keys.KeyDown, 0))
	_ = time.Second
}

// TestNavSetOnRelays: the item that is on is bold, hence wider: the bar is
// laid out again.
func TestNavSetOnRelays(t *testing.T) {
	t.Parallel()
	root := &stage{}
	nav := newNav(0)
	root.putSized(nav, 0, 0, tvtest.Width, tvtest.Height)
	tt := mount(t, root)
	before := element(t, tt, "div.tnav")
	nav.SetOn(3)
	tt.Elapse(tvtest.Frame)
	after := element(t, tt, "div.tnav")
	near(t, tvtest.ProtoBox(t, "TV12", "div.tnav"), after, boxTol, "TV12 bar")
	assert.NotEqual(t, before.W, after.W)
	nav.SetOn(3) // the same: nothing
}

// whiteHero paints the page white: the worst hero picture for a pill of white at
// .06 and gray text.
type whiteHero struct{ ui.Base }

// Paint fills the page white.
func (whiteHero) Paint(_ *ui.Context, p *ui.Painter) {
	p.Fill(geom.XYWH(0, 0, tvtest.Width, tvtest.Height), scene.Solid(color.RGB(255, 255, 255)), 0)
}

// TestNavIsLegibleOverALightHero: over a white page the pill is a dark scrim with
// a light ring, and the --tx text of an item has the contrast of 4.5 to it (K42,
// owner 2026-11-18: the four items sank into a hero of a like colour).
func TestNavIsLegibleOverALightHero(t *testing.T) {
	t.Parallel()
	root := &stage{}
	root.putSized(&whiteHero{}, 0, 0, tvtest.Width, tvtest.Height)
	nav := newNav(0)
	root.putSized(nav, 0, 0, tvtest.Width, tvtest.Height)
	tt := mount(t, root)
	bar := element(t, tt, "div.tnav")
	img := tt.Snapshot()
	px := func(x, y float32) color.Color {
		c := img.RGBAAt(int(x), int(y))

		return color.RGB(c.R, c.G, c.B)
	}
	lum := func(c color.Color) float64 {
		f := func(v uint8) float64 {
			x := float64(v) / 255
			if x <= .03928 {
				return x / 12.92
			}

			return math.Pow((x+.055)/1.055, 2.4)
		}

		return .2126*f(c.R) + .7152*f(c.G) + .0722*f(c.B)
	}
	inside := px(bar.X+bar.W/2, bar.Y+3) // the pill's padding above the items
	tx := color.RGB(244, 244, 242)
	hi, lo := max(lum(inside), lum(tx)), min(lum(inside), lum(tx))
	assert.GreaterOrEqual(t, (hi+.05)/(lo+.05), 4.5, "text on the pill over white: %v", inside)
	assert.Less(t, lum(inside), lum(px(bar.X-30, bar.Y+bar.H/2)), "darker than the page")
	ring := px(bar.X+bar.W/2, bar.Y)
	assert.Greater(t, lum(ring), lum(inside), "the ring is lighter than the pill: %v over %v", ring, inside)
}
