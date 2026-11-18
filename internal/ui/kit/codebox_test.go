package kit_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
)

// codeBoxesOf returns the six boxes of a row, in order.
func codeBoxesOf(row *kit.CodeBoxes) []ui.Widget {
	var boxes []ui.Widget
	for ch := range row.Children() {
		if strings.HasPrefix(ch.Node().Name, "span.codebox") {
			boxes = append(boxes, ch)
		}
	}

	return boxes
}

// TestCodeBoxesBoxesMatchPrototype compares the six boxes with M16 (empty, the
// first focused) and M16-code ("7K4M2P", the last focused): 46 x 60, 8 apart, the
// dash before the fourth, the row centered.
func TestCodeBoxesBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range []struct {
			state, value string
			focus        int
		}{{"M16", "", 0}, {"M16-code", "7K4M2P", 5}} {
			row := kit.NewCodeBoxes("Mã trên TV")
			row.SetValue(tc.value)
			kittest.MountReal(t, ph, newStage(placeAt(row, 20, 372, ph.Size.W-40)))
			boxes := codeBoxesOf(row)
			assert.Len(t, boxes, 6)
			plain := 0 // the prototype numbers the unfocused boxes from the first
			for i, b := range boxes {
				pk, name := "span.codebox.f", "span.codebox.f"
				if i != tc.focus {
					plain++
					pk, name = "span.codebox", "span.codebox"
					if plain > 1 {
						pk = "span.codebox#" + string(rune('0'+plain))
					}
				}
				assert.Equal(t, name, b.Node().Name)
				rectNear(t, onPhone(ph, protoBox(t, tc.state, pk), anchorCenter), frameOf(b), .01, tc.state+" "+pk)
			}
			assert.Equal(t, tc.focus, row.FocusedBox())
			// the field covers the row, as `input#codeInp` does (opacity 0)
			rectNear(t, onPhone(ph, protoBox(t, tc.state, "input"), anchorStretch), frameOf(part(t, row, "input")), .01, "input")
			// the dash: 12 x 3 between the third and fourth box
			d := dashFrame(t, row)
			assert.InDelta(t, 12, float64(d.W), 0)
			assert.InDelta(t, 3, float64(d.H), 0)
			assert.InDelta(t, float64(frameOf(boxes[2]).Max().X+8), float64(d.X), .01)
			assert.InDelta(t, float64(frameOf(boxes[3]).X-8-12), float64(d.X), .01)
			assert.InDelta(t, float64(frameOf(boxes[0]).Center().Y), float64(d.Center().Y), .01)
		}
	})
}

// TestCodeBoxesFocusBorderAndColors checks the 1.8 --pr border of the focused box
// and the 1 px --bd one of the others.
func TestCodeBoxesFocusBorderAndColors(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		row := kit.NewCodeBoxes("Mã trên TV")
		tt := mountStage(t, ph, placeAt(row, 20, 372, ph.Size.W-40))
		boxes := codeBoxesOf(row)
		img := tt.Snapshot()
		f0, f1 := frameOf(boxes[0]), frameOf(boxes[1])
		pixelNear(t, img, int(f0.X)+23, int(f0.Y)+30, look.T.Surface, 0, "--sf")
		pixelNear(t, img, int(f0.X)+23, int(f0.Y), look.T.Primary, 20, "the focused box's top edge is --pr")
		pixelNear(t, img, int(f1.X)+23, int(f1.Y), look.T.Border, 0, "an unfocused box's is --bd")
		pixelNear(t, img, int(f1.X)+23, int(f1.Y)+1, look.T.Surface, 0, "1 px")
		// dash
		d := dashFrame(t, row)
		pixelNear(t, img, int(d.X)+6, int(d.Y)+1, look.T.Text3, 0, "--tx3")
	})
}

// TestCodeBoxesTyping checks what the hidden field takes: upper case, 0-9 and
// A-Z, six at most, and the focus moving along.
func TestCodeBoxesTyping(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		var got []string
		row := kit.NewCodeBoxes("Mã trên TV")
		row.OnChange = func(s string) { got = append(got, s) }
		tt := mountStage(t, ph, placeAt(row, 20, 372, ph.Size.W-40))
		tt.ClickAt(frameOf(row).Center())
		assert.True(t, row.Field().Node().Focused(), "a tap anywhere on the row focuses the field")
		tt.Type("7k4")
		assert.Equal(t, "7K4", row.Value)
		assert.Equal(t, 3, row.FocusedBox())
		tt.Type("-m 2p9z")
		assert.Equal(t, "7K4M2P", row.Value, "no more than six, nothing but 0-9 and A-Z")
		assert.Equal(t, 5, row.FocusedBox())
		assert.Equal(t, "7K4M2P", got[len(got)-1])
		assert.True(t, tt.Shows("K"))
		assert.NotEmpty(t, got)

		row.SetValue("ab-12")
		tt.Elapse(kittest.Frame)
		assert.Equal(t, "AB12", row.Value)
		assert.Equal(t, 4, row.FocusedBox())
		tt.Type("c")
		assert.Equal(t, "AB12C", row.Value, "typing goes on after the code SetValue gave")
		row.SetValue("ab-12")
		assert.Equal(t, "AB12", row.Value)
		n := len(got)
		row.SetValue("")
		assert.Len(t, got, n, "SetValue does not tell OnChange")
	})
}

// TestCodeBoxesError checks the red state of a refused code: every box has a 1.8 --dg border
// (the focused one too), an edit of the user takes it back, SetError does not tell OnChange.
func TestCodeBoxesError(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		row := kit.NewCodeBoxes("Mã trên TV")
		row.SetValue("ABCDEF")
		told := 0
		row.OnChange = func(string) { told++ }
		tt := mountStage(t, ph, placeAt(row, 20, 372, ph.Size.W-40))
		boxes := codeBoxesOf(row)
		f0, f5 := frameOf(boxes[0]), frameOf(boxes[5])
		row.SetError(true)
		tt.Elapse(kittest.Frame)
		img := tt.Snapshot()
		pixelNear(t, img, int(f0.X)+23, int(f0.Y), look.T.Danger, 20, "an unfocused box's top edge is --dg")
		pixelNear(t, img, int(f5.X)+23, int(f5.Y), look.T.Danger, 20, "so is the focused box's")
		assert.Zero(t, told)
		tt.ClickAt(frameOf(row).Center())
		tt.Key(keys.KeyBackspace, 0)
		assert.False(t, row.Error, "an edit takes the red back")
		assert.Equal(t, "ABCDE", row.Value)
		tt.Elapse(kittest.Frame)
		img = tt.Snapshot()
		pixelNear(t, img, int(f0.X)+23, int(f0.Y), look.T.Border, 0, "--bd again")
		assert.Equal(t, 1, told)
	})
}

// TestFilterCode covers the filter of `oninput` (index.html:570), and the letters
// a Vietnamese keyboard makes of the keys of a code, which the prototype drops:
// a letter with marks is the letter under them, and the U with a horn is W, the
// key that types it alone (B-code3).
func TestFilterCode(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"", ""},
		{"7k4m2p", "7K4M2P"},
		{"7K4-M2P", "7K4M2P"},
		{"1234567", "123456"},
		{" a b c ", "ABC"},
		{"abcdefgh", "ABCDEF"},
		{"zZ9", "ZZ9"},
		{"7K4M2P", "7K4M2P"}, // already a code: returned as it is
		{"7K4M2P9", "7K4M2P"},
		{"7K4m", "7K4M"},
		{"ôê12", "OE12"},
		{"7á4rx", "7A4RX"},
		{"Việt", "VIET"},
		{"Đăuw", "DAUW"},
		{"đỷ", "DY"},
		{"DDAƯUư", "DDAWUW"},
		{"ừứửữự", "WWWWW"},
		{"ÀÁẢÃẠĂẰẮẲẴẶÂẦẤẨẪẬ", "AAAAAA"},
		{"èéẻẽẹêềếểễệìíỉĩị", "EEEEEE"},
		{"òóỏõọôồốổỗộơờớởỡợùúủũụỳýỷỹỵ", "OOOOOO"},
		{"ùúủũụỳýỷỹỵ", "UUUUUY"},
		{"a\u0301e\u0323\u0302", "AE"}, // marks apart from their letters (NFD) are dropped
		{"ßñçüÆ日本", ""},
		{"😀a", "A"},
	} {
		assert.Equal(t, tc.want, kit.FilterCode(tc.in), "%q", tc.in)
	}
	// every letter of the Vietnamese alphabet, in both cases, is its key
	for key, letters := range map[string]string{
		"A": "ÀÁẢÃẠĂẰẮẲẴẶÂẦẤẨẪẬ",
		"D": "Đ",
		"E": "ÈÉẺẼẸÊỀẾỂỄỆ",
		"I": "ÌÍỈĨỊ",
		"O": "ÒÓỎÕỌÔỒỐỔỖỘƠỜỚỞỠỢ",
		"U": "ÙÚỦŨỤ",
		"W": "ƯỪỨỬỮỰ",
		"Y": "ỲÝỶỸỴ",
	} {
		for _, r := range letters + strings.ToLower(letters) {
			assert.Equal(t, key, kit.FilterCode(string(r)), "%c (%U)", r, r)
		}
	}
	// and the letters beside them in Unicode are no key
	for _, r := range "ÄÅÆÇËÎÏÐÑÖ×ØÛÜÞßäåæçëîïðñö÷øûüþÿĀāĒēĪīŌōŪūƮƱẞẟỺỻ" {
		assert.Empty(t, kit.FilterCode(string(r)), "%c (%U)", r, r)
	}
}

// TestCodeBoxesDoesNotAllocate checks a steady frame allocates nothing.
func TestCodeBoxesDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		row := kit.NewCodeBoxes("Mã trên TV")
		row.SetValue("7K4")
		tt := mountStage(t, ph, placeAt(row, 20, 372, ph.Size.W-40))
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a steady frame", ph.Name)
	}
}

// dashFrame returns the frame of the dash of a code row: the 12 x 3 child (it is
// unnamed, the capture does not record it).
func dashFrame(t testing.TB, row ui.Widget) geom.Rect {
	t.Helper()
	for ch := range row.Node().Children() {
		if f := frameOf(ch); f.W == 12 && f.H == 3 {
			return f
		}
	}
	t.Fatal("no dash in the row")

	return geom.Rect{}
}
