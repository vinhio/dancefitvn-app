//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/locales"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gate holds the requests of a route for the whole test: the state of a screen
// whose data is on its way (M03B: `me.home` blocked forever, 03 section 4 of the
// plan P3).
func gate(routeID string) setup {
	return func(t *testing.T, f *shelltest.Fake) { t.Cleanup(f.Gate(routeID)) }
}

// atHomeLoading boots a signed-in app to Home, which shows its loading state: the
// request of GET /me/home is held, so nothing here waits for it (Settle would).
func atHomeLoading(t *testing.T, ph kittest.Phone, real bool) *shelltest.Rig {
	t.Helper()
	r := newRig(t, ph, real, shellOptions(), signed(), gate("me.home"))
	r.Settle()
	r.Tester.Elapse(600 * time.Millisecond)
	r.WaitFor(func() bool { return r.Route() == "home" }, "Home")
	r.Tester.Elapse(kittest.Frame)

	return r
}

// dumpNow records the layout as it stands, without waiting for the work off the UI
// thread (Rig.Dump does, and a held request never ends).
func dumpNow(r *shelltest.Rig, id string) layoutdump.Layout {
	r.Tester.Elapse(kittest.Frame)

	return layoutdump.Dump(r.Tester.View().Context(), r.Tester.View().Root(), id, "mobile", r.Phone.Size, nil)
}

// TestLoadingBoxesMatchPrototype lays the loading state out with the iPhone's real
// text engine and compares it with M03B within 1 DIP (the skeleton, the three
// cards, the sync line and its icon, the tab bar); on the wider phone the column
// stands at the same y, the cards share the 372 between them and the tab bar sits
// at the bottom.
func TestLoadingBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	ios, android := kittest.Phones[0], kittest.Phones[1]
	rI := atHomeLoading(t, ios, true)
	a := dumpNow(rI, "M03B")
	checkAgainstProto(t, "M03B", a, 1, nil)

	rA := atHomeLoading(t, android, true)
	b := dumpNow(rA, "M03B")
	for _, key := range []string{"div.card", "div.card#2", "div.card#3", "div.muted", "svg.i"} {
		ea, _ := a.Find(key)
		eb, ok := b.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, ea.Y, eb.Y, 0.5, key+": y")
	}
	for i, key := range []string{"div.card", "div.card#2", "div.card#3"} {
		e, _ := b.Find(key)
		assert.InDelta(t, (412.0-40-16)/3, e.W, 0.05, key+": a third of the room")
		assert.InDelta(t, 20+float64(i)*((412.0-40-16)/3+8), e.X, 0.05, key)
	}
	tabs, ok := b.Find("div.tabs")
	require.True(t, ok)
	assert.InDelta(t, 892, tabs.Y+tabs.H, 0.01, "the tab bar sits at the bottom")
	assert.InDelta(t, 84, tabs.H, 0.01)
}

// TestLoadingMirrorsTheCourseGrid checks K39 in the skeleton: under the sync line, two
// rows of two placeholder blocks in the grid's columns (20 margins, 12 apart, as wide as
// the tiles), 12 between the rows; the error variant leaves them out.
func TestLoadingMirrorsTheCourseGrid(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHomeLoading(t, ph, false)
		lay := dumpNow(r, "M03B")
		colW := (float64(ph.Size.W) - 40 - 12) / 2
		sync, ok := lay.Find("div.muted")
		require.True(t, ok)
		var prevBottom float64
		for i, key := range []string{"div.card#4", "div.card#5", "div.card#6", "div.card#7"} {
			e, ok := lay.Find(key)
			require.True(t, ok, key)
			assert.InDelta(t, colW, e.W, 0.01, key+": a column wide")
			assert.InDelta(t, 20+float64(i%2)*(colW+12), e.X, 0.01, key+": x")
			assert.InDelta(t, 174, e.H, 0.01, key)
			if i%2 == 0 && i > 0 {
				assert.InDelta(t, prevBottom+12, e.Y, 0.01, key+": 12 under the row above")
			}
			if i == 0 {
				assert.Greater(t, e.Y, sync.Y+sync.H, "under the sync line")
			}
			prevBottom = e.Y + e.H
		}
	})
}

// TestLoadingErrorHidesTheGridPlaceholders checks that the retry variant does not keep
// the placeholders under the button.
func TestLoadingErrorHidesTheGridPlaceholders(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		content := screens.NewLoadingContent(env.New(f, locales.MustTexts()))
		tt := kittest.Mount(t, ph, content)
		lay := layoutdump.Dump(tt.View().Context(), tt.View().Root(), "x", "mobile", ph.Size, nil)
		_, ok := lay.Find("div.card#4")
		assert.True(t, ok, "the placeholders while syncing")
		content.SetError(func() {})
		tt.Elapse(kittest.Frame)
		lay = layoutdump.Dump(tt.View().Context(), tt.View().Root(), "x", "mobile", ph.Size, nil)
		_, ok = lay.Find("div.card#4")
		assert.False(t, ok, "none under the retry button")
	})
}

// TestHomeShowsLoadingUnderTheHomeTab checks F-8: 03B is Home's loading state, a
// route of Home with the tab bar and its Home tab current, and no skeleton text but
// the sync line.
func TestHomeShowsLoadingUnderTheHomeTab(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHomeLoading(t, ph, false)
		bar := r.Shell.Nav().TabBar()
		assert.False(t, bar.Hidden)
		assert.True(t, r.Tester.Shows("Đang đồng bộ khoá học…"))
		assert.True(t, r.Tester.Shows("Trang chủ"))
		h, ok := r.Shell.Nav().Screen().(*screens.Home)
		require.True(t, ok)
		assert.Nil(t, h.Content(), "the skeleton shows until the data is in")
	})
}

// TestLoadingErrorShowsRetry checks the error variant (Q-P3-18): the skeleton stays,
// the sync line gives way to "Thử lại", a tap runs the retry, and no error text
// shows over a canceled screen.
func TestLoadingErrorShowsRetry(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		e := env.New(f, locales.MustTexts())
		content := screens.NewLoadingContent(e)
		tt := kittest.Mount(t, ph, content)
		assert.True(t, tt.Shows("Đang đồng bộ khoá học…"))
		assert.True(t, content.Retry().Hidden)

		retried := 0
		content.SetError(func() { retried++ })
		tt.Elapse(kittest.Frame)
		assert.False(t, tt.Shows("Đang đồng bộ khoá học…"))
		assert.True(t, tt.Shows("Thử lại"))
		assert.Positive(t, content.Retry().Node().WindowFrame().H, "shown: it has a frame")
		tt.ClickAt(content.Retry().Node().WindowFrame().Center())
		assert.Equal(t, 1, retried)

		content.SetError(nil)
		tt.Elapse(kittest.Frame)
		assert.True(t, tt.Shows("Đang đồng bộ khoá học…"))
		assert.True(t, content.Retry().Hidden)

		// the error again: the button shown a second time is laid out afresh and painted
		content.SetError(func() { retried++ })
		tt.Elapse(kittest.Frame)
		assert.True(t, tt.Shows("Thử lại"))
		assert.False(t, tt.Shows("Đang đồng bộ khoá học…"))
		assert.Positive(t, content.Retry().Node().WindowFrame().W)
		assert.Positive(t, content.Retry().Node().WindowFrame().H)
		tt.ClickAt(content.Retry().Node().WindowFrame().Center())
		assert.Equal(t, 2, retried)
	})
}

// TestLoadingDoesNotAllocateWhenSteady checks that a steady frame of the loading
// screen allocates nothing.
func TestLoadingDoesNotAllocateWhenSteady(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atHomeLoading(t, ph, false)
		for range 3 {
			r.Tester.Elapse(kittest.Frame)
		}
		r.Tester.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
	}
}
