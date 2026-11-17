package theme_test

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/bridge/text"
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lookOf returns the Look of a theme made for gotext.
func lookOf(t *testing.T) *theme.Look {
	t.Helper()
	l, ok := theme.New(theme.NamingGoText).Ext[*theme.Look]()
	require.True(t, ok)

	return l
}

// prototype is the prototype's page, the specification of every literal here.
const prototype = "../../../reference/prototype/index.html"

// page returns the prototype's text.
func page(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(prototype)
	require.NoError(t, err)

	return string(b)
}

// declaration returns the value of property in the rule of selector sel.
func declaration(t *testing.T, html, sel, property string) string {
	t.Helper()
	rule := regexp.MustCompile(`(?:^|[}\s,])` + regexp.QuoteMeta(sel) + `\{([^}]*)\}`)
	for _, m := range rule.FindAllStringSubmatch(html, -1) {
		decl := regexp.MustCompile(`(?:^|;)` + regexp.QuoteMeta(property) + `:([^;]+)`).FindStringSubmatch(m[1])
		if decl != nil {
			return strings.TrimSpace(decl[1])
		}
	}
	require.FailNow(t, "no rule", "%s{%s}", sel, property)

	return ""
}

// parseColor reads #rrggbb and rgba(r,g,b,a) as CSS paints them, taking the
// color of a longer value (`solid rgba(...)`) from its last word.
func parseColor(t *testing.T, v string) color.Color {
	t.Helper()
	if i := strings.Index(v, "rgba("); i >= 0 {
		parts := strings.Split(strings.TrimSuffix(v[i+5:], ")"), ",")
		require.Len(t, parts, 4, v)
		var ch [3]uint8
		for k := range ch {
			n, err := strconv.Atoi(strings.TrimSpace(parts[k]))
			require.NoError(t, err)
			ch[k] = uint8(n)
		}
		a, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
		require.NoError(t, err)

		return color.Color{R: ch[0], G: ch[1], B: ch[2], A: uint8(math.Round(a * 255))}
	}
	c, err := color.Hex(v)
	require.NoError(t, err, v)

	return c
}

// TestLiteralColorsMatchPrototypeCSS compares every literal color of
// theme.Colors with the declaration of the selector it is documented against.
func TestLiteralColorsMatchPrototypeCSS(t *testing.T) {
	t.Parallel()

	html := page(t)
	c := lookOf(t).C
	tests := []struct {
		name     string
		got      color.Color
		sel      string
		property string
	}{
		{"TabBar", c.TabBar, ".tabs", "background"},
		{"ToastBg", c.ToastBg, ".toast", "background"},
		{"ToastText", c.ToastText, ".toast", "color"},
		{"ChipRegularBg", c.ChipRegularBg, ".chip.c-th", "background"},
		{"ChipRegularText", c.ChipRegularText, ".chip.c-th", "color"},
		{"ChipSpecialBg", c.ChipSpecialBg, ".chip.c-db", "background"},
		{"ChipIntroBg", c.ChipIntroBg, ".chip.c-gt", "background"},
		{"ChipIntroBg okc", c.ChipIntroBg, ".chip.okc", "background"},
		{"ChipDarkBg", c.ChipDarkBg, ".chip.k", "background"},
		{"PillOn", c.PillOn, ".tabs button.on .pill", "background"},
		{"LessonCur", c.LessonCur, ".les.cur", "background"},
		{"OfflineBg", c.OfflineBg, ".offbar", "background"},
		{"RoundDark", c.RoundDark, ".rb.dk", "background"},
		{"CropBg", c.CropBg, ".crop", "background"},
		{"RingWhite13", c.RingWhite13, ".poster .ring", "border"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, parseColor(t, declaration(t, html, tc.sel, tc.property)), tc.got)
		})
	}

	// The category colors of a course card are written inline in courseCardH:
	// var(--info), var(--ok), then #FF6B6E.
	card := regexp.MustCompile(`function courseCardH[^\n]*`).FindString(html)
	require.NotEmpty(t, card)
	assert.Contains(t, card, "'var(--info)'")
	assert.Contains(t, card, "'var(--ok)'")
	assert.Contains(t, card, "'#FF6B6E'")
	assert.Equal(t, tokens.Dark.Info, c.CatSpecial)
	assert.Equal(t, tokens.Dark.Success, c.CatIntro)
	assert.Equal(t, color.RGB(0xff, 0x6b, 0x6e), c.CatRegular)
}

// TestMotionMatchesPrototypeCSS compares the durations and the scale with the
// rules that set them.
func TestMotionMatchesPrototypeCSS(t *testing.T) {
	t.Parallel()

	html := page(t)
	assert.Equal(t, ".25s ease-out", strings.TrimPrefix(declaration(t, html, ".sheet", "animation"), "su "))
	assert.InDelta(t, .25, theme.SheetIn.Seconds(), 0)
	assert.Equal(t, ".2s", declaration(t, html, ".dim-bg", "animation")[len("fi "):])
	assert.InDelta(t, .2, theme.FadeIn.Seconds(), 0)
	assert.Equal(t, ".2s", declaration(t, html, ".sw", "transition"))
	assert.InDelta(t, .2, theme.SwitchDur.Seconds(), 0)
	assert.Equal(t, "scale(.98)", declaration(t, html, ".btn:active", "transform"))
	assert.InDelta(t, .98, theme.PressScale, 1e-6)
	assert.Contains(t, html, "2200)")
	assert.InDelta(t, 2.2, theme.ToastTime.Seconds(), 0)
}

// TestEaseIsCSSEase compares CSSEase with cubic-bezier(.25, .1, .25, 1) solved
// independently, and ui.EaseOut with `ease-out`.
func TestEaseIsCSSEase(t *testing.T) {
	t.Parallel()

	// bezier solves x(s) = x for s by bisection, and returns y(s).
	bezier := func(x1, y1, x2, y2, x float64) float64 {
		at := func(a, b, s float64) float64 {
			return 3*(1-s)*(1-s)*s*a + 3*(1-s)*s*s*b + s*s*s
		}
		lo, hi := 0.0, 1.0
		for range 60 {
			mid := (lo + hi) / 2
			if at(x1, x2, mid) < x {
				lo = mid
			} else {
				hi = mid
			}
		}

		return at(y1, y2, (lo+hi)/2)
	}
	for _, x := range []float64{0, .1, .25, .5, .75, .9, 1} {
		assert.InDelta(t, bezier(.25, .1, .25, 1, x), theme.CSSEase(float32(x)), 2e-3, "ease at %v", x)
		assert.InDelta(t, bezier(0, 0, .58, 1, x), ui.EaseOut(float32(x)), 2e-3, "ease-out at %v", x)
	}
	// ease is not ease-in-out: it leaves the start faster.
	assert.Greater(t, theme.CSSEase(.25), ui.EaseInOut(.25))
}

// TestNewThemeDefaultsToInter checks that a text that names no face falls on
// Inter 16 / 400, as the page does, by the name each engine finds it under:
// Core Text must never be asked for the empty family, which draws SF (F-14).
func TestNewThemeDefaultsToInter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		naming theme.Naming
		face   text.Face
	}{
		{"core text", theme.NamingCoreText, text.Face{Family: "Inter-Regular", Size: 16, Weight: text.WeightRegular}},
		{"gotext", theme.NamingGoText, text.Face{Family: "Inter", Size: 16, Weight: text.WeightRegular}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th := theme.New(tc.naming)
			assert.Equal(t, tc.face, th.Font)
			look, ok := th.Ext[*theme.Look]()
			require.True(t, ok)
			assert.Equal(t, tc.naming, look.Faces.Naming())
			assert.Equal(t, tokens.Dark, look.T)
			assert.Equal(t, tokens.Dark.Bg, th.Background)
			assert.Equal(t, tokens.Dark.Text, th.Text)
			assert.Equal(t, tokens.Dark.Primary, th.Accent)
		})
	}
	// The unknown naming reads as the engine's default.
	th := theme.New(theme.NamingUnknown)
	assert.NotEmpty(t, th.Font.Family)
}

// TestFromWithoutLookIsTheDefault checks that a view that installed no Look
// reads the app's, without allocating.
func TestFromWithoutLookIsTheDefault(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	tt, err := ui.NewTester(&ui.Box{}, 100, 100, ui.WithTheme(ui.DarkTheme()))
	require.NoError(t, err)
	defer tt.Close()
	c := tt.View().Context()
	l := theme.From(c)
	require.NotNil(t, l)
	assert.Equal(t, tokens.Dark, l.T)
	assert.Zero(t, testing.AllocsPerRun(100, func() { _ = theme.From(c) }))
}

// TestInkIsThePalette checks every ink against its token or literal.
func TestInkIsThePalette(t *testing.T) {
	t.Parallel()

	l := lookOf(t)
	p := tokens.Dark
	tests := []struct {
		ink  theme.Ink
		want color.Color
	}{
		{theme.InkUnknown, p.Text}, {theme.InkText, p.Text}, {theme.InkText2, p.Text2}, {theme.InkText3, p.Text3},
		{theme.InkPrimary, p.Primary}, {theme.InkOnPrimary, p.OnPrimary}, {theme.InkDanger, p.Danger},
		{theme.InkSuccess, p.Success}, {theme.InkInfo, p.Info}, {theme.InkWarning, p.Warning},
		{theme.InkWhite, color.White}, {theme.Ink(200), p.Text},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, tc.ink.Of(l), tc.ink)
	}
}

// TestChromeLineBox checks the line box against Chrome's: the ascent and the
// descent each rounded (half away from zero), the half-leading floored.
func TestChromeLineBox(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	tests := []struct {
		name string
		m    text.Metrics
		lh   float32
		want theme.LineBox
	}{
		// Inter 11 px (ascent .96875, descent .2421875): `.eb`, 14 high.
		{"inter 11 normal", text.Metrics{Ascent: 10.65625, Descent: 2.6640625}, 0, theme.LineBox{Height: 14, Baseline: 11}},
		// Inter 16 px: 15.5 rounds up to 16, 3.875 to 4.
		{"inter 16 normal", text.Metrics{Ascent: 15.5, Descent: 3.875}, 0, theme.LineBox{Height: 20, Baseline: 16}},
		{"line gap is rounded and added", text.Metrics{Ascent: 10.4, Descent: 2.4, LineGap: 1.6}, 0, theme.LineBox{Height: 14, Baseline: 10}},
		// Barlow Condensed 28 px, line-height 1: ascent 28, descent 5.6 -> 28 + 6;
		// the leading is -6, half of it -3.
		{"barlow 28 lh 1", text.Metrics{Ascent: 28, Descent: 5.6}, 28, theme.LineBox{Height: 28, Baseline: 25}},
		// An odd leading: the half over the text floors (5 / 2 -> 2).
		{"odd leading floors", text.Metrics{Ascent: 10, Descent: 3}, 18, theme.LineBox{Height: 18, Baseline: 12}},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, theme.Chrome(tc.m, tc.lh), tc.name)
	}
	assert.Zero(t, testing.AllocsPerRun(100, func() { _ = theme.Chrome(text.Metrics{Ascent: 10, Descent: 3}, 18) }))
}
