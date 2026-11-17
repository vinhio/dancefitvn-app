package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRadioBoxesMatchPrototype compares both radios of M09 with their boxes:
// 61 tall, the dot 20, the muted line under the title, and the 10 DIP gap.
func TestRadioBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		one := protoBox(t, "M09", "dlChoice:one")
		all := protoBox(t, "M09", "dlChoice:all")
		a := kit.NewRadio("Chỉ bài này", "05 · Hãy Hát Lên · 03:30", "3,1 MB")
		a.Name, a.On = "dlChoice:one", true
		b := kit.NewRadio("Cả khoá Set 35", "4 file còn thiếu · đã có 6", "21,4 MB")
		b.Name = "dlChoice:all"
		mountStageReal(t, ph, placeAt(a, one.X, one.Y, one.W), placeAt(b, all.X, all.Y, all.W))
		rectNear(t, one, frameOf(a), 0, "dlChoice:one")
		rectNear(t, all, frameOf(b), 0, "dlChoice:all")
		assert.InDelta(t, kit.RadioGap, all.Y-(one.Y+one.H), 0, "the prototype's margin-bottom is RadioGap")
		rectNear(t, protoBox(t, "M09", "span.dot"), frameOf(part(t, a, "span.dot")), .01, "span.dot")
		rectNear(t, protoBox(t, "M09", "span.dot#2"), frameOf(part(t, b, "span.dot")), .01, "span.dot#2")
		sub := protoBox(t, "M09", "div.muted#3")
		got := frameOf(part(t, a, "div.muted"))
		assert.InDelta(t, sub.X, got.X, .5, "sub x")
		assert.InDelta(t, sub.Y, got.Y, .5, "sub y")
		assert.InDelta(t, sub.H, got.H, .5, "sub h")
	})
}

// TestRadioStates checks the pixels of the chosen and the other radio: border
// 1.6 --pr and a --pr dot with a 10 DIP center, or 1 px --bd and a --tx3 ring.
func TestRadioStates(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		a, b := kit.NewRadio("Chỉ bài này", "x", "3 MB"), kit.NewRadio("Cả khoá", "y", "9 MB")
		a.On = true
		tt := mountStage(t, ph, placeAt(a, 20, 100, 353), placeAt(b, 20, 200, 353))
		img := tt.Snapshot()
		fa, fb := frameOf(a), frameOf(b)
		pixelNear(t, img, 200, int(fa.Y), look.T.Primary, 3, "on border row 0")
		got := pixelAt(img, 200, int(fa.Y)+1)
		assert.Greater(t, int(got.R), int(look.T.Bg.R)+60, "on border: the .6 of row 1")
		pixelNear(t, img, 200, int(fb.Y), look.T.Border, 3, "off border")
		pixelNear(t, img, 200, int(fb.Y)+1, look.T.Bg, 3, "off: row 1 is the fill")
		pixelNear(t, img, 200, int(fb.Y)+30, look.T.Bg, 2, "the radio is on --bg")
		da, db := frameOf(part(t, a, "span.dot")), frameOf(part(t, b, "span.dot"))
		cx, cy := int(da.X+da.W/2), int(da.Y+da.H/2)
		pixelNear(t, img, cx, cy, look.T.Primary, 2, "on: center of the dot")
		pixelNear(t, img, cx+6, cy, look.T.Bg, 2, "on: between the center and the ring")
		pixelNear(t, img, int(da.X)+1, cy, look.T.Primary, 3, "on: ring")
		pixelNear(t, img, int(db.X)+10, int(db.Y)+10, look.T.Bg, 2, "off: no center")
		pixelNear(t, img, int(db.X)+1, int(db.Y)+10, look.T.Text3, 3, "off: ring in --tx3")
		// the center is 10 across: 3 DIPs from the middle is in, 6 is out
		pixelNear(t, img, cx+3, cy, look.T.Primary, 2, "center edge in")
	})
}

// TestRadioPress covers the tap, SetOn and the semantics.
func TestRadioPress(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var chosen string
		a, b := kit.NewRadio("Chỉ bài này", "x", "3 MB"), kit.NewRadio("Cả khoá", "y", "9 MB")
		a.On = true
		a.OnPress = func() { chosen = "one"; a.SetOn(true); b.SetOn(false) }
		b.OnPress = func() { chosen = "all"; b.SetOn(true); a.SetOn(false) }
		tt := mountStage(t, ph, placeAt(a, 20, 100, 353), placeAt(b, 20, 200, 353))
		tt.ClickAt(frameOf(b).Center())
		assert.Equal(t, "all", chosen)
		assert.True(t, b.On)
		assert.False(t, a.On)
		var on []string
		for _, n := range tt.Semantics().Nodes {
			if n.Role == platform.RoleRadio && n.Flags.Has(platform.FlagSelected) {
				on = append(on, n.Label)
			}
		}
		require.Equal(t, []string{"Cả khoá"}, on)
		b.Disabled = true
		chosen = ""
		tt.ClickAt(frameOf(b).Center())
		assert.Empty(t, chosen)
	})
}

// TestRadioIconAndDisabled checks the symbol of a device row (a 22 icon in --tx2 between the dot and
// the text, the text moved right by 34) and the disabled look (.45 opacity, no press); a radio with no
// symbol is the radio of M09 as before.
func TestRadioIconAndDisabled(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		plain := kit.NewRadio("Apple TV", "Hoạt động 13/11/2026", "")
		withIcon := kit.NewRadio("Apple TV", "Hoạt động 13/11/2026", "")
		withIcon.Icon = icon.Tv
		off := kit.NewRadio("iPhone 15", "Hoạt động 14/11/2026", "Máy này")
		off.Icon, off.Disabled = icon.Phone, true
		tt := mountStageReal(t, ph, placeAt(plain, 20, 100, 353), placeAt(withIcon, 20, 200, 353), placeAt(off, 20, 300, 353))
		assert.Nil(t, partOrNil(plain, "svg.i"), "no symbol, no icon box")
		ic := frameOf(part(t, withIcon, "svg.i"))
		fr := frameOf(withIcon)
		assert.InDelta(t, 22, float64(ic.W), 0)
		assert.InDelta(t, 22, float64(ic.H), 0)
		dot := frameOf(part(t, withIcon, "span.dot"))
		assert.InDelta(t, float64(dot.Max().X+12), float64(ic.X), .01, "12 after the dot")
		assert.InDelta(t, float64(fr.Center().Y), float64(ic.Center().Y), .5, "centred in the row")
		assert.InDelta(t, float64(frameOf(part(t, plain, "div.muted")).X+34), float64(frameOf(part(t, withIcon, "div.muted")).X), .01,
			"the text is 22 + 12 further right")
		assert.InDelta(t, float64(frameOf(plain).H), float64(frameOf(withIcon).H), 0, "the row is as tall")

		img := tt.Snapshot()
		fo := frameOf(off)
		// .45 of --bd over --bg on the border row, .45 of the dot's --tx3 ring
		got := pixelAt(img, int(fo.X)+200, int(fo.Y))
		assert.Less(t, int(got.R), int(look.T.Border.R), "the border is dimmed")
		pressed := 0
		off.OnPress = func() { pressed++ }
		tt.ClickAt(fo.Center())
		assert.Zero(t, pressed, "a disabled radio takes no press")
	})
}

// TestRadioPaintDoesNotAllocate checks that painting radios allocates nothing.
func TestRadioPaintDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		a, b := kit.NewRadio("Chỉ bài này", "05 · Hãy Hát Lên", "3,1 MB"), kit.NewRadio("Cả khoá", "4 file", "21 MB")
		a.On = true
		c := kit.NewRadio("iPhone 15", "Hoạt động 14/11/2026", "Máy này")
		c.Icon, c.Disabled = icon.Phone, true
		tt := mountStage(t, ph, placeAt(a, 20, 100, 353), placeAt(b, 20, 200, 353), placeAt(c, 20, 300, 353))
		tt.Elapse(kittest.Frame)
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a repaint", ph.Name)
	}
}
