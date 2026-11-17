package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
)

// TestChipBoxesMatchPrototype lays the prototype's chips out with the phones'
// real engines and compares each with its layout.json box: 22 tall, the width
// of the text plus 18.
func TestChipBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		state string
		key   string
		chip  func() *kit.Chip
	}{
		// M04 `span.chip.c-th` THƯỜNG (66.83), `span.chip.y` MỚI (39.92),
		// `span.chip.c-db` ĐẶC BIỆT (69.72), `span.chip.c-gt` GIỚI THIỆU (77).
		{"c-th", "M04", "span.chip.c-th", func() *kit.Chip { return kit.NewChip(kit.ChipTh, "THƯỜNG") }},
		{"y", "M04", "span.chip.y", func() *kit.Chip { return kit.NewChip(kit.ChipY, "MỚI") }},
		{"c-db", "M04", "span.chip.c-db", func() *kit.Chip { return kit.NewChip(kit.ChipDb, "ĐẶC BIỆT") }},
		{"c-gt", "M04", "span.chip.c-gt", func() *kit.Chip { return kit.NewChip(kit.ChipGt, "GIỚI THIỆU") }},
		// M03 `span.chip.y` TIẾP TỤC XEM: 800 with 1 px spacing (108.38).
		{"y 800/1px", "M03", "span.chip.y", func() *kit.Chip {
			c := kit.NewChip(kit.ChipY, "TIẾP TỤC XEM")
			c.Weight, c.Spacing = 800, 1

			return c
		}},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				want := protoBox(t, tc.state, tc.key)
				c := tc.chip()
				mountStageReal(t, ph, placeAt(c, want.X, want.Y, 0))
				rectNear(t, want, frameOf(c), .5, tc.key)
				assert.Equal(t, tc.key, c.Name)
			})
		}
	})
}

// TestChipOverridesAndColors checks the inline styles the prototype gives some
// chips and the colors of every kind.
func TestChipOverridesAndColors(t *testing.T) {
	t.Parallel()
	look := palette()
	bg := look.T.Bg
	tests := []struct {
		name string
		chip func() *kit.Chip
		h    float32
		fill color.Color // the pixel 4 DIPs in from the left, in the middle
	}{
		{"c-th", func() *kit.Chip { return kit.NewChip(kit.ChipTh, "THƯỜNG") }, 22, over(look.C.ChipRegularBg, bg)},
		{"c-db", func() *kit.Chip { return kit.NewChip(kit.ChipDb, "ĐẶC BIỆT") }, 22, over(look.C.ChipSpecialBg, bg)},
		{"c-gt", func() *kit.Chip { return kit.NewChip(kit.ChipGt, "GIỚI THIỆU") }, 22, over(look.C.ChipIntroBg, bg)},
		{"y", func() *kit.Chip { return kit.NewChip(kit.ChipY, "MỚI") }, 22, look.T.Primary},
		{"k", func() *kit.Chip { return kit.NewChip(kit.ChipK, "Video dọc 9:16") }, 22, over(look.C.ChipDarkBg, bg)},
		{"okc", func() *kit.Chip { return kit.NewChip(kit.ChipOkc, "Đã tải") }, 22, over(look.C.ChipIntroBg, bg)},
		// :398 "Tải hết": 36 high, 18 rounded, --sf2, 13 px, with the dl icon of 18.
		{"tai het", func() *kit.Chip {
			c := kit.NewChip(kit.ChipKindUnknown, "Tải hết")
			c.Icon, c.IconSize, c.Height, c.Radius, c.Size, c.Bg = icon.Dl, 18, 36, 18, 13, look.T.Surface2

			return c
		}, 36, look.T.Surface2},
		// :489 coach subject: 28 high, padding 14, #2F6FE0, 800, .8 px.
		{"coach subject", func() *kit.Chip {
			c := kit.NewChip(kit.ChipKindUnknown, "KHIÊU VŨ")
			c.Height, c.PadX, c.Weight, c.Spacing = 28, 14, 800, .8
			c.Bg, c.Fg = color.RGB(0x2f, 0x6f, 0xe0), color.White

			return c
		}, 28, color.RGB(0x2f, 0x6f, 0xe0)},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				c := tc.chip()
				tt := mountStage(t, ph, placeAt(c, 40, 100, 0))
				assert.InDelta(t, tc.h, frameOf(c).H, 0)
				// the radius is half the height unless set: the corner is bg
				r := frameOf(c)
				img := tt.Snapshot()
				pixelNear(t, img, int(r.X)+5, int(r.Y+r.H/2), tc.fill, 2, tc.name)
				pixelNear(t, img, int(r.X), int(r.Y), look.T.Bg, 2, "square corner is cut")
			})
		}
	})
}

// TestChipPaddingAndIcon checks that the padding is 9 and the icon sits 5 before
// the text.
func TestChipPaddingAndIcon(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		plain := kit.NewChip(kit.ChipTh, "A")
		withIcon := kit.NewChip(kit.ChipOkc, "A")
		withIcon.Icon, withIcon.IconSize = icon.Dl, 13
		tt := mountStage(t, ph, placeAt(plain, 0, 0, 0), placeAt(withIcon, 0, 40, 0))
		tt.Elapse(kittest.Frame)
		assert.InDelta(t, frameOf(plain).W+13+5, frameOf(withIcon).W, .01, "an icon adds its size and a gap of 5")
		n := 0
		for _, s := range tt.Semantics().Nodes {
			if s.Role == platform.RoleText && s.Label == "A" {
				n++
			}
		}
		assert.Positive(t, n, "a chip is text to a screen reader")
	})
}

// TestChipPaintDoesNotAllocate checks that painting chips in a steady frame
// allocates nothing.
func TestChipPaintDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		a := kit.NewChip(kit.ChipTh, "THƯỜNG · 10 BÀI")
		b := kit.NewChip(kit.ChipOkc, "Đã tải · 8,4 MB")
		b.Icon, b.IconSize, b.IconStroke = icon.Dl, 13, 2.4
		c := kit.NewChip(kit.ChipY, "TIẾP TỤC XEM")
		c.Weight, c.Spacing = 800, 1
		tt := mountStage(t, ph, placeAt(a, 10, 10, 0), placeAt(b, 10, 50, 0), placeAt(c, 10, 90, 0))
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a repaint of the chips", ph.Name)
		assert.Zero(t, testing.AllocsPerRun(50, func() { a.SetText("THƯỜNG · 10 BÀI") }), "%s: the same text", ph.Name)
	}
}
