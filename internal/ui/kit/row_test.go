package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rowRect returns the box of a row of the prototype: M15's rows are 325 wide in
// a card of padding 0 14 at x 20.
func rowRect(t testing.TB, state, key string) geom.Rect { return protoBox(t, state, key) }

// TestRowBoxesMatchPrototype lays rows out at the boxes of M15 and M07 and
// compares the row and its parts: the 36 square, the text block (title 19, 39
// with a sub line), the value and the trailing widget.
func TestRowBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("title and sub with a chevron", func(t *testing.T) {
			// M15 `go.info`: [34,436,325,52], ico [34,443.5,36,36], tt [82,442,245,39],
			// b#4 [82,442,245,19], chevron svg.i#3 [339,451.5,20,20].
			want := rowRect(t, "M15", "go.info")
			r := kit.NewRow(icon.User, "Thông tin tài khoản", "Tên, họ, email, số điện thoại")
			r.Name, r.Chevron = "go.info", true
			mountStageReal(t, ph, placeAt(r, want.X, want.Y, want.W))
			rectNear(t, want, frameOf(r), 0, "go.info")
			rectNear(t, protoBox(t, "M15", "span.ico"), frameOf(part(t, r, "span.ico")), .5, "span.ico")
			rectNear(t, protoBox(t, "M15", "span.tt"), frameOf(part(t, r, "span.tt")), .5, "span.tt")
			rectNear(t, protoBox(t, "M15", "b#4"), frameOf(part(t, r, "b")), .5, "title")
			rectNear(t, protoBox(t, "M15", "svg.i#2"), frameOf(part(t, part(t, r, "span.ico"), "svg.i")), .5, "ico svg")
		})
		t.Run("title only, last", func(t *testing.T) {
			// M15 `sheet:password`: [34,540,325,52] last row, tt [82,556.5,245,19].
			want := rowRect(t, "M15", "sheet:password")
			r := kit.NewRow(icon.Lock, "Đổi mật khẩu", "")
			r.Name, r.Chevron, r.Last = "sheet:password", true, true
			mountStageReal(t, ph, placeAt(r, want.X, want.Y, want.W))
			rectNear(t, want, frameOf(r), 0, "sheet:password")
			rectNear(t, protoBox(t, "M15", "span.tt#3"), frameOf(part(t, r, "span.tt")), .5, "span.tt#3")
		})
		t.Run("value and chevron", func(t *testing.T) {
			// M15 `go.connect`: [34,638,325,52], tt [82,654,149.59,19], val [243.59,655,83.41,17].
			want := rowRect(t, "M15", "go.connect")
			r := kit.NewRow(icon.Tv, "Kết nối Apple TV", "")
			r.Name, r.Value, r.Chevron = "go.connect", "Chưa kết nối", true
			mountStageReal(t, ph, placeAt(r, want.X, want.Y, want.W))
			rectNear(t, want, frameOf(r), 0, "go.connect")
			rectNear(t, protoBox(t, "M15", "span.val"), frameOf(part(t, r, "span.val")), 1, "span.val")
		})
		t.Run("switch", func(t *testing.T) {
			// M15 `pref:wifi`: [34,690,325,52], switch [308,700,51,31].
			want := rowRect(t, "M15", "pref:wifi")
			sw := kit.NewSwitch(true)
			r := kit.NewRow(icon.Globe, "Chỉ tải qua Wi‑Fi", "")
			r.Name, r.Trailing = "pref:wifi", sw
			mountStageReal(t, ph, placeAt(r, want.X, want.Y, want.W))
			rectNear(t, want, frameOf(r), 0, "pref:wifi")
			rectNear(t, protoBox(t, "M15", "span.sw.on"), frameOf(sw), .01, "span.sw.on")
		})
		t.Run("switch with a sub line in a sheet", func(t *testing.T) {
			// M07 rows of the player options: 52 high, ico at +7.5, switch at +10.
			want := rowRect(t, "M07", "span.ico")
			sw := kit.NewSwitch(true)
			r := kit.NewRow(icon.Mirror, "Lật gương", "Tập theo như đứng trước gương")
			r.Name, r.Trailing = "mirror", sw
			mountStageReal(t, ph, placeAt(r, 20, want.Y-7.5, 353))
			assert.InDelta(t, 52, frameOf(r).H, 0)
			rectNear(t, protoBox(t, "M07", "span.sw.on"), frameOf(sw), .01, "span.sw.on")
		})
	})
}

// TestRowTallAndBorder checks that a row with a long title grows, and the line
// under every row but the last.
func TestRowTallAndBorder(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		a := kit.NewRow(icon.User, "Một dòng", "")
		last := kit.NewRow(icon.User, "Dòng cuối", "")
		last.Last = true
		long := kit.NewRow(icon.NameUnknown, "Một tiêu đề rất dài, dài đến mức phải xuống dòng nhiều lần trong một hàng hẹp như thế này", "Và cả dòng phụ cũng dài không kém để thử xem hàng có cao lên")
		tt := mountStage(t, ph, placeAt(a, 20, 100, 325), placeAt(last, 20, 200, 325), placeAt(long, 20, 300, 200))
		img := tt.Snapshot()
		pixelNear(t, img, 100, 100+51, look.T.Border, 2, "the line is the last row of pixels")
		pixelNear(t, img, 100, 100+50, look.T.Bg, 2, "and nothing above it")
		pixelNear(t, img, 100, 200+51, look.T.Bg, 2, "the last row has no line")
		assert.Greater(t, frameOf(long).H, float32(60), "a long text makes the row taller")
		assert.InDelta(t, 52, frameOf(a).H, 0)
		assert.InDelta(t, 52, frameOf(last).H, 0)
	})
}

// TestRowPress covers the tap, a tap on the switch, the toggle through the row,
// disabled and the semantics.
func TestRowPress(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("tap anywhere on the row", func(t *testing.T) {
			presses := 0
			r := kit.NewRow(icon.User, "Thông tin tài khoản", "Tên, họ")
			r.Chevron, r.OnPress = true, func() { presses++ }
			tt := mountStage(t, ph, placeAt(r, 20, 100, 325))
			f := frameOf(r)
			tt.ClickAt(geom.Pt(f.X+5, f.Y+5))
			tt.ClickAt(geom.Pt(f.X+f.W-5, f.Y+f.H-5))
			require.NoError(t, tt.Click("Thông tin tài khoản"))
			assert.Equal(t, 3, presses)
		})
		t.Run("the row flips the switch, not the knob", func(t *testing.T) {
			sw := kit.NewSwitch(false)
			r := kit.NewRow(icon.Globe, "Chỉ tải qua Wi‑Fi", "")
			r.Trailing = sw
			r.OnPress = func() { sw.SetOn(!sw.On) }
			tt := mountStage(t, ph, placeAt(r, 20, 100, 325))
			tt.ClickAt(frameOf(sw).Center())
			assert.True(t, sw.On, "a tap on the switch is a tap on the row")
			tt.ClickAt(geom.Pt(frameOf(r).X+100, frameOf(r).Y+26))
			assert.False(t, sw.On)
			var nodes []platform.SemanticNode
			for _, n := range tt.Semantics().Nodes {
				if n.Label == "Chỉ tải qua Wi‑Fi" && n.Role == platform.RoleSwitch {
					nodes = append(nodes, n)
				}
			}
			require.Len(t, nodes, 1, "the row is a switch named by its title")
			assert.Equal(t, platform.ToggledOff, nodes[0].Toggle)
		})
		t.Run("disabled", func(t *testing.T) {
			presses := 0
			r := kit.NewRow(icon.User, "Đổi mật khẩu", "")
			r.OnPress, r.Disabled = func() { presses++ }, true
			tt := mountStage(t, ph, placeAt(r, 20, 100, 325))
			tt.ClickAt(frameOf(r).Center())
			assert.Zero(t, presses)
		})
	})
}

// TestRowPaintDoesNotAllocate checks that painting rows (with a sub line, a
// value, a chevron, a switch moving or not) allocates nothing.
func TestRowPaintDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		a := kit.NewRow(icon.User, "Thông tin tài khoản", "Tên, họ, email, số điện thoại")
		a.Chevron = true
		b := kit.NewRow(icon.Tv, "Kết nối Apple TV", "")
		b.Value, b.Chevron = "Chưa kết nối", true
		c := kit.NewRow(icon.Globe, "Chỉ tải qua Wi‑Fi", "")
		c.Trailing, c.Last = kit.NewSwitch(true), true
		tt := mountStage(t, ph, placeAt(a, 20, 100, 325), placeAt(b, 20, 160, 325), placeAt(c, 20, 220, 325))
		tt.Elapse(kittest.Frame)
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a repaint of the rows", ph.Name)
		presses := 0
		a.OnPress = func() { presses++ }
		assert.Zero(t, testing.AllocsPerRun(50, a.Press), "%s: a press", ph.Name)
	}
}
