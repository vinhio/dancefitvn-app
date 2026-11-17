package kit_test

import (
	"slices"
	"testing"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIconSizesAndInk checks the box of an icon (22 by default, as `svg.i`),
// its name, and the color it is drawn in, on both phones: its Ink, or an
// explicit Color over it.
func TestIconSizesAndInk(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		look, ok := theme.New(ph.Naming).Ext[*theme.Look]()
		require.True(t, ok)
		tests := []struct {
			name string
			ic   *kit.Icon
			size float32
			want color.Color
		}{
			{"default 22, --tx", kit.NewIcon(icon.Search, 0, theme.InkUnknown), 22, look.T.Text},
			{"24 --tx2", kit.NewIcon(icon.Home, 24, theme.InkText2), 24, look.T.Text2},
			{"15 --pr", kit.NewIcon(icon.Check, 15, theme.InkPrimary), 15, look.T.Primary},
			{"stroke 2.2", func() *kit.Icon {
				i := kit.NewIcon(icon.Dl, 20, theme.InkSuccess)
				i.Stroke = 2.2

				return i
			}(), 20, look.T.Success},
			{"color over ink", func() *kit.Icon {
				i := kit.NewIcon(icon.Bolt, 22, theme.InkDanger)
				i.Color = color.White

				return i
			}(), 22, color.White},
		}
		for _, tc := range tests {
			tt := mountAt(t, ph, tc.ic, 0)
			f := tc.ic.Frame()
			assert.InDelta(t, tc.size, f.W, 0, tc.name)
			assert.InDelta(t, tc.size, f.H, 0, tc.name)
			assert.Equal(t, "svg.i", tc.ic.Name)
			assert.True(t, hasPixelNear(tt.Snapshot(), tc.want), "%s: the icon is drawn in its color", tc.name)
		}
	})
}

// TestIconSetGlyph checks that changing the glyph repaints, and the same one
// does not.
func TestIconSetGlyph(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		i := kit.NewIcon(icon.Plus, 22, theme.InkText)
		tt := mountAt(t, ph, i, 0)
		before := slices.Clone(tt.Snapshot().Pix) // the tester reuses its frame
		i.SetGlyph(icon.Plus)
		tt.RunUntilIdle()
		assert.Equal(t, before, tt.Snapshot().Pix)
		i.SetGlyph(icon.Cloud)
		tt.RunUntilIdle()
		assert.NotEqual(t, before, tt.Snapshot().Pix)
	})
}

// TestIconPaintDoesNotAllocate checks that a steady repaint of icons
// allocates nothing.
func TestIconPaintDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		col := ui.Row(8, kit.NewIcon(icon.Home, 22, theme.InkPrimary), kit.NewIcon(icon.Play, 22, theme.InkText), kit.NewIcon(icon.Dl, 20, theme.InkText2))
		tt := mountAt(t, ph, col, 0)
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), ph.Name)
	}
}

// TestIconOriginSnapsToWholeDIPs checks that an icon at a fractional DIP
// position is drawn as the one at the whole DIP the position rounds to, at 3
// pixels to the DIP (Chrome snaps the box of an `<svg>` to whole CSS px), for a
// bare icon and for the icon of an IconButton (the eye of the password field).
func TestIconOriginSnapsToWholeDIPs(t *testing.T) {
	t.Parallel()

	const scale = 3
	tests := []struct {
		name string
		make func() ui.Widget
	}{
		{"icon", func() ui.Widget { return kit.NewIcon(icon.Lock, 22, theme.InkText) }},
		{"icon 16 stroke 2.6", func() ui.Widget {
			i := kit.NewIcon(icon.Check, 16, theme.InkPrimary)
			i.Stroke = 2.6

			return i
		}},
		{"icon button", func() ui.Widget { return kit.NewIconButton(icon.Eye, "eye") }},
	}
	offsets := []struct{ x, y, snapX, snapY float32 }{
		{35, 338.59, 35, 339}, // the mail icon of M02: device y 1017, not 1016
		{7.3, 10.4, 7, 10},
		{7.5, 10.6, 7, 11}, // x is floored (Chrome, 3x shots of M02-M04), y rounds .5 up
		{234.63, 781, 234, 781},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			for _, o := range offsets {
				frac := &ui.Box{Padding: geom.Insets{Left: o.x, Top: o.y}}
				frac.Add(tc.make())
				tf := kittest.MountReal(t, ph, frac)
				tf.SetScale(scale)
				got := slices.Clone(tf.Snapshot().Pix) // the tester reuses its frame

				snapped := &ui.Box{Padding: geom.Insets{Left: o.snapX, Top: o.snapY}}
				snapped.Add(tc.make())
				ts := kittest.MountReal(t, ph, snapped)
				ts.SetScale(scale)
				assert.Equal(t, ts.Snapshot().Pix, got, "%s at %v,%v is drawn at %v,%v", tc.name, o.x, o.y, o.snapX, o.snapY)
			}
		}
	})
}

// TestSnapKeepsIconSize checks that the snap moves the drawing and nothing
// else: the icon's frame is the 22 square at the layout's position.
func TestSnapKeepsIconSize(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		i := kit.NewIcon(icon.Lock, 22, theme.InkText)
		root := &ui.Box{Padding: geom.Insets{Left: 7.3, Top: 10.4}}
		root.Add(i)
		tt := kittest.MountReal(t, ph, root)
		tt.SetScale(3)
		f := i.Frame()
		assert.InDelta(t, 22, f.W, 0)
		assert.InDelta(t, 22, f.H, 0)
		assert.InDelta(t, 7.3, f.X, 1e-5, "the layout is not snapped, only the drawing")
		assert.InDelta(t, 10.4, f.Y, 1e-5)
	})
}
