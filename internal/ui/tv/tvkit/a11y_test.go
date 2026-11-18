//go:build tvos

// Pattern from jplayer-tv/internal/ui/kit/card_test.go and trk_test.go at 781bee5
// (a control's semantic node: role, label, value, state) for the TV widgets of
// this app (P8.3b).

package tvkit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// semOf mounts the widgets at the stage's points with the fake shaper and returns
// what a screen reader reads of them.
func semOf(t testing.TB, ws ...ui.Widget) *platform.Semantics {
	t.Helper()
	root := &stage{}
	for i, w := range ws {
		root.put(w, 100, float32(100+i*150))
	}
	tt := tvtest.Mount(t, root)

	return tt.Semantics()
}

// node returns the element called label.
func node(t testing.TB, sem *platform.Semantics, label string) platform.SemanticNode {
	t.Helper()
	for _, n := range sem.Nodes {
		if n.Label == label {
			return n
		}
	}
	require.Failf(t, "no element", "%q in %v", label, sem.Nodes)

	return platform.SemanticNode{}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestNavItemIsATab: an item of the nav bar is a tab named by its text, selected
// while its screen shows.
func TestNavItemIsATab(t *testing.T) {
	t.Parallel()

	n := tvkit.NewNav(1, "DANCEFIT", [tvkit.NavItems]string{"Trang chủ", "Khoá học", "Nhạc tập", "Cài đặt"})
	sem := semOf(t, n)
	home, on := node(t, sem, "Trang chủ"), node(t, sem, "Khoá học")
	assert.Equal(t, platform.RoleTab, home.Role)
	assert.False(t, home.Flags.Has(platform.FlagSelected))
	assert.Equal(t, platform.RoleTab, on.Role)
	assert.True(t, on.Flags.Has(platform.FlagSelected))
	assert.True(t, on.Actions.Has(platform.ActPress))
}

// TestPosterTileIsNamedByTheCourse: a poster tile is one button named by the
// course, not by the "SET 35" the poster paints; the poster is not read.
func TestPosterTileIsNamedByTheCourse(t *testing.T) {
	t.Parallel()

	tile := tvkit.NewPosterTile("hc0", poster.Spec{ID: "set-35", Title: "SET 35"}, 420, 236)
	tile.SetReading("DanceFIT Set 35", "10 bài học")
	sem := semOf(t, tile)
	require.Len(t, sem.Nodes, 2, "the window and the tile: %v", sem.Nodes)
	n := node(t, sem, "DanceFIT Set 35")
	assert.Equal(t, platform.RoleButton, n.Role)
	assert.Equal(t, "10 bài học", n.Value)
	assert.True(t, n.Actions.Has(platform.ActPress))
}

// TestRowsAreNamedByNumberAndTitle: a lesson row and a next row are buttons named
// by "03 · title" and valued by the line under it.
func TestRowsAreNamedByNumberAndTitle(t *testing.T) {
	t.Parallel()

	row := tvkit.NewRow("dl2", "03", "Stink and Dutty", "Bụng · 03:40")
	next := tvkit.NewNextRow("pn0", "04", "Em Yêu Người Ta", "Bachata · 04:10")
	sem := semOf(t, row, next)
	require.Len(t, sem.Nodes, 3)
	n := node(t, sem, "03 · Stink and Dutty")
	assert.Equal(t, platform.RoleButton, n.Role)
	assert.Equal(t, "Bụng · 03:40", n.Value)
	assert.Equal(t, "Bachata · 04:10", node(t, sem, "04 · Em Yêu Người Ta").Value)
	row.SetSub("Bụng · 03:40 · Đã xem")
	assert.Equal(t, "Bụng · 03:40 · Đã xem", node(t, semOf(t, row), "03 · Stink and Dutty").Value, "a new line is the new value")
}

// TestOptsSayTheirKind: a speed is a radio button valued by its hint and, while it
// has the check, by Now after it; a toggle a switch in its state; the close
// button a button. No speed is FlagSelected: the focused one is the selected one
// of the bridge, and the current one would be a second.
func TestOptsSayTheirKind(t *testing.T) {
	t.Parallel()

	speed := tvkit.NewOpt("o1", tvkit.OptRow, "0.75x")
	speed.SetSub("Chậm")
	speed.Now = "Đang dùng"
	speed.SetCheck(true)
	other := tvkit.NewOpt("o2", tvkit.OptRow, "1x")
	other.SetSub("Bình thường")
	other.Now = "Đang dùng"
	bare := tvkit.NewOpt("o3", tvkit.OptRow, "1.25x")
	bare.Now = "Đang dùng"
	bare.SetCheck(true)
	toggle := tvkit.NewOpt("ot0", tvkit.OptRow, "Lật gương")
	toggle.Switch, toggle.On = true, true
	toggle.SetValue("Bật")
	closeOpt := tvkit.NewOpt("oc", tvkit.OptClose, "Đóng")
	sem := semOf(t, speed, other, bare, toggle, closeOpt)

	n := node(t, sem, "0.75x")
	assert.Equal(t, platform.RoleRadio, n.Role)
	assert.Equal(t, "Chậm · Đang dùng", n.Value, "the hint, then the speed in force")
	assert.Equal(t, platform.ToggledUnknown, n.Toggle, "a speed is no toggle: the bridge would speak 0 or 1 for the hint")
	assert.False(t, n.Flags.Has(platform.FlagSelected), "the current speed is not a second selected")
	o := node(t, sem, "1x")
	assert.Equal(t, "Bình thường", o.Value, "the hint alone while it is not in force")
	assert.Equal(t, "Đang dùng", node(t, sem, "1.25x").Value, "no hint: the speed in force alone")
	sw := node(t, sem, "Lật gương")
	assert.Equal(t, platform.RoleSwitch, sw.Role)
	assert.Equal(t, "Bật", sw.Value)
	assert.Equal(t, platform.ToggledOn, sw.Toggle)
	c := node(t, sem, "Đóng")
	assert.Equal(t, platform.RoleButton, c.Role)
	assert.Empty(t, c.Value)
}

// TestFilterChipIsARadio: a filter is a radio button named by its text and count,
// on while it is the one in force.
func TestFilterChipIsARadio(t *testing.T) {
	t.Parallel()

	on, off := tvkit.NewFilterChip("cf0", "Tất cả · 15"), tvkit.NewFilterChip("cf1", "Thường · 10")
	on.SetSelected(true)
	sem := semOf(t, on, off)
	a, b := node(t, sem, "Tất cả · 15"), node(t, sem, "Thường · 10")
	assert.Equal(t, platform.RoleRadio, a.Role)
	assert.True(t, a.Flags.Has(platform.FlagSelected))
	assert.Equal(t, platform.ToggledUnknown, a.Toggle, "a filter is a selected button, not a toggle")
	assert.False(t, b.Flags.Has(platform.FlagSelected))
	assert.Equal(t, platform.ToggledUnknown, b.Toggle)
}

// TestDiscIsNamedForWhatAPressDoes: the disc has no text: it is "Phát" under the
// PLAY glyph and "Tạm dừng" under the PAUSE one.
func TestDiscIsNamedForWhatAPressDoes(t *testing.T) {
	t.Parallel()

	d := tvkit.NewDisc("mu1", icon.Play)
	d.PlayLabel, d.PauseLabel = "Phát", "Tạm dừng"
	root := &stage{}
	root.put(d, 100, 100)
	tt := tvtest.Mount(t, root)
	assert.Equal(t, "Phát", tt.Semantics().Nodes[1].Label)
	d.SetGlyph(icon.Pause)
	tt.Elapse(tvtest.Frame)
	assert.Equal(t, "Tạm dừng", tt.Semantics().Nodes[1].Label)
}

// TestBarsAreOneElementEach: the beats are one text named by Label (not eight
// numbers), the scrub and the wave progress bars at their value, the medal its
// caption, the QR box and the countdown nothing.
func TestBarsAreOneElementEach(t *testing.T) {
	t.Parallel()

	beats := tvkit.NewBeats()
	beats.Label = "Thanh đếm nhịp"
	scrub := tvkit.NewScrub()
	scrub.Label = "Vị trí phát"
	scrub.SetValue(.25)
	scrub.SetSpoken("01:12 / 03:40")
	wave := tvkit.NewWave()
	wave.Label = "Vị trí phát"
	wave.SetPos(.5)
	medal := tvkit.NewMedal("HDV StepX", color.RGB(0xff, 0x7a, 0x1a), "STEPX")
	sem := semOf(t, beats, scrub, wave, medal, tvkit.NewQR(), tvkit.NewCountdown(5))
	require.Len(t, sem.Nodes, 5, "the window, the beats, the two bars, the medal: %v", sem.Nodes)
	b := node(t, sem, "Thanh đếm nhịp")
	assert.Equal(t, platform.RoleText, b.Role)
	// a role the tvOS bridge gives a trait: RoleProgress has none (P8.3d); the
	// scrub is a control since K43, so an adjustable slider
	assert.Equal(t, platform.RoleSlider, sem.Nodes[2].Role)
	assert.True(t, sem.Nodes[2].Actions.Has(platform.ActIncrement|platform.ActDecrement))
	assert.InDelta(t, .25, sem.Nodes[2].Now, 1e-6)
	assert.Equal(t, "Vị trí phát", sem.Nodes[2].Label)
	assert.Equal(t, "01:12 / 03:40", sem.Nodes[2].Value, "the reading the screen made")
	assert.Equal(t, platform.RoleText, sem.Nodes[3].Role)
	assert.InDelta(t, .5, sem.Nodes[3].Now, .02)
	assert.Equal(t, "50%", sem.Nodes[3].Value, "the percent when the screen made no reading")
	assert.Equal(t, "HDV StepX", node(t, sem, "HDV StepX").Label)
	for _, n := range sem.Nodes {
		assert.NotContains(t, []string{"1", "5", "STEPX", "HDV"}, n.Label)
	}
}

// TestToastIsAStatus: the bubble is one status named by its message, announced as
// it shows, and the text inside it is not read a second time.
func TestToastIsAStatus(t *testing.T) {
	t.Parallel()

	toast, root := toastRig(t)
	tt := tvtest.Mount(t, root)
	tt.Semantics() // a screen reader is listening
	toast.Show(toastText)
	tt.Elapse(tvkit.ToastFade + tvtest.Frame)
	assert.Equal(t, []string{toastText}, tt.Announcements())
	sem := tt.Semantics()
	require.Len(t, sem.Nodes, 2, "the window and the status: %v", sem.Nodes)
	assert.Equal(t, platform.RoleStatus, sem.Nodes[1].Role)
	assert.Equal(t, toastText, sem.Nodes[1].Label)
	assert.Zero(t, sem.Nodes[1].Flags&platform.FlagFocusable)
}
