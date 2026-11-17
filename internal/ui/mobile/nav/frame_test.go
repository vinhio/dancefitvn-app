package nav_test

import (
	"dancefitvn-app/internal/ui/icon"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContentTop checks the rule of Q-P2-1: 54, or the safe area's top when it is
// deeper than 59. The measured tops: iPhone 16 59, the Android emulator 45.333.
func TestContentTop(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		top  float32
		want float64
	}{
		{"none", 0, 54},
		{"Android emulator, measured", 45.333, 54},
		{"iPhone 16, measured", 59, 54},
		{"a deeper notch", 70, 70},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
				r := newRig(t, ph)
				ph.Safe.Top = tc.top
				r.tt.SetSafeArea(ph.Safe)
				r.tt.Elapse(kittest.Frame)
				sc := r.last["home"].Scroll().WindowFrame()
				assert.InDelta(t, tc.want, float64(sc.Y), .01)
				assert.InDelta(t, float64(ph.Size.H)-tc.want, float64(sc.H), .01, "down to the bottom (bottom:0)")
				assert.InDelta(t, float64(ph.Size.W), float64(sc.W), .01)
			})
		})
	}
}

// TestPageFrameMatchesPrototype checks `.scroll` (index.html:74): content 20 in from
// each side, 120 under it, no bars, vertical only.
func TestPageFrameMatchesPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		content := &block{h: 100}
		r := newRigWith(t, ph, func(string) ui.Widget { return content })
		cf := content.WindowFrame()
		assert.InDelta(t, 20, float64(cf.X), .01)
		assert.InDelta(t, 54, float64(cf.Y), .01)
		assert.InDelta(t, float64(ph.Size.W-40), float64(cf.W), .01)
		sc := r.last["home"].Scroll()
		assert.InDelta(t, 220, float64(sc.ContentSize().H), .01, "100 and the 120 bottom padding")
		assert.True(t, sc.HideBars)
		assert.Equal(t, ui.ScrollVertical, sc.Axes)

		// a deeper tab bar (three-button navigation) adds the excess to the padding
		ph.Safe.Bottom = 48
		r.tt.SetSafeArea(ph.Safe)
		r.tt.Elapse(kittest.Frame)
		assert.InDelta(t, 234, float64(sc.ContentSize().H), .01)
		assert.InDelta(t, 98, float64(r.n.TabBar().WindowFrame().H), .01)

		assert.Equal(t, "div.scr", r.last["home"].Name)
		assert.Equal(t, "div.scroll", sc.Name)
	})
}

// TestPageBarPushesTheScroll checks the offline bar sits on the top line and the
// scroll starts at 94 (index.html:132, :362).
func TestPageBarPushesTheScroll(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		page := r.last["home"].Page
		bar := kit.NewOfflineBar("Không có mạng · Đang dùng chế độ ngoại tuyến")
		page.SetBar(bar)
		r.tt.Elapse(kittest.Frame)
		bf := bar.WindowFrame()
		assert.InDelta(t, 54, float64(bf.Y), .01)
		assert.InDelta(t, 40, float64(bf.H), .01)
		assert.InDelta(t, float64(ph.Size.W), float64(bf.W), .01)
		sc := page.Scroll().WindowFrame()
		assert.InDelta(t, 94, float64(sc.Y), .01)
		assert.InDelta(t, float64(ph.Size.H-94), float64(sc.H), .01)

		page.SetBar(nil)
		r.tt.Elapse(kittest.Frame)
		assert.InDelta(t, 54, float64(page.Scroll().WindowFrame().Y), .01)
	})
}

// TestFrameHelpers checks the numbers screens read.
func TestFrameHelpers(t *testing.T) {
	t.Parallel()
	assert.InDelta(t, 54, float64(nav.StatusBar), 0)
	assert.InDelta(t, 20, float64(nav.ScrollSide), 0)
	assert.InDelta(t, 120, float64(nav.ScrollBottom), 0)
}

// TestNoAllocationsInSteadyFrames checks a steady frame allocates nothing, with a
// sheet open and a toast showing, and Back at Home, a swipe that goes nowhere and
// a tap on a button allocate nothing either.
func TestNoAllocationsInSteadyFrames(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := newRig(t, ph)
		r.n.Sheet().Open(&block{h: 100}, kit.SheetOptions{})
		r.n.Toast("Đã tải xong bài 03")
		r.tt.Elapse(400 * time.Millisecond)
		r.tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, r.tt.Repaint), "%s: a steady frame", ph.Name)

		r.n.Sheet().Close()
		r.n.Toasts().Hide()
		r.tt.Elapse(kittest.Frame)
		r.tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, r.tt.Repaint), "%s: a steady frame, no overlay", ph.Name)
		assert.Zero(t, testing.AllocsPerRun(50, func() { r.n.Back() }), "%s: Back at Home", ph.Name)
		assert.Zero(t, testing.AllocsPerRun(50, func() { _ = r.n.CanBack() }), "%s: CanBack", ph.Name)
		swipe := func() { r.tt.Swipe(geom.Pt(10, 400), geom.Pt(210, 400), 64*time.Millisecond) }
		swipe()
		assert.Zero(t, testing.AllocsPerRun(10, swipe), "%s: a swipe with nowhere to go", ph.Name)
		require.Equal(t, "home", r.n.Current().Screen)
	}
}

// BenchmarkNavRepaint is a steady repaint of the Nav at Home with a tab bar.
func BenchmarkNavRepaint(b *testing.B) {
	r := newRig(b, kittest.Phones[0])
	r.tt.Repaint()
	b.ReportAllocs()
	for b.Loop() {
		r.tt.Repaint()
	}
}

// BenchmarkNavGoBack is a Go and a Back between two screens.
func BenchmarkNavGoBack(b *testing.B) {
	r := newRig(b, kittest.Phones[0])
	b.ReportAllocs()
	for b.Loop() {
		r.n.Go(route("detail"), false)
		r.n.Back()
		r.tt.Frame()
	}
}

// TestPageFlushSpansTheGutters checks a flush page: the content is as wide as the
// screen from x 0, the bottom padding is still the page's, a tap in the 20 DIP gutter
// reaches the content, and SetContent swaps it keeping the scroll.
func TestPageFlushSpansTheGutters(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		first := newTapBlock(2000)
		page := nav.NewPageFlush(first, 0)
		tt := kittest.Mount(t, ph, page)
		kittest.PinMotion(tt)
		tt.Elapse(kittest.Frame)

		cf := first.WindowFrame()
		assert.InDelta(t, 0, float64(cf.X), .01)
		assert.InDelta(t, 54, float64(cf.Y), .01)
		assert.InDelta(t, float64(ph.Size.W), float64(cf.W), .01, "as wide as the screen")
		assert.InDelta(t, 2120, float64(page.Scroll().ContentSize().H), .01, "the 120 bottom padding is kept")

		tt.ClickAt(geom.Pt(8, 100)) // in the gutter a padded page keeps
		assert.Equal(t, 1, first.taps, "a tap in the gutter hits the content")

		page.Scroll().ScrollTo(geom.Pt(0, 300), false)
		tt.Elapse(kittest.Frame)
		second := newTapBlock(1500)
		page.SetContent(second)
		tt.Elapse(kittest.Frame)
		assert.InDelta(t, 300, float64(page.Scroll().Offset().Y), .01, "the scroll stays")
		assert.InDelta(t, 1620, float64(page.Scroll().ContentSize().H), .01)
		assert.InDelta(t, 0, float64(second.WindowFrame().X), .01)
		assert.Nil(t, first.Parent(), "the old content is out")

		inner := newTapBlock(100)
		tp := kittest.Mount(t, ph, nav.NewPage(inner, 0))
		tp.Elapse(kittest.Frame)
		assert.InDelta(t, 20, float64(inner.WindowFrame().X), .01, "a padded page still keeps its gutters")
		tp.ClickAt(geom.Pt(8, 100))
		assert.Equal(t, 0, inner.taps, "a tap in its gutter misses the content")
	})
}

// TestPageKeepsTheFocusedFieldAboveTheKeyboard checks P6 section 4 decision 4: while
// the on-screen keyboard shows the content may scroll up by what it covers (the
// bottom padding is the keyboard's height and 16), and the field that has the focus
// is brought above it 250 ms after the keyboard rises; with the keyboard down the
// page is what it was.
func TestPageKeepsTheFocusedFieldAboveTheKeyboard(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		in := kit.NewInput(icon.Mail, "")
		top := &block{h: 900}
		col := ui.Column(0, top, in, &block{h: 20})
		col.Align = ui.CrossStretch
		r := newRigWith(t, ph, func(string) ui.Widget { return col })
		sc := r.last["home"].Scroll()
		base := sc.ContentSize().H
		assert.InDelta(t, 900+54+20+120, float64(base), .01, "the usual padding with the keyboard down")
		bottomOf := func() float32 { return in.Field().WindowFrame().Max().Y }
		require.Greater(t, bottomOf(), ph.Size.H, "the field starts below the screen")

		r.tt.View().Focus(in.Field())
		r.tt.SetKeyboard(geom.Insets{Bottom: 336})
		r.tt.Elapse(kittest.Frame)
		assert.InDelta(t, float64(base)-120+336+float64(nav.KeyboardGap), float64(sc.ContentSize().H), .01,
			"the padding is the keyboard's height and 16")
		r.tt.Elapse(300 * time.Millisecond) // the keyboard has risen: the field is revealed
		assert.LessOrEqual(t, bottomOf(), ph.Size.H-336, "the field is above the keyboard")
		assert.Greater(t, in.Field().WindowFrame().Y, float32(54), "and still on the page")

		r.tt.SetKeyboard(geom.Insets{})
		r.tt.Elapse(kittest.Frame)
		assert.InDelta(t, float64(base), float64(sc.ContentSize().H), .01, "the keyboard goes: the usual padding")
	})
}

// TestPageWithoutFocusDoesNotScrollForTheKeyboard checks that a keyboard that rises
// with no field of the page focused moves nothing but the padding.
func TestPageWithoutFocusDoesNotScrollForTheKeyboard(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph)
		sc := r.last["home"].Scroll()
		r.tt.SetKeyboard(geom.Insets{Bottom: 336})
		r.tt.Elapse(400 * time.Millisecond)
		assert.InDelta(t, 0, float64(sc.Offset().Y), .01)
	})
}
