//go:build tvos

package tvkit_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestPosterTileLookAndBox: a tile is the poster's size; focused it grows to 1.06
// about its centre, the capture's box (hc0 of TV04 is 420 x 236 at rest).
func TestPosterTileLookAndBox(t *testing.T) {
	t.Parallel()
	root := &stage{}
	tile := tvkit.NewPosterTile("hc0", poster.Spec{ID: "set-35", Title: "SET 35", Radius: tvkit.PosterRadius, TitleSize: 44}, 420, 236)
	root.put(tile, 90, 840)
	tt := mount(t, root)
	assert.InDelta(t, 420, tile.Frame().W, 0.01)
	assert.InDelta(t, 236, tile.Frame().H, 0.01)
	assert.Equal(t, float32(1), tile.Scale())
	s, txt := tile.LayoutStyle(tt.View().Context())
	assert.Equal(t, "20px", s.Radius)
	assert.Equal(t, "DANCEFIT\nSET 35", txt)
	assert.Equal(t, "hc0", tile.Name)

	tt.View().Focus(tile)
	tt.Elapse(tvtest.Frame * 20)
	assert.InDelta(t, tvkit.FocusScale, tile.Scale(), 0.001)
	assert.InDelta(t, 1, tile.Blend(), 0.001)
	tf, ok := tile.PaintTransform()
	assert.True(t, ok)
	assert.InDelta(t, tvkit.FocusScale, tf.XX, 0.001)
}

// TestFilterChipWidthsMatchPrototype: the selected chip has no border and is 4
// narrower than the same text not selected; the heights are 56 (cf0..cf3 of TV05).
func TestFilterChipWidthsMatchPrototype(t *testing.T) {
	t.Parallel()
	tests := []struct {
		key      string
		text     string
		selected bool
		scale    float32
	}{
		{"cf0", "Tất cả · 15", true, 1.06},
		{"cf1", "Thường · 12", false, 1},
		{"cf2", "Đặc biệt · 2", false, 1},
		{"cf3", "Giới thiệu · 1", false, 1},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			root := &stage{}
			chip := tvkit.NewFilterChip(tc.key, tc.text)
			chip.SetSelected(tc.selected)
			root.put(chip, 90, 250)
			mount(t, root)
			want := tvtest.ProtoBox(t, "TV05", tc.key)
			assert.InDelta(t, want.W/tc.scale, chip.Frame().W, 2)
			assert.InDelta(t, 56, chip.Frame().H, 0.01)
		})
	}
}

// TestFilterChipInlineFill: the chip's fill and ink are inline styles, so focusing
// it changes neither (F-9); with PrototypeInlineFocus off a focused chip is --pr
// with --on.
func TestFilterChipInlineFill(t *testing.T) {
	root := &stage{} //nolint:paralleltest // PrototypeInlineFocus is global
	chip := tvkit.NewFilterChip("cf1", "Thường · 12")
	root.put(chip, 90, 250)
	tt := mount(t, root)
	ctx := tt.View().Context()
	s, _ := chip.LayoutStyle(ctx)
	assert.Equal(t, "rgb(31, 33, 37)", s.Background, "--sf at rest")
	assert.Equal(t, "rgb(244, 244, 242)", s.Color)

	tt.View().Focus(chip)
	tt.Elapse(tvtest.Frame)
	s, _ = chip.LayoutStyle(ctx)
	assert.Equal(t, "rgb(31, 33, 37)", s.Background, "the inline fill beats the focus")
	assert.Equal(t, "rgb(244, 244, 242)", s.Color)
	assert.Equal(t, "28px", s.Radius)

	tvkit.PrototypeInlineFocus = false
	defer func() { tvkit.PrototypeInlineFocus = true }()
	s, _ = chip.LayoutStyle(ctx)
	assert.Equal(t, "rgb(255, 208, 40)", s.Background)
	assert.Equal(t, "rgb(20, 21, 24)", s.Color)

	chip.SetSelected(true)
	tt.View().Focus(nil)
	tt.Elapse(tvtest.Frame)
	tvkit.PrototypeInlineFocus = true
	s, _ = chip.LayoutStyle(ctx)
	assert.Equal(t, "rgb(255, 208, 40)", s.Background, "the selected chip is --pr")
}

// TestRowBoxesAndLook: a row is 88 tall; its number, texts and triangle sit where
// the capture has them (dl0 of TV06); focused it grows to 1.02, turns --pr with --on
// text and the number turns --red with white text.
func TestRowBoxesAndLook(t *testing.T) {
	t.Parallel()
	root := &stage{}
	row := tvkit.NewRow("dl0", "01", "DanceFIT Warm Up 13", "Warm Up · 05:20 · Đã xem")
	root.putSized(row, 968, 173, 854, 88)
	tt := mount(t, root)
	ctx := tt.View().Context()
	assert.InDelta(t, 88, row.Frame().H, 0.01)
	s, txt := row.LayoutStyle(ctx)
	assert.Equal(t, "rgb(31, 33, 37)", s.Background)
	assert.Equal(t, "22px", s.Radius)
	assert.Equal(t, "01\nDanceFIT Warm Up 13\nWarm Up · 05:20 · Đã xem", txt)
	assert.Equal(t, tvtest.ProtoBox(t, "TV06", "dl0").W, row.Frame().W)

	tt.View().Focus(row)
	tt.Elapse(tvtest.Frame * 20)
	s, _ = row.LayoutStyle(ctx)
	assert.Equal(t, "rgb(255, 208, 40)", s.Background)
	assert.Equal(t, "rgb(20, 21, 24)", s.Color)
	assert.InDelta(t, tvkit.RowFocusScale, row.Scale(), 0.001)
	assert.InDelta(t, 1, row.Blend(), 0.001)
}

// TestRowSetSub: the line under the title changes with the row laid out again.
func TestRowSetSub(t *testing.T) {
	t.Parallel()
	root := &stage{}
	row := tvkit.NewRow("dl0", "01", "A", "x")
	root.putSized(row, 0, 0, 854, 88)
	tt := mount(t, root)
	row.SetSub("y · Đã xem")
	tt.Elapse(tvtest.Frame)
	_, txt := row.LayoutStyle(tt.View().Context())
	require.Equal(t, "01\nA\ny · Đã xem", txt)
}

// TestMedalLongNamesWrapAtWordsAndFit is F5 on TV: codes of production are names whose
// underscores are spaces; the caption wraps between words inside the room it is given
// and a word too wide for it is made smaller, never broken inside; the sub text stays
// inside the hexagon.
func TestMedalLongNamesWrapAtWordsAndFit(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"FOUNDER_CEO", "DANCEFIT_MASTER_TRAINER", "HDV DanceFIT"} {
		m := tvkit.NewMedal(name, color.RGB(0x7a, 0xc9, 0x43), strings.ToUpper(name))
		root := &stage{}
		root.putSized(m, 100, 100, 180, 400)
		tt := tvtest.MountReal(t, root)
		tt.Elapse(tvtest.Frame)
		f := m.Node().WindowFrame()
		assert.LessOrEqual(t, f.W, float32(180)+.5, "the medal keeps to its room: %q", name)
		_, sr, ok := tt.Find(kit.MedalText(strings.ToUpper(name)))
		require.True(t, ok, "the sub text of %q", name)
		assert.LessOrEqual(t, sr.W, float32(kit.MedalSubWidth)*tvkit.MedalSize/100+.5, "sub text in the hexagon: %q", name)
	}
}
