//go:build tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/screens"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWelcomeBoxesMatchPrototype compares every element of TV01.layout.json
// with the screen's: the posters and their titles, the brand, the paragraph,
// `w1` (focused, so at its scaled box), its icon, `w2` and the hint.
func TestWelcomeBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	r := open(t, tvtest.NewServices(), env.RouteWelcome, true)
	require.Equal(t, "w1", r.key())
	r.tt.Elapse(tvkit.Transition)
	lay := r.layout()
	for _, key := range tvtest.ProtoKeys(t, "TV01") {
		e, ok := lay.Find(key)
		require.True(t, ok, "TV01 has no element %q", key)
		want := tvtest.ProtoBox(t, "TV01", key)
		if key == "div.d.acc" || key == "p.muted" {
			// K30: the second line is the user's notice, shorter than the prototype's
			// ("Dành cho huấn luyện viên đã có tài khoản dancefitvn.com"), so the block
			// is as wide as the notice now; x, y and h are the prototype's.
			want.W = welcomeBlockW
		}
		near(t, want, geom.XYWH(float32(e.X), float32(e.Y), float32(e.W), float32(e.H)), boxTol, "TV01 "+key)
	}
}

// welcomeBlockW is the width of the text block (brand and paragraph, `display:inline-block`
// as wide as the widest line) with the notice of K30 in the second line: the
// notice "Ứng dụng hỗ trợ cho các thành viên của DanceFITVN" at 30 px is the widest
// line, 755.26 (the prototype's 821.39 was its longer "Dành cho huấn luyện viên đã
// có tài khoản dancefitvn.com"). It ends at x 885, left of the posters at 1040.
const welcomeBlockW float32 = 755.26

// TestWelcomeNoticeFitsOnOneLine checks K30 on the TV stage: the notice is the
// second line of the paragraph at the prototype's style (30 px, line 1.5), one line
// that is not cut, inside the 1920 stage and left of the posters.
func TestWelcomeNoticeFitsOnOneLine(t *testing.T) {
	t.Parallel()
	r := open(t, tvtest.NewServices(), env.RouteWelcome, true)
	lay := r.layout()
	e, ok := lay.Find("p.muted")
	require.True(t, ok)
	assert.InDelta(t, 90, e.H, 1, "two lines of 45 (30 px x 1.5): the notice does not wrap")
	assert.InDelta(t, welcomeBlockW, e.W, boxTol)
	assert.Less(t, e.X+e.W, 1040.0, "left of the posters, inside 1920")
	assert.Contains(t, e.Text, "Ứng dụng hỗ trợ cho các thành viên của DanceFITVN")
	assert.NotContains(t, e.Text, "…", "not truncated")
	assert.NotContains(t, e.Text, "tài khoản")
}

// TestWelcomeText: the strings are the prototype's (the notice of K30 in place of
// the audience line), the paragraph is two lines.
func TestWelcomeText(t *testing.T) {
	t.Parallel()
	r := open(t, tvtest.NewServices(), env.RouteWelcome, true)
	lay := r.layout()
	want := map[string]string{
		"div.d.acc":          "DANCEFIT",
		"tv.welcome.country": "VIỆT NAM",
		"p.muted":            "Dễ. Vui. Khoẻ. — Xem bài học trên màn hình lớn\nỨng dụng hỗ trợ cho các thành viên của DanceFITVN",
		"w1":                 "Đăng nhập",
		"w2":                 "Tìm hiểu thêm",
		"div.muted":          "Vuốt trên remote để di chuyển · Nhấn để chọn",
	}
	for key, txt := range want {
		e, ok := lay.Find(key)
		require.True(t, ok, key)
		assert.Equal(t, txt, e.Text, key)
	}
	for _, key := range []string{"div.d.acc", "p.muted"} {
		e, _ := lay.Find(key)
		assert.InDelta(t, welcomeBlockW, e.W, boxTol, key+": a block as wide as the widest line")
	}
}

// TestWelcomeDefaultFocusAndMoves: `w1` has the focus, Right goes to `w2`, Left
// back, and the other arrows stay (nothing there).
func TestWelcomeDefaultFocusAndMoves(t *testing.T) {
	t.Parallel()
	r := open(t, tvtest.NewServices(), env.RouteWelcome, true)
	assert.Equal(t, "w1", r.key())
	assert.True(t, r.press(keys.KeyRight))
	assert.Equal(t, "w2", r.key())
	r.press(keys.KeyDown)
	assert.Equal(t, "w2", r.key())
	r.press(keys.KeyLeft)
	assert.Equal(t, "w1", r.key())
	assert.Empty(t, r.tt.Leaks())
}

// TestWelcomeMoreToast: Select on `w2` is a toast "Xem dancefitvn.com để biết
// thêm" for 2400 ms; it does not leave the screen.
func TestWelcomeMoreToast(t *testing.T) {
	t.Parallel()
	r := open(t, tvtest.NewServices(), env.RouteWelcome, false).armed()
	r.press(keys.KeyRight)
	r.tt.Key(keys.KeyEnter, 0)
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, "Xem dancefitvn.com để biết thêm", r.sh.ToastHost().Message())
	assert.Equal(t, env.RouteWelcome, r.sh.Stack().Screen())
	r.tt.Elapse(tvkit.ToastTime)
	assert.False(t, r.sh.ToastHost().Visible())
}

// TestWelcomeLoginAndMenu: Select on `w1` opens TV02 over TV01; Menu goes back
// to TV01 (focus `w1` again); Menu at TV01 is unused, left to the system.
func TestWelcomeLoginAndMenu(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewServices()
	r := open(t, svc, env.RouteWelcome, false).armed()
	assert.True(t, r.tt.Key(keys.KeyEnter, 0))
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, env.RouteCode, r.sh.Stack().Screen())
	_, ok := r.sh.Screen().(*screens.Code)
	assert.True(t, ok)
	assert.Equal(t, "c2", r.key())
	assert.Equal(t, 1, r.sh.Stack().Depth())

	assert.True(t, r.press(keys.KeyEscape), "Menu on TV02 is Back")
	assert.Equal(t, env.RouteWelcome, r.sh.Stack().Screen())
	assert.Equal(t, "w1", r.key())
	assert.False(t, r.press(keys.KeyEscape), "Menu on TV01 is left to the system")
	assert.Equal(t, env.RouteWelcome, r.sh.Stack().Screen())
	r.tt.Elapse(time.Second)
	assert.Empty(t, r.tt.Leaks())
}

// TestWelcomeSteadyFrameDoesNotAllocate: a repaint of TV01 with the focus
// settled allocates nothing.
func TestWelcomeSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r := open(t, tvtest.NewServices(), env.RouteWelcome, false).armed()
	r.tt.Elapse(time.Second)
	r.tt.Repaint()
	assert.Zero(t, testing.AllocsPerRun(50, r.tt.Repaint))
}
