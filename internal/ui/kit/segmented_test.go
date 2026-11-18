package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSegmentedBoxesMatchPrototype compares both controls of the prototype
// with their layout.json: M07's five speeds (69 wide from x 24) and M04's four
// filters (86.25).
func TestSegmentedBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, state string
		options     []string
		keys        []string
		prefix      string
		selected    int
	}{
		{"speed", "M07", []string{"0.5x", "0.75x", "1x", "1.25x", "1.5x"}, []string{"0.5", "0.75", "1", "1.25", "1.5"}, "speed:", 1},
		{"filter", "M04", []string{"Tất cả 15", "Thường", "Đặc biệt", "Giới thiệu"}, []string{"Tất cả", "Thường", "Đặc biệt", "Giới thiệu"}, "filter:", 0},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				want := protoBox(t, tc.state, "div.segc")
				s := kit.NewSegmented(tc.options...)
				s.Keys, s.NamePrefix, s.Selected = tc.keys, tc.prefix, tc.selected
				mountStageReal(t, ph, placeAt(s, want.X, want.Y, want.W))
				rectNear(t, want, frameOf(s), 0, "div.segc")
				assert.Equal(t, "div.segc", s.Name)
				for i, k := range tc.keys {
					rectNear(t, protoBox(t, tc.state, tc.prefix+k), frameOf(s.Item(i)), .01, tc.prefix+k)
					assert.Equal(t, tc.prefix+k, s.Item(i).Node().Name)
				}
			})
		}
	})
}

// TestSegmentedSelect covers the tap on a button, the plate's colors and the
// semantics.
func TestSegmentedSelect(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var got []int
		s := kit.NewSegmented("0.5x", "0.75x", "1x", "1.25x", "1.5x")
		s.Selected = 2
		s.OnSelect = func(i int) { got = append(got, i) }
		tt := mountStage(t, ph, placeAt(s, 20, 100, 353))
		img := tt.Snapshot()
		// plate --sf with a --bd border; the chosen button --pr
		pixelNear(t, img, 120, 100, look.T.Border, 3, "border")
		pixelNear(t, img, 120, 101, look.T.Surface, 3, "plate")
		sel := frameOf(s.Item(2))
		pixelNear(t, img, int(sel.X)+3, int(sel.Y+sel.H/2), look.T.Primary, 2, "chosen button")
		other := frameOf(s.Item(0))
		pixelNear(t, img, int(other.X)+3, int(other.Y+other.H/2), look.T.Surface, 2, "other button")
		// pill radius 17: the button's corner pixel is the plate's
		pixelNear(t, img, int(sel.X), int(sel.Y), look.T.Surface, 3, "round button corner")

		tt.ClickAt(frameOf(s.Item(3)).Center())
		assert.Equal(t, []int{3}, got)
		assert.Equal(t, 3, s.Selected)
		img = tt.Snapshot()
		pixelNear(t, img, int(sel.X)+3, int(sel.Y+sel.H/2), look.T.Surface, 2, "no longer chosen: no transition")
		n := frameOf(s.Item(3))
		pixelNear(t, img, int(n.X)+3, int(n.Y+n.H/2), look.T.Primary, 2, "now chosen")

		tt.ClickAt(frameOf(s.Item(3)).Center())
		assert.Equal(t, []int{3}, got, "the chosen one pressed again tells nothing")
		tt.ClickAt(geom.Pt(frameOf(s.Item(0)).X+2, frameOf(s.Item(0)).Y+2))
		assert.Equal(t, []int{3, 0}, got)

		s.Select(4)
		assert.Equal(t, []int{3, 0}, got, "Select does not tell OnSelect")
		tt.Elapse(kittest.Frame)
		var on []string
		for _, nd := range tt.Semantics().Nodes {
			if nd.Role == platform.RoleRadio && nd.Flags.Has(platform.FlagSelected) {
				on = append(on, nd.Label)
			}
		}
		require.Equal(t, []string{"1.5x"}, on)
	})
}

// TestSegmentedPaintDoesNotAllocate checks that painting a control allocates
// nothing, and that a press does not either.
func TestSegmentedPaintDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		s := kit.NewSegmented("0.5x", "0.75x", "1x", "1.25x", "1.5x")
		tt := mountStage(t, ph, placeAt(s, 20, 100, 353))
		tt.Elapse(kittest.Frame)
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a repaint", ph.Name)
	}
}

// TestSegmentedNamesAndSelectDoNotAllocate checks that the buttons are named by
// prefix and key (and again when either changes), and that a selection with its
// layout and frame allocates nothing: the names are joined once, not on every
// layout.
func TestSegmentedNamesAndSelectDoNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		s := kit.NewSegmented("Tất cả 15", "Thường", "Đặc biệt")
		s.Keys = []string{"Tất cả", "Thường", "Đặc biệt"}
		s.NamePrefix = "filter:"
		tt := mountStage(t, ph, placeAt(s, 20, 100, 353))
		tt.Elapse(kittest.Frame)
		assert.Equal(t, "filter:Tất cả", s.Item(0).Node().Name, ph.Name)
		assert.Equal(t, "filter:Đặc biệt", s.Item(2).Node().Name, ph.Name)

		s.NamePrefix = "f:"
		s.Keys[1] = "Reg"
		s.Relayout()
		tt.Elapse(kittest.Frame)
		assert.Equal(t, "f:Tất cả", s.Item(0).Node().Name, ph.Name+": a new prefix")
		assert.Equal(t, "f:Reg", s.Item(1).Node().Name, ph.Name+": a new key")

		i := 0
		step := func() {
			s.Select(i % 3)
			i++
			tt.Frame()
		}
		for range 6 {
			step()
		}
		assert.Zero(t, testing.AllocsPerRun(30, step), ph.Name+": select, layout and frame")
	}
}

// TestSegmentedRestylesTheTextOnChange picks another speed twice, by a tap and by
// Select: the newly chosen button's text turns --on over --pr and the one left
// turns back to --tx2 over the plate, so neither is unreadable (the text style
// is chosen in the button's Layout, which the change must run again).
func TestSegmentedRestylesTheTextOnChange(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewSegmented("0.5x", "0.75x", "1x", "1.25x", "1.5x")
		s.Selected = 2
		tt := mountStage(t, ph, placeAt(s, 20, 100, 353))
		inks := func(i int, ink color.Color) int {
			img, r, n := tt.Snapshot(), frameOf(s.Item(i)), 0
			d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
			for y := int(r.Y); y < int(r.Y+r.H); y++ {
				for x := int(r.X); x < int(r.X+r.W); x++ {
					got := pixelAt(img, x, y)
					if d(got.R, ink.R) <= 24 && d(got.G, ink.G) <= 24 && d(got.B, ink.B) <= 24 {
						n++
					}
				}
			}

			return n
		}
		check := func(chosen, left int, how string) {
			assert.Greater(t, inks(chosen, look.T.OnPrimary), 8, "%s: chosen text in --on", how)
			assert.Greater(t, inks(left, look.T.Text2), 8, "%s: the button left shows --tx2 text", how)
		}

		tt.ClickAt(frameOf(s.Item(4)).Center())
		tt.Elapse(kittest.Frame)
		check(4, 2, "tap")

		s.Select(1)
		tt.Elapse(kittest.Frame)
		check(1, 4, "Select")
	})
}
