package kit_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// solidImage returns a w x h image of one opaque color.
func solidImage(t testing.TB, w, h int, c color.Color) *scene.Image {
	t.Helper()
	img, err := scene.NewImage(w, h)
	require.NoError(t, err)
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
	}

	return img
}

// TestBadgeBoxesMatchPrototype compares the three badges and their captions with
// M15: 111 x 114 at 20, 141 and 262, the caption centered 8 under the medal.
func TestBadgeBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		specs := []struct {
			label, sub, key, bkey string
			accent                color.Color
		}{
			{"HDV DanceFIT", "DANCEFIT", "div.badge", "b", color.RGB(0x7a, 0xc9, 0x43)},
			{"HDV StepX", "STEPX", "div.badge#2", "b#2", color.RGB(0xff, 0x7a, 0x1a)},
			{"HDV BodyX", "BODYX", "div.badge#3", "b#3", color.RGB(0xe5, 0x38, 0x3b)},
		}
		var items []stageItem
		var badges []*kit.Badge
		for _, s := range specs {
			b := kit.NewBadge(s.label, s.accent, s.sub)
			badges = append(badges, b)
			want := protoBox(t, "M15", s.key)
			items = append(items, placeAt(b, want.X, want.Y, want.W))
		}
		mountStageReal(t, ph, items...)
		for i, s := range specs {
			rectNear(t, protoBox(t, "M15", s.key), frameOf(badges[i]), 1, s.key)
			cap := frameOf(part(t, badges[i], "b"))
			want := protoBox(t, "M15", s.bkey)
			rectNear(t, want, cap, 1.5, "caption "+s.bkey)
			med := frameOf(badges[i]).Center().X
			assert.InDelta(t, float64(med), float64(cap.Center().X), .6, "caption centered")
			assert.Equal(t, "div.badge", badges[i].Name)
		}
	})
}

// TestBadgeDrawsTheMedalOrThePicture checks the fallback medal's colors, and that
// the API picture replaces it.
func TestBadgeDrawsTheMedalOrThePicture(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		accent := color.RGB(0x7a, 0xc9, 0x43)
		b := kit.NewBadge("HDV DanceFIT", accent, "DANCEFIT")
		tt := mountStageReal(t, ph, placeAt(b, 20, 276, 111))
		f := frameOf(b)
		pixelNear(t, tt.Snapshot(), int(f.X)+3, int(f.Y)+30, look.T.Surface, 0, "the card is --sf")
		pixelNear(t, tt.Snapshot(), int(f.X), int(f.Y), look.T.Bg, 8, "rounded 16")
		mx, my := f.X+(111-72)/2, f.Y+10
		k := float32(.72)
		pixelNear(t, tt.Snapshot(), int(mx+42*k), int(my+10*k), accent, 3, "ribbon stripe in the accent")
		pixelNear(t, tt.Snapshot(), int(mx+38*k), int(my+10*k), color.RGB(0x2a, 0x2d, 0x33), 3, "ribbon")
		pixelNear(t, tt.Snapshot(), int(mx+50*k), int(my+50*k), color.RGB(0xee, 0xf0, 0xf3), 3, "inner hexagon")
		pixelNear(t, tt.Snapshot(), int(mx+50*k), int(my+(62-36+1)*k), accent, 12, "rim in the accent")

		b.SetImage(solidImage(t, 8, 8, color.RGB(0, 0, 200)))
		tt.Elapse(kittest.Frame)
		pixelNear(t, tt.Snapshot(), int(mx+36), int(my+36), color.RGB(0, 0, 200), 1, "the API picture")
		pixelNear(t, tt.Snapshot(), int(mx+42*k), int(my+10*k), color.RGB(0, 0, 200), 1, "replaces the drawing")
		assert.False(t, tt.Shows("HDV"), "and its texts")
		b.SetImage(nil)
		tt.Elapse(kittest.Frame)
		pixelNear(t, tt.Snapshot(), int(mx+42*k), int(my+10*k), accent, 3, "and back")
		assert.True(t, tt.Shows("HDV"), "the fallback's texts are back with it (nil -> picture -> nil)")
	})
}

// TestBadgeDoesNotAllocate checks a steady frame allocates nothing.
func TestBadgeDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		tt := mountStage(t, ph, placeAt(kit.NewBadge("HDV DanceFIT", color.RGB(0x7a, 0xc9, 0x43), "DANCEFIT"), 20, 276, 111),
			placeAt(kit.NewAvatarButton(76, color.RGB(0x14, 0x15, 0x18), "Ảnh"), 36, 134, 0))
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a steady frame", ph.Name)
	}
}

// TestAvatarBoxesMatchPrototype compares the avatar buttons with M03 (44 x 48,
// at the right) and M15 (76 x 80 with the camera badge).
func TestAvatarBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		small := kit.NewAvatarButton(44, look.T.Primary, "Tài khoản")
		small.Name = "go.account"
		big := kit.NewAvatarButton(76, look.T.OnPrimary, "Đổi ảnh")
		big.Name, big.Camera = "sheet:avatar", true
		m3 := onPhone(ph, protoBox(t, "M03", "go.account"), anchorRight)
		m15 := protoBox(t, "M15", "sheet:avatar")
		mountStageReal(t, ph, placeAt(small, m3.X, m3.Y, 0), placeAt(big, m15.X, m15.Y, 0))
		rectNear(t, m3, frameOf(small), 0, "go.account")
		rectNear(t, m15, frameOf(big), 0, "sheet:avatar")
		rectNear(t, protoBox(t, "M15", "svg.i"), frameOf(part(t, big, "svg.i")), .01, "camera icon")
		assert.Equal(t, 4*float32(1), frameOf(small).H-44, "the strut under the inline svg")
	})
}

// TestAvatarDrawsThePersonThePictureAndTheRing checks the default person's
// colors, the ring outside the circle, the camera badge, and a picture with its
// zoom.
func TestAvatarDrawsThePersonThePictureAndTheRing(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		ring := look.T.Primary
		b := kit.NewAvatarButton(76, ring, "Đổi ảnh")
		b.Camera = true
		tt := mountStageReal(t, ph, placeAt(b, 36, 134, 0))
		img := tt.Snapshot()
		k := float32(.76)
		at := func(x, y float32) (int, int) { return int(36 + x*k), int(134 + y*k) }
		x, y := at(20, 30)
		pixelNear(t, img, x, y, color.RGB(0xdc, 0xd8, 0xd2), 2, "background of the person")
		x, y = at(50, 68)
		pixelNear(t, img, x, y, color.RGB(0xd3, 0x20, 0x2a), 2, "shirt")
		x, y = at(50, 45)
		pixelNear(t, img, x, y, color.RGB(0xe8, 0xb8, 0x94), 2, "face")
		x, y = at(50, 29)
		pixelNear(t, img, x, y, color.RGB(0x2a, 0x1f, 0x1a), 4, "hair")
		pixelNear(t, img, 36-1, 134+38, ring, 4, "the ring: 3 outside the circle")
		pixelNear(t, img, 36-5, 134+38, look.T.Bg, 0, "and no more")
		pixelNear(t, img, 36+1, 134+38, color.RGB(0xdc, 0xd8, 0xd2), 4, "the circle itself")
		pixelNear(t, img, 36+8, 134+38, color.RGB(0xdc, 0xd8, 0xd2), 4, "inside")
		pixelNear(t, img, 92, 202, color.White, 2, "the camera badge is white")
		pixelNear(t, img, 102, 189, look.T.OnPrimary, 2, "with a 2 px #141518 border")

		b.Avatar.SetImage(solidImage(t, 16, 16, color.RGB(0, 150, 0)))
		tt.Elapse(kittest.Frame)
		pixelNear(t, tt.Snapshot(), 36+38, 134+38, color.RGB(0, 150, 0), 1, "the picture")
		pixelNear(t, tt.Snapshot(), 36+1, 134+1, look.T.Bg, 8, "clipped to the circle")
		b.Avatar.Zoom, b.Avatar.X = 2, 300 // twice the size, moved by 1.5 sizes right: off the circle
		b.Avatar.Repaint()
		tt.Elapse(kittest.Frame)
		pixelNear(t, tt.Snapshot(), 36+6, 134+38, color.RGB(0xcc, 0xcc, 0xcc), 2, "#ccc where the moved picture leaves the circle bare")
	})
}

// TestAvatarButtonPress checks the button takes the press.
func TestAvatarButtonPress(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		taps := 0
		b := kit.NewAvatarButton(44, color.RGB(255, 208, 40), "Tài khoản")
		b.OnPress = func() { taps++ }
		tt := mountStage(t, ph, placeAt(b, 100, 100, 0))
		require.NoError(t, tt.Click("Tài khoản"))
		assert.Equal(t, 1, taps)
		b.Disabled = true
		tt.ClickAt(frameOf(b).Center())
		assert.Equal(t, 1, taps)
	})
}

// TestBadgeLongNamesWrapAtWordsAndFit is F5 on both phones: codes of production
// ("FOUNDER_CEO", "DANCEFIT_MASTER_TRAINER") are names whose underscores are spaces; the
// caption wraps between words and a word too wide for its room is made smaller, never
// broken inside ("FOUNDE" over "R"); the sub text of the medal stays inside its hexagon.
func TestBadgeLongNamesWrapAtWordsAndFit(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, width := range []float32{111, 63} { // three badges in a row, and five
			var items []stageItem
			var badges []*kit.Badge
			for i, name := range []string{"FOUNDER_CEO", "DANCEFIT_MASTER_TRAINER", "HDV DanceFIT"} {
				b := kit.NewBadge(name, color.RGB(0x7a, 0xc9, 0x43), strings.ToUpper(name))
				badges = append(badges, b)
				items = append(items, placeAt(b, 10+float32(i)*130, 20, width))
			}
			tt := mountStageReal(t, ph, items...)
			for i, b := range badges {
				cap, ok := part(t, b, "b").(*kit.Label)
				require.True(t, ok)
				assert.False(t, cap.BreaksWord(), "no word is broken inside: %q at %v", b.Label, width)
				bf, cf := frameOf(b), frameOf(cap)
				assert.GreaterOrEqual(t, cf.X, bf.X-.01, "caption inside the badge")
				assert.LessOrEqual(t, cf.X+cf.W, bf.X+bf.W+.01, "caption inside the badge")
				// the sub text of the fallback medal is one line within the hexagon
				sub := kit.MedalText(strings.ToUpper(b.Label))
				_, sr, found := tt.Find(sub)
				require.True(t, found, "the sub text of %q", b.Label)
				assert.LessOrEqual(t, sr.W, float32(kit.MedalSubWidth)*72/100+.5, "sub text in the hexagon: %q", b.Label)
				assert.Equal(t, kit.MedalText(b.Label), caption(t, b), "the reading is the displayed name")
				_ = i
			}
		}
	})
}

// caption returns the label a screen reader reads for the badge.
func caption(t testing.TB, b *kit.Badge) string {
	t.Helper()
	var n platform.SemanticNode
	b.Describe(nil, &n)

	return n.Label
}
