//go:build tvos

package tvkit_test

import (
	"fmt"
	"math"
	"testing"

	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lum is the relative luminance of "rgb(r, g, b)" (WCAG 2).
func lum(t testing.TB, css string) float64 {
	t.Helper()
	var r, g, b int
	_, err := fmt.Sscanf(css, "rgb(%d, %d, %d)", &r, &g, &b)
	require.NoError(t, err, css)
	lin := func(v int) float64 {
		c := float64(v) / 255
		if c <= .03928 {
			return c / 12.92
		}

		return math.Pow((c+.055)/1.055, 2.4)
	}

	return .2126*lin(r) + .7152*lin(g) + .0722*lin(b)
}

// contrast is the WCAG contrast ratio of two colors.
func contrast(t testing.TB, a, b string) float64 {
	t.Helper()
	x, y := lum(t, a), lum(t, b)
	if x < y {
		x, y = y, x
	}

	return (x + .05) / (y + .05)
}

// TestFocusedPlayerRowsAreLegible: the next-lesson rows `pn*` and the panel's `oc`
// have inline fills in the prototype, which beat the focus CSS and left their
// --on text on --sf and --sf2, unreadable over the video (owner, 2026-11-18, K26
// accepted for them). Focused or not, their text has the contrast of 4.5 to the
// fill it lies on, and a focused row is --pr.
func TestFocusedPlayerRowsAreLegible(t *testing.T) {
	t.Parallel()
	next := tvkit.NewNextRow("pn0", "04", "Em Yêu Người Ta", "Bachata · 04:10")
	closeOpt := tvkit.NewOpt("oc", tvkit.OptClose, "Đóng")
	closeOpt.OnSelect = func() {}
	root := &stage{}
	root.putSized(next, 1340, 200, 490, 110)
	root.putSized(closeOpt, 1300, 800, 560, 74)
	tt := mount(t, root)
	ctx := tt.View().Context()

	for _, tc := range []struct {
		name  string
		w     ui.Widget
		style func() (string, string)
	}{
		{"pn0", next, func() (string, string) { s, _ := next.LayoutStyle(ctx); return s.Background, s.Color }},
		{"oc", closeOpt, func() (string, string) { s, _ := closeOpt.LayoutStyle(ctx); return s.Background, s.Color }},
	} {
		bg, fg := tc.style()
		assert.GreaterOrEqual(t, contrast(t, fg, bg), 4.5, "%s at rest: %s on %s", tc.name, fg, bg)
		tt.View().Focus(tc.w)
		tt.Elapse(tvtest.Frame)
		bgF, fgF := tc.style()
		assert.Equal(t, "rgb(255, 208, 40)", bgF, "%s focused is --pr", tc.name)
		assert.GreaterOrEqual(t, contrast(t, fgF, bgF), 4.5, "%s focused: %s on %s", tc.name, fgF, bgF)
		assert.NotEqual(t, bg, bgF)
		tt.View().Focus(nil)
		tt.Elapse(tvtest.Frame)
	}
}

// TestFocusedNextRowIsPaintedPrimary: the pixel under a focused row's padding is
// --pr, the one under a row at rest --sf.
func TestFocusedNextRowIsPaintedPrimary(t *testing.T) {
	t.Parallel()
	next := tvkit.NewNextRow("pn0", "04", "Em Yêu Người Ta", "Bachata · 04:10")
	root := &stage{}
	root.putSized(next, 1340, 200, 490, 110)
	tt := mount(t, root)
	at := func() [3]uint8 {
		c := tt.Snapshot().RGBAAt(1340+10, 200+55)

		return [3]uint8{c.R, c.G, c.B}
	}
	assert.Equal(t, [3]uint8{0x1f, 0x21, 0x25}, at())
	tt.View().Focus(next)
	tt.Elapse(tvtest.Frame * 20)
	assert.Equal(t, [3]uint8{255, 208, 40}, at())
}
